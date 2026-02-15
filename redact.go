package main

import (
	"context"
	"log/slog"
	"strings"
)

// redactingHandler wraps an slog.Handler and replaces known secrets with
// "[REDACTED]" in log messages and string attribute values.
type redactingHandler struct {
	inner   slog.Handler
	secrets []string
}

// newRedactingHandler returns a handler that scrubs every occurrence of the
// supplied secrets from log output. Empty strings are filtered out to avoid
// replacing everything.
func newRedactingHandler(inner slog.Handler, secrets []string) slog.Handler {
	filtered := make([]string, 0, len(secrets))
	for _, s := range secrets {
		if s != "" {
			filtered = append(filtered, s)
		}
	}
	return &redactingHandler{inner: inner, secrets: filtered}
}

// Enabled delegates to the inner handler.
func (h *redactingHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

// Handle redacts secrets from the log message and all attributes before
// forwarding to the inner handler.
func (h *redactingHandler) Handle(ctx context.Context, r slog.Record) error {
	r.Message = h.redact(r.Message)

	var attrs []slog.Attr
	r.Attrs(func(a slog.Attr) bool {
		attrs = append(attrs, h.redactAttr(a))
		return true
	})

	nr := slog.NewRecord(r.Time, r.Level, r.Message, r.PC)
	nr.AddAttrs(attrs...)
	return h.inner.Handle(ctx, nr)
}

// WithAttrs returns a new handler with redacted pre-set attributes.
func (h *redactingHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	redacted := make([]slog.Attr, len(attrs))
	for i, a := range attrs {
		redacted[i] = h.redactAttr(a)
	}
	return &redactingHandler{inner: h.inner.WithAttrs(redacted), secrets: h.secrets}
}

// WithGroup returns a new handler scoped to the named group.
func (h *redactingHandler) WithGroup(name string) slog.Handler {
	return &redactingHandler{inner: h.inner.WithGroup(name), secrets: h.secrets}
}

func (h *redactingHandler) redact(s string) string {
	for _, secret := range h.secrets {
		s = strings.ReplaceAll(s, secret, "[REDACTED]")
	}
	return s
}

func (h *redactingHandler) redactAttr(a slog.Attr) slog.Attr {
	switch a.Value.Kind() {
	case slog.KindString:
		a.Value = slog.StringValue(h.redact(a.Value.String()))
	case slog.KindGroup:
		attrs := a.Value.Group()
		redacted := make([]slog.Attr, len(attrs))
		for i, ga := range attrs {
			redacted[i] = h.redactAttr(ga)
		}
		a.Value = slog.GroupValue(redacted...)
	}
	return a
}
