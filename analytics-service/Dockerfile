# Собирается с билд-контекстом = корень репозитория (не analytics-service/),
# потому что go.mod этого сервиса содержит "replace pet-ticket => ../" —
# Go-модулю нужен доступ к родительскому репозиторию (proto-типы, pkg/logger).
# См. docker-compose.yml: context: ., dockerfile: analytics-service/Dockerfile

# Build stage
FROM golang:1.24-alpine3.21 AS builder

RUN apk add --no-cache git

WORKDIR /app

# Сначала оба go.mod/go.sum (root + analytics-service), чтобы слой с
# зависимостями кешировался отдельно от исходников и не инвалидировался
# при каждом изменении кода.
COPY go.mod go.sum ./
COPY analytics-service/go.mod analytics-service/go.sum ./analytics-service/

WORKDIR /app/analytics-service
RUN go mod download

WORKDIR /app
COPY . .

WORKDIR /app/analytics-service
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH:-amd64} go build -ldflags="-w -s" -o analytics-server cmd/analytics-server/main.go

# Runtime stage
FROM alpine:3.21

RUN apk --no-cache add ca-certificates tzdata

RUN addgroup -g 1000 app && \
    adduser -D -u 1000 -G app app

WORKDIR /app

COPY --from=builder /app/analytics-service/analytics-server .

RUN chown -R app:app /app

USER app

EXPOSE 8081

CMD ["./analytics-server"]
