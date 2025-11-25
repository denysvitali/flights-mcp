# Build stage
FROM golang:1.22-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /flights-mcp ./cmd/flights-mcp

# Runtime stage
FROM alpine:3.19

# Install Chrome/Chromium and dependencies for chromedp
RUN apk add --no-cache \
    chromium \
    chromium-chromedriver \
    nss \
    freetype \
    harfbuzz \
    ca-certificates \
    ttf-freefont \
    font-noto-emoji \
    && rm -rf /var/cache/apk/*

# Set Chrome path for chromedp
ENV CHROME_PATH=/usr/bin/chromium-browser
ENV HEADLESS_MODE=true

# Copy binary from builder
COPY --from=builder /flights-mcp /usr/local/bin/flights-mcp

# Copy airports database
COPY --from=builder /app/internal/airports/airports.json /app/internal/airports/airports.json

# Set working directory
WORKDIR /app

# Set environment variables
ENV AIRPORTS_FILE=/app/internal/airports/airports.json
ENV LOG_LEVEL=info

# Create non-root user for security
RUN adduser -D -g '' appuser
USER appuser

# Entry point
ENTRYPOINT ["flights-mcp"]
CMD ["run"]
