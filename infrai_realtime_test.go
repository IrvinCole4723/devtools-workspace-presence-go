package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestCreateChannelOmitsVendor(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/realtime/channel/create" {
			t.Fatalf("request = %s %s", r.Method, r.URL.Path)
		}
		var body map[string]json.RawMessage
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if _, present := body["vendor"]; present {
			t.Fatalf("unexpected vendor field: %s", body["vendor"])
		}
		if string(body["channel"]) != `"test-channel"` || string(body["type"]) != `"presence"` {
			t.Fatalf("unexpected request body: %v", body)
		}
		_, _ = w.Write([]byte(`{"ok":true,"data":{},"error":null}`))
	}))
	defer server.Close()
	client := NewRealtimeClient("test-key")
	client.baseURL = server.URL
	client.httpClient = server.Client()
	if err := client.CreateChannel(context.Background(), "test-channel", "channel-test"); err != nil {
		t.Fatal(err)
	}
}

func TestPublishRetriesRateLimitWithSameIdempotencyKey(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.Method != http.MethodPost || r.URL.Path != "/v1/realtime/publish" {
			t.Fatalf("request = %s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("Idempotency-Key"); got != "evt-42" {
			t.Fatalf("idempotency key = %q", got)
		}
		_, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		if requests == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"ok":false,"data":null,"error":{"code":"RATE_LIMITED","message":"retry later"},"metadata":{}}`))
			return
		}
		_, _ = w.Write([]byte(`{"ok":true,"data":{"published":true},"error":null,"metadata":{}}`))
	}))
	defer server.Close()

	client := NewRealtimeClient("test-key")
	client.baseURL = server.URL
	client.httpClient = server.Client()
	client.sleep = func(context.Context, time.Duration) error { return nil }
	err := client.Publish(context.Background(), "developer-tools", "build.completed", []byte(`{"status":"passed"}`), "dev-7", "evt-42")
	if err != nil {
		t.Fatal(err)
	}
	if requests != 2 {
		t.Fatalf("requests = %d, want 2", requests)
	}
}
