module analytics-service

go 1.24.0

require (
	github.com/gofiber/fiber/v2 v2.52.6
	github.com/joho/godotenv v1.5.1
	github.com/kelseyhightower/envconfig v1.4.0
	github.com/redis/go-redis/v9 v9.7.0
	github.com/rs/zerolog v1.33.0
	google.golang.org/grpc v1.79.1
	google.golang.org/protobuf v1.36.11
	pet-ticket v0.0.0-00010101000000-000000000000
)

require (
	github.com/andybalholm/brotli v1.2.0 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/klauspost/compress v1.18.0 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/mattn/go-runewidth v0.0.16 // indirect
	github.com/rivo/uniseg v0.2.0 // indirect
	github.com/valyala/bytebufferpool v1.0.0 // indirect
	github.com/valyala/fasthttp v1.51.0 // indirect
	github.com/valyala/tcplisten v1.0.0 // indirect
	golang.org/x/net v0.48.0 // indirect
	golang.org/x/sys v0.39.0 // indirect
	golang.org/x/text v0.34.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260209200024-4cfbd4190f57 // indirect
)

// analytics-service не тянет pet-ticket из внешнего репозитория — ему нужны
// только сгенерированные gRPC-типы (api/gen/go/ticket/v1) и общий logger
// (pkg/logger) из соседнего модуля. Локальный replace на "../" превращает
// весь корень репозитория в зависимость этого модуля без публикации отдельного
// пакета и без дублирования proto-типов копипастой.
replace pet-ticket => ../
