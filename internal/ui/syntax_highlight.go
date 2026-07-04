package ui

import (
	"path/filepath"
	"strings"
	"sync"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/formatters"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/charmbracelet/lipgloss"
)

var (
	lexerCacheMu sync.RWMutex
	lexerCache   = make(map[string]chroma.Lexer)
)

// cachedLexer returns a cached chroma lexer for the given language.
func cachedLexer(lang string) chroma.Lexer {
	if lang == "" {
		return nil
	}
	lexerCacheMu.RLock()
	lexer, ok := lexerCache[lang]
	lexerCacheMu.RUnlock()
	if ok {
		return lexer
	}
	lexer = lexers.Get(lang)
	if lexer == nil {
		lexer = lexers.Fallback
	}
	lexerCacheMu.Lock()
	lexerCache[lang] = lexer
	lexerCacheMu.Unlock()
	return lexer
}

// cachedLexerForPath returns a cached lexer based on file extension.
func cachedLexerForPath(path string) chroma.Lexer {
	ext := filepath.Ext(path)
	if ext == "" {
		return nil
	}
	lang := strings.TrimPrefix(ext, ".")
	return cachedLexer(lang)
}

const syntaxHighlightStyle = "catppuccin"

// highlightCode syntax-highlights a code string for the given language.
func highlightCode(code string, lang string) string {
	lexer := cachedLexer(lang)
	if lexer == nil {
		return code
	}

	style := styles.Get(syntaxHighlightStyle)
	if style == nil {
		style = styles.Fallback
	}

	iterator, err := lexer.Tokenise(nil, code)
	if err != nil {
		return code
	}

	formatter := formatters.Get("terminal256")
	if formatter == nil {
		return code
	}

	var buf strings.Builder
	if err := formatter.Format(&buf, style, iterator); err != nil {
		return code
	}

	return buf.String()
}

// highlightCodeWithTheme syntax-highlights code using terminal theme-aware colors.
// Falls back to the simple version if custom theming isn't available.
func highlightCodeWithTheme(code string, lang string, _ lipgloss.Color, _ lipgloss.Color) string {
	if code == "" {
		return ""
	}
	return highlightCode(code, lang)
}
