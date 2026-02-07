FROM golang:1.25-alpine AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .

ARG VERSION=dev
RUN CGO_ENABLED=0 go build -ldflags "-s -w -X main.version=${VERSION}" -o /mcp-icloud-email .

FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=builder /mcp-icloud-email /mcp-icloud-email

USER nonroot:nonroot
ENTRYPOINT ["/mcp-icloud-email"]
