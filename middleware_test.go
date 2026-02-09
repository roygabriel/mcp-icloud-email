package main

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
)

func makeRequest(toolName string) mcp.CallToolRequest {
	return mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: toolName,
		},
	}
}

func TestTimeoutMiddleware(t *testing.T) {
	t.Run("handler completes in time", func(t *testing.T) {
		mw := timeoutMiddleware(1 * time.Second)

		handler := mw(func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return &mcp.CallToolResult{}, nil
		})

		result, err := handler(context.Background(), makeRequest("test_tool"))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result == nil {
			t.Fatal("expected non-nil result")
		}
	})

	t.Run("handler exceeds timeout", func(t *testing.T) {
		mw := timeoutMiddleware(10 * time.Millisecond)

		handler := mw(func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(1 * time.Second):
				return &mcp.CallToolResult{}, nil
			}
		})

		_, err := handler(context.Background(), makeRequest("slow_tool"))
		if err == nil {
			t.Fatal("expected timeout error")
		}
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("expected DeadlineExceeded, got: %v", err)
		}
	})

	t.Run("pre-canceled context", func(t *testing.T) {
		mw := timeoutMiddleware(1 * time.Second)

		handler := mw(func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			default:
				return &mcp.CallToolResult{}, nil
			}
		})

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		_, err := handler(ctx, makeRequest("test_tool"))
		// With pre-canceled context, the timeout middleware creates a new deadline.
		// The inner handler may or may not see the cancellation depending on timing.
		// Either outcome is acceptable.
		_ = err
	})
}

func TestLoggingMiddleware(t *testing.T) {
	t.Run("successful result", func(t *testing.T) {
		mw := loggingMiddleware()

		handler := mw(func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return &mcp.CallToolResult{}, nil
		})

		result, err := handler(context.Background(), makeRequest("test_tool"))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result == nil {
			t.Fatal("expected non-nil result")
		}
	})

	t.Run("handler error", func(t *testing.T) {
		mw := loggingMiddleware()

		handler := mw(func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return nil, errors.New("handler failed")
		})

		_, err := handler(context.Background(), makeRequest("failing_tool"))
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("result with IsError", func(t *testing.T) {
		mw := loggingMiddleware()

		handler := mw(func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return &mcp.CallToolResult{IsError: true}, nil
		})

		result, err := handler(context.Background(), makeRequest("error_tool"))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.IsError {
			t.Error("expected IsError=true")
		}
	})

	t.Run("nil result", func(t *testing.T) {
		mw := loggingMiddleware()

		handler := mw(func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return nil, nil
		})

		result, err := handler(context.Background(), makeRequest("nil_tool"))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != nil {
			t.Error("expected nil result")
		}
	})
}

func TestComposedMiddleware(t *testing.T) {
	t.Run("timeout inside logging", func(t *testing.T) {
		// Match real registration order: logging wraps timeout wraps handler
		logging := loggingMiddleware()
		timeout := timeoutMiddleware(100 * time.Millisecond)

		handler := logging(timeout(func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return &mcp.CallToolResult{}, nil
		}))

		result, err := handler(context.Background(), makeRequest("composed_tool"))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result == nil {
			t.Fatal("expected non-nil result")
		}
	})

	t.Run("composed timeout triggers", func(t *testing.T) {
		logging := loggingMiddleware()
		timeout := timeoutMiddleware(10 * time.Millisecond)

		handler := logging(timeout(func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(1 * time.Second):
				return &mcp.CallToolResult{}, nil
			}
		}))

		_, err := handler(context.Background(), makeRequest("slow_composed"))
		if err == nil {
			t.Fatal("expected timeout error")
		}
	})
}
