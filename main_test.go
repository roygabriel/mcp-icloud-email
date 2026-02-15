package main

import (
	"bytes"
	"context"
	"log/slog"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func makeRequest(toolName string) mcp.CallToolRequest {
	return mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: toolName,
		},
	}
}

func TestToolMiddleware_AuditDestructive(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	slog.SetDefault(slog.New(handler))

	cb := NewCircuitBreaker(5, 30*time.Second)
	mw := toolMiddleware(5*time.Second, cb)
	wrapped := mw(func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return &mcp.CallToolResult{}, nil
	})

	req := mcp.CallToolRequest{}
	req.Params.Name = "delete_email"
	req.Params.Arguments = map[string]interface{}{"email_id": "123"}

	_, _ = wrapped(context.Background(), req)

	output := buf.String()
	if !strings.Contains(output, `"audit":true`) {
		t.Errorf("expected audit:true for destructive tool, got: %s", output)
	}
	if !strings.Contains(output, "email_id") {
		t.Errorf("expected arguments in audit log, got: %s", output)
	}
}

func TestToolMiddleware_NoAuditReadOnly(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	slog.SetDefault(slog.New(handler))

	cb := NewCircuitBreaker(5, 30*time.Second)
	mw := toolMiddleware(5*time.Second, cb)
	wrapped := mw(func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return &mcp.CallToolResult{}, nil
	})

	req := mcp.CallToolRequest{}
	req.Params.Name = "search_emails"

	_, _ = wrapped(context.Background(), req)

	output := buf.String()
	if strings.Contains(output, `"audit"`) {
		t.Errorf("read-only tool should not have audit field, got: %s", output)
	}
}

func TestToolMiddleware_Timeout(t *testing.T) {
	cb := NewCircuitBreaker(5, 30*time.Second)
	mw := toolMiddleware(50*time.Millisecond, cb)

	wrapped := mw(func(ctx context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(5 * time.Second):
			return &mcp.CallToolResult{}, nil
		}
	})

	req := mcp.CallToolRequest{}
	req.Params.Name = "search_emails"

	_, err := wrapped(context.Background(), req)
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if !strings.Contains(err.Error(), "deadline exceeded") {
		t.Errorf("expected deadline exceeded, got: %v", err)
	}
}

func TestToolMiddleware_CircuitBreakerRejectsWhenOpen(t *testing.T) {
	cb := NewCircuitBreaker(1, 30*time.Second)

	// Trip the breaker
	done, _ := cb.Allow()
	done(false)

	mw := toolMiddleware(5*time.Second, cb)
	wrapped := mw(func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		t.Fatal("handler should not be called when circuit is open")
		return nil, nil
	})

	_, err := wrapped(context.Background(), makeRequest("search_emails"))
	if err == nil {
		t.Fatal("expected error from open circuit breaker")
	}
	if !strings.Contains(err.Error(), "circuit breaker is open") {
		t.Errorf("expected circuit breaker error, got: %v", err)
	}
}

func TestToolMiddleware_CircuitBreakerCountsFailures(t *testing.T) {
	cb := NewCircuitBreaker(2, 30*time.Second)
	mw := toolMiddleware(5*time.Second, cb)

	errHandler := mw(func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return nil, context.DeadlineExceeded
	})

	// Two failures should trip the breaker
	_, _ = errHandler(context.Background(), makeRequest("test"))
	_, _ = errHandler(context.Background(), makeRequest("test"))

	if cb.State() != StateOpen {
		t.Errorf("expected open after 2 failures, got %s", cb.State())
	}
}

func TestDestructiveTools_Membership(t *testing.T) {
	expected := []string{
		"delete_email", "delete_folder", "send_email",
		"reply_email", "draft_email",
	}
	for _, name := range expected {
		if !destructiveTools[name] {
			t.Errorf("%s should be in destructiveTools", name)
		}
	}

	notExpected := []string{
		"search_emails", "get_email", "list_folders",
		"count_emails", "mark_read", "move_email",
		"flag_email", "get_attachment", "create_folder",
	}
	for _, name := range notExpected {
		if destructiveTools[name] {
			t.Errorf("%s should not be in destructiveTools", name)
		}
	}
}

func TestConcurrencyMiddleware_LimitsConcurrency(t *testing.T) {
	const maxConcurrent = 3
	mw := concurrencyMiddleware(maxConcurrent)

	var running atomic.Int32
	var peak atomic.Int32
	barrier := make(chan struct{})

	handler := mw(func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		cur := running.Add(1)
		for {
			old := peak.Load()
			if cur <= old || peak.CompareAndSwap(old, cur) {
				break
			}
		}
		<-barrier
		running.Add(-1)
		return &mcp.CallToolResult{}, nil
	})

	done := make(chan struct{}, maxConcurrent+2)
	for i := 0; i < maxConcurrent+2; i++ {
		go func() {
			_, _ = handler(context.Background(), mcp.CallToolRequest{})
			done <- struct{}{}
		}()
	}

	time.Sleep(50 * time.Millisecond)

	if r := running.Load(); r > int32(maxConcurrent) {
		t.Errorf("expected at most %d concurrent, got %d", maxConcurrent, r)
	}

	close(barrier)
	for i := 0; i < maxConcurrent+2; i++ {
		<-done
	}

	if p := peak.Load(); p > int32(maxConcurrent) {
		t.Errorf("peak concurrency %d exceeded limit %d", p, maxConcurrent)
	}
}

func TestConcurrencyMiddleware_RespectsContextCancellation(t *testing.T) {
	mw := concurrencyMiddleware(1)

	blocking := make(chan struct{})
	go func() {
		handler := mw(func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			<-blocking
			return &mcp.CallToolResult{}, nil
		})
		_, _ = handler(context.Background(), mcp.CallToolRequest{})
	}()
	time.Sleep(20 * time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	handler := mw(func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		t.Fatal("handler should not be called")
		return nil, nil
	})

	_, err := handler(ctx, mcp.CallToolRequest{})
	if err == nil {
		t.Fatal("expected error from cancelled context")
	}
	close(blocking)
}

func TestChainMiddleware_Order(t *testing.T) {
	var order []string

	mw1 := func(next server.ToolHandlerFunc) server.ToolHandlerFunc {
		return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			order = append(order, "mw1-before")
			r, e := next(ctx, req)
			order = append(order, "mw1-after")
			return r, e
		}
	}

	mw2 := func(next server.ToolHandlerFunc) server.ToolHandlerFunc {
		return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			order = append(order, "mw2-before")
			r, e := next(ctx, req)
			order = append(order, "mw2-after")
			return r, e
		}
	}

	chained := chainMiddleware(mw1, mw2)
	handler := chained(func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		order = append(order, "handler")
		return &mcp.CallToolResult{}, nil
	})

	_, _ = handler(context.Background(), mcp.CallToolRequest{})

	expected := []string{"mw1-before", "mw2-before", "handler", "mw2-after", "mw1-after"}
	if len(order) != len(expected) {
		t.Fatalf("expected %d entries, got %d: %v", len(expected), len(order), order)
	}
	for i, v := range expected {
		if order[i] != v {
			t.Errorf("position %d: expected %q, got %q", i, v, order[i])
		}
	}
}

func TestLogLevel(t *testing.T) {
	tests := []struct {
		env  string
		want slog.Level
	}{
		{"DEBUG", slog.LevelDebug},
		{"WARN", slog.LevelWarn},
		{"ERROR", slog.LevelError},
		{"INFO", slog.LevelInfo},
		{"", slog.LevelInfo},
		{"debug", slog.LevelDebug},
		{"unknown", slog.LevelInfo},
	}

	for _, tt := range tests {
		t.Run(tt.env, func(t *testing.T) {
			os.Setenv("LOG_LEVEL", tt.env)
			defer os.Unsetenv("LOG_LEVEL")
			if got := logLevel(); got != tt.want {
				t.Errorf("logLevel(%q) = %v, want %v", tt.env, got, tt.want)
			}
		})
	}
}

func TestSetupLogger(t *testing.T) {
	opts := setupLogger()
	if opts == nil {
		t.Fatal("expected non-nil options")
	}
}

func TestInstallRedaction(t *testing.T) {
	opts := &slog.HandlerOptions{Level: slog.LevelDebug}
	installRedaction(opts, []string{"my-secret"})

	var buf bytes.Buffer
	handler := newRedactingHandler(
		slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}),
		[]string{"my-secret"},
	)
	slog.SetDefault(slog.New(handler))

	slog.Info("token is my-secret")
	if strings.Contains(buf.String(), "my-secret") {
		t.Error("secret was not redacted")
	}
}

func TestGenerateRequestID(t *testing.T) {
	id := generateRequestID()
	if len(id) != 8 {
		t.Errorf("expected 8-char hex string, got %d: %s", len(id), id)
	}
	if generateRequestID() == id {
		t.Error("two consecutive IDs should not be equal")
	}
}
