package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

var ErrInvalidName = errors.New("repository name must use lowercase letters, numbers, dots, hyphens, or underscores")
var ErrExists = errors.New("repository already exists")
var ErrNotFound = errors.New("repository not found")

type Repository struct {
	Name      string    `json:"name"`
	UpdatedAt time.Time `json:"updatedAt"`
	Path      string    `json:"-"`
}

type Store struct {
	root   string
	db     *sql.DB
	driver string
}

func NewStore(root string, db *sql.DB, driver string) *Store {
	return &Store{root: root, db: db, driver: driver}
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) List() ([]Repository, error) {
	rows, err := s.db.Query(`SELECT name, updated_at FROM repositories ORDER BY updated_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	repos := make([]Repository, 0)
	for rows.Next() {
		var repo Repository
		if err := rows.Scan(&repo.Name, &repo.UpdatedAt); err != nil {
			return nil, err
		}
		repo.Path = filepath.Join(s.root, repo.Name+".git")
		repos = append(repos, repo)
	}
	return repos, rows.Err()
}

func (s *Store) Create(ctx context.Context, name string) (Repository, error) {
	if !validName(name) {
		return Repository{}, ErrInvalidName
	}
	if _, err := s.Get(name); err == nil {
		return Repository{}, ErrExists
	} else if !errors.Is(err, ErrNotFound) {
		return Repository{}, err
	}

	path := filepath.Join(s.root, name+".git")
	if _, err := os.Stat(path); err == nil {
		return Repository{}, ErrExists
	} else if !os.IsNotExist(err) {
		return Repository{}, err
	}
	if output, err := exec.CommandContext(ctx, "git", "init", "--bare", path).CombinedOutput(); err != nil {
		return Repository{}, fmt.Errorf("initialize repository: %w: %s", err, strings.TrimSpace(string(output)))
	}
	if output, err := exec.CommandContext(ctx, "git", "-C", path, "config", "http.receivepack", "true").CombinedOutput(); err != nil {
		return Repository{}, fmt.Errorf("configure repository: %w: %s", err, strings.TrimSpace(string(output)))
	}

	now := time.Now().UTC()
	if _, err := s.db.ExecContext(ctx, `INSERT INTO repositories (name, created_at, updated_at) VALUES (`+s.placeholders(1, 2, 3)+`)`, name, now, now); err != nil {
		if isUniqueViolation(err) {
			return Repository{}, ErrExists
		}
		return Repository{}, fmt.Errorf("store repository metadata: %w", err)
	}
	return Repository{Name: name, UpdatedAt: now, Path: path}, nil
}

func (s *Store) Get(name string) (Repository, error) {
	if !validName(name) {
		return Repository{}, ErrNotFound
	}
	var repo Repository
	err := s.db.QueryRow(`SELECT name, updated_at FROM repositories WHERE name = `+s.placeholders(1), name).Scan(&repo.Name, &repo.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Repository{}, ErrNotFound
	}
	if err != nil {
		return Repository{}, err
	}
	repo.Path = filepath.Join(s.root, repo.Name+".git")
	if info, err := os.Stat(repo.Path); err != nil || !info.IsDir() {
		return Repository{}, ErrNotFound
	}
	return repo, nil
}

func (s *Store) placeholders(numbers ...int) string {
	items := make([]string, len(numbers))
	for i, number := range numbers {
		if s.driver == "pgsql" {
			items[i] = fmt.Sprintf("$%d", number)
		} else {
			items[i] = "?"
		}
	}
	return strings.Join(items, ", ")
}

func isUniqueViolation(err error) bool {
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "unique") || strings.Contains(message, "duplicate key")
}

func validName(name string) bool {
	if name == "" || len(name) > 80 || strings.Contains(name, "..") {
		return false
	}
	for _, r := range name {
		if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-' || r == '_' || r == '.') {
			return false
		}
	}
	return true
}
