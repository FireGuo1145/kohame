package server

import (
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"net"
	"net/http"
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
	return &Server{repos: repository.NewStore(cfg.Storage.RepositoryRoot, db, cfg.Database.Driver), forge: forge.NewStore(db, cfg.Database.Driver), captcha: cfg.Captcha}, nil
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
	mux.HandleFunc("POST /api/auth/logout", s.logout)
	mux.HandleFunc("GET /api/auth/me", s.me)
	mux.HandleFunc("GET /api/scopes", s.scopes)
	mux.HandleFunc("POST /api/organizations", s.createOrganization)
	mux.HandleFunc("GET /api/repos", s.listRepos)
	mux.HandleFunc("POST /api/repos", s.createRepo)
	mux.HandleFunc("GET /api/repos/{scope}/{name}/tree", s.tree)
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
	writeJSON(w, http.StatusOK, repo)
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
		s.startSession(w, r, user)
		writeJSON(w, http.StatusCreated, user)
	}
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
func (s *Server) settings(w http.ResponseWriter, r *http.Request) {
	value, err := s.forge.Settings(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Could not load site settings.")
		return
	}
	writeJSON(w, http.StatusOK, value)
}
func (s *Server) adminSettings(w http.ResponseWriter, r *http.Request) {
	user, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	_ = user
	s.settings(w, r)
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
