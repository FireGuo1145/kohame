package forge

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

var ErrNotFound = errors.New("not found")
var ErrUnauthorized = errors.New("unauthorized")
var ErrForbidden = errors.New("forbidden")
var ErrConflict = errors.New("already exists")
var ErrSetupComplete = errors.New("setup is already complete")

type Store struct {
	db     *sql.DB
	driver string
}

type User struct {
	ID        int64     `json:"id"`
	Username  string    `json:"username"`
	Email     string    `json:"email"`
	IsAdmin   bool      `json:"isAdmin"`
	CreatedAt time.Time `json:"createdAt"`
}
type SiteSettings struct {
	Title             string `json:"title"`
	Description       string `json:"description"`
	AllowRegistration bool   `json:"allowRegistration"`
}
type Issue struct {
	ID        int64     `json:"id"`
	Title     string    `json:"title"`
	Body      string    `json:"body"`
	State     string    `json:"state"`
	Author    string    `json:"author"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}
type PullRequest struct {
	ID           int64     `json:"id"`
	Title        string    `json:"title"`
	Body         string    `json:"body"`
	SourceBranch string    `json:"sourceBranch"`
	TargetBranch string    `json:"targetBranch"`
	State        string    `json:"state"`
	Author       string    `json:"author"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}
type Release struct {
	ID        int64     `json:"id"`
	TagName   string    `json:"tagName"`
	Title     string    `json:"title"`
	Notes     string    `json:"notes"`
	Author    string    `json:"author"`
	CreatedAt time.Time `json:"createdAt"`
}
type Contributor struct {
	Username      string `json:"username"`
	Contributions int    `json:"contributions"`
}

func NewStore(db *sql.DB, driver string) *Store { return &Store{db: db, driver: driver} }

func (s *Store) NeedsSetup(ctx context.Context) (bool, error) {
	var count int
	err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM users").Scan(&count)
	return count == 0, err
}

func (s *Store) CreateAdmin(ctx context.Context, username, email, password string) (User, error) {
	needsSetup, err := s.NeedsSetup(ctx)
	if err != nil {
		return User{}, err
	}
	if !needsSetup {
		return User{}, ErrSetupComplete
	}
	return s.createUser(ctx, username, email, password, true)
}

func (s *Store) Register(ctx context.Context, username, email, password string) (User, error) {
	settings, err := s.Settings(ctx)
	if err != nil {
		return User{}, err
	}
	if !settings.AllowRegistration {
		return User{}, ErrForbidden
	}
	needsSetup, err := s.NeedsSetup(ctx)
	if err != nil {
		return User{}, err
	}
	if needsSetup {
		return User{}, ErrSetupComplete
	}
	return s.createUser(ctx, username, email, password, false)
}

func (s *Store) createUser(ctx context.Context, username, email, password string, admin bool) (User, error) {
	username, email = strings.TrimSpace(strings.ToLower(username)), strings.TrimSpace(strings.ToLower(email))
	if len(username) < 3 || len(username) > 40 || !strings.Contains(email, "@") || len(password) < 8 {
		return User{}, fmt.Errorf("username must be 3–40 characters, email must be valid, and password must be at least 8 characters")
	}
	for _, r := range username {
		if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-' || r == '_') {
			return User{}, fmt.Errorf("username can use lowercase letters, numbers, hyphens, and underscores")
		}
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return User{}, err
	}
	now := time.Now().UTC()
	query := `INSERT INTO users (username, email, password_hash, is_admin, created_at) VALUES (` + s.args(1, 2, 3, 4, 5) + `)`
	if s.driver == "pgsql" {
		query += " RETURNING id"
		var id int64
		err = s.db.QueryRowContext(ctx, query, username, email, string(hash), admin, now).Scan(&id)
		if err == nil {
			return User{ID: id, Username: username, Email: email, IsAdmin: admin, CreatedAt: now}, nil
		}
	} else {
		result, insertErr := s.db.ExecContext(ctx, query, username, email, string(hash), admin, now)
		err = insertErr
		if err == nil {
			id, _ := result.LastInsertId()
			return User{ID: id, Username: username, Email: email, IsAdmin: admin, CreatedAt: now}, nil
		}
	}
	if isUnique(err) {
		return User{}, ErrConflict
	}
	return User{}, err
}

func (s *Store) Authenticate(ctx context.Context, identity, password string) (User, error) {
	var user User
	var hash string
	err := s.db.QueryRowContext(ctx, `SELECT id, username, email, password_hash, is_admin, created_at FROM users WHERE username = `+s.arg(1)+` OR email = `+s.arg(2), strings.ToLower(identity), strings.ToLower(identity)).Scan(&user.ID, &user.Username, &user.Email, &hash, &user.IsAdmin, &user.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrUnauthorized
	}
	if err != nil {
		return User{}, err
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) != nil {
		return User{}, ErrUnauthorized
	}
	return user, nil
}

func (s *Store) CreateSession(ctx context.Context, userID int64) (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	token := hex.EncodeToString(raw)
	_, err := s.db.ExecContext(ctx, `INSERT INTO sessions (token_hash, user_id, expires_at) VALUES (`+s.args(1, 2, 3)+`)`, tokenHash(token), userID, time.Now().UTC().Add(30*24*time.Hour))
	return token, err
}
func (s *Store) DeleteSession(ctx context.Context, token string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE token_hash = `+s.arg(1), tokenHash(token))
	return err
}
func (s *Store) UserBySession(ctx context.Context, token string) (User, error) {
	var user User
	err := s.db.QueryRowContext(ctx, `SELECT u.id,u.username,u.email,u.is_admin,u.created_at FROM sessions s JOIN users u ON u.id=s.user_id WHERE s.token_hash=`+s.arg(1)+` AND s.expires_at > `+s.arg(2), tokenHash(token), time.Now().UTC()).Scan(&user.ID, &user.Username, &user.Email, &user.IsAdmin, &user.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrUnauthorized
	}
	return user, err
}

func (s *Store) Settings(ctx context.Context) (SiteSettings, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT key,value FROM settings")
	if err != nil {
		return SiteSettings{}, err
	}
	defer rows.Close()
	settings := SiteSettings{}
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return settings, err
		}
		switch key {
		case "site_title":
			settings.Title = value
		case "site_description":
			settings.Description = value
		case "allow_registration":
			settings.AllowRegistration = value == "true"
		}
	}
	return settings, rows.Err()
}
func (s *Store) UpdateSettings(ctx context.Context, value SiteSettings) error {
	if strings.TrimSpace(value.Title) == "" {
		return fmt.Errorf("site title is required")
	}
	items := map[string]string{"site_title": strings.TrimSpace(value.Title), "site_description": strings.TrimSpace(value.Description), "allow_registration": fmt.Sprintf("%t", value.AllowRegistration)}
	for key, item := range items {
		if _, err := s.db.ExecContext(ctx, `INSERT INTO settings (key,value) VALUES (`+s.args(1, 2)+`) ON CONFLICT (key) DO UPDATE SET value=EXCLUDED.value`, key, item); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) ListIssues(ctx context.Context, repo string) ([]Issue, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT i.id,i.title,i.body,i.state,u.username,i.created_at,i.updated_at FROM issues i JOIN users u ON u.id=i.author_id WHERE i.repository_name=`+s.arg(1)+` ORDER BY i.updated_at DESC`, repo)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Issue, 0)
	for rows.Next() {
		var v Issue
		if err := rows.Scan(&v.ID, &v.Title, &v.Body, &v.State, &v.Author, &v.CreatedAt, &v.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (s *Store) CreateIssue(ctx context.Context, repo string, user User, title, body string) (Issue, error) {
	if strings.TrimSpace(title) == "" {
		return Issue{}, fmt.Errorf("issue title is required")
	}
	now := time.Now().UTC()
	q := `INSERT INTO issues (repository_name,author_id,title,body,state,created_at,updated_at) VALUES (` + s.args(1, 2, 3, 4, 5, 6, 7) + `)`
	id, err := s.insertID(ctx, q, repo, user.ID, strings.TrimSpace(title), strings.TrimSpace(body), "open", now, now)
	if err != nil {
		return Issue{}, err
	}
	_ = s.activity(ctx, repo, user.ID, "issue")
	return Issue{ID: id, Title: strings.TrimSpace(title), Body: strings.TrimSpace(body), State: "open", Author: user.Username, CreatedAt: now, UpdatedAt: now}, nil
}
func (s *Store) UpdateIssueState(ctx context.Context, repo string, id int64, state string) error {
	if state != "open" && state != "closed" {
		return fmt.Errorf("state must be open or closed")
	}
	result, err := s.db.ExecContext(ctx, `UPDATE issues SET state=`+s.arg(1)+`,updated_at=`+s.arg(2)+` WHERE id=`+s.arg(3)+` AND repository_name=`+s.arg(4), state, time.Now().UTC(), id, repo)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) DeleteIssue(ctx context.Context, repo string, id int64) error {
	return s.deleteItem(ctx, "issues", repo, id)
}

func (s *Store) ListPullRequests(ctx context.Context, repo string) ([]PullRequest, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT p.id,p.title,p.body,p.source_branch,p.target_branch,p.state,u.username,p.created_at,p.updated_at FROM pull_requests p JOIN users u ON u.id=p.author_id WHERE p.repository_name=`+s.arg(1)+` ORDER BY p.updated_at DESC`, repo)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]PullRequest, 0)
	for rows.Next() {
		var v PullRequest
		if err := rows.Scan(&v.ID, &v.Title, &v.Body, &v.SourceBranch, &v.TargetBranch, &v.State, &v.Author, &v.CreatedAt, &v.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (s *Store) CreatePullRequest(ctx context.Context, repo string, user User, title, body, source, target string) (PullRequest, error) {
	if strings.TrimSpace(title) == "" || strings.TrimSpace(source) == "" || strings.TrimSpace(target) == "" {
		return PullRequest{}, fmt.Errorf("title, source branch, and target branch are required")
	}
	now := time.Now().UTC()
	q := `INSERT INTO pull_requests (repository_name,author_id,title,body,source_branch,target_branch,state,created_at,updated_at) VALUES (` + s.args(1, 2, 3, 4, 5, 6, 7, 8, 9) + `)`
	id, err := s.insertID(ctx, q, repo, user.ID, strings.TrimSpace(title), strings.TrimSpace(body), strings.TrimSpace(source), strings.TrimSpace(target), "open", now, now)
	if err != nil {
		return PullRequest{}, err
	}
	_ = s.activity(ctx, repo, user.ID, "pull_request")
	return PullRequest{ID: id, Title: strings.TrimSpace(title), Body: strings.TrimSpace(body), SourceBranch: strings.TrimSpace(source), TargetBranch: strings.TrimSpace(target), State: "open", Author: user.Username, CreatedAt: now, UpdatedAt: now}, nil
}
func (s *Store) UpdatePullRequestState(ctx context.Context, repo string, id int64, state string) error {
	if state != "open" && state != "closed" && state != "merged" {
		return fmt.Errorf("state must be open, closed, or merged")
	}
	result, err := s.db.ExecContext(ctx, `UPDATE pull_requests SET state=`+s.arg(1)+`,updated_at=`+s.arg(2)+` WHERE id=`+s.arg(3)+` AND repository_name=`+s.arg(4), state, time.Now().UTC(), id, repo)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) DeletePullRequest(ctx context.Context, repo string, id int64) error {
	return s.deleteItem(ctx, "pull_requests", repo, id)
}

func (s *Store) ListReleases(ctx context.Context, repo string) ([]Release, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT r.id,r.tag_name,r.title,r.notes,u.username,r.created_at FROM releases r JOIN users u ON u.id=r.author_id WHERE r.repository_name=`+s.arg(1)+` ORDER BY r.created_at DESC`, repo)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Release, 0)
	for rows.Next() {
		var v Release
		if err := rows.Scan(&v.ID, &v.TagName, &v.Title, &v.Notes, &v.Author, &v.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (s *Store) CreateRelease(ctx context.Context, repo string, user User, tag, title, notes string) (Release, error) {
	if strings.TrimSpace(tag) == "" || strings.TrimSpace(title) == "" {
		return Release{}, fmt.Errorf("tag and title are required")
	}
	now := time.Now().UTC()
	q := `INSERT INTO releases (repository_name,author_id,tag_name,title,notes,created_at) VALUES (` + s.args(1, 2, 3, 4, 5, 6) + `)`
	id, err := s.insertID(ctx, q, repo, user.ID, strings.TrimSpace(tag), strings.TrimSpace(title), strings.TrimSpace(notes), now)
	if err != nil {
		return Release{}, err
	}
	_ = s.activity(ctx, repo, user.ID, "release")
	return Release{ID: id, TagName: strings.TrimSpace(tag), Title: strings.TrimSpace(title), Notes: strings.TrimSpace(notes), Author: user.Username, CreatedAt: now}, nil
}

func (s *Store) DeleteRelease(ctx context.Context, repo string, id int64) error {
	return s.deleteItem(ctx, "releases", repo, id)
}

func (s *Store) deleteItem(ctx context.Context, table, repo string, id int64) error {
	result, err := s.db.ExecContext(ctx, "DELETE FROM "+table+" WHERE id="+s.arg(1)+" AND repository_name="+s.arg(2), id, repo)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
func (s *Store) Contributors(ctx context.Context, repo string) ([]Contributor, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT u.username,COUNT(*) FROM activities a JOIN users u ON u.id=a.user_id WHERE a.repository_name=`+s.arg(1)+` GROUP BY u.id,u.username ORDER BY COUNT(*) DESC,u.username`, repo)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Contributor, 0)
	for rows.Next() {
		var v Contributor
		if err := rows.Scan(&v.Username, &v.Contributions); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (s *Store) activity(ctx context.Context, repo string, userID int64, kind string) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO activities (repository_name,user_id,kind,created_at) VALUES (`+s.args(1, 2, 3, 4)+`)`, repo, userID, kind, time.Now().UTC())
	return err
}
func (s *Store) insertID(ctx context.Context, query string, args ...any) (int64, error) {
	if s.driver == "pgsql" {
		var id int64
		err := s.db.QueryRowContext(ctx, query+" RETURNING id", args...).Scan(&id)
		return id, err
	}
	result, err := s.db.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}
func (s *Store) arg(n int) string {
	if s.driver == "pgsql" {
		return fmt.Sprintf("$%d", n)
	}
	return "?"
}
func (s *Store) args(numbers ...int) string {
	values := make([]string, len(numbers))
	for i, n := range numbers {
		values[i] = s.arg(n)
	}
	return strings.Join(values, ",")
}
func tokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
func isUnique(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "unique") || strings.Contains(message, "duplicate key")
}
