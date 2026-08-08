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
	"unicode/utf8"
)

var ErrInvalidName = errors.New("scope and repository name must use lowercase letters, numbers, dots, hyphens, or underscores")
var ErrExists = errors.New("repository already exists")
var ErrNotFound = errors.New("repository not found")

type Repository struct {
	Scope      string    `json:"scope"`
	Name       string    `json:"name"`
	FullName   string    `json:"fullName"`
	UpdatedAt  time.Time `json:"updatedAt"`
	ForkedFrom string    `json:"forkedFrom,omitempty"`
	Stars      int       `json:"stars"`
	Forks      int       `json:"forks"`
	Path       string    `json:"-"`
}

type TreeEntry struct {
	Name string `json:"name"`
	Path string `json:"path"`
	Type string `json:"type"`
}
type Blob struct {
	Path    string `json:"path"`
	Content string `json:"content"`
	IsText  bool   `json:"isText"`
}
type Settings struct {
	Description   string   `json:"description"`
	Visibility    string   `json:"visibility"`
	DefaultBranch string   `json:"defaultBranch"`
	Topics        []string `json:"topics"`
}
type Ref struct {
	Name string `json:"name"`
	Hash string `json:"hash"`
}
type Commit struct {
	Hash    string `json:"hash"`
	Subject string `json:"subject"`
	Author  string `json:"author"`
	Date    string `json:"date"`
}
type CommitDetail struct {
	Commit
	Body    string `json:"body"`
	Changes string `json:"changes"`
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
	rows, err := s.db.Query(`SELECT r.name,r.updated_at,COALESCE(f.parent_name,''),COUNT(DISTINCT st.user_id),COUNT(DISTINCT children.repository_name) FROM repositories r LEFT JOIN repository_forks f ON f.repository_name=r.name LEFT JOIN repository_stars st ON st.repository_name=r.name LEFT JOIN repository_forks children ON children.parent_name=r.name GROUP BY r.name,r.updated_at,f.parent_name ORDER BY r.updated_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	repos := make([]Repository, 0)
	for rows.Next() {
		var fullName string
		var updatedAt time.Time
		var forkedFrom string
		var stars, forks int
		if err := rows.Scan(&fullName, &updatedAt, &forkedFrom, &stars, &forks); err != nil {
			return nil, err
		}
		repo, err := s.fromFullName(fullName, updatedAt)
		if err == nil {
			repo.ForkedFrom, repo.Stars, repo.Forks = forkedFrom, stars, forks
			repos = append(repos, repo)
		}
	}
	return repos, rows.Err()
}

func (s *Store) ListByScope(scope string) ([]Repository, error) {
	items, err := s.List()
	if err != nil {
		return nil, err
	}
	out := make([]Repository, 0)
	for _, item := range items {
		if item.Scope == scope {
			out = append(out, item)
		}
	}
	return out, nil
}

func (s *Store) Search(query string) ([]Repository, error) {
	items, err := s.List()
	if err != nil {
		return nil, err
	}
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return []Repository{}, nil
	}
	out := make([]Repository, 0)
	for _, item := range items {
		if strings.Contains(strings.ToLower(item.FullName), query) {
			out = append(out, item)
		}
	}
	return out, nil
}

func (s *Store) Create(ctx context.Context, scope, name string) (Repository, error) {
	if !validPart(scope) || !validPart(name) {
		return Repository{}, ErrInvalidName
	}
	fullName := scope + "/" + name
	if _, err := s.Get(scope, name); err == nil {
		return Repository{}, ErrExists
	} else if !errors.Is(err, ErrNotFound) {
		return Repository{}, err
	}
	path := filepath.Join(s.root, scope, name+".git")
	if _, err := os.Stat(path); err == nil {
		return Repository{}, ErrExists
	} else if !os.IsNotExist(err) {
		return Repository{}, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return Repository{}, err
	}
	if output, err := exec.CommandContext(ctx, "git", "init", "--bare", path).CombinedOutput(); err != nil {
		return Repository{}, fmt.Errorf("initialize repository: %w: %s", err, strings.TrimSpace(string(output)))
	}
	if output, err := exec.CommandContext(ctx, "git", "-C", path, "config", "http.receivepack", "true").CombinedOutput(); err != nil {
		return Repository{}, fmt.Errorf("configure repository: %w: %s", err, strings.TrimSpace(string(output)))
	}
	now := time.Now().UTC()
	if _, err := s.db.ExecContext(ctx, `INSERT INTO repositories (name, created_at, updated_at) VALUES (`+s.placeholders(1, 2, 3)+`)`, fullName, now, now); err != nil {
		if isUniqueViolation(err) {
			return Repository{}, ErrExists
		}
		return Repository{}, fmt.Errorf("store repository metadata: %w", err)
	}
	_, _ = s.db.ExecContext(ctx, `INSERT INTO repository_settings (repository_name,description,visibility,default_branch,topics) VALUES (`+s.placeholders(1, 2, 3, 4, 5)+`)`, fullName, "", "private", "main", "")
	return Repository{Scope: scope, Name: name, FullName: fullName, UpdatedAt: now, Path: path}, nil
}

func (s *Store) Fork(ctx context.Context, source Repository, scope, name string) (Repository, error) {
	if !validPart(scope) || !validPart(name) {
		return Repository{}, ErrInvalidName
	}
	fullName := scope + "/" + name
	if _, err := s.Get(scope, name); err == nil {
		return Repository{}, ErrExists
	} else if !errors.Is(err, ErrNotFound) {
		return Repository{}, err
	}
	target := filepath.Join(s.root, scope, name+".git")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return Repository{}, err
	}
	if output, err := exec.CommandContext(ctx, "git", "clone", "--bare", source.Path, target).CombinedOutput(); err != nil {
		return Repository{}, fmt.Errorf("fork repository: %w: %s", err, strings.TrimSpace(string(output)))
	}
	now := time.Now().UTC()
	if _, err := s.db.ExecContext(ctx, `INSERT INTO repositories (name,created_at,updated_at) VALUES (`+s.placeholders(1, 2, 3)+`)`, fullName, now, now); err != nil {
		return Repository{}, err
	}
	if _, err := s.db.ExecContext(ctx, `INSERT INTO repository_forks (repository_name,parent_name) VALUES (`+s.placeholders(1, 2)+`)`, fullName, source.FullName); err != nil {
		return Repository{}, err
	}
	settings, _ := s.Settings(ctx, source.FullName)
	_, _ = s.db.ExecContext(ctx, `INSERT INTO repository_settings (repository_name,description,visibility,default_branch,topics) VALUES (`+s.placeholders(1, 2, 3, 4, 5)+`)`, fullName, settings.Description, settings.Visibility, settings.DefaultBranch, strings.Join(settings.Topics, ","))
	return Repository{Scope: scope, Name: name, FullName: fullName, UpdatedAt: now, Path: target, ForkedFrom: source.FullName}, nil
}

func (s *Store) ToggleStar(ctx context.Context, repositoryName string, userID int64) (bool, int, error) {
	var exists int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM repository_stars WHERE repository_name=`+s.placeholders(1)+` AND user_id=`+s.placeholders(2), repositoryName, userID).Scan(&exists)
	if err != nil {
		return false, 0, err
	}
	starred := exists == 0
	if starred {
		_, err = s.db.ExecContext(ctx, `INSERT INTO repository_stars (repository_name,user_id,created_at) VALUES (`+s.placeholders(1, 2, 3)+`)`, repositoryName, userID, time.Now().UTC())
	} else {
		_, err = s.db.ExecContext(ctx, `DELETE FROM repository_stars WHERE repository_name=`+s.placeholders(1)+` AND user_id=`+s.placeholders(2), repositoryName, userID)
	}
	if err != nil {
		return false, 0, err
	}
	var count int
	err = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM repository_stars WHERE repository_name=`+s.placeholders(1), repositoryName).Scan(&count)
	return starred, count, err
}

func (s *Store) Starred(ctx context.Context, repositoryName string, userID int64) (bool, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM repository_stars WHERE repository_name=`+s.placeholders(1)+` AND user_id=`+s.placeholders(2), repositoryName, userID).Scan(&count)
	return count > 0, err
}

func (s *Store) Settings(ctx context.Context, fullName string) (Settings, error) {
	var value Settings
	var topics string
	err := s.db.QueryRowContext(ctx, `SELECT description,visibility,default_branch,topics FROM repository_settings WHERE repository_name=`+s.placeholders(1), fullName).Scan(&value.Description, &value.Visibility, &value.DefaultBranch, &topics)
	if errors.Is(err, sql.ErrNoRows) {
		return Settings{Visibility: "private", DefaultBranch: "main", Topics: []string{}}, nil
	}
	if err != nil {
		return value, err
	}
	value.Topics = splitTopics(topics)
	return value, nil
}
func (s *Store) UpdateSettings(ctx context.Context, fullName string, value Settings) error {
	if value.Visibility != "private" && value.Visibility != "public" {
		return errors.New("visibility must be private or public")
	}
	if !safeRef(value.DefaultBranch) {
		return errors.New("invalid default branch")
	}
	topics := splitTopics(strings.Join(value.Topics, ","))
	_, err := s.db.ExecContext(ctx, `INSERT INTO repository_settings (repository_name,description,visibility,default_branch,topics) VALUES (`+s.placeholders(1, 2, 3, 4, 5)+`) ON CONFLICT (repository_name) DO UPDATE SET description=EXCLUDED.description,visibility=EXCLUDED.visibility,default_branch=EXCLUDED.default_branch,topics=EXCLUDED.topics`, fullName, strings.TrimSpace(value.Description), value.Visibility, value.DefaultBranch, strings.Join(topics, ","))
	return err
}
func (s *Store) Branches(ctx context.Context, repo Repository) ([]Ref, error) {
	return s.refs(ctx, repo, "refs/heads")
}
func (s *Store) Tags(ctx context.Context, repo Repository) ([]Ref, error) {
	return s.refs(ctx, repo, "refs/tags")
}
func (s *Store) refs(ctx context.Context, repo Repository, prefix string) ([]Ref, error) {
	out, err := exec.CommandContext(ctx, "git", "-C", repo.Path, "for-each-ref", "--format=%(refname:short)|%(objectname:short)", prefix).Output()
	if err != nil {
		return nil, err
	}
	refs := []Ref{}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		p := strings.SplitN(line, "|", 2)
		if len(p) == 2 {
			refs = append(refs, Ref{Name: p[0], Hash: p[1]})
		}
	}
	return refs, nil
}
func (s *Store) Commits(ctx context.Context, repo Repository, ref string) ([]Commit, error) {
	out, err := exec.CommandContext(ctx, "git", "-C", repo.Path, "log", "-n", "30", "--format=%h|%s|%an|%aI", ref).Output()
	if err != nil {
		return []Commit{}, nil
	}
	items := []Commit{}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		p := strings.SplitN(line, "|", 4)
		if len(p) == 4 {
			items = append(items, Commit{Hash: p[0], Subject: p[1], Author: p[2], Date: p[3]})
		}
	}
	return items, nil
}
func (s *Store) Commit(ctx context.Context, repo Repository, hash string) (CommitDetail, error) {
	if !safeRef(hash) {
		return CommitDetail{}, ErrNotFound
	}
	output, err := exec.CommandContext(ctx, "git", "-C", repo.Path, "show", "--format=%H|%s|%an|%aI%n%B%n---CHANGES---", "--stat", hash).Output()
	if err != nil {
		return CommitDetail{}, ErrNotFound
	}
	parts := strings.SplitN(string(output), "\n", 2)
	fields := strings.SplitN(parts[0], "|", 4)
	if len(fields) != 4 {
		return CommitDetail{}, ErrNotFound
	}
	body, changes := "", ""
	if len(parts) == 2 {
		segments := strings.SplitN(parts[1], "---CHANGES---\n", 2)
		body = strings.TrimSpace(segments[0])
		if len(segments) == 2 {
			changes = strings.TrimSpace(segments[1])
		}
	}
	return CommitDetail{Commit: Commit{Hash: fields[0], Subject: fields[1], Author: fields[2], Date: fields[3]}, Body: body, Changes: changes}, nil
}
func (s *Store) Archive(ctx context.Context, repo Repository, ref string) ([]byte, error) {
	if !safeRef(ref) {
		return nil, ErrNotFound
	}
	output, err := exec.CommandContext(ctx, "git", "-C", repo.Path, "archive", "--format=zip", "--prefix="+repo.Name+"/", ref).Output()
	if err != nil {
		return nil, ErrNotFound
	}
	return output, nil
}
func (s *Store) WriteFile(ctx context.Context, repo Repository, branch, filePath, content, author, email, message string) (Commit, error) {
	if !safeRef(branch) || !safeGitPath(filePath) || filePath == "" || len(content) > 1<<20 {
		return Commit{}, ErrNotFound
	}
	work, err := os.MkdirTemp("", "kohame-work-")
	if err != nil {
		return Commit{}, err
	}
	defer os.RemoveAll(work)
	if output, err := exec.CommandContext(ctx, "git", "clone", repo.Path, work).CombinedOutput(); err != nil {
		return Commit{}, fmt.Errorf("clone repository: %w: %s", err, strings.TrimSpace(string(output)))
	}
	if _, err := exec.CommandContext(ctx, "git", "-C", work, "checkout", "-B", branch).CombinedOutput(); err != nil {
		return Commit{}, fmt.Errorf("checkout branch: %w", err)
	}
	fullPath := filepath.Join(work, filepath.FromSlash(filePath))
	if !strings.HasPrefix(filepath.Clean(fullPath), filepath.Clean(work)+string(os.PathSeparator)) {
		return Commit{}, ErrNotFound
	}
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		return Commit{}, err
	}
	if err := os.WriteFile(fullPath, []byte(content), 0o644); err != nil {
		return Commit{}, err
	}
	for _, args := range [][]string{{"config", "user.name", author}, {"config", "user.email", email}, {"add", "--", filePath}, {"commit", "-m", valueOrMessage(message, "更新 "+filePath)}} {
		if output, err := exec.CommandContext(ctx, "git", append([]string{"-C", work}, args...)...).CombinedOutput(); err != nil {
			return Commit{}, fmt.Errorf("update file: %w: %s", err, strings.TrimSpace(string(output)))
		}
	}
	if output, err := exec.CommandContext(ctx, "git", "-C", work, "push", "origin", "HEAD:refs/heads/"+branch).CombinedOutput(); err != nil {
		return Commit{}, fmt.Errorf("push file: %w: %s", err, strings.TrimSpace(string(output)))
	}
	_, _ = exec.CommandContext(ctx, "git", "-C", repo.Path, "symbolic-ref", "HEAD", "refs/heads/"+branch).CombinedOutput()
	items, err := s.Commits(ctx, repo, branch)
	if err != nil || len(items) == 0 {
		return Commit{}, err
	}
	_, _ = s.db.ExecContext(ctx, `UPDATE repositories SET updated_at=`+s.placeholders(1)+` WHERE name=`+s.placeholders(2), time.Now().UTC(), repo.FullName)
	return items[0], nil
}
func valueOrMessage(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return strings.TrimSpace(value)
}
func splitTopics(value string) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, item := range strings.Split(value, ",") {
		item = strings.TrimSpace(strings.ToLower(item))
		if item != "" && !seen[item] {
			seen[item] = true
			out = append(out, item)
		}
	}
	return out
}

func (s *Store) Get(scope, name string) (Repository, error) {
	if !validPart(scope) || !validPart(name) {
		return Repository{}, ErrNotFound
	}
	return s.GetFullName(scope + "/" + name)
}
func (s *Store) GetFullName(fullName string) (Repository, error) {
	scope, name, ok := strings.Cut(fullName, "/")
	if !ok || !validPart(scope) || !validPart(name) {
		return Repository{}, ErrNotFound
	}
	var updatedAt time.Time
	var forkedFrom string
	var stars, forks int
	err := s.db.QueryRow(`SELECT r.updated_at,COALESCE(f.parent_name,''),(SELECT COUNT(*) FROM repository_stars WHERE repository_name=r.name),(SELECT COUNT(*) FROM repository_forks WHERE parent_name=r.name) FROM repositories r LEFT JOIN repository_forks f ON f.repository_name=r.name WHERE r.name = `+s.placeholders(1), fullName).Scan(&updatedAt, &forkedFrom, &stars, &forks)
	if errors.Is(err, sql.ErrNoRows) {
		return Repository{}, ErrNotFound
	}
	if err != nil {
		return Repository{}, err
	}
	repo, _ := s.fromFullName(fullName, updatedAt)
	repo.ForkedFrom, repo.Stars, repo.Forks = forkedFrom, stars, forks
	if info, err := os.Stat(repo.Path); err != nil || !info.IsDir() {
		return Repository{}, ErrNotFound
	}
	return repo, nil
}
func (s *Store) fromFullName(fullName string, updatedAt time.Time) (Repository, error) {
	scope, name, ok := strings.Cut(fullName, "/")
	if !ok || !validPart(scope) || !validPart(name) {
		return Repository{}, ErrNotFound
	}
	return Repository{Scope: scope, Name: name, FullName: fullName, UpdatedAt: updatedAt, Path: filepath.Join(s.root, scope, name+".git")}, nil
}

func (s *Store) Tree(ctx context.Context, repo Repository, ref, directory string) ([]TreeEntry, error) {
	if !safeGitPath(directory) || !safeRef(ref) {
		return nil, ErrNotFound
	}
	object := ref
	if directory != "" {
		object += ":" + directory
	}
	output, err := exec.CommandContext(ctx, "git", "-C", repo.Path, "ls-tree", object).Output()
	if err != nil {
		// A newly created bare repository has no HEAD yet; it has an empty tree.
		if ref == "HEAD" {
			return []TreeEntry{}, nil
		}
		return nil, ErrNotFound
	}
	entries := make([]TreeEntry, 0)
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) != 2 {
			continue
		}
		meta := strings.Fields(parts[0])
		if len(meta) < 2 {
			continue
		}
		entryPath := parts[1]
		if directory != "" {
			entryPath = directory + "/" + entryPath
		}
		entries = append(entries, TreeEntry{Name: parts[1], Path: entryPath, Type: meta[1]})
	}
	return entries, nil
}
func (s *Store) Blob(ctx context.Context, repo Repository, ref, filePath string) (Blob, error) {
	if !safeGitPath(filePath) || filePath == "" || !safeRef(ref) {
		return Blob{}, ErrNotFound
	}
	output, err := exec.CommandContext(ctx, "git", "-C", repo.Path, "show", ref+":"+filePath).Output()
	if err != nil {
		return Blob{}, ErrNotFound
	}
	if len(output) > 1<<20 {
		return Blob{Path: filePath, Content: "File is larger than 1 MiB.", IsText: false}, nil
	}
	return Blob{Path: filePath, Content: string(output), IsText: utf8.Valid(output) && !strings.Contains(string(output), "\x00")}, nil
}

func (s *Store) placeholders(numbers ...int) string {
	items := make([]string, len(numbers))
	for i, n := range numbers {
		if s.driver == "pgsql" {
			items[i] = fmt.Sprintf("$%d", n)
		} else {
			items[i] = "?"
		}
	}
	return strings.Join(items, ",")
}
func isUniqueViolation(err error) bool {
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "unique") || strings.Contains(message, "duplicate key")
}
func validPart(value string) bool {
	if value == "" || len(value) > 80 || strings.Contains(value, "..") {
		return false
	}
	for _, r := range value {
		if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-' || r == '_' || r == '.') {
			return false
		}
	}
	return true
}
func safeGitPath(value string) bool {
	return !strings.Contains(value, "..") && !strings.HasPrefix(value, "/") && !strings.Contains(value, "\\")
}
func safeRef(value string) bool {
	if value == "" || strings.Contains(value, "..") {
		return false
	}
	for _, r := range value {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' || r == '.' || r == '/') {
			return false
		}
	}
	return true
}
