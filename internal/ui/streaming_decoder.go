package ui

const maxStreamingPathScanBytes = 8192

type streamingDecoder struct {
	pathKeys    []string
	contentKeys []string
	tailCap     int

	rawLen   int
	head     []byte
	pathScan []byte
	path     string
	pathDone bool

	inContent bool
	closed    bool

	tail       []string
	cur        []byte
	lineCount  int
	pendingEsc bool
	uSkip      int
}

func newStreamingDecoder() *streamingDecoder {
	return &streamingDecoder{
		pathKeys:    []string{"path", "file", "file_path", "filepath", "filename"},
		contentKeys: []string{"content", "new_string", "new_str", "new_text", "new", "replacement", "patch", "input"},
		tailCap:     streamingTailLines,
	}
}

func (d *streamingDecoder) feed(fragment string) {
	d.rawLen += len(fragment)
	if d.closed {
		d.scanPathFragment(fragment)
		return
	}
	if d.inContent {
		if remainder := d.consume(fragment); remainder != "" {
			d.scanPathFragment(remainder)
		}
		return
	}
	d.head = append(d.head, fragment...)
	if !d.pathDone {
		d.setPathFrom(string(d.head))
	}
	start := d.contentValueStart()
	if start < 0 {
		return
	}
	d.inContent = true
	rest := string(d.head[start:])
	d.head = nil
	if remainder := d.consume(rest); remainder != "" {
		d.scanPathFragment(remainder)
	}
}

func (d *streamingDecoder) setPathFrom(args string) {
	if d.pathDone {
		return
	}
	if v := streamingFilePath(args); v != "" {
		d.path = v
		d.pathDone = true
	}
}

func (d *streamingDecoder) scanPathFragment(fragment string) {
	if d.pathDone || fragment == "" {
		return
	}
	d.pathScan = append(d.pathScan, fragment...)
	if len(d.pathScan) > maxStreamingPathScanBytes {
		d.pathScan = d.pathScan[len(d.pathScan)-maxStreamingPathScanBytes:]
	}
	d.setPathFrom(string(d.pathScan))
}

func (d *streamingDecoder) contentValueStart() int {
	s := string(d.head)
	best := -1
	for _, key := range d.contentKeys {
		if idx := jsonStringValueStart(s, key); idx >= 0 && (best < 0 || idx < best) {
			best = idx
		}
	}
	return best
}

func (d *streamingDecoder) consume(b string) string {
	for i := 0; i < len(b); i++ {
		c := b[i]
		if d.uSkip > 0 {
			d.uSkip--
			continue
		}
		if d.pendingEsc {
			d.pendingEsc = false
			switch c {
			case 'n':
				d.newline()
			case 't':
				d.cur = append(d.cur, '\t')
			case 'r':
			case 'u':
				d.uSkip = 4
			default:
				d.cur = append(d.cur, c)
			}
			continue
		}
		switch c {
		case '\\':
			d.pendingEsc = true
		case '"':
			d.closed = true
			return b[i+1:]
		default:
			d.cur = append(d.cur, c)
		}
	}
	return ""
}

func (d *streamingDecoder) newline() {
	d.tail = append(d.tail, string(d.cur))
	if len(d.tail) > d.tailCap {
		d.tail = d.tail[len(d.tail)-d.tailCap:]
	}
	d.cur = d.cur[:0]
	d.lineCount++
}

func (d *streamingDecoder) hasContent() bool { return d.inContent }

func (d *streamingDecoder) lineTotal() int {
	if !d.inContent {
		return 0
	}
	n := d.lineCount
	if len(d.cur) > 0 || !d.closed {
		n++
	}
	if n < 1 {
		n = 1
	}
	return n
}

func (d *streamingDecoder) tailLines() []string {
	out := append([]string(nil), d.tail...)
	if len(d.cur) > 0 {
		out = append(out, string(d.cur))
	}
	if len(out) > d.tailCap {
		out = out[len(out)-d.tailCap:]
	}
	return out
}
