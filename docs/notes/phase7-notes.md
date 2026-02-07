# Phase 7: Observability - Working Notes

## What Was Done

### Logging Middleware (`loggingMiddleware` in main.go)
- Generates a UUID `request_id` per tool call for log correlation
- Extracts tool name from `req.Params.Name`
- Logs at 3 levels:
  - **DEBUG**: Tool call started (before execution)
  - **INFO**: Tool call completed successfully (with duration_ms)
  - **WARN**: Tool returned an application error (IsError=true, with duration_ms)
  - **ERROR**: Tool returned a Go error (with duration_ms and error message)
- Middleware is outermost in the chain: `logging(timeout(handler))` so it captures the full duration including timeout handling

### Middleware Chain Order
```go
server.WithToolHandlerMiddleware(timeoutMiddleware(60*time.Second)),
server.WithToolHandlerMiddleware(loggingMiddleware()),
```
mcp-go applies middleware in reverse order, so logging wraps timeout wraps the actual handler.

### Example Log Output
```json
{"time":"...","level":"DEBUG","msg":"tool call started","request_id":"abc-123","tool":"search_emails"}
{"time":"...","level":"INFO","msg":"tool call completed","request_id":"abc-123","tool":"search_emails","duration_ms":142}
```

## What Was NOT Added (and Why)

### Health/Ready Endpoints
MCP servers communicate over stdio (JSON-RPC), not HTTP. There's no HTTP listener to expose `/healthz` or `/readyz`. The IMAP connection test at startup already validates readiness. Not applicable.

### Prometheus Metrics
No HTTP server to expose a `/metrics` endpoint. For a stdio-based MCP server, structured logs with request IDs and durations provide sufficient observability. Metrics can be derived from logs using tools like `jq` or log aggregators.

## Findings Addressed
- F8 (partial): Added request correlation with request_id field in all tool call logs
- General observability: Duration tracking, error-level differentiation

## Verification
- `go build` ✓
- `go vet ./...` ✓
- `go test -race ./...` ✓ (all pass)
