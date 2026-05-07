package logger

import (
	"io"
	"os"
	"time"

	"github.com/rs/zerolog"
)

// Config конфигурация логгера
type Config struct {
	Level        string // debug, info, warn, error
	Format       string // console, json
	ReportCaller bool
	Output       io.Writer // optional, defaults to os.Stdout
}

// New создаёт настроенный экземпляр zerolog.Logger
func New(cfg Config) zerolog.Logger {
	// Парсим уровень логирования
	level := zerolog.InfoLevel
	if newLevel, err := zerolog.ParseLevel(cfg.Level); err == nil {
		level = newLevel
	}

	// Определяем output writer
	var output io.Writer = os.Stdout
	if cfg.Output != nil {
		output = cfg.Output
	}

	// Форматирование: console или json
	if cfg.Format != "json" {
		output = zerolog.ConsoleWriter{
			Out:        output,
			TimeFormat: time.RFC3339,
			NoColor:    false, // Цветной вывод для console
		}
	}

	// Создаём базовый logger
	logger := zerolog.New(output).Level(level).With().Timestamp()

	// Добавляем caller (имя файла и строка) если нужно
	if cfg.ReportCaller {
		logger = logger.Caller()
	}

	return logger.Logger()
}

// Default создаёт логгер с настройками по умолчанию для dev
func Default() zerolog.Logger {
	return New(Config{
		Level:        "info",
		Format:       "console",
		ReportCaller: true,
	})
}

// Production создаёт логгер с настройками для production (JSON, без caller)
func Production(level string) zerolog.Logger {
	return New(Config{
		Level:        level,
		Format:       "json",
		ReportCaller: false,
	})
}
