package server

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"net/smtp"
	"net/url"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"
	"time"

	"kohame/internal/config"
	"kohame/internal/database"
	"kohame/internal/forge"
	"kohame/internal/repository"
	"kohame/internal/web"
)

type Server struct {
	repos   *repository.Store
	forge   *forge.Store
	captcha config.CaptchaConfig
}

func New(cfg config.Config) (*Server, error) {
	if err := os.MkdirAll(cfg.Storage.RepositoryRoot, 0o755); err != nil {
		return nil, err
	}
	db, err := database.Open(cfg.Database)
	if err != nil {
		return nil, err
	}
	forgeStore := forge.NewStore(db, cfg.Database.Driver)
	settings, err := forgeStore.Settings(context.Background())
	if err != nil {
		db.Close()
		return nil, err
	}
	captcha := config.CaptchaConfig{Enabled: settings.CaptchaEnabled, SiteKey: settings.CaptchaSiteKey, Secret: settings.CaptchaSecret}
	return &Server{repos: repository.NewStore(cfg.Storage.RepositoryRoot, db, cfg.Database.Driver), forge: forgeStore, captcha: captcha}, nil
}

func (s *Server) Close() error { return s.repos.Close() }

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/setup/status", s.setupStatus)
	mux.HandleFunc("POST /api/setup/admin", s.createAdmin)
	mux.HandleFunc("GET /api/settings", s.settings)
	mux.HandleFunc("GET /api/captcha", s.captchaSettings)
	mux.HandleFunc("GET /api/admin/settings", s.adminSettings)
	mux.HandleFunc("PATCH /api/admin/settings", s.updateSettings)
	mux.HandleFunc("POST /api/auth/register", s.register)
	mux.HandleFunc("POST /api/auth/login", s.login)
	mux.HandleFunc("GET /api/auth/verify", s.verifyEmail)
	mux.HandleFunc("POST /api/auth/verification", s.resendVerification)
	mux.HandleFunc("POST /api/auth/logout", s.logout)
	mux.HandleFunc("GET /api/auth/me", s.me)
	mux.HandleFunc("GET /api/user/settings", s.personalSettings)
	mux.HandleFunc("PATCH /api/user/settings", s.updatePersonalSettings)
	mux.HandleFunc("GET /api/scopes", s.scopes)
	mux.HandleFunc("POST /api/organizations", s.createOrganization)
	mux.HandleFunc("GET /api/organizations/{name}", s.organization)
	mux.HandleFunc("GET /api/organizations/{name}/members", s.organizationMembers)
	mux.HandleFunc("GET /api/organizations/{name}/repos", s.organizationRepos)
	mux.HandleFunc("GET /api/users/{username}", s.userProfile)
	mux.HandleFunc("GET /api/users/{username}/repos", s.userRepos)
	mux.HandleFunc("GET /api/notifications", s.notifications)
	mux.HandleFunc("PATCH /api/notifications/{id}/read", s.readNotification)
	mux.HandleFunc("GET /api/repos", s.listRepos)
	mux.HandleFunc("GET /api/search", s.search)
	mux.HandleFunc("POST /api/repos", s.createRepo)
	mux.HandleFunc("POST /api/repos/{scope}/{name}/fork", s.forkRepo)
	mux.HandleFunc("POST /api/repos/{scope}/{name}/star", s.starRepo)
	mux.HandleFunc("GET /api/repos/{scope}/{name}/tree", s.tree)
	mux.HandleFunc("GET /api/repos/{scope}/{name}/branches", s.branches)
	mux.HandleFunc("GET /api/repos/{scope}/{name}/tags", s.tags)
	mux.HandleFunc("GET /api/repos/{scope}/{name}/commits", s.commits)
	mux.HandleFunc("GET /api/repos/{scope}/{name}/settings", s.repositorySettings)
	mux.HandleFunc("PATCH /api/repos/{scope}/{name}/settings", s.updateRepositorySettings)
	mux.HandleFunc("GET /api/repos/{scope}/{name}/blob", s.blob)
	mux.HandleFunc("GET /api/repos/{scope}/{name}/issues", s.listIssues)
	mux.HandleFunc("POST /api/repos/{scope}/{name}/issues", s.createIssue)
	mux.HandleFunc("PATCH /api/repos/{scope}/{name}/issues/{id}", s.updateIssue)
	mux.HandleFunc("DELETE /api/repos/{scope}/{name}/issues/{id}", s.deleteIssue)
	mux.HandleFunc("GET /api/repos/{scope}/{name}/pulls", s.listPullRequests)
	mux.HandleFunc("POST /api/repos/{scope}/{name}/pulls", s.createPullRequest)
	mux.HandleFunc("PATCH /api/repos/{scope}/{name}/pulls/{id}", s.updatePullRequest)
	mux.HandleFunc("DELETE /api/repos/{scope}/{name}/pulls/{id}", s.deletePullRequest)
	mux.HandleFunc("GET /api/repos/{scope}/{name}/releases", s.listReleases)
	mux.HandleFunc("POST /api/repos/{scope}/{name}/releases", s.createRelease)
	mux.HandleFunc("DELETE /api/repos/{scope}/{name}/releases/{id}", s.deleteRelease)
	mux.HandleFunc("GET /api/repos/{scope}/{name}/contributors", s.contributors)
	mux.HandleFunc("GET /api/repos/{scope}/{name}", s.getRepo)
	mux.HandleFunc("/{scope}/{rest...}", s.gitHTTPDirect)
	mux.Handle("/", spa())
	return securityHeaders(mux)
}

func (s *Server) listRepos(w http.ResponseWriter, r *http.Request) {
	repos, err := s.repos.List()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Could not read repositories.")
		return
	}
	writeJSON(w, http.StatusOK, repos)
}

func (s *Server) search(w http.ResponseWriter, r *http.Request) {
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	repositories, err := s.repos.Search(query)
	if err != nil {
		writeError(w, 500, "Could not search repositories.")
		return
	}
	users, err := s.forge.SearchProfiles(r.Context(), query)
	if err != nil {
		writeError(w, 500, "Could not search users.")
		return
	}
	writeJSON(w, 200, map[string]any{"repositories": repositories, "users": users})
}

func (s *Server) getRepo(w http.ResponseWriter, r *http.Request) {
	repo, err := s.repos.Get(r.PathValue("scope"), r.PathValue("name"))
	if errors.Is(err, repository.ErrNotFound) {
		writeError(w, http.StatusNotFound, "Repository not found.")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Could not read repository.")
		return
	}
	starred := false
	if user, err := s.currentUser(r); err == nil {
		starred, _ = s.repos.Starred(r.Context(), repo.FullName, user.ID)
	}
	writeJSON(w, http.StatusOK, struct {
		repository.Repository
		Starred bool `json:"starred"`
	}{repo, starred})
}

func (s *Server) createRepo(w http.ResponseWriter, r *http.Request) {
	user, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	var input struct {
		Scope string `json:"scope"`
		Name  string `json:"name"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "Provide a repository name.")
		return
	}
	input.Scope = strings.TrimSpace(input.Scope)
	if allowed, err := s.forge.CanUseScope(r.Context(), user, input.Scope); err != nil {
		writeError(w, 500, "Could not verify repository scope.")
		return
	} else if !allowed {
		writeError(w, 403, "Choose your own username or an organization you belong to.")
		return
	}
	repo, err := s.repos.Create(r.Context(), input.Scope, strings.TrimSpace(input.Name))
	switch {
	case errors.Is(err, repository.ErrInvalidName):
		writeError(w, http.StatusBadRequest, "Scope and repository name use 1–80 lowercase letters, numbers, dots, hyphens, or underscores.")
	case errors.Is(err, repository.ErrExists):
		writeError(w, http.StatusConflict, "That repository already exists.")
	case err != nil:
		writeError(w, http.StatusInternalServerError, "Could not create repository.")
	default:
		writeJSON(w, http.StatusCreated, repo)
	}
}

func (s *Server) setupStatus(w http.ResponseWriter, r *http.Request) {
	needsSetup, err := s.forge.NeedsSetup(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Could not read setup status.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"needsSetup": needsSetup})
}

func (s *Server) createAdmin(w http.ResponseWriter, r *http.Request) {
	var input credentials
	if !decodeJSON(w, r, &input) {
		return
	}
	user, err := s.forge.CreateAdmin(r.Context(), input.Username, input.Email, input.Password)
	if errors.Is(err, forge.ErrSetupComplete) {
		writeError(w, http.StatusConflict, "Setup is already complete.")
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.startSession(w, r, user)
	writeJSON(w, http.StatusCreated, user)
}

func (s *Server) register(w http.ResponseWriter, r *http.Request) {
	var input credentials
	if !decodeJSON(w, r, &input) {
		return
	}
	if err := s.verifyCaptcha(r, input.CaptchaToken); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	user, err := s.forge.Register(r.Context(), input.Username, input.Email, input.Password)
	switch {
	case errors.Is(err, forge.ErrForbidden):
		writeError(w, http.StatusForbidden, "Registration is disabled by the site administrator.")
	case errors.Is(err, forge.ErrSetupComplete):
		writeError(w, http.StatusConflict, "Create the first administrator before registering users.")
	case errors.Is(err, forge.ErrConflict):
		writeError(w, http.StatusConflict, "That username or email is already in use.")
	case err != nil:
		writeError(w, http.StatusBadRequest, err.Error())
	default:
		// An SMTP outage must not discard a successfully created account; users can resend from their profile.
		_ = s.sendVerification(r, user)
		s.startSession(w, r, user)
		writeJSON(w, http.StatusCreated, user)
	}
}

func (s *Server) verifyEmail(w http.ResponseWriter, r *http.Request) {
	if err := s.forge.VerifyEmail(r.Context(), r.URL.Query().Get("token")); errors.Is(err, forge.ErrNotFound) {
		writeError(w, 400, "This verification link is invalid or expired.")
	} else if err != nil {
		writeError(w, 500, "Could not verify email.")
	} else {
		writeJSON(w, 200, map[string]string{"message": "Email verified. You can now use your account."})
	}
}
func (s *Server) resendVerification(w http.ResponseWriter, r *http.Request) {
	user, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	if user.EmailVerified {
		writeJSON(w, 200, map[string]string{"message": "Your email is already verified."})
		return
	}
	if err := s.sendVerification(r, user); err != nil {
		writeError(w, 400, err.Error())
		return
	}
	writeJSON(w, 200, map[string]string{"message": "Verification email sent."})
}
func (s *Server) sendVerification(r *http.Request, user forge.User) error {
	settings, err := s.forge.Settings(r.Context())
	if err != nil {
		return err
	}
	token, err := s.forge.CreateEmailVerification(r.Context(), user.ID)
	if err != nil {
		return err
	}
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	link := scheme + "://" + r.Host + "/verify?token=" + url.QueryEscape(token)
	return sendMail(settings, user.Email, "Verify your "+settings.Title+" email", "Hello "+user.Username+",\r\n\r\nVerify your email address by opening:\r\n"+link+"\r\n\r\nThis link expires in 24 hours.\r\n")
}
func (s *Server) captchaSettings(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]any{"enabled": s.captcha.Enabled, "siteKey": s.captcha.SiteKey})
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Identity string `json:"identity"`
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	user, err := s.forge.Authenticate(r.Context(), input.Identity, input.Password)
	if errors.Is(err, forge.ErrUnauthorized) {
		writeError(w, http.StatusUnauthorized, "Invalid username, email, or password.")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Could not sign in.")
		return
	}
	s.startSession(w, r, user)
	writeJSON(w, http.StatusOK, user)
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie("kohame_session"); err == nil {
		_ = s.forge.DeleteSession(r.Context(), cookie.Value)
	}
	http.SetCookie(w, &http.Cookie{Name: "kohame_session", Value: "", Path: "/", MaxAge: -1, HttpOnly: true, SameSite: http.SameSiteLaxMode})
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	user, ok := s.requireUser(w, r)
	if ok {
		writeJSON(w, http.StatusOK, user)
	}
}
func (s *Server) personalSettings(w http.ResponseWriter, r *http.Request) {
	user, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	value, err := s.forge.PersonalSettings(r.Context(), user)
	if err != nil {
		writeError(w, 500, "Could not load personal settings.")
		return
	}
	writeJSON(w, 200, value)
}
func (s *Server) updatePersonalSettings(w http.ResponseWriter, r *http.Request) {
	user, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	var input forge.PersonalSettings
	if !decodeJSON(w, r, &input) {
		return
	}
	value, err := s.forge.UpdatePersonalSettings(r.Context(), user, input)
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	writeJSON(w, 200, value)
}
func (s *Server) scopes(w http.ResponseWriter, r *http.Request) {
	user, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	items, err := s.forge.Scopes(r.Context(), user)
	if err != nil {
		writeError(w, 500, "Could not load scopes.")
		return
	}
	writeJSON(w, 200, items)
}
func (s *Server) createOrganization(w http.ResponseWriter, r *http.Request) {
	user, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	var input struct {
		Name string `json:"name"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	org, err := s.forge.CreateOrganization(r.Context(), user, input.Name)
	if errors.Is(err, forge.ErrConflict) {
		writeError(w, 409, "That organization already exists.")
		return
	}
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	writeJSON(w, 201, org)
}
func (s *Server) organization(w http.ResponseWriter, r *http.Request) {
	org, err := s.forge.Organization(r.Context(), r.PathValue("name"))
	if errors.Is(err, forge.ErrNotFound) {
		writeError(w, 404, "Organization not found.")
		return
	}
	if err != nil {
		writeError(w, 500, "Could not load organization.")
		return
	}
	writeJSON(w, 200, org)
}
func (s *Server) organizationMembers(w http.ResponseWriter, r *http.Request) {
	items, err := s.forge.OrganizationMembers(r.Context(), r.PathValue("name"))
	if err != nil {
		writeError(w, 500, "Could not load organization members.")
		return
	}
	writeJSON(w, 200, items)
}
func (s *Server) organizationRepos(w http.ResponseWriter, r *http.Request) {
	items, err := s.repos.ListByScope(r.PathValue("name"))
	if err != nil {
		writeError(w, 500, "Could not load repositories.")
		return
	}
	writeJSON(w, 200, items)
}
func (s *Server) userProfile(w http.ResponseWriter, r *http.Request) {
	value, err := s.forge.Profile(r.Context(), r.PathValue("username"))
	if errors.Is(err, forge.ErrNotFound) {
		writeError(w, 404, "User not found.")
		return
	}
	if err != nil {
		writeError(w, 500, "Could not load user.")
		return
	}
	writeJSON(w, 200, value)
}
func (s *Server) userRepos(w http.ResponseWriter, r *http.Request) {
	items, err := s.repos.ListByScope(r.PathValue("username"))
	if err != nil {
		writeError(w, 500, "Could not load repositories.")
		return
	}
	writeJSON(w, 200, items)
}
func (s *Server) notifications(w http.ResponseWriter, r *http.Request) {
	user, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	items, err := s.forge.Notifications(r.Context(), user.ID)
	if err != nil {
		writeError(w, 500, "Could not load notifications.")
		return
	}
	writeJSON(w, 200, items)
}
func (s *Server) readNotification(w http.ResponseWriter, r *http.Request) {
	user, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if err := s.forge.ReadNotification(r.Context(), user.ID, id); err != nil {
		writeError(w, 500, "Could not update notification.")
		return
	}
	w.WriteHeader(204)
}

func (s *Server) forkRepo(w http.ResponseWriter, r *http.Request) {
	user, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	source, ok := s.requireRepo(w, r)
	if !ok {
		return
	}
	var input struct {
		Scope string `json:"scope"`
		Name  string `json:"name"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	if input.Scope == "" {
		input.Scope = user.Username
	}
	allowed, err := s.forge.CanUseScope(r.Context(), user, input.Scope)
	if err != nil || !allowed {
		writeError(w, 403, "Choose your own username or an organization you belong to.")
		return
	}
	repo, err := s.repos.Fork(r.Context(), source, strings.TrimSpace(input.Scope), strings.TrimSpace(input.Name))
	if errors.Is(err, repository.ErrExists) {
		writeError(w, 409, "That repository already exists.")
		return
	}
	if errors.Is(err, repository.ErrInvalidName) {
		writeError(w, 400, "Choose a valid fork name.")
		return
	}
	if err != nil {
		writeError(w, 500, "Could not fork repository.")
		return
	}
	if owner, err := s.forge.UserByUsername(r.Context(), source.Scope); err == nil && owner.ID != user.ID {
		_ = s.forge.AddNotification(r.Context(), owner.ID, "fork", user.Username+" forked "+source.FullName, "A new fork is available.", "/"+repo.FullName)
	}
	writeJSON(w, 201, repo)
}
func (s *Server) starRepo(w http.ResponseWriter, r *http.Request) {
	user, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	repo, ok := s.requireRepo(w, r)
	if !ok {
		return
	}
	starred, count, err := s.repos.ToggleStar(r.Context(), repo.FullName, user.ID)
	if err != nil {
		writeError(w, 500, "Could not update star.")
		return
	}
	if starred {
		if owner, err := s.forge.UserByUsername(r.Context(), repo.Scope); err == nil && owner.ID != user.ID {
			_ = s.forge.AddNotification(r.Context(), owner.ID, "star", user.Username+" starred "+repo.FullName, "Your repository has a new star.", "/"+repo.FullName)
		}
	}
	writeJSON(w, 200, map[string]any{"starred": starred, "stars": count})
}
func (s *Server) notifyRepositoryOwner(r *http.Request, actor forge.User, kind, title, body string) {
	if owner, err := s.forge.UserByUsername(r.Context(), r.PathValue("scope")); err == nil && owner.ID != actor.ID {
		_ = s.forge.AddNotification(r.Context(), owner.ID, kind, title, body, "/"+repoKey(r))
	}
}
func (s *Server) settings(w http.ResponseWriter, r *http.Request) {
	value, err := s.forge.Settings(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Could not load site settings.")
		return
	}
	// Public site metadata must never expose SMTP or CAPTCHA credentials.
	value.CaptchaSecret = ""
	value.SMTPHost, value.SMTPPort, value.SMTPUsername, value.SMTPPassword, value.SMTPFrom = "", "", "", "", ""
	writeJSON(w, http.StatusOK, value)
}
func (s *Server) adminSettings(w http.ResponseWriter, r *http.Request) {
	user, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	_ = user
	value, err := s.forge.Settings(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Could not load site settings.")
		return
	}
	writeJSON(w, http.StatusOK, value)
}
func (s *Server) updateSettings(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	var input forge.SiteSettings
	if !decodeJSON(w, r, &input) {
		return
	}
	if err := s.forge.UpdateSettings(r.Context(), input); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, input)
}

func (s *Server) listIssues(w http.ResponseWriter, r *http.Request) {
	if !s.hasRepo(w, r) {
		return
	}
	items, err := s.forge.ListIssues(r.Context(), repoKey(r))
	if err != nil {
		writeError(w, 500, "Could not load issues.")
		return
	}
	writeJSON(w, 200, items)
}
func (s *Server) createIssue(w http.ResponseWriter, r *http.Request) {
	if !s.hasRepo(w, r) {
		return
	}
	user, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	var input struct {
		Title string `json:"title"`
		Body  string `json:"body"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	item, err := s.forge.CreateIssue(r.Context(), repoKey(r), user, input.Title, input.Body)
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	s.notifyRepositoryOwner(r, user, "issue", user.Username+" opened an issue", item.Title)
	writeJSON(w, 201, item)
}
func (s *Server) updateIssue(w http.ResponseWriter, r *http.Request) {
	if !s.hasRepo(w, r) {
		return
	}
	if _, ok := s.requireUser(w, r); !ok {
		return
	}
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var input struct {
		State string `json:"state"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	err := s.forge.UpdateIssueState(r.Context(), repoKey(r), id, input.State)
	if errors.Is(err, forge.ErrNotFound) {
		writeError(w, 404, "Issue not found.")
		return
	}
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (s *Server) deleteIssue(w http.ResponseWriter, r *http.Request) {
	if !s.hasRepo(w, r) {
		return
	}
	if _, ok := s.requireUser(w, r); !ok {
		return
	}
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if err := s.forge.DeleteIssue(r.Context(), repoKey(r), id); errors.Is(err, forge.ErrNotFound) {
		writeError(w, 404, "Issue not found.")
	} else if err != nil {
		writeError(w, 500, "Could not delete issue.")
	} else {
		w.WriteHeader(204)
	}
}
func (s *Server) listPullRequests(w http.ResponseWriter, r *http.Request) {
	if !s.hasRepo(w, r) {
		return
	}
	items, err := s.forge.ListPullRequests(r.Context(), repoKey(r))
	if err != nil {
		writeError(w, 500, "Could not load pull requests.")
		return
	}
	writeJSON(w, 200, items)
}
func (s *Server) createPullRequest(w http.ResponseWriter, r *http.Request) {
	if !s.hasRepo(w, r) {
		return
	}
	user, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	var input struct {
		Title        string `json:"title"`
		Body         string `json:"body"`
		SourceBranch string `json:"sourceBranch"`
		TargetBranch string `json:"targetBranch"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	item, err := s.forge.CreatePullRequest(r.Context(), repoKey(r), user, input.Title, input.Body, input.SourceBranch, input.TargetBranch)
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	s.notifyRepositoryOwner(r, user, "pull_request", user.Username+" opened a pull request", item.Title)
	writeJSON(w, 201, item)
}
func (s *Server) updatePullRequest(w http.ResponseWriter, r *http.Request) {
	if !s.hasRepo(w, r) {
		return
	}
	if _, ok := s.requireUser(w, r); !ok {
		return
	}
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var input struct {
		State string `json:"state"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	err := s.forge.UpdatePullRequestState(r.Context(), repoKey(r), id, input.State)
	if errors.Is(err, forge.ErrNotFound) {
		writeError(w, 404, "Pull request not found.")
		return
	}
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (s *Server) deletePullRequest(w http.ResponseWriter, r *http.Request) {
	if !s.hasRepo(w, r) {
		return
	}
	if _, ok := s.requireUser(w, r); !ok {
		return
	}
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if err := s.forge.DeletePullRequest(r.Context(), repoKey(r), id); errors.Is(err, forge.ErrNotFound) {
		writeError(w, 404, "Pull request not found.")
	} else if err != nil {
		writeError(w, 500, "Could not delete pull request.")
	} else {
		w.WriteHeader(204)
	}
}
func (s *Server) listReleases(w http.ResponseWriter, r *http.Request) {
	if !s.hasRepo(w, r) {
		return
	}
	items, err := s.forge.ListReleases(r.Context(), repoKey(r))
	if err != nil {
		writeError(w, 500, "Could not load releases.")
		return
	}
	writeJSON(w, 200, items)
}
func (s *Server) createRelease(w http.ResponseWriter, r *http.Request) {
	if !s.hasRepo(w, r) {
		return
	}
	user, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	var input struct {
		TagName string `json:"tagName"`
		Title   string `json:"title"`
		Notes   string `json:"notes"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	item, err := s.forge.CreateRelease(r.Context(), repoKey(r), user, input.TagName, input.Title, input.Notes)
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	writeJSON(w, 201, item)
}
func (s *Server) deleteRelease(w http.ResponseWriter, r *http.Request) {
	if !s.hasRepo(w, r) {
		return
	}
	if _, ok := s.requireUser(w, r); !ok {
		return
	}
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if err := s.forge.DeleteRelease(r.Context(), repoKey(r), id); errors.Is(err, forge.ErrNotFound) {
		writeError(w, 404, "Release not found.")
	} else if err != nil {
		writeError(w, 500, "Could not delete release.")
	} else {
		w.WriteHeader(204)
	}
}
func (s *Server) contributors(w http.ResponseWriter, r *http.Request) {
	if !s.hasRepo(w, r) {
		return
	}
	items, err := s.forge.Contributors(r.Context(), repoKey(r))
	if err != nil {
		writeError(w, 500, "Could not load contributors.")
		return
	}
	writeJSON(w, 200, items)
}

func (s *Server) tree(w http.ResponseWriter, r *http.Request) {
	repo, ok := s.requireRepo(w, r)
	if !ok {
		return
	}
	entries, err := s.repos.Tree(r.Context(), repo, valueOr(r.URL.Query().Get("ref"), "HEAD"), r.URL.Query().Get("path"))
	if errors.Is(err, repository.ErrNotFound) {
		writeError(w, http.StatusNotFound, "Tree not found.")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Could not browse repository.")
		return
	}
	writeJSON(w, http.StatusOK, entries)
}

func (s *Server) blob(w http.ResponseWriter, r *http.Request) {
	repo, ok := s.requireRepo(w, r)
	if !ok {
		return
	}
	file, err := s.repos.Blob(r.Context(), repo, valueOr(r.URL.Query().Get("ref"), "HEAD"), r.URL.Query().Get("path"))
	if errors.Is(err, repository.ErrNotFound) {
		writeError(w, http.StatusNotFound, "File not found.")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Could not read file.")
		return
	}
	writeJSON(w, http.StatusOK, file)
}
func (s *Server) branches(w http.ResponseWriter, r *http.Request) {
	repo, ok := s.requireRepo(w, r)
	if !ok {
		return
	}
	items, err := s.repos.Branches(r.Context(), repo)
	if err != nil {
		writeError(w, 500, "Could not list branches.")
		return
	}
	writeJSON(w, 200, items)
}
func (s *Server) tags(w http.ResponseWriter, r *http.Request) {
	repo, ok := s.requireRepo(w, r)
	if !ok {
		return
	}
	items, err := s.repos.Tags(r.Context(), repo)
	if err != nil {
		writeError(w, 500, "Could not list tags.")
		return
	}
	writeJSON(w, 200, items)
}
func (s *Server) commits(w http.ResponseWriter, r *http.Request) {
	repo, ok := s.requireRepo(w, r)
	if !ok {
		return
	}
	items, err := s.repos.Commits(r.Context(), repo, valueOr(r.URL.Query().Get("ref"), "HEAD"))
	if err != nil {
		writeError(w, 500, "Could not list commits.")
		return
	}
	writeJSON(w, 200, items)
}
func (s *Server) repositorySettings(w http.ResponseWriter, r *http.Request) {
	if !s.hasRepo(w, r) {
		return
	}
	value, err := s.repos.Settings(r.Context(), repoKey(r))
	if err != nil {
		writeError(w, 500, "Could not load repository settings.")
		return
	}
	writeJSON(w, 200, value)
}
func (s *Server) updateRepositorySettings(w http.ResponseWriter, r *http.Request) {
	if !s.hasRepo(w, r) {
		return
	}
	user, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	allowed, err := s.forge.CanUseScope(r.Context(), user, r.PathValue("scope"))
	if err != nil || !allowed {
		writeError(w, 403, "No permission to manage this repository.")
		return
	}
	var value repository.Settings
	if !decodeJSON(w, r, &value) {
		return
	}
	if err := s.repos.UpdateSettings(r.Context(), repoKey(r), value); err != nil {
		writeError(w, 400, err.Error())
		return
	}
	writeJSON(w, 200, value)
}

type credentials struct {
	Username     string `json:"username"`
	Email        string `json:"email"`
	Password     string `json:"password"`
	CaptchaToken string `json:"captchaToken"`
}

func decodeJSON(w http.ResponseWriter, r *http.Request, target any) bool {
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(target); err != nil {
		writeError(w, 400, "Invalid request body.")
		return false
	}
	return true
}
func (s *Server) hasRepo(w http.ResponseWriter, r *http.Request) bool {
	if _, err := s.repos.Get(r.PathValue("scope"), r.PathValue("name")); err != nil {
		writeError(w, 404, "Repository not found.")
		return false
	}
	return true
}

func (s *Server) requireRepo(w http.ResponseWriter, r *http.Request) (repository.Repository, bool) {
	repo, err := s.repos.Get(r.PathValue("scope"), r.PathValue("name"))
	if err != nil {
		writeError(w, http.StatusNotFound, "Repository not found.")
		return repository.Repository{}, false
	}
	return repo, true
}

func repoKey(r *http.Request) string { return r.PathValue("scope") + "/" + r.PathValue("name") }
func valueOr(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
func (s *Server) currentUser(r *http.Request) (forge.User, error) {
	cookie, err := r.Cookie("kohame_session")
	if err != nil {
		return forge.User{}, forge.ErrUnauthorized
	}
	return s.forge.UserBySession(r.Context(), cookie.Value)
}

func (s *Server) gitUser(r *http.Request) (forge.User, error) {
	if user, err := s.currentUser(r); err == nil {
		return user, nil
	}
	username, password, ok := r.BasicAuth()
	if !ok {
		return forge.User{}, forge.ErrUnauthorized
	}
	return s.forge.Authenticate(r.Context(), username, password)
}
func (s *Server) requireUser(w http.ResponseWriter, r *http.Request) (forge.User, bool) {
	user, err := s.currentUser(r)
	if errors.Is(err, forge.ErrUnauthorized) {
		writeError(w, 401, "Sign in to continue.")
		return forge.User{}, false
	}
	if err != nil {
		writeError(w, 500, "Could not verify session.")
		return forge.User{}, false
	}
	return user, true
}
func (s *Server) requireAdmin(w http.ResponseWriter, r *http.Request) (forge.User, bool) {
	user, ok := s.requireUser(w, r)
	if ok && !user.IsAdmin {
		writeError(w, 403, "Administrator access is required.")
		return forge.User{}, false
	}
	return user, ok
}
func (s *Server) startSession(w http.ResponseWriter, r *http.Request, user forge.User) {
	token, err := s.forge.CreateSession(r.Context(), user.ID)
	if err != nil {
		return
	}
	http.SetCookie(w, &http.Cookie{Name: "kohame_session", Value: token, Path: "/", Expires: time.Now().Add(30 * 24 * time.Hour), HttpOnly: true, SameSite: http.SameSiteLaxMode, Secure: r.TLS != nil})
}

func (s *Server) verifyCaptcha(r *http.Request, token string) error {
	if !s.captcha.Enabled {
		return nil
	}
	if token == "" {
		return errors.New("请完成人机验证")
	}
	values := make(url.Values)
	values.Set("secret", s.captcha.Secret)
	values.Set("response", token)
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		values.Set("remoteip", host)
	}
	request, err := http.NewRequestWithContext(r.Context(), http.MethodPost, "https://hcaptcha.com/siteverify", strings.NewReader(values.Encode()))
	if err != nil {
		return errors.New("人机验证请求创建失败")
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return errors.New("人机验证服务不可用")
	}
	defer response.Body.Close()
	var result struct {
		Success bool `json:"success"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&result); err != nil || !result.Success {
		return errors.New("人机验证失败，请重试")
	}
	return nil
}
func pathID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id < 1 {
		writeError(w, 400, "Invalid item ID.")
		return 0, false
	}
	return id, true
}

// gitHTTP delegates Git smart-HTTP protocol framing to the installed git executable.
func (s *Server) gitHTTPDirect(w http.ResponseWriter, r *http.Request) {
	scope, raw := r.PathValue("scope"), r.PathValue("rest")
	nameWithSuffix, rest, ok := strings.Cut(raw, "/")
	if !ok || !strings.HasSuffix(nameWithSuffix, ".git") {
		spa().ServeHTTP(w, r)
		return
	}
	s.serveGit(w, r, scope, strings.TrimSuffix(nameWithSuffix, ".git"), rest)
}

func (s *Server) serveGit(w http.ResponseWriter, r *http.Request, scope, name, rest string) {
	user, err := s.gitUser(r)
	if err != nil {
		w.Header().Set("WWW-Authenticate", `Basic realm="Kohame Git"`)
		writeError(w, http.StatusUnauthorized, "Sign in with your Kohame username and password to use Git HTTP.")
		return
	}
	if _, err := s.repos.Get(scope, name); err != nil {
		http.NotFound(w, r)
		return
	}
	if strings.Contains(rest, "..") {
		http.NotFound(w, r)
		return
	}
	cmd := exec.CommandContext(r.Context(), "git", "http-backend")
	cmd.Env = append(os.Environ(),
		"GIT_HTTP_EXPORT_ALL=1",
		"GIT_PROJECT_ROOT="+s.reposRoot(),
		"PATH_INFO=/"+scope+"/"+name+".git/"+rest,
		"REQUEST_METHOD="+r.Method,
		"QUERY_STRING="+r.URL.RawQuery,
		"CONTENT_TYPE="+r.Header.Get("Content-Type"),
		"CONTENT_LENGTH="+r.Header.Get("Content-Length"),
		"REMOTE_USER="+user.Username,
	)
	cmd.Stdin = r.Body
	output, err := cmd.Output()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Git service failed.")
		return
	}
	headerEnd := strings.Index(string(output), "\r\n\r\n")
	separator := 4
	if headerEnd < 0 {
		headerEnd = strings.Index(string(output), "\n\n")
		separator = 2
	}
	if headerEnd < 0 {
		writeError(w, http.StatusInternalServerError, "Invalid Git service response.")
		return
	}
	for _, line := range strings.Split(string(output[:headerEnd]), "\n") {
		parts := strings.SplitN(strings.TrimSpace(line), ":", 2)
		if len(parts) == 2 {
			w.Header().Add(strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]))
		}
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(output[headerEnd+separator:])
}

func (s *Server) reposRoot() string { return getStoreRoot(s.repos) }

// Keep the store root private while avoiding exposing filesystem paths in API objects.
func getStoreRoot(store *repository.Store) string { return store.Root() }

func spa() http.Handler {
	assets, _ := fs.Sub(web.Files, "dist")
	fileServer := http.FileServer(http.FS(assets))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requested := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if requested != "" {
			if _, err := fs.Stat(assets, requested); err == nil {
				fileServer.ServeHTTP(w, r)
				return
			}
		}
		r.URL.Path = "/"
		fileServer.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		next.ServeHTTP(w, r)
	})
}

func sendMail(settings forge.SiteSettings, to, subject, body string) error {
	host, port := strings.TrimSpace(settings.SMTPHost), strings.TrimSpace(settings.SMTPPort)
	if host == "" || port == "" || strings.TrimSpace(settings.SMTPFrom) == "" {
		return errors.New("SMTP is not configured by the administrator")
	}
	address := net.JoinHostPort(host, port)
	client, err := smtp.Dial(address)
	if err != nil {
		return fmt.Errorf("connect to SMTP: %w", err)
	}
	defer client.Close()
	if ok, _ := client.Extension("STARTTLS"); ok {
		if err := client.StartTLS(&tls.Config{ServerName: host, MinVersion: tls.VersionTLS12}); err != nil {
			return fmt.Errorf("start SMTP TLS: %w", err)
		}
	}
	if settings.SMTPUsername != "" {
		if err := client.Auth(smtp.PlainAuth("", settings.SMTPUsername, settings.SMTPPassword, host)); err != nil {
			return fmt.Errorf("authenticate SMTP: %w", err)
		}
	}
	if err := client.Mail(settings.SMTPFrom); err != nil {
		return err
	}
	if err := client.Rcpt(to); err != nil {
		return err
	}
	writer, err := client.Data()
	if err != nil {
		return err
	}
	_, err = writer.Write([]byte("From: " + settings.SMTPFrom + "\r\nTo: " + to + "\r\nSubject: " + subject + "\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n" + body))
	if closeErr := writer.Close(); err == nil {
		err = closeErr
	}
	return err
}
