package main

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"
)

type presenceService struct {
	client  *RealtimeClient
	channel string
	now     func() time.Time
}

func (s *presenceService) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { writeJSON(w, 200, map[string]string{"status": "ok"}) })
	mux.HandleFunc("GET /online", s.online)
	mux.HandleFunc("POST /tokens", s.token)
	mux.HandleFunc("POST /events", s.event)
	return requestLog(mux)
}

func (s *presenceService) online(w http.ResponseWriter, r *http.Request) {
	data, err := s.client.Presence(r.Context(), s.channel)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	writeRaw(w, http.StatusOK, data)
}

func (s *presenceService) token(w http.ResponseWriter, r *http.Request) {
	var input struct {
		DeveloperID string `json:"developer_id"`
		RequestID   string `json:"request_id"`
	}
	if err := decodeJSON(r, &input); err != nil || input.DeveloperID == "" || input.RequestID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "developer_id and request_id are required"})
		return
	}
	data, err := s.client.IssueToken(r.Context(), input.DeveloperID, s.channel, 900, input.RequestID)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	writeRaw(w, http.StatusOK, data)
}

func (s *presenceService) event(w http.ResponseWriter, r *http.Request) {
	var input WorkspaceEvent
	if err := decodeJSON(r, &input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
		return
	}
	name, payload, err := prepareEvent(input, s.now())
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if err := s.client.Publish(r.Context(), s.channel, name, payload, input.ActorID, input.RequestID); err != nil {
		writeAPIError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"event": name, "request_id": input.RequestID})
}

func decodeJSON(r *http.Request, target any) error {
	defer r.Body.Close()
	decoder := json.NewDecoder(http.MaxBytesReader(nil, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	return decoder.Decode(target)
}

func writeAPIError(w http.ResponseWriter, err error) {
	status := http.StatusBadGateway
	var apiErr *InfraiError
	if errors.As(err, &apiErr) && apiErr.Status >= 400 && apiErr.Status < 500 {
		status = apiErr.Status
	}
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

func writeRaw(w http.ResponseWriter, status int, raw json.RawMessage) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(raw)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func requestLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		next.ServeHTTP(w, r)
		slog.Info("request", "method", r.Method, "path", r.URL.Path, "duration_ms", strconv.FormatInt(time.Since(started).Milliseconds(), 10))
	})
}
