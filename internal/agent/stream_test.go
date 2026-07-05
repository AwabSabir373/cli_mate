package agent

import (
	"context"
	"testing"

	"cli_mate/internal/providers"
)

func TestStreamHandlerReportsHeartbeatActivity(t *testing.T) {
	events := make(chan providers.StreamEvent, 2)
	events <- providers.StreamEvent{Heartbeat: true}
	events <- providers.StreamEvent{Delta: "done"}
	close(events)

	activities := 0
	answer, _, err := (StreamHandler{
		OnActivity: func() { activities++ },
	}).Consume(context.Background(), events)
	if err != nil {
		t.Fatal(err)
	}
	if answer != "done" {
		t.Fatalf("expected answer, got %q", answer)
	}
	if activities != 2 {
		t.Fatalf("expected heartbeat and delta activity, got %d", activities)
	}
}
