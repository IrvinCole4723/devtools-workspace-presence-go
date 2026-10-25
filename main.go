package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	key := os.Getenv("INFRAI_API_KEY")
	if key == "" {
		slog.Error("INFRAI_API_KEY is required")
		os.Exit(2)
	}
	channel := envOr("WORKSPACE_CHANNEL", "developer-tools")
	client := NewRealtimeClient(key)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := client.CreateChannel(ctx, channel, "channel-"+channel); err != nil {
		slog.Error("create presence channel", "error", err)
		os.Exit(1)
	}

	service := &presenceService{client: client, channel: channel, now: time.Now}
	server := &http.Server{Addr: envOr("LISTEN_ADDR", ":8080"), Handler: service.routes(), ReadHeaderTimeout: 5 * time.Second}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()
	slog.Info("presence service listening", "address", server.Addr, "channel", channel)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		slog.Error("serve", "error", err)
		os.Exit(1)
	}
}

func envOr(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
