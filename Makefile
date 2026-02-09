VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-s -w -X main.version=$(VERSION)"
BINARY := mcp-icloud-email

.PHONY: build test cover lint clean run vet

build:
	CGO_ENABLED=0 go build $(LDFLAGS) -o $(BINARY) .

test:
	go test -race -coverprofile=coverage.out -covermode=atomic ./...
	@echo ""
	@go tool cover -func=coverage.out | grep ^total:

cover: test
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

vet:
	go vet ./...

lint:
	golangci-lint run ./...

clean:
	rm -f $(BINARY) coverage.out coverage.html

run: build
	./$(BINARY)

docker:
	docker build -t $(BINARY):$(VERSION) .

# Install dev tools
tools:
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install golang.org/x/vuln/cmd/govulncheck@latest

vuln:
	govulncheck ./...

all: vet lint test build
