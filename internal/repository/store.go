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
	Description     string   `json:"description"`
	Visibility      string   `json:"visibility"`
	DefaultBranch   string   `json:"defaultBranch"`
	Topics          []string `json:"topics"`
	IssuesEnabled   bool     `json:"issuesEnabled"`
	PullsEnabled    bool     `json:"pullsEnabled"`
	ReleasesEnabled bool     `json:"releasesEnabled"`
	WikiEnabled     bool     `json:"wikiEnabled"`
	AutoCloseIssues bool     `json:"autoCloseIssues"`
	Archived        bool     `json:"archived"`
}
type Collaborator struct {
	Username   string `json:"username"`
	Permission string `json:"permission"`
}
type ProtectedBranch struct {
	Branch             string `json:"branch"`
	RequirePullRequest bool   `json:"requirePullRequest"`
	RequireApprovals   int    `json:"requireApprovals"`
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
	_, _ = s.db.ExecContext(ctx, `INSERT INTO repository_settings (repository_name,description,visibility,default_branch,topics,issues_enabled,pulls_enabled,releases_enabled,wiki_enabled,auto_close_issues,archived) VALUES (`+s.placeholders(1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11)+`)`, fullName, settings.Description, settings.Visibility, settings.DefaultBranch, strings.Join(settings.Topics, ","), settings.IssuesEnabled, settings.PullsEnabled, settings.ReleasesEnabled, settings.WikiEnabled, settings.AutoCloseIssues, settings.Archived)
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
	err := s.db.QueryRowContext(ctx, `SELECT description,visibility,default_branch,topics,issues_enabled,pulls_enabled,releases_enabled,wiki_enabled,auto_close_issues,archived FROM repository_settings WHERE repository_name=`+s.placeholders(1), fullName).Scan(&value.Description, &value.Visibility, &value.DefaultBranch, &topics, &value.IssuesEnabled, &value.PullsEnabled, &value.ReleasesEnabled, &value.WikiEnabled, &value.AutoCloseIssues, &value.Archived)
	if errors.Is(err, sql.ErrNoRows) {
		return Settings{Visibility: "private", DefaultBranch: "main", Topics: []string{}, IssuesEnabled: true, PullsEnabled: true, ReleasesEnabled: true}, nil
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
	_, err := s.db.ExecContext(ctx, `INSERT INTO repository_settings (repository_name,description,visibility,default_branch,topics,issues_enabled,pulls_enabled,releases_enabled,wiki_enabled,auto_close_issues,archived) VALUES (`+s.placeholders(1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11)+`) ON CONFLICT (repository_name) DO UPDATE SET description=EXCLUDED.description,visibility=EXCLUDED.visibility,default_branch=EXCLUDED.default_branch,topics=EXCLUDED.topics,issues_enabled=EXCLUDED.issues_enabled,pulls_enabled=EXCLUDED.pulls_enabled,releases_enabled=EXCLUDED.releases_enabled,wiki_enabled=EXCLUDED.wiki_enabled,auto_close_issues=EXCLUDED.auto_close_issues,archived=EXCLUDED.archived`, fullName, strings.TrimSpace(value.Description), value.Visibility, value.DefaultBranch, strings.Join(topics, ","), value.IssuesEnabled, value.PullsEnabled, value.ReleasesEnabled, value.WikiEnabled, value.AutoCloseIssues, value.Archived)
	return err
}

func (s *Store) UpdateVisibility(ctx context.Context, fullName, visibility string) error {
	if visibility != "private" && visibility != "public" {
		return errors.New("visibility must be private or public")
	}
	_, err := s.db.ExecContext(ctx, `UPDATE repository_settings SET visibility=`+s.placeholders(1)+` WHERE repository_name=`+s.placeholders(2), visibility, fullName)
	return err
}

func (s *Store) Collaborators(ctx context.Context, fullName string) ([]Collaborator, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT u.username,c.permission FROM repository_collaborators c JOIN users u ON u.id=c.user_id WHERE c.repository_name=`+s.placeholders(1)+` ORDER BY u.username`, fullName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Collaborator{}
	for rows.Next() {
		var item Collaborator
		if err := rows.Scan(&item.Username, &item.Permission); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) SetCollaborator(ctx context.Context, fullName, username, permission string) error {
	if permission != "read" && permission != "write" && permission != "maintain" && permission != "admin" {
		return errors.New("invalid collaborator permission")
	}
	var userID int64
	if err := s.db.QueryRowContext(ctx, `SELECT id FROM users WHERE username=`+s.placeholders(1), strings.ToLower(strings.TrimSpace(username))).Scan(&userID); errors.Is(err, sql.ErrNoRows) {
		return errors.New("user not found")
	} else if err != nil {
		return err
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO repository_collaborators (repository_name,user_id,permission) VALUES (`+s.placeholders(1, 2, 3)+`) ON CONFLICT (repository_name,user_id) DO UPDATE SET permission=EXCLUDED.permission`, fullName, userID, permission)
	return err
}

func (s *Store) RemoveCollaborator(ctx context.Context, fullName, username string) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM repository_collaborators WHERE repository_name=`+s.placeholders(1)+` AND user_id=(SELECT id FROM users WHERE username=`+s.placeholders(2)+`)`, fullName, strings.ToLower(strings.TrimSpace(username)))
	if err != nil {
		return err
	}
	count, _ := result.RowsAffected()
	if count == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) CollaboratorPermission(ctx context.Context, fullName, username string) (string, error) {
	var permission string
	err := s.db.QueryRowContext(ctx, `SELECT c.permission FROM repository_collaborators c JOIN users u ON u.id=c.user_id WHERE c.repository_name=`+s.placeholders(1)+` AND u.username=`+s.placeholders(2), fullName, strings.ToLower(strings.TrimSpace(username))).Scan(&permission)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return permission, err
}

func (s *Store) ProtectedBranches(ctx context.Context, fullName string) ([]ProtectedBranch, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT branch,require_pull_request,require_approvals FROM protected_branches WHERE repository_name=`+s.placeholders(1)+` ORDER BY branch`, fullName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []ProtectedBranch{}
	for rows.Next() {
		var item ProtectedBranch
		if err := rows.Scan(&item.Branch, &item.RequirePullRequest, &item.RequireApprovals); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) SetProtectedBranch(ctx context.Context, fullName string, value ProtectedBranch) error {
	if !safeRef(value.Branch) || value.RequireApprovals < 0 || value.RequireApprovals > 10 {
		return errors.New("invalid branch protection")
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO protected_branches (repository_name,branch,require_pull_request,require_approvals) VALUES (`+s.placeholders(1, 2, 3, 4)+`) ON CONFLICT (repository_name,branch) DO UPDATE SET require_pull_request=EXCLUDED.require_pull_request,require_approvals=EXCLUDED.require_approvals`, fullName, value.Branch, value.RequirePullRequest, value.RequireApprovals)
	return err
}

func (s *Store) RemoveProtectedBranch(ctx context.Context, fullName, branch string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM protected_branches WHERE repository_name=`+s.placeholders(1)+` AND branch=`+s.placeholders(2), fullName, branch)
	return err
}

func (s *Store) Transfer(ctx context.Context, repo Repository, targetScope string) (Repository, error) {
	if !validPart(targetScope) || targetScope == repo.Scope {
		return Repository{}, errors.New("invalid target scope")
	}
	nextName := targetScope + "/" + repo.Name
	if _, err := s.Get(targetScope, repo.Name); err == nil {
		return Repository{}, ErrExists
	} else if !errors.Is(err, ErrNotFound) {
		return Repository{}, err
	}
	nextPath := filepath.Join(s.root, targetScope, repo.Name+".git")
	if err := os.MkdirAll(filepath.Dir(nextPath), 0o755); err != nil {
		return Repository{}, err
	}
	if err := os.Rename(repo.Path, nextPath); err != nil {
		return Repository{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		_ = os.Rename(nextPath, repo.Path)
		return Repository{}, err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `UPDATE repositories SET name=`+s.placeholders(1)+` WHERE name=`+s.placeholders(2), nextName, repo.FullName); err != nil {
		_ = os.Rename(nextPath, repo.Path)
		return Repository{}, err
	}
	for _, table := range []string{"repository_settings", "repository_stars", "activities", "issues", "issue_comments", "pull_requests", "releases", "labels", "repository_collaborators", "protected_branches"} {
		if _, err = tx.ExecContext(ctx, `UPDATE `+table+` SET repository_name=`+s.placeholders(1)+` WHERE repository_name=`+s.placeholders(2), nextName, repo.FullName); err != nil {
			_ = os.Rename(nextPath, repo.Path)
			return Repository{}, err
		}
	}
	if _, err = tx.ExecContext(ctx, `UPDATE repository_forks SET repository_name=`+s.placeholders(1)+` WHERE repository_name=`+s.placeholders(2), nextName, repo.FullName); err != nil {
		return Repository{}, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE repository_forks SET parent_name=`+s.placeholders(1)+` WHERE parent_name=`+s.placeholders(2), nextName, repo.FullName); err != nil {
		return Repository{}, err
	}
	if err = tx.Commit(); err != nil {
		_ = os.Rename(nextPath, repo.Path)
		return Repository{}, err
	}
	return s.Get(targetScope, repo.Name)
}

func (s *Store) Delete(ctx context.Context, repo Repository) error {
	trash := repo.Path + ".deleting-" + fmt.Sprint(time.Now().UnixNano())
	if err := os.Rename(repo.Path, trash); err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		_ = os.Rename(trash, repo.Path)
		return err
	}
	defer tx.Rollback()
	for _, table := range []string{"repository_settings", "repository_stars", "activities", "issues", "issue_comments", "pull_requests", "releases", "labels", "repository_collaborators", "protected_branches", "repository_forks", "repositories"} {
		column := "repository_name"
		if table == "repositories" {
			column = "name"
		}
		if _, err = tx.ExecContext(ctx, `DELETE FROM `+table+` WHERE `+column+`=`+s.placeholders(1), repo.FullName); err != nil {
			_ = os.Rename(trash, repo.Path)
			return err
		}
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM repository_forks WHERE parent_name=`+s.placeholders(1), repo.FullName); err != nil {
		_ = os.Rename(trash, repo.Path)
		return err
	}
	if err = tx.Commit(); err != nil {
		_ = os.Rename(trash, repo.Path)
		return err
	}
	return os.RemoveAll(trash)
}
func (s *Store) Branches(ctx context.Context, repo Repository) ([]Ref, error) {
	return s.refs(ctx, repo, "refs/heads")
}
func (s *Store) Tags(ctx context.Context, repo Repository) ([]Ref, error) {
	return s.refs(ctx, repo, "refs/tags")
}
func (s *Store) CreateTag(ctx context.Context, repo Repository, tag, ref string) error {
	if !safeRef(tag) || !safeRef(ref) {
		return ErrNotFound
	}
	if output, err := exec.CommandContext(ctx, "git", "-C", repo.Path, "tag", tag, ref).CombinedOutput(); err != nil {
		return fmt.Errorf("create tag: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
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
