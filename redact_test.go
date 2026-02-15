package main

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
)

func TestRedactingHandler_Message(t *testing.T) {
	var buf bytes.Buffer
	base := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	h := newRedactingHandler(base, []string{"secret-token-123"})
	logger := slog.New(h)

	logger.Info("connecting with token secret-token-123 to API")

	if strings.Contains(buf.String(), "secret-token-123") {
		t.Errorf("secret leaked in message: %s", buf.String())
	}
	if !strings.Contains(buf.String(), "[REDACTED]") {
		t.Errorf("expected [REDACTED] in output: %s", buf.String())
	}
}

func TestRedactingHandler_StringAttr(t *testing.T) {
	var buf bytes.Buffer
	base := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	h := newRedactingHandler(base, []string{"secret-token-123"})
	logger := slog.New(h)

	logger.Info("request", "auth", "Bearer secret-token-123")

	if strings.Contains(buf.String(), "secret-token-123") {
		t.Errorf("secret leaked in attr: %s", buf.String())
	}
	if !strings.Contains(buf.String(), "[REDACTED]") {
		t.Errorf("expected [REDACTED] in output: %s", buf.String())
	}
}

func TestRedactingHandler_GroupAttr(t *testing.T) {
	var buf bytes.Buffer
	base := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	h := newRedactingHandler(base, []string{"secret-token-123"})
	logger := slog.New(h)

	logger.Info("grouped",
		slog.Group("config",
			slog.String("token", "secret-token-123"),
			slog.String("host", "imap.mail.me.com"),
		),
	)

	if strings.Contains(buf.String(), "secret-token-123") {
		t.Errorf("secret leaked in group attr: %s", buf.String())
	}
}

func TestRedactingHandler_WithAttrs(t *testing.T) {
	var buf bytes.Buffer
	base := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	h := newRedactingHandler(base, []string{"secret-token-123"})
	logger := slog.New(h).With("token", "secret-token-123")

	logger.Info("test")

	if strings.Contains(buf.String(), "secret-token-123") {
		t.Errorf("secret leaked via WithAttrs: %s", buf.String())
	}
}

func TestRedactingHandler_WithGroup(t *testing.T) {
	var buf bytes.Buffer
	base := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	h := newRedactingHandler(base, []string{"secret-token-123"})
	logger := slog.New(h).WithGroup("request")

	logger.Info("auth", "header", "Bearer secret-token-123")

	if strings.Contains(buf.String(), "secret-token-123") {
		t.Errorf("secret leaked via WithGroup: %s", buf.String())
	}
}

func TestRedactingHandler_NoSecrets(t *testing.T) {
	var buf bytes.Buffer
	base := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	h := newRedactingHandler(base, nil)
	logger := slog.New(h)

	logger.Info("safe message", "key", "value")

	if !strings.Contains(buf.String(), "safe message") {
		t.Errorf("message was altered: %s", buf.String())
	}
}

func TestRedactingHandler_EmptySecret(t *testing.T) {
	var buf bytes.Buffer
	base := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	h := newRedactingHandler(base, []string{""})
	logger := slog.New(h)

	logger.Info("hello world")

	if !strings.Contains(buf.String(), "hello world") {
		t.Errorf("empty secret should not alter output: %s", buf.String())
	}
}

func TestRedactingHandler_Enabled(t *testing.T) {
	base := slog.NewJSONHandler(&bytes.Buffer{}, &slog.HandlerOptions{Level: slog.LevelWarn})
	h := newRedactingHandler(base, []string{"secret"})

	if h.Enabled(context.Background(), slog.LevelDebug) {
		t.Error("debug should not be enabled when base level is warn")
	}
	if !h.Enabled(context.Background(), slog.LevelWarn) {
		t.Error("warn should be enabled when base level is warn")
	}
}

func TestRedactingHandler_MultipleSecrets(t *testing.T) {
	var buf bytes.Buffer
	base := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	h := newRedactingHandler(base, []string{"secret1", "secret2"})
	logger := slog.New(h)

	logger.Info("tokens: secret1 and secret2")

	output := buf.String()
	if strings.Contains(output, "secret1") || strings.Contains(output, "secret2") {
		t.Errorf("secrets leaked: %s", output)
	}
}
