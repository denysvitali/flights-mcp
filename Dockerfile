# Build stage
FROM golang:1.25-alpine AS builder

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary (airports database is embedded)
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /flights-mcp ./cmd/flights-mcp

# Runtime stage — the default HTTP scraper needs no browser,
# so a bare Alpine image with CA certificates is enough.
FROM alpine:3.20

RUN apk add --no-cache ca-certificates

# Copy binary from builder
COPY --from=builder /flights-mcp /usr/local/bin/flights-mcp

ENV LOG_LEVEL=info

# Create non-root user for security
RUN adduser -D -g '' appuser
USER appuser

# Entry point
ENTRYPOINT ["flights-mcp"]
CMD ["run"]
