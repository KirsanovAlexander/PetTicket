# Build stage
FROM golang:1.25-alpine3.21 AS builder

# Install build dependencies
RUN apk add --no-cache git

WORKDIR /app

# Copy go mod files and download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build binary for target architecture (auto-detected or passed via buildx)
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH:-amd64} go build -ldflags="-w -s" -o pet-ticket cmd/api-server/main.go

# Runtime stage
FROM alpine:3.21

# Install runtime dependencies
RUN apk --no-cache add ca-certificates tzdata

# Create non-root user
RUN addgroup -g 1000 app && \
    adduser -D -u 1000 -G app app

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/pet-ticket .
COPY --from=builder /app/migrations ./migrations

# Change ownership to app user
RUN chown -R app:app /app

# Switch to non-root user
USER app

# Expose port
EXPOSE 9000

# Run
CMD ["./pet-ticket"]

