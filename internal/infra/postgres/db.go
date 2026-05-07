package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"pet-ticket/internal/app/tickets"

	_ "github.com/lib/pq"
)

// DB представляет подключение к PostgreSQL
type DB struct {
	conn *sql.DB
}

// Options содержит параметры подключения к БД
type Options struct {
	MaxOpenConn     int
	MaxIdleConn     int
	ConnMaxLifetime time.Duration
}

// New создаёт новое подключение к PostgreSQL
func New(dsn string, opts Options) (*DB, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	if err := db.PingContext(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	db.SetMaxOpenConns(opts.MaxOpenConn)
	db.SetMaxIdleConns(opts.MaxIdleConn)
	db.SetConnMaxLifetime(opts.ConnMaxLifetime)

	return &DB{conn: db}, nil
}

// Close закрывает соединение с БД
func (db *DB) Close() error {
	return db.conn.Close()
}

// Conn возвращает *sql.DB для использования в миграциях
func (db *DB) Conn() *sql.DB {
	return db.conn
}

// BeginTx начинает транзакцию и возвращает интерфейс TxCommitter
func (db *DB) BeginTx(ctx context.Context) (tickets.TxCommitter, error) {
	tx, err := db.conn.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	return &TxAdapter{tx: tx}, nil
}

// TxAdapter адаптер для *sql.Tx, реализующий интерфейс TxCommitter
type TxAdapter struct {
	tx *sql.Tx
}

// Commit коммитит транзакцию
func (t *TxAdapter) Commit() error {
	return t.tx.Commit()
}

// Rollback откатывает транзакцию
func (t *TxAdapter) Rollback() error {
	return t.tx.Rollback()
}
