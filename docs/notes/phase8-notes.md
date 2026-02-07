# Phase 8: Advanced Features - Working Notes

## What Was Done

### F26: Pagination Offset for search_emails
Added `offset` parameter to `search_emails` for paginating through large result sets.

**Changes:**
- `imap/client.go`: Added `Offset int` field to `EmailFilters` struct
- `imap/client.go`: `SearchEmails` now returns `([]Email, int, error)` where `int` is the total matching count before offset/limit. Offset trims from the most-recent end before limit is applied.
- `tools/interfaces.go`: Updated `EmailReader.SearchEmails` signature
- `tools/search_emails.go`: Parses `offset` from args, includes `total` in response
- `main.go`: Added `offset` parameter to search_emails tool schema (Min: 0, Default: 0)
- `tools/mock_test.go`: Updated mock to match new signature
- `tools/handlers_test.go`: Added test for offset passing, added total field assertion

**Pagination logic in SearchEmails:**
```
UIDs: [1, 2, 3, 4, 5, 6, 7, 8, 9, 10]  (oldest→newest)
total = 10
offset=0, limit=3 → returns UIDs [8, 9, 10] (3 most recent)
offset=3, limit=3 → returns UIDs [5, 6, 7] (next 3)
offset=6, limit=3 → returns UIDs [2, 3, 4]
offset=9, limit=3 → returns UID  [1]
offset=10+        → returns []   (empty, total still = 10)
```

**Response format now includes `total`:**
```json
{
  "count": 3,
  "total": 150,
  "emails": [...],
  "folder": "INBOX"
}
```

### F27: Multi-Account Support - Deferred
Multi-account would require significant architectural changes:
- Multiple IMAP/SMTP client pairs
- Account routing in every handler
- Configuration for multiple accounts (env vars or config file)
- Account selection parameter on every tool

This is best done as a separate feature branch when there's a concrete use case.

## Verification
- `go build` ✓
- `go vet ./...` ✓
- `go test -race ./...` ✓ (all pass, including new offset test)
