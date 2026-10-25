package main

import (
	"encoding/json"
	"testing"
	"time"
)

func TestPrepareEventChoosesDeveloperEvent(t *testing.T) {
	now := time.Date(2026, 9, 5, 4, 3, 2, 0, time.UTC)
	tests := []struct {
		name      string
		input     WorkspaceEvent
		wantEvent string
		wantErr   bool
	}{
		{name: "completed build", input: WorkspaceEvent{RequestID: "evt-101", Kind: "build", ActorID: "dev-7", Target: "api", Status: "passed"}, wantEvent: "build.completed"},
		{name: "release operation", input: WorkspaceEvent{RequestID: "evt-102", Kind: "release", ActorID: "dev-8", Target: "web", Status: "started"}, wantEvent: "release.changed"},
		{name: "developer diagnostic", input: WorkspaceEvent{RequestID: "evt-103", Kind: "diagnostic", ActorID: "dev-9", Target: "runner", Status: "warning"}, wantEvent: "diagnostic.reported"},
		{name: "unknown operation", input: WorkspaceEvent{RequestID: "evt-104", Kind: "deploy", ActorID: "dev-9", Target: "runner", Status: "started"}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event, raw, err := prepareEvent(tt.input, now)
			if (err != nil) != tt.wantErr {
				t.Fatalf("prepareEvent() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if event != tt.wantEvent {
				t.Fatalf("event = %q, want %q", event, tt.wantEvent)
			}
			var output PublishedEvent
			if err := json.Unmarshal(raw, &output); err != nil {
				t.Fatal(err)
			}
			if output.ObservedAt != now || output.Target != tt.input.Target {
				t.Fatalf("payload = %+v", output)
			}
		})
	}
}
