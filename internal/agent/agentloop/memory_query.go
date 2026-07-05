package agentloop

type MemoryQuery struct {
	Scope string
	Type  MemoryType
	Text  string
	Limit int
}
