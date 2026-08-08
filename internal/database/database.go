package database

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	"kohame/internal/config"

	_ "github.com/jackc/pgx/v5/stdlib"
	_ "modernc.org/sqlite"
)

func Open(cfg config.DatabaseConfig) (*sql.DB, error) {
	driver := cfg.Driver
	if driver == "pgsql" {
		driver = "pgx"
	} else if err := os.MkdirAll(filepath.Dir(cfg.DSN), 0o755); err != nil {
		return nil, fmt.Errorf("create database directory: %w", err)
	}

	db, err := sql.Open(driver, cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("connect to database: %w", err)
	}
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS repositories (
		name VARCHAR(80) PRIMARY KEY,
		created_at TIMESTAMP NOT NULL,
		updated_at TIMESTAMP NOT NULL
	)`); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate repositories table: %w", err)
	}
	return db, nil
}
