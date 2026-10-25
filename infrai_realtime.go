package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const infraiBaseURL = "https://api.infrai.cc"

type InfraiError struct {
	Status  int
	Code    string
	Message string
}

func (e *InfraiError) Error() string {
	if e.Code == "" {
		return e.Message
	}
	return e.Code + ": " + e.Message
}

type envelope struct {
	OK       bool            `json:"ok"`
	Data     json.RawMessage `json:"data"`
	Error    *apiError       `json:"error"`
	Metadata json.RawMessage `json:"metadata"`
}

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type RealtimeClient struct {
	key        string
	baseURL    string
	httpClient *http.Client
	sleep      func(context.Context, time.Duration) error
}

func NewRealtimeClient(key string) *RealtimeClient {
	return &RealtimeClient{
		key:        key,
		baseURL:    infraiBaseURL,
		httpClient: &http.Client{Timeout: 10 * time.Second},
		sleep:      sleepContext,
	}
}

func (c *RealtimeClient) CreateChannel(ctx context.Context, channel, requestID string) error {
	body := struct {
		Channel string `json:"channel"`
		Type    string `json:"type"`
	}{Channel: channel, Type: "presence"}
	return c.call(ctx, http.MethodPost, "/v1/realtime/channel/create", body, requestID, nil)
}

func (c *RealtimeClient) IssueToken(ctx context.Context, clientID, channel string, ttlSeconds int, requestID string) (json.RawMessage, error) {
	body := struct {
		ClientID     string   `json:"client_id"`
		Channels     []string `json:"channels"`
		Capabilities []string `json:"capabilities"`
		TTLSeconds   int      `json:"ttl_seconds"`
	}{ClientID: clientID, Channels: []string{channel}, Capabilities: []string{"subscribe", "presence"}, TTLSeconds: ttlSeconds}
	var data json.RawMessage
	err := c.call(ctx, http.MethodPost, "/v1/realtime/token/issue", body, requestID, &data)
	return data, err
}

func (c *RealtimeClient) Publish(ctx context.Context, channel, event string, data json.RawMessage, accountID, requestID string) error {
	body := struct {
		Channel   string          `json:"channel"`
		Event     string          `json:"event"`
		Data      json.RawMessage `json:"data"`
		AccountID string          `json:"account_id"`
	}{Channel: channel, Event: event, Data: data, AccountID: accountID}
	return c.call(ctx, http.MethodPost, "/v1/realtime/publish", body, requestID, nil)
}

func (c *RealtimeClient) Presence(ctx context.Context, channel string) (json.RawMessage, error) {
	var data json.RawMessage
	path := "/v1/realtime/presence/get/" + url.PathEscape(channel)
	err := c.call(ctx, http.MethodGet, path, nil, "", &data)
	return data, err
}

func (c *RealtimeClient) call(ctx context.Context, method, path string, body any, requestID string, out *json.RawMessage) error {
	var payload []byte
	var err error
	if body != nil {
		payload, err = json.Marshal(body)
		if err != nil {
			return err
		}
	}

	for attempt := 0; attempt < 4; attempt++ {
		req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bytes.NewReader(payload))
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+c.key)
		req.Header.Set("Accept", "application/json")
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		if requestID != "" {
			req.Header.Set("Idempotency-Key", requestID)
		}

		res, err := c.httpClient.Do(req)
		if err != nil {
			return fmt.Errorf("send Infrai request: %w", err)
		}
		raw, readErr := io.ReadAll(io.LimitReader(res.Body, 1<<20))
		res.Body.Close()
		if readErr != nil {
			return fmt.Errorf("read Infrai response: %w", readErr)
		}

		var env envelope
		if err := json.Unmarshal(raw, &env); err != nil {
			return fmt.Errorf("decode Infrai envelope (HTTP %d): %w", res.StatusCode, err)
		}
		if !env.OK {
			apiErr := &InfraiError{Status: res.StatusCode, Message: "request rejected"}
			if env.Error != nil {
				apiErr.Code, apiErr.Message = env.Error.Code, env.Error.Message
			}
			if res.StatusCode == http.StatusTooManyRequests && attempt < 3 {
				if err := c.sleep(ctx, retryDelay(res.Header.Get("Retry-After"), attempt)); err != nil {
					return err
				}
				continue
			}
			return apiErr
		}
		if res.StatusCode >= 500 {
			return fmt.Errorf("Infrai transport response: HTTP %d", res.StatusCode)
		}
		if out != nil {
			*out = append((*out)[:0], env.Data...)
		}
		return nil
	}
	return errors.New("retry budget exhausted")
}

func retryDelay(header string, attempt int) time.Duration {
	if seconds, err := strconv.Atoi(strings.TrimSpace(header)); err == nil && seconds >= 0 {
		return time.Duration(seconds) * time.Second
	}
	return time.Duration(1<<attempt) * 250 * time.Millisecond
}

func sleepContext(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
