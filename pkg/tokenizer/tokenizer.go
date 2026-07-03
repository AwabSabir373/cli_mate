package tokenizer

import (
	"unicode/utf8"

	tiktoken "github.com/pkoukk/tiktoken-go"
)

type Counter interface {
	Count(string) int
}

type ApproxCounter struct{}

type TiktokenCounter struct {
	encoding *tiktoken.Tiktoken
	fallback ApproxCounter
}

func New(model string) Counter {
	encoding, err := tiktoken.EncodingForModel(model)
	if err != nil {
		return NewApproxCounter()
	}
	return TiktokenCounter{encoding: encoding, fallback: NewApproxCounter()}
}

func NewApproxCounter() ApproxCounter {
	return ApproxCounter{}
}

func (c TiktokenCounter) Count(text string) int {
	if c.encoding == nil {
		return c.fallback.Count(text)
	}
	return len(c.encoding.Encode(text, nil, nil))
}

func (ApproxCounter) Count(text string) int {
	runes := utf8.RuneCountInString(text)
	if runes == 0 {
		return 0
	}
	return (runes + 3) / 4
}
