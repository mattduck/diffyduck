# Sample Dockerfile for syntax highlighting
FROM golang:1.21-alpine AS builder

# Build arguments
ARG VERSION=1.0.0
ARG BUILD_DATE

# Labels
LABEL maintainer="developer@example.com"
LABEL version="${VERSION}"

# Environment variables
ENV GO111MODULE=on \
    CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64

# Set working directory
WORKDIR /app

# Copy dependency files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN go build -ldflags="-s -w -X main.version=${VERSION}" \
    -o /app/server ./cmd/server

# Production stage
FROM alpine:3.18

# Install certificates and timezone data
RUN apk --no-cache add ca-certificates tzdata

# Create non-root user
RUN adduser -D -g '' appuser

# Copy binary from builder
COPY --from=builder /app/server /usr/local/bin/server

# Switch to non-root user
USER appuser

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --quiet --tries=1 --spider http://localhost:8080/health || exit 1

# Set entrypoint
ENTRYPOINT ["server"]

# Default command
CMD ["--port", "8080"]
