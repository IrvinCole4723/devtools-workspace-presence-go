package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

var eventNames = map[string]string{
	"build":      "build.completed",
	"release":    "release.changed",
	"diagnostic": "diagnostic.reported",
}

type WorkspaceEvent struct {
	RequestID string          `json:"request_id"`
	Kind      string          `json:"kind"`
	ActorID   string          `json:"actor_id"`
	Target    string          `json:"target"`
	Status    string          `json:"status"`
	Details   json.RawMessage `json:"details,omitempty"`
}

type PublishedEvent struct {
	Kind       string          `json:"kind"`
	Target     string          `json:"target"`
	Status     string          `json:"status"`
	Details    json.RawMessage `json:"details,omitempty"`
	ObservedAt time.Time       `json:"observed_at"`
}

func prepareEvent(input WorkspaceEvent, now time.Time) (string, []byte, error) {
	name, ok := eventNames[input.Kind]
	if !ok {
		return "", nil, errors.New("kind must be build, release, or diagnostic")
	}
	if input.RequestID == "" || input.ActorID == "" || input.Target == "" || input.Status == "" {
		return "", nil, errors.New("request_id, actor_id, target, and status are required")
	}
	payload, err := json.Marshal(PublishedEvent{
		Kind: input.Kind, Target: input.Target, Status: input.Status,
		Details: input.Details, ObservedAt: now.UTC(),
	})
	if err != nil {
		return "", nil, fmt.Errorf("encode workspace event: %w", err)
	}
	return name, payload, nil
}
