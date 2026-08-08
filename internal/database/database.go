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
	idColumn := "INTEGER PRIMARY KEY AUTOINCREMENT"
	if cfg.Driver == "pgsql" {
		idColumn = "BIGSERIAL PRIMARY KEY"
	}
	statements := []string{
		`CREATE TABLE IF NOT EXISTS repositories (name VARCHAR(80) PRIMARY KEY, created_at TIMESTAMP NOT NULL, updated_at TIMESTAMP NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS users (id ` + idColumn + `, username VARCHAR(40) UNIQUE NOT NULL, email VARCHAR(255) UNIQUE NOT NULL, password_hash TEXT NOT NULL, is_admin BOOLEAN NOT NULL DEFAULT FALSE, created_at TIMESTAMP NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS sessions (token_hash VARCHAR(64) PRIMARY KEY, user_id BIGINT NOT NULL, expires_at TIMESTAMP NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS settings (key VARCHAR(80) PRIMARY KEY, value TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS issues (id ` + idColumn + `, repository_name VARCHAR(80) NOT NULL, author_id BIGINT NOT NULL, title VARCHAR(255) NOT NULL, body TEXT NOT NULL, state VARCHAR(16) NOT NULL, created_at TIMESTAMP NOT NULL, updated_at TIMESTAMP NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS pull_requests (id ` + idColumn + `, repository_name VARCHAR(80) NOT NULL, author_id BIGINT NOT NULL, title VARCHAR(255) NOT NULL, body TEXT NOT NULL, source_branch VARCHAR(255) NOT NULL, target_branch VARCHAR(255) NOT NULL, state VARCHAR(16) NOT NULL, created_at TIMESTAMP NOT NULL, updated_at TIMESTAMP NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS releases (id ` + idColumn + `, repository_name VARCHAR(80) NOT NULL, author_id BIGINT NOT NULL, tag_name VARCHAR(255) NOT NULL, title VARCHAR(255) NOT NULL, notes TEXT NOT NULL, created_at TIMESTAMP NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS activities (id ` + idColumn + `, repository_name VARCHAR(80) NOT NULL, user_id BIGINT NOT NULL, kind VARCHAR(32) NOT NULL, created_at TIMESTAMP NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS organizations (id ` + idColumn + `, name VARCHAR(80) UNIQUE NOT NULL, created_at TIMESTAMP NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS organization_members (organization_id BIGINT NOT NULL, user_id BIGINT NOT NULL, role VARCHAR(16) NOT NULL, PRIMARY KEY (organization_id, user_id))`,
		`CREATE TABLE IF NOT EXISTS repository_settings (repository_name VARCHAR(161) PRIMARY KEY, description TEXT NOT NULL DEFAULT '', visibility VARCHAR(16) NOT NULL DEFAULT 'private', default_branch VARCHAR(255) NOT NULL DEFAULT 'main', topics TEXT NOT NULL DEFAULT '')`,
	}
	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			db.Close()
			return nil, fmt.Errorf("migrate database: %w", err)
		}
	}
	if _, err := db.Exec(`INSERT INTO settings (key, value) VALUES ('site_title', 'Kohame') ON CONFLICT (key) DO NOTHING`); err != nil {
		db.Close()
		return nil, fmt.Errorf("seed settings: %w", err)
	}
	if _, err := db.Exec(`INSERT INTO settings (key, value) VALUES ('site_description', 'Self-hosted Git, kept simple') ON CONFLICT (key) DO NOTHING`); err != nil {
		db.Close()
		return nil, fmt.Errorf("seed settings: %w", err)
	}
	if _, err := db.Exec(`INSERT INTO settings (key, value) VALUES ('allow_registration', 'true') ON CONFLICT (key) DO NOTHING`); err != nil {
		db.Close()
		return nil, fmt.Errorf("seed settings: %w", err)
	}
	return db, nil
}
