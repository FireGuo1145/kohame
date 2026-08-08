package database

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

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
		`CREATE TABLE IF NOT EXISTS users (id ` + idColumn + `, username VARCHAR(40) UNIQUE NOT NULL, email VARCHAR(255) UNIQUE NOT NULL, password_hash TEXT NOT NULL, is_admin BOOLEAN NOT NULL DEFAULT FALSE, email_verified BOOLEAN NOT NULL DEFAULT FALSE, created_at TIMESTAMP NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS sessions (token_hash VARCHAR(64) PRIMARY KEY, user_id BIGINT NOT NULL, expires_at TIMESTAMP NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS settings (key VARCHAR(80) PRIMARY KEY, value TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS issues (id ` + idColumn + `, repository_name VARCHAR(80) NOT NULL, author_id BIGINT NOT NULL, title VARCHAR(255) NOT NULL, body TEXT NOT NULL, state VARCHAR(16) NOT NULL, created_at TIMESTAMP NOT NULL, updated_at TIMESTAMP NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS issue_comments (id ` + idColumn + `, repository_name VARCHAR(80) NOT NULL, issue_id BIGINT NOT NULL, author_id BIGINT NOT NULL, body TEXT NOT NULL, created_at TIMESTAMP NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS pull_requests (id ` + idColumn + `, repository_name VARCHAR(80) NOT NULL, author_id BIGINT NOT NULL, title VARCHAR(255) NOT NULL, body TEXT NOT NULL, source_branch VARCHAR(255) NOT NULL, target_branch VARCHAR(255) NOT NULL, state VARCHAR(16) NOT NULL, created_at TIMESTAMP NOT NULL, updated_at TIMESTAMP NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS releases (id ` + idColumn + `, repository_name VARCHAR(80) NOT NULL, author_id BIGINT NOT NULL, tag_name VARCHAR(255) NOT NULL, title VARCHAR(255) NOT NULL, notes TEXT NOT NULL, created_at TIMESTAMP NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS activities (id ` + idColumn + `, repository_name VARCHAR(80) NOT NULL, user_id BIGINT NOT NULL, kind VARCHAR(32) NOT NULL, created_at TIMESTAMP NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS organizations (id ` + idColumn + `, name VARCHAR(80) UNIQUE NOT NULL, created_at TIMESTAMP NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS organization_members (organization_id BIGINT NOT NULL, user_id BIGINT NOT NULL, role VARCHAR(16) NOT NULL, PRIMARY KEY (organization_id, user_id))`,
		`CREATE TABLE IF NOT EXISTS repository_settings (repository_name VARCHAR(161) PRIMARY KEY, description TEXT NOT NULL DEFAULT '', visibility VARCHAR(16) NOT NULL DEFAULT 'private', default_branch VARCHAR(255) NOT NULL DEFAULT 'main', topics TEXT NOT NULL DEFAULT '')`,
		`CREATE TABLE IF NOT EXISTS repository_forks (repository_name VARCHAR(161) PRIMARY KEY, parent_name VARCHAR(161) NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS repository_stars (repository_name VARCHAR(161) NOT NULL, user_id BIGINT NOT NULL, created_at TIMESTAMP NOT NULL, PRIMARY KEY (repository_name, user_id))`,
		`CREATE TABLE IF NOT EXISTS email_verifications (token_hash VARCHAR(64) PRIMARY KEY, user_id BIGINT NOT NULL, expires_at TIMESTAMP NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS notifications (id ` + idColumn + `, user_id BIGINT NOT NULL, kind VARCHAR(32) NOT NULL, title VARCHAR(255) NOT NULL, body TEXT NOT NULL, link VARCHAR(255) NOT NULL, is_read BOOLEAN NOT NULL DEFAULT FALSE, created_at TIMESTAMP NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS user_settings (user_id BIGINT PRIMARY KEY, display_name VARCHAR(80) NOT NULL DEFAULT '', bio TEXT NOT NULL DEFAULT '', location VARCHAR(120) NOT NULL DEFAULT '', website VARCHAR(255) NOT NULL DEFAULT '', avatar_url VARCHAR(255) NOT NULL DEFAULT '')`,
		`CREATE TABLE IF NOT EXISTS user_follows (follower_id BIGINT NOT NULL, target_user_id BIGINT NOT NULL, created_at TIMESTAMP NOT NULL, PRIMARY KEY (follower_id, target_user_id))`,
		`CREATE TABLE IF NOT EXISTS organization_follows (follower_id BIGINT NOT NULL, organization_id BIGINT NOT NULL, created_at TIMESTAMP NOT NULL, PRIMARY KEY (follower_id, organization_id))`,
		`CREATE TABLE IF NOT EXISTS ssh_keys (id ` + idColumn + `, user_id BIGINT NOT NULL, title VARCHAR(120) NOT NULL, key_value TEXT NOT NULL UNIQUE, created_at TIMESTAMP NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS release_assets (id ` + idColumn + `, release_id BIGINT NOT NULL, file_name VARCHAR(255) NOT NULL, storage_name VARCHAR(255) NOT NULL, size BIGINT NOT NULL, created_at TIMESTAMP NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS labels (id ` + idColumn + `, repository_name VARCHAR(161) NOT NULL, name VARCHAR(80) NOT NULL, color VARCHAR(7) NOT NULL, description VARCHAR(255) NOT NULL DEFAULT '', UNIQUE(repository_name, name))`,
		`CREATE TABLE IF NOT EXISTS issue_labels (issue_id BIGINT NOT NULL, label_id BIGINT NOT NULL, PRIMARY KEY(issue_id, label_id))`,
	}
	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			db.Close()
			return nil, fmt.Errorf("migrate database: %w", err)
		}
	}
	// A compact forward migration for installations created before email verification.
	// Both databases report a harmless duplicate-column error when it already exists.
	if _, err := db.Exec(`ALTER TABLE users ADD COLUMN email_verified BOOLEAN NOT NULL DEFAULT FALSE`); err != nil && !isDuplicateColumn(err) {
		db.Close()
		return nil, fmt.Errorf("migrate users: %w", err)
	}
	if _, err := db.Exec(`ALTER TABLE user_settings ADD COLUMN avatar_url VARCHAR(255) NOT NULL DEFAULT ''`); err != nil && !isDuplicateColumn(err) {
		db.Close()
		return nil, fmt.Errorf("migrate user settings: %w", err)
	}
	if _, err := db.Exec(`INSERT INTO settings (key, value) VALUES ('site_title', 'Kohame') ON CONFLICT (key) DO NOTHING`); err != nil {
		db.Close()
		return nil, fmt.Errorf("seed settings: %w", err)
	}
	if _, err := db.Exec(`INSERT INTO settings (key, value) VALUES ('site_description', '简洁自托管的 Git 代码协作平台') ON CONFLICT (key) DO NOTHING`); err != nil {
		db.Close()
		return nil, fmt.Errorf("seed settings: %w", err)
	}
	if _, err := db.Exec(`INSERT INTO settings (key, value) VALUES ('allow_registration', 'true') ON CONFLICT (key) DO NOTHING`); err != nil {
		db.Close()
		return nil, fmt.Errorf("seed settings: %w", err)
	}
	for key, value := range map[string]string{"repository_root": "data/repos", "captcha_enabled": "false", "captcha_site_key": "", "captcha_secret": "", "gravatar_mirror": "https://www.gravatar.com/avatar/"} {
		seed := `INSERT INTO settings (key, value) VALUES (?, ?) ON CONFLICT (key) DO NOTHING`
		if cfg.Driver == "pgsql" {
			seed = `INSERT INTO settings (key, value) VALUES ($1, $2) ON CONFLICT (key) DO NOTHING`
		}
		if _, err := db.Exec(seed, key, value); err != nil {
			db.Close()
			return nil, fmt.Errorf("seed settings: %w", err)
		}
	}
	return db, nil
}

func isDuplicateColumn(err error) bool {
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "duplicate column") || strings.Contains(message, "already exists")
}
