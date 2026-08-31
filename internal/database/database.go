package database

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"kohame/internal/config"

	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func Open(cfg config.DatabaseConfig) (*gorm.DB, error) {
	if cfg.Driver != "pgsql" {
		if err := os.MkdirAll(filepath.Dir(cfg.DSN), 0o755); err != nil {
			return nil, fmt.Errorf("create database directory: %w", err)
		}
	}

	var dialector gorm.Dialector
	if cfg.Driver == "pgsql" {
		dialector = postgres.Open(cfg.DSN)
	} else {
		dialector = sqlite.Open(cfg.DSN)
	}
	db, err := gorm.Open(dialector, &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("open database connection: %w", err)
	}
	if err := sqlDB.Ping(); err != nil {
		_ = sqlDB.Close()
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
		`CREATE TABLE IF NOT EXISTS pull_request_comments (id ` + idColumn + `, repository_name VARCHAR(161) NOT NULL, pull_request_id BIGINT NOT NULL, author_id BIGINT NOT NULL, body TEXT NOT NULL, created_at TIMESTAMP NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS pull_request_reviews (id ` + idColumn + `, repository_name VARCHAR(161) NOT NULL, pull_request_id BIGINT NOT NULL, reviewer_id BIGINT NOT NULL, state VARCHAR(32) NOT NULL, body TEXT NOT NULL DEFAULT '', created_at TIMESTAMP NOT NULL, updated_at TIMESTAMP NOT NULL, UNIQUE(pull_request_id, reviewer_id))`,
		`CREATE TABLE IF NOT EXISTS releases (id ` + idColumn + `, repository_name VARCHAR(80) NOT NULL, author_id BIGINT NOT NULL, tag_name VARCHAR(255) NOT NULL, title VARCHAR(255) NOT NULL, notes TEXT NOT NULL, created_at TIMESTAMP NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS activities (id ` + idColumn + `, repository_name VARCHAR(80) NOT NULL, user_id BIGINT NOT NULL, kind VARCHAR(32) NOT NULL, created_at TIMESTAMP NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS organizations (id ` + idColumn + `, name VARCHAR(80) UNIQUE NOT NULL, created_at TIMESTAMP NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS organization_members (organization_id BIGINT NOT NULL, user_id BIGINT NOT NULL, role VARCHAR(16) NOT NULL, PRIMARY KEY (organization_id, user_id))`,
		`CREATE TABLE IF NOT EXISTS repository_settings (repository_name VARCHAR(161) PRIMARY KEY, description TEXT NOT NULL DEFAULT '', homepage_url VARCHAR(2048) NOT NULL DEFAULT '', visibility VARCHAR(16) NOT NULL DEFAULT 'private', default_branch VARCHAR(255) NOT NULL DEFAULT 'main', topics TEXT NOT NULL DEFAULT '', issues_enabled BOOLEAN NOT NULL DEFAULT TRUE, pulls_enabled BOOLEAN NOT NULL DEFAULT TRUE, releases_enabled BOOLEAN NOT NULL DEFAULT TRUE, wiki_enabled BOOLEAN NOT NULL DEFAULT FALSE, auto_close_issues BOOLEAN NOT NULL DEFAULT FALSE, allow_forks BOOLEAN NOT NULL DEFAULT TRUE, archived BOOLEAN NOT NULL DEFAULT FALSE)`,
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
		`CREATE TABLE IF NOT EXISTS repository_collaborators (repository_name VARCHAR(161) NOT NULL, user_id BIGINT NOT NULL, permission VARCHAR(16) NOT NULL, PRIMARY KEY(repository_name, user_id))`,
		`CREATE TABLE IF NOT EXISTS protected_branches (repository_name VARCHAR(161) NOT NULL, branch VARCHAR(255) NOT NULL, require_pull_request BOOLEAN NOT NULL DEFAULT TRUE, require_approvals INTEGER NOT NULL DEFAULT 1, PRIMARY KEY(repository_name, branch))`,
		`CREATE TABLE IF NOT EXISTS wiki_pages (repository_name VARCHAR(161) NOT NULL, slug VARCHAR(80) NOT NULL, title VARCHAR(160) NOT NULL, content TEXT NOT NULL, author VARCHAR(80) NOT NULL, created_at TIMESTAMP NOT NULL, updated_at TIMESTAMP NOT NULL, PRIMARY KEY(repository_name, slug))`,
		`CREATE TABLE IF NOT EXISTS wiki_page_revisions (id ` + idColumn + `, repository_name VARCHAR(161) NOT NULL, slug VARCHAR(80) NOT NULL, title VARCHAR(160) NOT NULL, content TEXT NOT NULL, author VARCHAR(80) NOT NULL, edited_at TIMESTAMP NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS oidc_providers (id ` + idColumn + `, slug VARCHAR(80) UNIQUE NOT NULL, name VARCHAR(120) NOT NULL, issuer_url VARCHAR(2048) NOT NULL, client_id VARCHAR(255) NOT NULL, client_secret TEXT NOT NULL, scopes VARCHAR(1000) NOT NULL DEFAULT 'openid profile email', enabled BOOLEAN NOT NULL DEFAULT TRUE, created_at TIMESTAMP NOT NULL, updated_at TIMESTAMP NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS oidc_identities (provider_id BIGINT NOT NULL, subject VARCHAR(255) NOT NULL, user_id BIGINT NOT NULL, email VARCHAR(255) NOT NULL DEFAULT '', created_at TIMESTAMP NOT NULL, PRIMARY KEY (provider_id, subject))`,
		`CREATE TABLE IF NOT EXISTS oidc_states (state VARCHAR(128) PRIMARY KEY, provider_id BIGINT NOT NULL, redirect_path VARCHAR(2048) NOT NULL DEFAULT '/', expires_at TIMESTAMP NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS workflows (id ` + idColumn + `, repository_name VARCHAR(161) NOT NULL, name VARCHAR(120) NOT NULL, config TEXT NOT NULL, enabled BOOLEAN NOT NULL DEFAULT TRUE, created_at TIMESTAMP NOT NULL, updated_at TIMESTAMP NOT NULL, UNIQUE(repository_name, name))`,
		`CREATE TABLE IF NOT EXISTS workflow_runs (id ` + idColumn + `, workflow_id BIGINT NOT NULL, repository_name VARCHAR(161) NOT NULL, event VARCHAR(80) NOT NULL, status VARCHAR(16) NOT NULL, output TEXT NOT NULL DEFAULT '', started_at TIMESTAMP NOT NULL, finished_at TIMESTAMP)`,
	}
	for _, statement := range statements {
		if err := db.Exec(statement).Error; err != nil {
			_ = sqlDB.Close()
			return nil, fmt.Errorf("migrate database: %w", err)
		}
	}
	// A compact forward migration for installations created before email verification.
	// Both databases report a harmless duplicate-column error when it already exists.
	if err := db.Exec(`ALTER TABLE users ADD COLUMN email_verified BOOLEAN NOT NULL DEFAULT FALSE`).Error; err != nil && !isDuplicateColumn(err) {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("migrate users: %w", err)
	}
	if err := db.Exec(`ALTER TABLE user_settings ADD COLUMN avatar_url VARCHAR(255) NOT NULL DEFAULT ''`).Error; err != nil && !isDuplicateColumn(err) {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("migrate user settings: %w", err)
	}
	for _, statement := range []string{
		`ALTER TABLE repository_settings ADD COLUMN homepage_url VARCHAR(2048) NOT NULL DEFAULT ''`,
		`ALTER TABLE repository_settings ADD COLUMN issues_enabled BOOLEAN NOT NULL DEFAULT TRUE`,
		`ALTER TABLE repository_settings ADD COLUMN pulls_enabled BOOLEAN NOT NULL DEFAULT TRUE`,
		`ALTER TABLE repository_settings ADD COLUMN releases_enabled BOOLEAN NOT NULL DEFAULT TRUE`,
		`ALTER TABLE repository_settings ADD COLUMN wiki_enabled BOOLEAN NOT NULL DEFAULT FALSE`,
		`ALTER TABLE repository_settings ADD COLUMN auto_close_issues BOOLEAN NOT NULL DEFAULT FALSE`,
		`ALTER TABLE repository_settings ADD COLUMN allow_forks BOOLEAN NOT NULL DEFAULT TRUE`,
		`ALTER TABLE repository_settings ADD COLUMN archived BOOLEAN NOT NULL DEFAULT FALSE`,
	} {
		if err := db.Exec(statement).Error; err != nil && !isDuplicateColumn(err) {
			_ = sqlDB.Close()
			return nil, fmt.Errorf("migrate repository settings: %w", err)
		}
	}
	if err := db.Exec(`INSERT INTO settings (key, value) VALUES ('site_title', 'Kohame') ON CONFLICT (key) DO NOTHING`).Error; err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("seed settings: %w", err)
	}
	if err := db.Exec(`INSERT INTO settings (key, value) VALUES ('site_description', '简洁自托管的 Git 代码协作平台') ON CONFLICT (key) DO NOTHING`).Error; err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("seed settings: %w", err)
	}
	if err := db.Exec(`INSERT INTO settings (key, value) VALUES ('allow_registration', 'true') ON CONFLICT (key) DO NOTHING`).Error; err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("seed settings: %w", err)
	}
	for key, value := range map[string]string{"repository_root": "data/repos", "workflow_directory": "/.kohame/workflow", "runner_enabled": "false", "runner_url": "", "runner_token": "", "captcha_enabled": "false", "captcha_site_key": "", "captcha_secret": "", "gravatar_mirror": "https://www.gravatar.com/avatar/"} {
		seed := `INSERT INTO settings (key, value) VALUES (?, ?) ON CONFLICT (key) DO NOTHING`
		if cfg.Driver == "pgsql" {
			seed = `INSERT INTO settings (key, value) VALUES ($1, $2) ON CONFLICT (key) DO NOTHING`
		}
		if err := db.Exec(seed, key, value).Error; err != nil {
			_ = sqlDB.Close()
			return nil, fmt.Errorf("seed settings: %w", err)
		}
	}
	return db, nil
}

// SQLDB exposes the connection for repositories that retain parameterized raw SQL.
func SQLDB(db *gorm.DB) (*sql.DB, error) { return db.DB() }

func isDuplicateColumn(err error) bool {
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "duplicate column") || strings.Contains(message, "already exists")
}
