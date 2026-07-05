package openrouter

import (
	"context"
	"io"
	"strings"
	"testing"
)

func TestStreamOpenAICompatibleEmitsHeartbeatForKeepaliveLines(t *testing.T) {
	stream := streamOpenAICompatible(context.Background(), io.NopCloser(strings.NewReader(": keepalive\n\ndata: [DONE]\n\n")))

	heartbeats := 0
	for event := range stream {
		if event.Heartbeat {
			heartbeats++
			continue
		}
		if event.Done {
			if heartbeats == 0 {
				t.Fatal("expected at least one heartbeat before done")
			}
			return
		}
		t.Fatalf("unexpected event %#v", event)
	}
	t.Fatal("expected done event")
}
