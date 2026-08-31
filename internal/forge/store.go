package forge

import (
	"context"
	"crypto/md5"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gopkg.in/yaml.v3"
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
	ID            int64     `json:"id"`
	Username      string    `json:"username"`
	Email         string    `json:"email"`
	IsAdmin       bool      `json:"isAdmin"`
	EmailVerified bool      `json:"emailVerified"`
	CreatedAt     time.Time `json:"createdAt"`
}
type OIDCProvider struct {
	ID           int64     `json:"id"`
	Slug         string    `json:"slug"`
	Name         string    `json:"name"`
	IssuerURL    string    `json:"issuerUrl"`
	ClientID     string    `json:"clientId"`
	ClientSecret string    `json:"clientSecret,omitempty"`
	Scopes       string    `json:"scopes"`
	Enabled      bool      `json:"enabled"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}
type Workflow struct {
	ID             int64     `json:"id"`
	RepositoryName string    `json:"repositoryName"`
	Name           string    `json:"name"`
	Config         string    `json:"config"`
	Enabled        bool      `json:"enabled"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

type WorkflowDefinition struct {
	Name  string
	On    []string
	Jobs  map[string]WorkflowJob
	Steps []WorkflowStep
}
type WorkflowJob struct {
	RunsOn string         `yaml:"runs-on"`
	Steps  []WorkflowStep `yaml:"steps"`
}
type WorkflowStep struct {
	Name string            `yaml:"name,omitempty" json:"name,omitempty"`
	Run  string            `yaml:"run" json:"run,omitempty"`
	Uses string            `yaml:"uses" json:"uses,omitempty"`
	With map[string]string `yaml:"with,omitempty" json:"with,omitempty"`
}

// ParseWorkflowDefinition accepts GitHub Actions YAML and the legacy compact JSON shape.
func ParseWorkflowDefinition(content string) (WorkflowDefinition, error) {
	var raw struct {
		Name  string                 `yaml:"name"`
		On    yaml.Node              `yaml:"on"`
		Jobs  map[string]WorkflowJob `yaml:"jobs"`
		Steps []WorkflowStep         `yaml:"steps"`
	}
	if err := yaml.Unmarshal([]byte(content), &raw); err != nil {
		return WorkflowDefinition{}, err
	}
	events := []string{}
	collect := func(node *yaml.Node) {
		if node == nil {
			return
		}
		switch node.Kind {
		case yaml.SequenceNode:
			for _, item := range node.Content {
				if strings.TrimSpace(item.Value) != "" {
					events = append(events, strings.TrimSpace(item.Value))
				}
			}
		case yaml.MappingNode:
			for index := 0; index+1 < len(node.Content); index += 2 {
				if strings.TrimSpace(node.Content[index].Value) != "" {
					events = append(events, strings.TrimSpace(node.Content[index].Value))
				}
			}
		case yaml.ScalarNode:
			if strings.TrimSpace(node.Value) != "" {
				events = append(events, strings.TrimSpace(node.Value))
			}
		}
	}
	collect(&raw.On)
	if len(events) == 0 && raw.On.Kind == 0 {
		return WorkflowDefinition{}, errors.New("workflow config must include on and jobs")
	}
	steps := append([]WorkflowStep{}, raw.Steps...)
	for _, job := range raw.Jobs {
		steps = append(steps, job.Steps...)
	}
	if len(events) == 0 || len(steps) == 0 {
		return WorkflowDefinition{}, errors.New("workflow config must include on and jobs with steps")
	}
	for _, step := range steps {
		if strings.TrimSpace(step.Run) == "" && strings.TrimSpace(step.Uses) == "" {
			return WorkflowDefinition{}, errors.New("workflow steps must include run or uses")
		}
	}
	return WorkflowDefinition{Name: strings.TrimSpace(raw.Name), On: events, Jobs: raw.Jobs, Steps: steps}, nil
}

type WorkflowRun struct {
	ID             int64      `json:"id"`
	WorkflowID     int64      `json:"workflowId"`
	RepositoryName string     `json:"repositoryName"`
	Event          string     `json:"event"`
	Status         string     `json:"status"`
	Output         string     `json:"output"`
	StartedAt      time.Time  `json:"startedAt"`
	FinishedAt     *time.Time `json:"finishedAt,omitempty"`
}
type Runner struct {
	ID         int64      `json:"id"`
	Name       string     `json:"name"`
	CreatedAt  time.Time  `json:"createdAt"`
	LastSeenAt *time.Time `json:"lastSeenAt,omitempty"`
}
type RunnerRegistration struct {
	Runner
	Token string `json:"token"`
}
type RunnerJob struct {
	ID         int64          `json:"id"`
	Repository string         `json:"repository"`
	Workflow   string         `json:"workflow"`
	Event      string         `json:"event"`
	Workspace  []byte         `json:"workspace"`
	Steps      []WorkflowStep `json:"steps"`
}
type SiteSettings struct {
	Title             string `json:"title"`
	Description       string `json:"description"`
	AllowRegistration bool   `json:"allowRegistration"`
	RepositoryRoot    string `json:"repositoryRoot"`
	WorkflowDirectory string `json:"workflowDirectory"`
	RunnerEnabled     bool   `json:"runnerEnabled"`
	RunnerURL         string `json:"runnerUrl"`
	RunnerToken       string `json:"runnerToken,omitempty"`
	CaptchaEnabled    bool   `json:"captchaEnabled"`
	CaptchaSiteKey    string `json:"captchaSiteKey"`
	CaptchaSecret     string `json:"captchaSecret,omitempty"`
	SMTPHost          string `json:"smtpHost"`
	SMTPPort          string `json:"smtpPort"`
	SMTPUsername      string `json:"smtpUsername"`
	SMTPPassword      string `json:"smtpPassword,omitempty"`
	SMTPFrom          string `json:"smtpFrom"`
	GravatarMirror    string `json:"gravatarMirror"`
}
type Issue struct {
	ID        int64     `json:"id"`
	Title     string    `json:"title"`
	Body      string    `json:"body"`
	State     string    `json:"state"`
	Author    string    `json:"author"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
	Labels    []Label   `json:"labels"`
	Assignees []string  `json:"assignees"`
}
type Milestone struct {
	ID          int64      `json:"id"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	DueAt       *time.Time `json:"dueAt,omitempty"`
	State       string     `json:"state"`
	CreatedAt   time.Time  `json:"createdAt"`
}
type Label struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Color       string `json:"color"`
	Description string `json:"description"`
}
type IssueComment struct {
	ID        int64     `json:"id"`
	Author    string    `json:"author"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"createdAt"`
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
type PullRequestComment struct {
	ID        int64     `json:"id"`
	Author    string    `json:"author"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"createdAt"`
}
type PullRequestReview struct {
	ID        int64     `json:"id"`
	Reviewer  string    `json:"reviewer"`
	State     string    `json:"state"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}
type Release struct {
	ID        int64          `json:"id"`
	TagName   string         `json:"tagName"`
	Title     string         `json:"title"`
	Notes     string         `json:"notes"`
	Author    string         `json:"author"`
	CreatedAt time.Time      `json:"createdAt"`
	Assets    []ReleaseAsset `json:"assets"`
}
type ReleaseAsset struct {
	ID          int64  `json:"id"`
	FileName    string `json:"fileName"`
	StorageName string `json:"-"`
	Size        int64  `json:"size"`
	URL         string `json:"url"`
}
type Contributor struct {
	Username      string `json:"username"`
	Contributions int    `json:"contributions"`
}
type Organization struct {
	Name      string `json:"name"`
	Role      string `json:"role"`
	Followers int    `json:"followers"`
	Followed  bool   `json:"followed"`
}
type OrganizationMember struct {
	Username string `json:"username"`
	Role     string `json:"role"`
}
type FollowTarget struct {
	Name string `json:"name"`
	Type string `json:"type"`
}
type Notification struct {
	ID        int64     `json:"id"`
	Kind      string    `json:"kind"`
	Title     string    `json:"title"`
	Body      string    `json:"body"`
	Link      string    `json:"link"`
	IsRead    bool      `json:"isRead"`
	CreatedAt time.Time `json:"createdAt"`
}
type Profile struct {
	Username     string    `json:"username"`
	DisplayName  string    `json:"displayName"`
	Bio          string    `json:"bio"`
	Location     string    `json:"location"`
	Website      string    `json:"website"`
	CreatedAt    time.Time `json:"createdAt"`
	Repositories int       `json:"repositories"`
	Stars        int       `json:"stars"`
	AvatarURL    string    `json:"avatarUrl"`
	Followers    int       `json:"followers"`
	Following    int       `json:"following"`
	Followed     bool      `json:"followed"`
}
type PersonalSettings struct {
	Username    string `json:"username"`
	Email       string `json:"email"`
	Verified    bool   `json:"verified"`
	DisplayName string `json:"displayName"`
	Bio         string `json:"bio"`
	Location    string `json:"location"`
	Website     string `json:"website"`
	AvatarURL   string `json:"avatarUrl"`
}
type SSHKey struct {
	ID        int64     `json:"id"`
	Title     string    `json:"title"`
	Key       string    `json:"key"`
	CreatedAt time.Time `json:"createdAt"`
}
type PersonalAccessToken struct {
	ID         int64      `json:"id"`
	Name       string     `json:"name"`
	CreatedAt  time.Time  `json:"createdAt"`
	LastUsedAt *time.Time `json:"lastUsedAt,omitempty"`
	ExpiresAt  *time.Time `json:"expiresAt,omitempty"`
}
type PersonalAccessTokenRegistration struct {
	PersonalAccessToken
	Token string `json:"token"`
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

func (s *Store) CreateOrganization(ctx context.Context, user User, name string) (Organization, error) {
	name = strings.TrimSpace(strings.ToLower(name))
	if len(name) < 2 || !validScope(name) {
		return Organization{}, fmt.Errorf("organization name is invalid")
	}
	var users int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE username=`+s.arg(1), name).Scan(&users); err != nil {
		return Organization{}, err
	}
	if users > 0 {
		return Organization{}, ErrConflict
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Organization{}, err
	}
	defer tx.Rollback()
	now := time.Now().UTC()
	query := `INSERT INTO organizations (name, created_at) VALUES (` + s.args(1, 2) + `)`
	var id int64
	if s.driver == "pgsql" {
		err = tx.QueryRowContext(ctx, query+" RETURNING id", name, now).Scan(&id)
	} else {
		result, insertErr := tx.ExecContext(ctx, query, name, now)
		err = insertErr
		if err == nil {
			id, _ = result.LastInsertId()
		}
	}
	if err != nil {
		if isUnique(err) {
			return Organization{}, ErrConflict
		}
		return Organization{}, err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO organization_members (organization_id,user_id,role) VALUES (`+s.args(1, 2, 3)+`)`, id, user.ID, "owner"); err != nil {
		return Organization{}, err
	}
	if err = tx.Commit(); err != nil {
		return Organization{}, err
	}
	return Organization{Name: name, Role: "owner"}, nil
}
func (s *Store) Scopes(ctx context.Context, user User) ([]Organization, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT o.name,m.role FROM organizations o JOIN organization_members m ON m.organization_id=o.id WHERE m.user_id=`+s.arg(1)+` ORDER BY o.name`, user.ID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Organization{{Name: user.Username, Role: "user"}}
	for rows.Next() {
		var org Organization
		if err := rows.Scan(&org.Name, &org.Role); err != nil {
			return nil, err
		}
		out = append(out, org)
	}
	return out, rows.Err()
}
func (s *Store) CanUseScope(ctx context.Context, user User, scope string) (bool, error) {
	if scope == user.Username {
		return true, nil
	}
	var count int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM organizations o JOIN organization_members m ON m.organization_id=o.id WHERE o.name=`+s.arg(1)+` AND m.user_id=`+s.arg(2), scope, user.ID).Scan(&count)
	return count > 0, err
}
func (s *Store) Organization(ctx context.Context, name string) (Organization, error) {
	var org Organization
	err := s.db.QueryRowContext(ctx, `SELECT o.name,(SELECT COUNT(*) FROM organization_follows f WHERE f.organization_id=o.id) FROM organizations o WHERE o.name=`+s.arg(1), strings.ToLower(strings.TrimSpace(name))).Scan(&org.Name, &org.Followers)
	if errors.Is(err, sql.ErrNoRows) {
		return Organization{}, ErrNotFound
	}
	return org, err
}

func (s *Store) ToggleOrganizationFollow(ctx context.Context, user User, name string) (bool, int, error) {
	var organizationID int64
	if err := s.db.QueryRowContext(ctx, `SELECT id FROM organizations WHERE name=`+s.arg(1), strings.ToLower(strings.TrimSpace(name))).Scan(&organizationID); err != nil {
		return false, 0, err
	}
	var exists int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM organization_follows WHERE follower_id=`+s.arg(1)+` AND organization_id=`+s.arg(2), user.ID, organizationID).Scan(&exists); err != nil {
		return false, 0, err
	}
	followed := exists == 0
	var err error
	if followed {
		_, err = s.db.ExecContext(ctx, `INSERT INTO organization_follows (follower_id,organization_id,created_at) VALUES (`+s.args(1, 2, 3)+`)`, user.ID, organizationID, time.Now().UTC())
	} else {
		_, err = s.db.ExecContext(ctx, `DELETE FROM organization_follows WHERE follower_id=`+s.arg(1)+` AND organization_id=`+s.arg(2), user.ID, organizationID)
	}
	if err != nil {
		return false, 0, err
	}
	var count int
	err = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM organization_follows WHERE organization_id=`+s.arg(1), organizationID).Scan(&count)
	return followed, count, err
}
func (s *Store) OrganizationFollowed(ctx context.Context, userID int64, name string) (bool, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM organization_follows f JOIN organizations o ON o.id=f.organization_id WHERE f.follower_id=`+s.arg(1)+` AND o.name=`+s.arg(2), userID, strings.ToLower(strings.TrimSpace(name))).Scan(&count)
	return count > 0, err
}
func (s *Store) OrganizationMembers(ctx context.Context, name string) ([]OrganizationMember, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT u.username,m.role FROM organizations o JOIN organization_members m ON m.organization_id=o.id JOIN users u ON u.id=m.user_id WHERE o.name=`+s.arg(1)+` ORDER BY m.role,u.username`, name)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []OrganizationMember{}
	for rows.Next() {
		var m OrganizationMember
		if err := rows.Scan(&m.Username, &m.Role); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}
func (s *Store) OrganizationFollowers(ctx context.Context, name string) ([]FollowTarget, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT u.username FROM organization_follows f JOIN organizations o ON o.id=f.organization_id JOIN users u ON u.id=f.follower_id WHERE o.name=`+s.arg(1)+` ORDER BY u.username`, strings.ToLower(strings.TrimSpace(name)))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []FollowTarget{}
	for rows.Next() {
		var value FollowTarget
		if err := rows.Scan(&value.Name); err != nil {
			return nil, err
		}
		value.Type = "user"
		items = append(items, value)
	}
	return items, rows.Err()
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
	query := `INSERT INTO users (username, email, password_hash, is_admin, email_verified, created_at) VALUES (` + s.args(1, 2, 3, 4, 5, 6) + `)`
	verified := admin
	if s.driver == "pgsql" {
		query += " RETURNING id"
		var id int64
		err = s.db.QueryRowContext(ctx, query, username, email, string(hash), admin, verified, now).Scan(&id)
		if err == nil {
			return User{ID: id, Username: username, Email: email, IsAdmin: admin, EmailVerified: verified, CreatedAt: now}, nil
		}
	} else {
		result, insertErr := s.db.ExecContext(ctx, query, username, email, string(hash), admin, verified, now)
		err = insertErr
		if err == nil {
			id, _ := result.LastInsertId()
			return User{ID: id, Username: username, Email: email, IsAdmin: admin, EmailVerified: verified, CreatedAt: now}, nil
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
	err := s.db.QueryRowContext(ctx, `SELECT id, username, email, password_hash, is_admin, email_verified, created_at FROM users WHERE username = `+s.arg(1)+` OR email = `+s.arg(2), strings.ToLower(identity), strings.ToLower(identity)).Scan(&user.ID, &user.Username, &user.Email, &hash, &user.IsAdmin, &user.EmailVerified, &user.CreatedAt)
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

func (s *Store) CreatePersonalAccessToken(ctx context.Context, user User, name string, expiresAt *time.Time) (PersonalAccessTokenRegistration, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return PersonalAccessTokenRegistration{}, errors.New("token name is required")
	}
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return PersonalAccessTokenRegistration{}, err
	}
	token := "koh_" + hex.EncodeToString(raw)
	now := time.Now().UTC()
	id, err := s.insertID(ctx, `INSERT INTO personal_access_tokens (user_id,name,token_hash,created_at,expires_at) VALUES (`+s.args(1, 2, 3, 4, 5)+`)`, user.ID, name, tokenHash(token), now, expiresAt)
	if err != nil {
		return PersonalAccessTokenRegistration{}, err
	}
	return PersonalAccessTokenRegistration{PersonalAccessToken: PersonalAccessToken{ID: id, Name: name, CreatedAt: now, ExpiresAt: expiresAt}, Token: token}, nil
}
func (s *Store) PersonalAccessTokens(ctx context.Context, userID int64) ([]PersonalAccessToken, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,name,created_at,last_used_at,expires_at FROM personal_access_tokens WHERE user_id=`+s.arg(1)+` ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []PersonalAccessToken{}
	for rows.Next() {
		var x PersonalAccessToken
		if err := rows.Scan(&x.ID, &x.Name, &x.CreatedAt, &x.LastUsedAt, &x.ExpiresAt); err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}
func (s *Store) DeletePersonalAccessToken(ctx context.Context, userID, id int64) error {
	r, e := s.db.ExecContext(ctx, `DELETE FROM personal_access_tokens WHERE id=`+s.arg(1)+` AND user_id=`+s.arg(2), id, userID)
	if e != nil {
		return e
	}
	if n, _ := r.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}
func (s *Store) AuthenticatePersonalAccessToken(ctx context.Context, identity, token string) (User, error) {
	var user User
	err := s.db.QueryRowContext(ctx, `SELECT u.id,u.username,u.email,u.is_admin,u.email_verified,u.created_at FROM personal_access_tokens t JOIN users u ON u.id=t.user_id WHERE u.username=`+s.arg(1)+` AND t.token_hash=`+s.arg(2)+` AND (t.expires_at IS NULL OR t.expires_at>`+s.arg(3)+`)`, strings.ToLower(strings.TrimSpace(identity)), tokenHash(token), time.Now().UTC()).Scan(&user.ID, &user.Username, &user.Email, &user.IsAdmin, &user.EmailVerified, &user.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrUnauthorized
	}
	if err == nil {
		_, _ = s.db.ExecContext(ctx, `UPDATE personal_access_tokens SET last_used_at=`+s.arg(1)+` WHERE token_hash=`+s.arg(2), time.Now().UTC(), tokenHash(token))
	}
	return user, err
}

func (s *Store) OIDCProviders(ctx context.Context, includeDisabled bool) ([]OIDCProvider, error) {
	query := `SELECT id,slug,name,issuer_url,client_id,scopes,enabled,created_at,updated_at FROM oidc_providers`
	if !includeDisabled {
		query += ` WHERE enabled=` + s.arg(1)
	}
	query += ` ORDER BY name,slug`
	args := []any{}
	if !includeDisabled {
		args = append(args, true)
	}
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []OIDCProvider{}
	for rows.Next() {
		var item OIDCProvider
		if err := rows.Scan(&item.ID, &item.Slug, &item.Name, &item.IssuerURL, &item.ClientID, &item.Scopes, &item.Enabled, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) OIDCProvider(ctx context.Context, slug string) (OIDCProvider, error) {
	var item OIDCProvider
	err := s.db.QueryRowContext(ctx, `SELECT id,slug,name,issuer_url,client_id,client_secret,scopes,enabled,created_at,updated_at FROM oidc_providers WHERE slug=`+s.arg(1), strings.ToLower(strings.TrimSpace(slug))).Scan(&item.ID, &item.Slug, &item.Name, &item.IssuerURL, &item.ClientID, &item.ClientSecret, &item.Scopes, &item.Enabled, &item.CreatedAt, &item.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return OIDCProvider{}, ErrNotFound
	}
	return item, err
}
func (s *Store) OIDCProviderByID(ctx context.Context, id int64) (OIDCProvider, error) {
	var item OIDCProvider
	err := s.db.QueryRowContext(ctx, `SELECT id,slug,name,issuer_url,client_id,client_secret,scopes,enabled,created_at,updated_at FROM oidc_providers WHERE id=`+s.arg(1), id).Scan(&item.ID, &item.Slug, &item.Name, &item.IssuerURL, &item.ClientID, &item.ClientSecret, &item.Scopes, &item.Enabled, &item.CreatedAt, &item.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return OIDCProvider{}, ErrNotFound
	}
	return item, err
}

func (s *Store) SaveOIDCProvider(ctx context.Context, item OIDCProvider) (OIDCProvider, error) {
	item.Slug = strings.ToLower(strings.TrimSpace(item.Slug))
	item.Name = strings.TrimSpace(item.Name)
	item.IssuerURL = strings.TrimRight(strings.TrimSpace(item.IssuerURL), "/")
	item.ClientID = strings.TrimSpace(item.ClientID)
	item.Scopes = strings.TrimSpace(item.Scopes)
	if item.Slug == "" || !validScope(item.Slug) || item.Name == "" || item.IssuerURL == "" || item.ClientID == "" {
		return OIDCProvider{}, fmt.Errorf("OIDC provider slug, name, issuer URL, and client ID are required")
	}
	if item.Scopes == "" {
		item.Scopes = "openid profile email"
	}
	if item.ID == 0 && item.ClientSecret == "" {
		return OIDCProvider{}, fmt.Errorf("OIDC client secret is required")
	}
	now := time.Now().UTC()
	if item.ID == 0 {
		query := `INSERT INTO oidc_providers (slug,name,issuer_url,client_id,client_secret,scopes,enabled,created_at,updated_at) VALUES (` + s.args(1, 2, 3, 4, 5, 6, 7, 8, 9) + `)`
		if s.driver == "pgsql" {
			query += " RETURNING id"
			if err := s.db.QueryRowContext(ctx, query, item.Slug, item.Name, item.IssuerURL, item.ClientID, item.ClientSecret, item.Scopes, item.Enabled, now, now).Scan(&item.ID); err != nil {
				return OIDCProvider{}, err
			}
		} else {
			result, err := s.db.ExecContext(ctx, query, item.Slug, item.Name, item.IssuerURL, item.ClientID, item.ClientSecret, item.Scopes, item.Enabled, now, now)
			if err != nil {
				return OIDCProvider{}, err
			}
			item.ID, _ = result.LastInsertId()
		}
		item.CreatedAt, item.UpdatedAt = now, now
	} else {
		if item.ClientSecret == "" {
			var existing OIDCProvider
			existing, _ = s.OIDCProviderByID(ctx, item.ID)
			item.ClientSecret = existing.ClientSecret
		}
		result, err := s.db.ExecContext(ctx, `UPDATE oidc_providers SET slug=`+s.arg(1)+`,name=`+s.arg(2)+`,issuer_url=`+s.arg(3)+`,client_id=`+s.arg(4)+`,client_secret=`+s.arg(5)+`,scopes=`+s.arg(6)+`,enabled=`+s.arg(7)+`,updated_at=`+s.arg(8)+` WHERE id=`+s.arg(9), item.Slug, item.Name, item.IssuerURL, item.ClientID, item.ClientSecret, item.Scopes, item.Enabled, now, item.ID)
		if err != nil {
			return OIDCProvider{}, err
		}
		if affected, affectedErr := result.RowsAffected(); affectedErr == nil && affected == 0 {
			return OIDCProvider{}, ErrNotFound
		}
	}
	item.ClientSecret = ""
	return item, nil
}

func (s *Store) DeleteOIDCProvider(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM oidc_providers WHERE id=`+s.arg(1), id)
	return err
}
func (s *Store) SaveOIDCState(ctx context.Context, state string, providerID int64, redirectPath string, expires time.Time) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO oidc_states (state,provider_id,redirect_path,expires_at) VALUES (`+s.args(1, 2, 3, 4)+`)`, state, providerID, redirectPath, expires)
	return err
}
func (s *Store) ConsumeOIDCState(ctx context.Context, state string) (int64, string, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, "", err
	}
	defer tx.Rollback()
	var providerID int64
	var redirect string
	err = tx.QueryRowContext(ctx, `SELECT provider_id,redirect_path FROM oidc_states WHERE state=`+s.arg(1)+` AND expires_at > `+s.arg(2), state, time.Now().UTC()).Scan(&providerID, &redirect)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, "", ErrNotFound
	}
	if err != nil {
		return 0, "", err
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM oidc_states WHERE state=`+s.arg(1), state); err != nil {
		return 0, "", err
	}
	if err = tx.Commit(); err != nil {
		return 0, "", err
	}
	return providerID, redirect, nil
}

func (s *Store) OIDCUser(ctx context.Context, providerID int64, subject, email, displayName string, emailVerified, allowRegistration bool) (User, error) {
	var user User
	err := s.db.QueryRowContext(ctx, `SELECT u.id,u.username,u.email,u.is_admin,u.email_verified,u.created_at FROM oidc_identities i JOIN users u ON u.id=i.user_id WHERE i.provider_id=`+s.arg(1)+` AND i.subject=`+s.arg(2), providerID, subject).Scan(&user.ID, &user.Username, &user.Email, &user.IsAdmin, &user.EmailVerified, &user.CreatedAt)
	if err == nil {
		return user, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return User{}, err
	}
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		email = strings.ToLower(strings.TrimSpace(subject)) + "@oidc.invalid"
	}
	if emailVerified {
		_ = s.db.QueryRowContext(ctx, `SELECT id,username,email,is_admin,email_verified,created_at FROM users WHERE email=`+s.arg(1), email).Scan(&user.ID, &user.Username, &user.Email, &user.IsAdmin, &user.EmailVerified, &user.CreatedAt)
	}
	if user.ID == 0 {
		if !allowRegistration {
			return User{}, ErrForbidden
		}
		base := strings.ToLower(strings.TrimSpace(displayName))
		if base == "" {
			base = strings.Split(email, "@")[0]
		}
		base = sanitizeUsername(base)
		if len(base) < 3 {
			base = "oidc-user"
		}
		candidate := base
		for index := 2; ; index++ {
			var count int
			_ = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE username=`+s.arg(1), candidate).Scan(&count)
			if count == 0 {
				break
			}
			candidate = fmt.Sprintf("%s-%d", base, index)
		}
		raw := make([]byte, 24)
		if _, err := rand.Read(raw); err != nil {
			return User{}, err
		}
		hash, err := bcrypt.GenerateFromPassword(raw, bcrypt.DefaultCost)
		if err != nil {
			return User{}, err
		}
		now := time.Now().UTC()
		query := `INSERT INTO users (username,email,password_hash,is_admin,email_verified,created_at) VALUES (` + s.args(1, 2, 3, 4, 5, 6) + `)`
		if s.driver == "pgsql" {
			query += " RETURNING id"
			err = s.db.QueryRowContext(ctx, query, candidate, email, string(hash), false, true, now).Scan(&user.ID)
		} else {
			result, insertErr := s.db.ExecContext(ctx, query, candidate, email, string(hash), false, true, now)
			err = insertErr
			if err == nil {
				user.ID, _ = result.LastInsertId()
			}
		}
		if err != nil {
			return User{}, err
		}
		user = User{ID: user.ID, Username: candidate, Email: email, EmailVerified: emailVerified, CreatedAt: now}
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO oidc_identities (provider_id,subject,user_id,email,created_at) VALUES (`+s.args(1, 2, 3, 4, 5)+`)`, providerID, subject, user.ID, email, time.Now().UTC())
	if err != nil {
		return User{}, err
	}
	return user, nil
}

func sanitizeUsername(value string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(value) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			b.WriteRune(r)
		}
	}
	return b.String()
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
	err := s.db.QueryRowContext(ctx, `SELECT u.id,u.username,u.email,u.is_admin,u.email_verified,u.created_at FROM sessions s JOIN users u ON u.id=s.user_id WHERE s.token_hash=`+s.arg(1)+` AND s.expires_at > `+s.arg(2), tokenHash(token), time.Now().UTC()).Scan(&user.ID, &user.Username, &user.Email, &user.IsAdmin, &user.EmailVerified, &user.CreatedAt)
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
		case "repository_root":
			settings.RepositoryRoot = value
		case "workflow_directory":
			settings.WorkflowDirectory = value
		case "runner_enabled":
			settings.RunnerEnabled = value == "true"
		case "runner_url":
			settings.RunnerURL = value
		case "runner_token":
			settings.RunnerToken = value
		case "captcha_enabled":
			settings.CaptchaEnabled = value == "true"
		case "captcha_site_key":
			settings.CaptchaSiteKey = value
		case "captcha_secret":
			settings.CaptchaSecret = value
		case "smtp_host":
			settings.SMTPHost = value
		case "smtp_port":
			settings.SMTPPort = value
		case "smtp_username":
			settings.SMTPUsername = value
		case "smtp_password":
			settings.SMTPPassword = value
		case "smtp_from":
			settings.SMTPFrom = value
		case "gravatar_mirror":
			settings.GravatarMirror = value
		}
	}
	return settings, rows.Err()
}
func (s *Store) UpdateSettings(ctx context.Context, value SiteSettings) error {
	if strings.TrimSpace(value.Title) == "" {
		return fmt.Errorf("site title is required")
	}
	workflowDirectory := strings.TrimSpace(value.WorkflowDirectory)
	if workflowDirectory == "" {
		workflowDirectory = "/.kohame/workflow"
	}
	workflowDirectory = strings.Trim(workflowDirectory, "/")
	if workflowDirectory == "" || workflowDirectory == "." || strings.Contains(workflowDirectory, "..") || strings.ContainsAny(workflowDirectory, `\\`) {
		return fmt.Errorf("workflow directory must be a repository-relative path")
	}
	runnerURL := strings.TrimRight(strings.TrimSpace(value.RunnerURL), "/")
	if value.RunnerEnabled && runnerURL == "" {
		return fmt.Errorf("runner URL is required when runner is enabled")
	}
	items := map[string]string{"site_title": strings.TrimSpace(value.Title), "site_description": strings.TrimSpace(value.Description), "allow_registration": fmt.Sprintf("%t", value.AllowRegistration), "repository_root": strings.TrimSpace(value.RepositoryRoot), "workflow_directory": "/" + workflowDirectory, "runner_enabled": fmt.Sprintf("%t", value.RunnerEnabled), "runner_url": runnerURL, "runner_token": strings.TrimSpace(value.RunnerToken), "captcha_enabled": fmt.Sprintf("%t", value.CaptchaEnabled), "captcha_site_key": strings.TrimSpace(value.CaptchaSiteKey), "captcha_secret": strings.TrimSpace(value.CaptchaSecret), "smtp_host": strings.TrimSpace(value.SMTPHost), "smtp_port": strings.TrimSpace(value.SMTPPort), "smtp_username": strings.TrimSpace(value.SMTPUsername), "smtp_password": strings.TrimSpace(value.SMTPPassword), "smtp_from": strings.TrimSpace(value.SMTPFrom), "gravatar_mirror": strings.TrimSpace(value.GravatarMirror)}
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
	for index := range out {
		out[index].Labels, _ = s.IssueLabels(ctx, out[index].ID)
		out[index].Assignees, _ = s.IssueAssignees(ctx, repo, out[index].ID)
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
	return Issue{ID: id, Title: strings.TrimSpace(title), Body: strings.TrimSpace(body), State: "open", Author: user.Username, CreatedAt: now, UpdatedAt: now, Labels: []Label{}}, nil
}
func (s *Store) Issue(ctx context.Context, repo string, id int64) (Issue, error) {
	var item Issue
	err := s.db.QueryRowContext(ctx, `SELECT i.id,i.title,i.body,i.state,u.username,i.created_at,i.updated_at FROM issues i JOIN users u ON u.id=i.author_id WHERE i.repository_name=`+s.arg(1)+` AND i.id=`+s.arg(2), repo, id).Scan(&item.ID, &item.Title, &item.Body, &item.State, &item.Author, &item.CreatedAt, &item.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Issue{}, ErrNotFound
	}
	if err == nil {
		item.Labels, _ = s.IssueLabels(ctx, item.ID)
		item.Assignees, _ = s.IssueAssignees(ctx, repo, item.ID)
	}
	return item, err
}
func (s *Store) Labels(ctx context.Context, repo string) ([]Label, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,name,color,description FROM labels WHERE repository_name=`+s.arg(1)+` ORDER BY name`, repo)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Label{}
	for rows.Next() {
		var label Label
		if err := rows.Scan(&label.ID, &label.Name, &label.Color, &label.Description); err != nil {
			return nil, err
		}
		out = append(out, label)
	}
	return out, rows.Err()
}
func (s *Store) CreateLabel(ctx context.Context, repo, name, color, description string) (Label, error) {
	name = strings.TrimSpace(name)
	color = strings.TrimSpace(color)
	if name == "" || len(name) > 80 || len(color) != 7 || color[0] != '#' {
		return Label{}, fmt.Errorf("label name or color is invalid")
	}
	q := `INSERT INTO labels (repository_name,name,color,description) VALUES (` + s.args(1, 2, 3, 4) + `)`
	id, err := s.insertID(ctx, q, repo, name, color, strings.TrimSpace(description))
	if err != nil {
		return Label{}, err
	}
	return Label{ID: id, Name: name, Color: color, Description: strings.TrimSpace(description)}, nil
}
func (s *Store) UpdateLabel(ctx context.Context, repo string, id int64, name, color, description string) (Label, error) {
	name, color, description = strings.TrimSpace(name), strings.TrimSpace(color), strings.TrimSpace(description)
	if name == "" || len(name) > 80 || len(color) != 7 || color[0] != '#' {
		return Label{}, fmt.Errorf("label name or color is invalid")
	}
	result, err := s.db.ExecContext(ctx, `UPDATE labels SET name=`+s.arg(1)+`,color=`+s.arg(2)+`,description=`+s.arg(3)+` WHERE id=`+s.arg(4)+` AND repository_name=`+s.arg(5), name, color, description, id, repo)
	if err != nil {
		return Label{}, err
	}
	count, _ := result.RowsAffected()
	if count == 0 {
		return Label{}, ErrNotFound
	}
	return Label{ID: id, Name: name, Color: color, Description: description}, nil
}
func (s *Store) DeleteLabel(ctx context.Context, repo string, id int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `DELETE FROM issue_labels WHERE label_id=`+s.arg(1), id); err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, `DELETE FROM labels WHERE id=`+s.arg(1)+` AND repository_name=`+s.arg(2), id, repo)
	if err != nil {
		return err
	}
	count, _ := result.RowsAffected()
	if count == 0 {
		return ErrNotFound
	}
	return tx.Commit()
}
func (s *Store) IssueLabels(ctx context.Context, issueID int64) ([]Label, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT l.id,l.name,l.color,l.description FROM labels l JOIN issue_labels il ON il.label_id=l.id WHERE il.issue_id=`+s.arg(1)+` ORDER BY l.name`, issueID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Label{}
	for rows.Next() {
		var label Label
		if err := rows.Scan(&label.ID, &label.Name, &label.Color, &label.Description); err != nil {
			return nil, err
		}
		out = append(out, label)
	}
	return out, rows.Err()
}
func (s *Store) SetIssueLabels(ctx context.Context, repo string, issueID int64, labelIDs []int64) error {
	if _, err := s.Issue(ctx, repo, issueID); err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `DELETE FROM issue_labels WHERE issue_id=`+s.arg(1), issueID); err != nil {
		return err
	}
	for _, id := range labelIDs {
		var count int
		if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM labels WHERE id=`+s.arg(1)+` AND repository_name=`+s.arg(2), id, repo).Scan(&count); err != nil || count == 0 {
			return ErrNotFound
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO issue_labels (issue_id,label_id) VALUES (`+s.args(1, 2)+`)`, issueID, id); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) IssueAssignees(ctx context.Context, repo string, issueID int64) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT u.username FROM issue_assignees a JOIN users u ON u.id=a.user_id JOIN issues i ON i.id=a.issue_id WHERE i.repository_name=`+s.arg(1)+` AND a.issue_id=`+s.arg(2)+` ORDER BY u.username`, repo, issueID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []string{}
	for rows.Next() {
		var username string
		if err := rows.Scan(&username); err != nil {
			return nil, err
		}
		items = append(items, username)
	}
	return items, rows.Err()
}

func (s *Store) SetIssueAssignees(ctx context.Context, repo string, issueID int64, usernames []string) error {
	if _, err := s.Issue(ctx, repo, issueID); err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM issue_assignees WHERE issue_id=`+s.arg(1), issueID); err != nil {
		return err
	}
	for _, username := range usernames {
		var userID int64
		err := tx.QueryRowContext(ctx, `SELECT id FROM users WHERE username=`+s.arg(1), strings.ToLower(strings.TrimSpace(username))).Scan(&userID)
		if errors.Is(err, sql.ErrNoRows) {
			return errors.New("assignee not found")
		}
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO issue_assignees (issue_id,user_id) VALUES (`+s.args(1, 2)+`) ON CONFLICT(issue_id,user_id) DO NOTHING`, issueID, userID); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) Milestones(ctx context.Context, repo string) ([]Milestone, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,title,description,due_at,state,created_at FROM milestones WHERE repository_name=`+s.arg(1)+` ORDER BY state,due_at,title`, repo)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Milestone{}
	for rows.Next() {
		var item Milestone
		if err := rows.Scan(&item.ID, &item.Title, &item.Description, &item.DueAt, &item.State, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) SaveMilestone(ctx context.Context, repo string, item Milestone) (Milestone, error) {
	item.Title = strings.TrimSpace(item.Title)
	if item.Title == "" || (item.State != "" && item.State != "open" && item.State != "closed") {
		return Milestone{}, errors.New("invalid milestone")
	}
	if item.State == "" {
		item.State = "open"
	}
	now := time.Now().UTC()
	if item.ID == 0 {
		id, err := s.insertID(ctx, `INSERT INTO milestones (repository_name,title,description,due_at,state,created_at) VALUES (`+s.args(1, 2, 3, 4, 5, 6)+`)`, repo, item.Title, strings.TrimSpace(item.Description), item.DueAt, item.State, now)
		item.ID = id
		item.CreatedAt = now
		return item, err
	}
	result, err := s.db.ExecContext(ctx, `UPDATE milestones SET title=`+s.arg(1)+`,description=`+s.arg(2)+`,due_at=`+s.arg(3)+`,state=`+s.arg(4)+` WHERE id=`+s.arg(5)+` AND repository_name=`+s.arg(6), item.Title, strings.TrimSpace(item.Description), item.DueAt, item.State, item.ID, repo)
	if err != nil {
		return item, err
	}
	if n, _ := result.RowsAffected(); n == 0 {
		return item, ErrNotFound
	}
	return item, nil
}
func (s *Store) ListIssueComments(ctx context.Context, repo string, issueID int64) ([]IssueComment, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT c.id,u.username,c.body,c.created_at FROM issue_comments c JOIN users u ON u.id=c.author_id WHERE c.repository_name=`+s.arg(1)+` AND c.issue_id=`+s.arg(2)+` ORDER BY c.created_at ASC`, repo, issueID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	comments := make([]IssueComment, 0)
	for rows.Next() {
		var item IssueComment
		if err := rows.Scan(&item.ID, &item.Author, &item.Body, &item.CreatedAt); err != nil {
			return nil, err
		}
		comments = append(comments, item)
	}
	return comments, rows.Err()
}
func (s *Store) CreateIssueComment(ctx context.Context, repo string, issueID int64, user User, body string) (IssueComment, error) {
	body = strings.TrimSpace(body)
	if body == "" {
		return IssueComment{}, fmt.Errorf("comment body is required")
	}
	if _, err := s.Issue(ctx, repo, issueID); err != nil {
		return IssueComment{}, err
	}
	now := time.Now().UTC()
	query := `INSERT INTO issue_comments (repository_name,issue_id,author_id,body,created_at) VALUES (` + s.args(1, 2, 3, 4, 5) + `)`
	id, err := s.insertID(ctx, query, repo, issueID, user.ID, body, now)
	if err != nil {
		return IssueComment{}, err
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE issues SET updated_at=`+s.arg(1)+` WHERE repository_name=`+s.arg(2)+` AND id=`+s.arg(3), now, repo, issueID); err != nil {
		return IssueComment{}, err
	}
	_ = s.activity(ctx, repo, user.ID, "issue_comment")
	return IssueComment{ID: id, Author: user.Username, Body: body, CreatedAt: now}, nil
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

func (s *Store) PullRequest(ctx context.Context, repo string, id int64) (PullRequest, error) {
	var value PullRequest
	err := s.db.QueryRowContext(ctx, `SELECT p.id,p.title,p.body,p.source_branch,p.target_branch,p.state,u.username,p.created_at,p.updated_at FROM pull_requests p JOIN users u ON u.id=p.author_id WHERE p.repository_name=`+s.arg(1)+` AND p.id=`+s.arg(2), repo, id).Scan(&value.ID, &value.Title, &value.Body, &value.SourceBranch, &value.TargetBranch, &value.State, &value.Author, &value.CreatedAt, &value.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return PullRequest{}, ErrNotFound
	}
	return value, err
}
func (s *Store) ListPullRequestComments(ctx context.Context, repo string, pullID int64) ([]PullRequestComment, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT c.id,u.username,c.body,c.created_at FROM pull_request_comments c JOIN users u ON u.id=c.author_id WHERE c.repository_name=`+s.arg(1)+` AND c.pull_request_id=`+s.arg(2)+` ORDER BY c.created_at ASC`, repo, pullID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []PullRequestComment{}
	for rows.Next() {
		var item PullRequestComment
		if err := rows.Scan(&item.ID, &item.Author, &item.Body, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
func (s *Store) CreatePullRequestComment(ctx context.Context, repo string, pullID int64, user User, body string) (PullRequestComment, error) {
	body = strings.TrimSpace(body)
	if body == "" {
		return PullRequestComment{}, fmt.Errorf("comment body is required")
	}
	if _, err := s.PullRequest(ctx, repo, pullID); err != nil {
		return PullRequestComment{}, err
	}
	now := time.Now().UTC()
	query := `INSERT INTO pull_request_comments (repository_name,pull_request_id,author_id,body,created_at) VALUES (` + s.args(1, 2, 3, 4, 5) + `)`
	id, err := s.insertID(ctx, query, repo, pullID, user.ID, body, now)
	if err != nil {
		return PullRequestComment{}, err
	}
	if _, err = s.db.ExecContext(ctx, `UPDATE pull_requests SET updated_at=`+s.arg(1)+` WHERE repository_name=`+s.arg(2)+` AND id=`+s.arg(3), now, repo, pullID); err != nil {
		return PullRequestComment{}, err
	}
	_ = s.activity(ctx, repo, user.ID, "pull_request_comment")
	return PullRequestComment{ID: id, Author: user.Username, Body: body, CreatedAt: now}, nil
}

func (s *Store) PullRequestReviews(ctx context.Context, repo string, pullID int64) ([]PullRequestReview, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT r.id,u.username,r.state,r.body,r.created_at,r.updated_at FROM pull_request_reviews r JOIN users u ON u.id=r.reviewer_id WHERE r.repository_name=`+s.arg(1)+` AND r.pull_request_id=`+s.arg(2)+` ORDER BY r.updated_at DESC`, repo, pullID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []PullRequestReview{}
	for rows.Next() {
		var item PullRequestReview
		if err := rows.Scan(&item.ID, &item.Reviewer, &item.State, &item.Body, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) SavePullRequestReview(ctx context.Context, repo string, pullID int64, user User, state, body string) (PullRequestReview, error) {
	state = strings.TrimSpace(strings.ToLower(state))
	if state != "approved" && state != "changes_requested" && state != "commented" {
		return PullRequestReview{}, errors.New("review state must be approved, changes_requested, or commented")
	}
	pull, err := s.PullRequest(ctx, repo, pullID)
	if err != nil {
		return PullRequestReview{}, err
	}
	if pull.Author == user.Username && state == "approved" {
		return PullRequestReview{}, errors.New("pull request authors cannot approve their own changes")
	}
	now := time.Now().UTC()
	query := `INSERT INTO pull_request_reviews (repository_name,pull_request_id,reviewer_id,state,body,created_at,updated_at) VALUES (` + s.args(1, 2, 3, 4, 5, 6, 7) + `) ON CONFLICT(pull_request_id,reviewer_id) DO UPDATE SET state=EXCLUDED.state,body=EXCLUDED.body,updated_at=EXCLUDED.updated_at`
	if _, err := s.db.ExecContext(ctx, query, repo, pullID, user.ID, state, strings.TrimSpace(body), now, now); err != nil {
		return PullRequestReview{}, err
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE pull_requests SET updated_at=`+s.arg(1)+` WHERE repository_name=`+s.arg(2)+` AND id=`+s.arg(3), now, repo, pullID); err != nil {
		return PullRequestReview{}, err
	}
	return PullRequestReview{Reviewer: user.Username, State: state, Body: strings.TrimSpace(body), CreatedAt: now, UpdatedAt: now}, nil
}

func (s *Store) PullRequestApprovalCount(ctx context.Context, repo string, pullID int64) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM pull_request_reviews WHERE repository_name=`+s.arg(1)+` AND pull_request_id=`+s.arg(2)+` AND state='approved'`, repo, pullID).Scan(&count)
	return count, err
}

func (s *Store) DeletePullRequest(ctx context.Context, repo string, id int64) error {
	_, _ = s.db.ExecContext(ctx, `DELETE FROM pull_request_comments WHERE repository_name=`+s.arg(1)+` AND pull_request_id=`+s.arg(2), repo, id)
	_, _ = s.db.ExecContext(ctx, `DELETE FROM pull_request_reviews WHERE repository_name=`+s.arg(1)+` AND pull_request_id=`+s.arg(2), repo, id)
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
	for index := range out {
		out[index].Assets, _ = s.ReleaseAssets(ctx, out[index].ID)
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
	return Release{ID: id, TagName: strings.TrimSpace(tag), Title: strings.TrimSpace(title), Notes: strings.TrimSpace(notes), Author: user.Username, CreatedAt: now, Assets: []ReleaseAsset{}}, nil
}

func (s *Store) ReleaseAssets(ctx context.Context, releaseID int64) ([]ReleaseAsset, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,file_name,storage_name,size FROM release_assets WHERE release_id=`+s.arg(1)+` ORDER BY id`, releaseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ReleaseAsset{}
	for rows.Next() {
		var asset ReleaseAsset
		if err := rows.Scan(&asset.ID, &asset.FileName, &asset.StorageName, &asset.Size); err != nil {
			return nil, err
		}
		out = append(out, asset)
	}
	return out, rows.Err()
}
func (s *Store) AddReleaseAsset(ctx context.Context, releaseID int64, fileName, storageName string, size int64) (ReleaseAsset, error) {
	q := `INSERT INTO release_assets (release_id,file_name,storage_name,size,created_at) VALUES (` + s.args(1, 2, 3, 4, 5) + `)`
	id, err := s.insertID(ctx, q, releaseID, strings.TrimSpace(fileName), storageName, size, time.Now().UTC())
	if err != nil {
		return ReleaseAsset{}, err
	}
	return ReleaseAsset{ID: id, FileName: fileName, StorageName: storageName, Size: size}, nil
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

func (s *Store) CreateEmailVerification(ctx context.Context, userID int64) (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	token := hex.EncodeToString(raw)
	_, err := s.db.ExecContext(ctx, `DELETE FROM email_verifications WHERE user_id=`+s.arg(1), userID)
	if err != nil {
		return "", err
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO email_verifications (token_hash,user_id,expires_at) VALUES (`+s.args(1, 2, 3)+`)`, tokenHash(token), userID, time.Now().UTC().Add(24*time.Hour))
	return token, err
}

func (s *Store) VerifyEmail(ctx context.Context, token string) error {
	if len(token) != 64 {
		return ErrNotFound
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var userID int64
	err = tx.QueryRowContext(ctx, `SELECT user_id FROM email_verifications WHERE token_hash=`+s.arg(1)+` AND expires_at > `+s.arg(2), tokenHash(token), time.Now().UTC()).Scan(&userID)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE users SET email_verified=`+s.arg(1)+` WHERE id=`+s.arg(2), true, userID); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM email_verifications WHERE user_id=`+s.arg(1), userID); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) UserByUsername(ctx context.Context, username string) (User, error) {
	var user User
	err := s.db.QueryRowContext(ctx, `SELECT id,username,email,is_admin,email_verified,created_at FROM users WHERE username=`+s.arg(1), strings.ToLower(strings.TrimSpace(username))).Scan(&user.ID, &user.Username, &user.Email, &user.IsAdmin, &user.EmailVerified, &user.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrNotFound
	}
	return user, err
}

func (s *Store) Profile(ctx context.Context, username string) (Profile, error) {
	user, err := s.UserByUsername(ctx, username)
	if err != nil {
		return Profile{}, err
	}
	p := Profile{Username: user.Username, CreatedAt: user.CreatedAt}
	_ = s.db.QueryRowContext(ctx, `SELECT display_name,bio,location,website,avatar_url FROM user_settings WHERE user_id=`+s.arg(1), user.ID).Scan(&p.DisplayName, &p.Bio, &p.Location, &p.Website, &p.AvatarURL)
	_ = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM repositories WHERE name LIKE `+s.arg(1), user.Username+"/%").Scan(&p.Repositories)
	_ = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM repository_stars WHERE user_id=`+s.arg(1), user.ID).Scan(&p.Stars)
	_ = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM user_follows WHERE target_user_id=`+s.arg(1), user.ID).Scan(&p.Followers)
	_ = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM user_follows WHERE follower_id=`+s.arg(1), user.ID).Scan(&p.Following)
	if p.AvatarURL == "" {
		p.AvatarURL = s.gravatarURL(ctx, user.Email)
	}
	return p, nil
}

func (s *Store) PersonalSettings(ctx context.Context, user User) (PersonalSettings, error) {
	value := PersonalSettings{Username: user.Username, Email: user.Email, Verified: user.EmailVerified}
	err := s.db.QueryRowContext(ctx, `SELECT display_name,bio,location,website,avatar_url FROM user_settings WHERE user_id=`+s.arg(1), user.ID).Scan(&value.DisplayName, &value.Bio, &value.Location, &value.Website, &value.AvatarURL)
	if errors.Is(err, sql.ErrNoRows) {
		value.AvatarURL = s.gravatarURL(ctx, user.Email)
		return value, nil
	}
	if err == nil && value.AvatarURL == "" {
		value.AvatarURL = s.gravatarURL(ctx, user.Email)
	}
	return value, err
}

func (s *Store) UpdatePersonalSettings(ctx context.Context, user User, value PersonalSettings) (PersonalSettings, error) {
	value.DisplayName = strings.TrimSpace(value.DisplayName)
	value.Bio = strings.TrimSpace(value.Bio)
	value.Location = strings.TrimSpace(value.Location)
	value.Website = strings.TrimSpace(value.Website)
	if len(value.DisplayName) > 80 || len(value.Bio) > 500 || len(value.Location) > 120 || len(value.Website) > 255 {
		return PersonalSettings{}, fmt.Errorf("profile fields are too long")
	}
	if value.Website != "" && !(strings.HasPrefix(value.Website, "https://") || strings.HasPrefix(value.Website, "http://")) {
		return PersonalSettings{}, fmt.Errorf("website must start with http:// or https://")
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO user_settings (user_id,display_name,bio,location,website,avatar_url) VALUES (`+s.args(1, 2, 3, 4, 5, 6)+`) ON CONFLICT (user_id) DO UPDATE SET display_name=EXCLUDED.display_name,bio=EXCLUDED.bio,location=EXCLUDED.location,website=EXCLUDED.website,avatar_url=EXCLUDED.avatar_url`, user.ID, value.DisplayName, value.Bio, value.Location, value.Website, value.AvatarURL)
	if err != nil {
		return PersonalSettings{}, err
	}
	value.Username, value.Email, value.Verified = user.Username, user.Email, user.EmailVerified
	return value, nil
}

func (s *Store) SetAvatar(ctx context.Context, user User, avatarURL string) (PersonalSettings, error) {
	value, err := s.PersonalSettings(ctx, user)
	if err != nil {
		return PersonalSettings{}, err
	}
	value.AvatarURL = avatarURL
	return s.UpdatePersonalSettings(ctx, user, value)
}
func (s *Store) SSHKeys(ctx context.Context, userID int64) ([]SSHKey, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,title,key_value,created_at FROM ssh_keys WHERE user_id=`+s.arg(1)+` ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []SSHKey{}
	for rows.Next() {
		var key SSHKey
		if err := rows.Scan(&key.ID, &key.Title, &key.Key, &key.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, key)
	}
	return out, rows.Err()
}
func (s *Store) AddSSHKey(ctx context.Context, user User, title, value string) (SSHKey, error) {
	title = strings.TrimSpace(title)
	value = strings.TrimSpace(value)
	if title == "" || value == "" {
		return SSHKey{}, fmt.Errorf("key title and public key are required")
	}
	q := `INSERT INTO ssh_keys (user_id,title,key_value,created_at) VALUES (` + s.args(1, 2, 3, 4) + `)`
	id, err := s.insertID(ctx, q, user.ID, title, value, time.Now().UTC())
	if err != nil {
		return SSHKey{}, err
	}
	return SSHKey{ID: id, Title: title, Key: value, CreatedAt: time.Now().UTC()}, nil
}
func (s *Store) DeleteSSHKey(ctx context.Context, userID, id int64) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM ssh_keys WHERE id=`+s.arg(1)+` AND user_id=`+s.arg(2), id, userID)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
func (s *Store) UserBySSHKey(ctx context.Context, value string) (User, error) {
	var user User
	err := s.db.QueryRowContext(ctx, `SELECT u.id,u.username,u.email,u.is_admin,u.email_verified,u.created_at FROM ssh_keys k JOIN users u ON u.id=k.user_id WHERE k.key_value=`+s.arg(1), strings.TrimSpace(value)).Scan(&user.ID, &user.Username, &user.Email, &user.IsAdmin, &user.EmailVerified, &user.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrNotFound
	}
	return user, err
}
func (s *Store) ToggleUserFollow(ctx context.Context, follower User, username string) (bool, int, error) {
	target, err := s.UserByUsername(ctx, username)
	if err != nil {
		return false, 0, err
	}
	if target.ID == follower.ID {
		return false, 0, fmt.Errorf("cannot follow yourself")
	}
	var exists int
	if err = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM user_follows WHERE follower_id=`+s.arg(1)+` AND target_user_id=`+s.arg(2), follower.ID, target.ID).Scan(&exists); err != nil {
		return false, 0, err
	}
	followed := exists == 0
	if followed {
		_, err = s.db.ExecContext(ctx, `INSERT INTO user_follows (follower_id,target_user_id,created_at) VALUES (`+s.args(1, 2, 3)+`)`, follower.ID, target.ID, time.Now().UTC())
	} else {
		_, err = s.db.ExecContext(ctx, `DELETE FROM user_follows WHERE follower_id=`+s.arg(1)+` AND target_user_id=`+s.arg(2), follower.ID, target.ID)
	}
	if err != nil {
		return false, 0, err
	}
	var count int
	err = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM user_follows WHERE target_user_id=`+s.arg(1), target.ID).Scan(&count)
	return followed, count, err
}
func (s *Store) UserFollowed(ctx context.Context, followerID int64, username string) (bool, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM user_follows f JOIN users u ON u.id=f.target_user_id WHERE f.follower_id=`+s.arg(1)+` AND u.username=`+s.arg(2), followerID, strings.ToLower(strings.TrimSpace(username))).Scan(&count)
	return count > 0, err
}
func (s *Store) UserFollowers(ctx context.Context, username string) ([]FollowTarget, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT follower.username FROM user_follows f JOIN users target ON target.id=f.target_user_id JOIN users follower ON follower.id=f.follower_id WHERE target.username=`+s.arg(1)+` ORDER BY follower.username`, strings.ToLower(strings.TrimSpace(username)))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []FollowTarget{}
	for rows.Next() {
		var value FollowTarget
		if err := rows.Scan(&value.Name); err != nil {
			return nil, err
		}
		value.Type = "user"
		items = append(items, value)
	}
	return items, rows.Err()
}
func (s *Store) UserFollowing(ctx context.Context, username string) ([]FollowTarget, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT u.username,'user' FROM user_follows f JOIN users source ON source.id=f.follower_id JOIN users u ON u.id=f.target_user_id WHERE source.username=`+s.arg(1)+` UNION ALL SELECT o.name,'organization' FROM organization_follows f JOIN users source ON source.id=f.follower_id JOIN organizations o ON o.id=f.organization_id WHERE source.username=`+s.arg(1)+` ORDER BY 1`, strings.ToLower(strings.TrimSpace(username)))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []FollowTarget{}
	for rows.Next() {
		var value FollowTarget
		if err := rows.Scan(&value.Name, &value.Type); err != nil {
			return nil, err
		}
		items = append(items, value)
	}
	return items, rows.Err()
}
func (s *Store) gravatarURL(ctx context.Context, email string) string {
	settings, err := s.Settings(ctx)
	mirror := "https://www.gravatar.com/avatar/"
	if err == nil && strings.TrimSpace(settings.GravatarMirror) != "" {
		mirror = strings.TrimRight(strings.TrimSpace(settings.GravatarMirror), "/") + "/"
	}
	sum := md5.Sum([]byte(strings.ToLower(strings.TrimSpace(email))))
	return mirror + hex.EncodeToString(sum[:]) + "?d=identicon"
}

func (s *Store) SearchProfiles(ctx context.Context, query string) ([]Profile, error) {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return []Profile{}, nil
	}
	rows, err := s.db.QueryContext(ctx, `SELECT username,created_at FROM users WHERE LOWER(username) LIKE `+s.arg(1)+` ORDER BY username LIMIT 20`, "%"+query+"%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Profile{}
	for rows.Next() {
		var p Profile
		if err := rows.Scan(&p.Username, &p.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *Store) AddNotification(ctx context.Context, userID int64, kind, title, body, link string) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO notifications (user_id,kind,title,body,link,is_read,created_at) VALUES (`+s.args(1, 2, 3, 4, 5, 6, 7)+`)`, userID, kind, title, body, link, false, time.Now().UTC())
	return err
}

func (s *Store) Notifications(ctx context.Context, userID int64) ([]Notification, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,kind,title,body,link,is_read,created_at FROM notifications WHERE user_id=`+s.arg(1)+` ORDER BY created_at DESC LIMIT 50`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Notification{}
	for rows.Next() {
		var n Notification
		if err := rows.Scan(&n.ID, &n.Kind, &n.Title, &n.Body, &n.Link, &n.IsRead, &n.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

func (s *Store) ReadNotification(ctx context.Context, userID, id int64) error {
	_, err := s.db.ExecContext(ctx, `UPDATE notifications SET is_read=`+s.arg(1)+` WHERE id=`+s.arg(2)+` AND user_id=`+s.arg(3), true, id, userID)
	return err
}

func (s *Store) CreateRunner(ctx context.Context, user User, name string) (RunnerRegistration, error) {
	name = strings.TrimSpace(name)
	if name == "" || len(name) > 120 {
		return RunnerRegistration{}, errors.New("runner name is required and must be at most 120 characters")
	}
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return RunnerRegistration{}, err
	}
	token := hex.EncodeToString(raw)
	now := time.Now().UTC()
	query := `INSERT INTO runners (owner_id,name,token_hash,created_at) VALUES (` + s.args(1, 2, 3, 4) + `)`
	id, err := s.insertID(ctx, query, user.ID, name, tokenHash(token), now)
	if err != nil {
		if isUnique(err) {
			return RunnerRegistration{}, ErrConflict
		}
		return RunnerRegistration{}, err
	}
	return RunnerRegistration{Runner: Runner{ID: id, Name: name, CreatedAt: now}, Token: token}, nil
}

func (s *Store) Runners(ctx context.Context, userID int64) ([]Runner, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,name,created_at,last_seen_at FROM runners WHERE owner_id=`+s.arg(1)+` ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Runner{}
	for rows.Next() {
		var item Runner
		if err := rows.Scan(&item.ID, &item.Name, &item.CreatedAt, &item.LastSeenAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) DeleteRunner(ctx context.Context, userID, id int64) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM runners WHERE id=`+s.arg(1)+` AND owner_id=`+s.arg(2), id, userID)
	if err != nil {
		return err
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) runnerByToken(ctx context.Context, token string) (int64, error) {
	var id int64
	err := s.db.QueryRowContext(ctx, `SELECT id FROM runners WHERE token_hash=`+s.arg(1), tokenHash(token)).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrUnauthorized
	}
	return id, err
}

func (s *Store) EnqueueWorkflowJob(ctx context.Context, runnerID, runID int64, repository, workflow, event string, workspace []byte, steps []WorkflowStep) error {
	encoded, err := json.Marshal(steps)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO workflow_jobs (workflow_run_id,runner_id,repository_name,workflow_name,event,workspace,steps,status) VALUES (`+s.args(1, 2, 3, 4, 5, 6, 7, 8)+`)`, runID, runnerID, repository, workflow, event, workspace, string(encoded), "queued")
	return err
}

func (s *Store) ClaimWorkflowJob(ctx context.Context, token string) (RunnerJob, bool, error) {
	runnerID, err := s.runnerByToken(ctx, token)
	if err != nil {
		return RunnerJob{}, false, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return RunnerJob{}, false, err
	}
	defer tx.Rollback()
	var job RunnerJob
	var steps string
	err = tx.QueryRowContext(ctx, `SELECT id,repository_name,workflow_name,event,workspace,steps FROM workflow_jobs WHERE runner_id=`+s.arg(1)+` AND status='queued' ORDER BY id LIMIT 1`, runnerID).Scan(&job.ID, &job.Repository, &job.Workflow, &job.Event, &job.Workspace, &steps)
	if errors.Is(err, sql.ErrNoRows) {
		_, _ = tx.ExecContext(ctx, `UPDATE runners SET last_seen_at=`+s.arg(1)+` WHERE id=`+s.arg(2), time.Now().UTC(), runnerID)
		return RunnerJob{}, false, tx.Commit()
	}
	if err != nil {
		return RunnerJob{}, false, err
	}
	if err := json.Unmarshal([]byte(steps), &job.Steps); err != nil {
		return RunnerJob{}, false, err
	}
	result, err := tx.ExecContext(ctx, `UPDATE workflow_jobs SET status='running',claimed_at=`+s.arg(1)+` WHERE id=`+s.arg(2)+` AND runner_id=`+s.arg(3)+` AND status='queued'`, time.Now().UTC(), job.ID, runnerID)
	if err != nil {
		return RunnerJob{}, false, err
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return RunnerJob{}, false, tx.Commit()
	}
	_, _ = tx.ExecContext(ctx, `UPDATE runners SET last_seen_at=`+s.arg(1)+` WHERE id=`+s.arg(2), time.Now().UTC(), runnerID)
	return job, true, tx.Commit()
}

func (s *Store) CompleteWorkflowJob(ctx context.Context, token string, jobID int64, status, output string) error {
	if status != "success" && status != "failure" {
		return errors.New("job status must be success or failure")
	}
	runnerID, err := s.runnerByToken(ctx, token)
	if err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var runID int64
	err = tx.QueryRowContext(ctx, `SELECT workflow_run_id FROM workflow_jobs WHERE id=`+s.arg(1)+` AND runner_id=`+s.arg(2)+` AND status='running'`, jobID, runnerID).Scan(&runID)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	if _, err := tx.ExecContext(ctx, `UPDATE workflow_jobs SET status='completed',completed_at=`+s.arg(1)+` WHERE id=`+s.arg(2), now, jobID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE workflow_runs SET status=`+s.arg(1)+`,output=`+s.arg(2)+`,finished_at=`+s.arg(3)+` WHERE id=`+s.arg(4), status, output, now, runID); err != nil {
		return err
	}
	_, _ = tx.ExecContext(ctx, `UPDATE runners SET last_seen_at=`+s.arg(1)+` WHERE id=`+s.arg(2), now, runnerID)
	return tx.Commit()
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
func validScope(value string) bool {
	for _, r := range value {
		if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-' || r == '_') {
			return false
		}
	}
	return true
}
