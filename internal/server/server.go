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
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	sshserver "github.com/gliderlabs/ssh"
	cryptossh "golang.org/x/crypto/ssh"
	"kohame/internal/config"
	"kohame/internal/database"
	"kohame/internal/forge"
	"kohame/internal/repository"
	"kohame/internal/web"
)

type Server struct {
	repos      *repository.Store
	forge      *forge.Store
	captcha    config.CaptchaConfig
	avatarDir  string
	releaseDir string
	sshServer  *sshserver.Server
	sshHost    string
	sshPort    string
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
	avatarDir := filepath.Join(filepath.Dir(cfg.Storage.RepositoryRoot), "avatars")
	if err := os.MkdirAll(avatarDir, 0o755); err != nil {
		db.Close()
		return nil, err
	}
	releaseDir := filepath.Join(filepath.Dir(cfg.Storage.RepositoryRoot), "release-assets")
	if err := os.MkdirAll(releaseDir, 0o755); err != nil {
		db.Close()
		return nil, err
	}
	_, sshPort, _ := net.SplitHostPort(cfg.SSH.Addr)
	server := &Server{repos: repository.NewStore(cfg.Storage.RepositoryRoot, db, cfg.Database.Driver), forge: forgeStore, captcha: captcha, avatarDir: avatarDir, releaseDir: releaseDir, sshHost: cfg.SSH.Host, sshPort: sshPort}
	if cfg.SSH.Addr != "" {
		server.startSSH(cfg.SSH.Addr)
	}
	return server, nil
}

func (s *Server) Close() error {
	if s.sshServer != nil {
		_ = s.sshServer.Close()
	}
	return s.repos.Close()
}

func (s *Server) startSSH(addr string) {
	s.sshServer = &sshserver.Server{Addr: addr, Handler: s.gitSSH, PublicKeyHandler: func(ctx sshserver.Context, key sshserver.PublicKey) bool {
		value := strings.TrimSpace(string(cryptossh.MarshalAuthorizedKey(key)))
		user, err := s.forge.UserBySSHKey(context.Background(), value)
		if err != nil {
			return false
		}
		ctx.Permissions().Extensions = map[string]string{"username": user.Username}
		return true
	}}
	go func() { _ = s.sshServer.ListenAndServe() }()
}
func (s *Server) gitSSH(session sshserver.Session) {
	command := session.Command()
	if len(command) != 2 || (command[0] != "git-upload-pack" && command[0] != "git-receive-pack") {
		_, _ = io.WriteString(session, "仅支持 Git SSH 操作。\n")
		_ = session.Exit(1)
		return
	}
	repoName := strings.Trim(strings.TrimSpace(command[1]), "'\"")
	repoName = strings.TrimPrefix(repoName, "/")
	if !strings.HasSuffix(repoName, ".git") {
		_ = session.Exit(1)
		return
	}
	repoName = strings.TrimSuffix(repoName, ".git")
	scope, name, ok := strings.Cut(repoName, "/")
	if !ok {
		_ = session.Exit(1)
		return
	}
	repo, err := s.repos.Get(scope, name)
	if err != nil {
		_ = session.Exit(1)
		return
	}
	cmd := exec.Command(command[0], repo.Path)
	cmd.Stdin = session
	cmd.Stdout = session
	cmd.Stderr = session.Stderr()
	if err := cmd.Run(); err != nil {
		_ = session.Exit(1)
		return
	}
	_ = session.Exit(0)
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/setup/status", s.setupStatus)
	mux.HandleFunc("POST /api/setup/admin", s.createAdmin)
	mux.HandleFunc("GET /api/settings", s.settings)
	mux.HandleFunc("GET /api/captcha", s.captchaSettings)
	mux.HandleFunc("GET /api/ssh", s.sshSettings)
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
	mux.HandleFunc("POST /api/user/avatar", s.uploadAvatar)
	mux.HandleFunc("GET /api/user/ssh-keys", s.sshKeys)
	mux.HandleFunc("POST /api/user/ssh-keys", s.addSSHKey)
	mux.HandleFunc("DELETE /api/user/ssh-keys/{id}", s.deleteSSHKey)
	mux.HandleFunc("GET /api/scopes", s.scopes)
	mux.HandleFunc("POST /api/organizations", s.createOrganization)
	mux.HandleFunc("GET /api/organizations/{name}", s.organization)
	mux.HandleFunc("GET /api/organizations/{name}/members", s.organizationMembers)
	mux.HandleFunc("GET /api/organizations/{name}/repos", s.organizationRepos)
	mux.HandleFunc("GET /api/organizations/{name}/followers", s.organizationFollowers)
	mux.HandleFunc("POST /api/organizations/{name}/follow", s.followOrganization)
	mux.HandleFunc("GET /api/users/{username}", s.userProfile)
	mux.HandleFunc("GET /api/users/{username}/repos", s.userRepos)
	mux.HandleFunc("GET /api/users/{username}/followers", s.userFollowers)
	mux.HandleFunc("GET /api/users/{username}/following", s.userFollowing)
	mux.HandleFunc("POST /api/users/{username}/follow", s.followUser)
	mux.HandleFunc("GET /api/notifications", s.notifications)
	mux.HandleFunc("PATCH /api/notifications/{id}/read", s.readNotification)
	mux.HandleFunc("GET /api/repos", s.listRepos)
	mux.HandleFunc("GET /api/search", s.search)
	mux.HandleFunc("POST /api/repos", s.createRepo)
	mux.HandleFunc("POST /api/repos/{scope}/{name}/fork", s.forkRepo)
	mux.HandleFunc("GET /api/repos/{scope}/{name}/forks", s.forks)
	mux.HandleFunc("POST /api/repos/{scope}/{name}/star", s.starRepo)
	mux.HandleFunc("GET /api/repos/{scope}/{name}/tree", s.tree)
	mux.HandleFunc("GET /api/repos/{scope}/{name}/file-tree", s.fileTree)
	mux.HandleFunc("GET /api/repos/{scope}/{name}/languages", s.languages)
	mux.HandleFunc("GET /api/repos/{scope}/{name}/branches", s.branches)
	mux.HandleFunc("GET /api/repos/{scope}/{name}/tags", s.tags)
	mux.HandleFunc("GET /api/repos/{scope}/{name}/commits", s.commits)
	mux.HandleFunc("GET /api/repos/{scope}/{name}/commits/{hash}", s.commit)
	mux.HandleFunc("GET /api/repos/{scope}/{name}/settings", s.repositorySettings)
	mux.HandleFunc("GET /api/repos/{scope}/{name}/license", s.repositoryLicense)
	mux.HandleFunc("PATCH /api/repos/{scope}/{name}/settings", s.updateRepositorySettings)
	mux.HandleFunc("PUT /api/repos/{scope}/{name}/visibility", s.updateRepositoryVisibility)
	mux.HandleFunc("GET /api/repos/{scope}/{name}/collaborators", s.collaborators)
	mux.HandleFunc("PUT /api/repos/{scope}/{name}/collaborators/{username}", s.setCollaborator)
	mux.HandleFunc("DELETE /api/repos/{scope}/{name}/collaborators/{username}", s.removeCollaborator)
	mux.HandleFunc("GET /api/repos/{scope}/{name}/branch-protections", s.protectedBranches)
	mux.HandleFunc("PUT /api/repos/{scope}/{name}/branch-protections/{branch}", s.setProtectedBranch)
	mux.HandleFunc("DELETE /api/repos/{scope}/{name}/branch-protections/{branch}", s.removeProtectedBranch)
	mux.HandleFunc("POST /api/repos/{scope}/{name}/transfer", s.transferRepository)
	mux.HandleFunc("POST /api/repos/{scope}/{name}/rename", s.renameRepository)
	mux.HandleFunc("DELETE /api/repos/{scope}/{name}", s.deleteRepository)
	mux.HandleFunc("GET /api/repos/{scope}/{name}/blob", s.blob)
	mux.HandleFunc("GET /api/repos/{scope}/{name}/raw", s.rawFile)
	mux.HandleFunc("GET /api/repos/{scope}/{name}/file-commit", s.fileCommit)
	mux.HandleFunc("PUT /api/repos/{scope}/{name}/blob", s.writeFile)
	mux.HandleFunc("GET /api/repos/{scope}/{name}/archive", s.archive)
	mux.HandleFunc("GET /api/repos/{scope}/{name}/issues", s.listIssues)
	mux.HandleFunc("POST /api/repos/{scope}/{name}/issues", s.createIssue)
	mux.HandleFunc("GET /api/repos/{scope}/{name}/labels", s.labels)
	mux.HandleFunc("POST /api/repos/{scope}/{name}/labels", s.createLabel)
	mux.HandleFunc("PATCH /api/repos/{scope}/{name}/labels/{id}", s.updateLabel)
	mux.HandleFunc("DELETE /api/repos/{scope}/{name}/labels/{id}", s.deleteLabel)
	mux.HandleFunc("PUT /api/repos/{scope}/{name}/issues/{id}/labels", s.setIssueLabels)
	mux.HandleFunc("GET /api/repos/{scope}/{name}/issues/{id}", s.issue)
	mux.HandleFunc("PATCH /api/repos/{scope}/{name}/issues/{id}", s.updateIssue)
	mux.HandleFunc("DELETE /api/repos/{scope}/{name}/issues/{id}", s.deleteIssue)
	mux.HandleFunc("GET /api/repos/{scope}/{name}/issues/{id}/comments", s.listIssueComments)
	mux.HandleFunc("POST /api/repos/{scope}/{name}/issues/{id}/comments", s.createIssueComment)
	mux.HandleFunc("GET /api/repos/{scope}/{name}/pulls", s.listPullRequests)
	mux.HandleFunc("POST /api/repos/{scope}/{name}/pulls", s.createPullRequest)
	mux.HandleFunc("GET /api/repos/{scope}/{name}/pulls/{id}", s.pullRequest)
	mux.HandleFunc("GET /api/repos/{scope}/{name}/pulls/{id}/comments", s.pullRequestComments)
	mux.HandleFunc("POST /api/repos/{scope}/{name}/pulls/{id}/comments", s.createPullRequestComment)
	mux.HandleFunc("GET /api/repos/{scope}/{name}/pulls/{id}/files", s.pullRequestFiles)
	mux.HandleFunc("POST /api/repos/{scope}/{name}/pulls/{id}/merge", s.mergePullRequest)
	mux.HandleFunc("PATCH /api/repos/{scope}/{name}/pulls/{id}", s.updatePullRequest)
	mux.HandleFunc("DELETE /api/repos/{scope}/{name}/pulls/{id}", s.deletePullRequest)
	mux.HandleFunc("GET /api/repos/{scope}/{name}/releases", s.listReleases)
	mux.HandleFunc("POST /api/repos/{scope}/{name}/releases", s.createRelease)
	mux.HandleFunc("DELETE /api/repos/{scope}/{name}/releases/{id}", s.deleteRelease)
	mux.HandleFunc("POST /api/repos/{scope}/{name}/releases/{id}/assets", s.uploadReleaseAsset)
	mux.HandleFunc("GET /api/repos/{scope}/{name}/contributors", s.contributors)
	mux.HandleFunc("GET /api/repos/{scope}/{name}", s.getRepo)
	mux.Handle("GET /uploads/avatars/", http.StripPrefix("/uploads/avatars/", http.FileServer(http.Dir(s.avatarDir))))
	mux.Handle("GET /uploads/releases/", http.StripPrefix("/uploads/releases/", http.FileServer(http.Dir(s.releaseDir))))
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
		Scope        string `json:"scope"`
		Name         string `json:"name"`
		Description  string `json:"description"`
		Visibility   string `json:"visibility"`
		License      string `json:"license"`
		CreateReadme bool   `json:"createReadme"`
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
		settings, err := s.repos.Settings(r.Context(), repo.FullName)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Could not configure repository.")
			return
		}
		settings.Description = strings.TrimSpace(input.Description)
		if input.Visibility != "" {
			settings.Visibility = input.Visibility
		}
		if err := s.repos.UpdateSettings(r.Context(), repo.FullName, settings); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := s.repos.Initialize(r.Context(), repo, user.Username, user.Email, input.CreateReadme, input.License); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
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
func (s *Server) sshSettings(w http.ResponseWriter, r *http.Request) {
	host := s.sshHost
	if host == "" {
		host, _, _ = net.SplitHostPort(r.Host)
		if host == "" {
			host = r.Host
		}
	}
	writeJSON(w, 200, map[string]string{"host": host, "port": s.sshPort})
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
func (s *Server) uploadAvatar(w http.ResponseWriter, r *http.Request) {
	user, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	if err := r.ParseMultipartForm(2 << 20); err != nil {
		writeError(w, 400, "头像文件过大。 ")
		return
	}
	file, _, err := r.FormFile("avatar")
	if err != nil {
		writeError(w, 400, "请选择头像文件。 ")
		return
	}
	defer file.Close()
	content, err := io.ReadAll(io.LimitReader(file, 2<<20))
	if err != nil {
		writeError(w, 400, "无法读取头像文件。 ")
		return
	}
	kind := http.DetectContentType(content)
	ext := map[string]string{"image/jpeg": ".jpg", "image/png": ".png", "image/gif": ".gif", "image/webp": ".webp"}[kind]
	if ext == "" {
		writeError(w, 400, "头像仅支持 JPG、PNG、GIF 或 WebP。 ")
		return
	}
	filename := fmt.Sprintf("%s-%d%s", user.Username, time.Now().UnixNano(), ext)
	if err := os.WriteFile(filepath.Join(s.avatarDir, filename), content, 0o644); err != nil {
		writeError(w, 500, "无法保存头像。 ")
		return
	}
	value, err := s.forge.SetAvatar(r.Context(), user, "/uploads/avatars/"+filename)
	if err != nil {
		writeError(w, 500, "无法更新头像。 ")
		return
	}
	writeJSON(w, 200, value)
}
func (s *Server) sshKeys(w http.ResponseWriter, r *http.Request) {
	user, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	items, err := s.forge.SSHKeys(r.Context(), user.ID)
	if err != nil {
		writeError(w, 500, "无法读取 SSH 密钥。")
		return
	}
	writeJSON(w, 200, items)
}
func (s *Server) addSSHKey(w http.ResponseWriter, r *http.Request) {
	user, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	var input struct {
		Title string `json:"title"`
		Key   string `json:"key"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	key, _, _, _, err := cryptossh.ParseAuthorizedKey([]byte(input.Key))
	if err != nil {
		writeError(w, 400, "无效的 SSH 公钥。")
		return
	}
	item, err := s.forge.AddSSHKey(r.Context(), user, input.Title, strings.TrimSpace(string(cryptossh.MarshalAuthorizedKey(key))))
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	writeJSON(w, 201, item)
}
func (s *Server) deleteSSHKey(w http.ResponseWriter, r *http.Request) {
	user, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if err := s.forge.DeleteSSHKey(r.Context(), user.ID, id); errors.Is(err, forge.ErrNotFound) {
		writeError(w, 404, "未找到 SSH 密钥。")
	} else if err != nil {
		writeError(w, 500, "无法删除 SSH 密钥。")
	} else {
		w.WriteHeader(204)
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
	if user, err := s.currentUser(r); err == nil {
		org.Followed, _ = s.forge.OrganizationFollowed(r.Context(), user.ID, org.Name)
	}
	writeJSON(w, 200, org)
}
func (s *Server) followOrganization(w http.ResponseWriter, r *http.Request) {
	user, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	followed, followers, err := s.forge.ToggleOrganizationFollow(r.Context(), user, r.PathValue("name"))
	if errors.Is(err, forge.ErrNotFound) {
		writeError(w, 404, "未找到组织。 ")
		return
	}
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"followed": followed, "followers": followers})
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
func (s *Server) organizationFollowers(w http.ResponseWriter, r *http.Request) {
	items, err := s.forge.OrganizationFollowers(r.Context(), r.PathValue("name"))
	if err != nil {
		writeError(w, 500, "Could not load organization followers.")
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
	if user, err := s.currentUser(r); err == nil {
		value.Followed, _ = s.forge.UserFollowed(r.Context(), user.ID, value.Username)
	}
	writeJSON(w, 200, value)
}
func (s *Server) followUser(w http.ResponseWriter, r *http.Request) {
	user, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	followed, followers, err := s.forge.ToggleUserFollow(r.Context(), user, r.PathValue("username"))
	if errors.Is(err, forge.ErrNotFound) {
		writeError(w, 404, "未找到用户。 ")
		return
	}
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"followed": followed, "followers": followers})
}
func (s *Server) userRepos(w http.ResponseWriter, r *http.Request) {
	items, err := s.repos.ListByScope(r.PathValue("username"))
	if err != nil {
		writeError(w, 500, "Could not load repositories.")
		return
	}
	writeJSON(w, 200, items)
}
func (s *Server) userFollowers(w http.ResponseWriter, r *http.Request) {
	items, err := s.forge.UserFollowers(r.Context(), r.PathValue("username"))
	if err != nil {
		writeError(w, 500, "Could not load followers.")
		return
	}
	writeJSON(w, 200, items)
}
func (s *Server) userFollowing(w http.ResponseWriter, r *http.Request) {
	items, err := s.forge.UserFollowing(r.Context(), r.PathValue("username"))
	if err != nil {
		writeError(w, 500, "Could not load following.")
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
	if settings, err := s.repos.Settings(r.Context(), source.FullName); err != nil {
		writeError(w, 500, "Could not load repository settings.")
		return
	} else if !settings.AllowForks {
		writeError(w, 403, "Forking is disabled for this repository.")
		return
	}
	var input struct {
		Scope             string `json:"scope"`
		Name              string `json:"name"`
		DefaultBranchOnly bool   `json:"defaultBranchOnly"`
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
	repo, err := s.repos.Fork(r.Context(), source, strings.TrimSpace(input.Scope), strings.TrimSpace(input.Name), input.DefaultBranchOnly)
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
func (s *Server) forks(w http.ResponseWriter, r *http.Request) {
	if !s.hasRepo(w, r) {
		return
	}
	items, err := s.repos.Forks(r.Context(), repoKey(r))
	if err != nil {
		writeError(w, 500, "Could not load forks.")
		return
	}
	writeJSON(w, 200, items)
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
func (s *Server) labels(w http.ResponseWriter, r *http.Request) {
	if !s.hasRepo(w, r) {
		return
	}
	items, err := s.forge.Labels(r.Context(), repoKey(r))
	if err != nil {
		writeError(w, 500, "无法读取标签。")
		return
	}
	writeJSON(w, 200, items)
}
func (s *Server) createLabel(w http.ResponseWriter, r *http.Request) {
	if !s.hasRepo(w, r) {
		return
	}
	user, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	if !s.canManageRepository(r, user) {
		writeError(w, 403, "无权管理仓库标签。")
		return
	}
	var input forge.Label
	if !decodeJSON(w, r, &input) {
		return
	}
	item, err := s.forge.CreateLabel(r.Context(), repoKey(r), input.Name, input.Color, input.Description)
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	writeJSON(w, 201, item)
}
func (s *Server) updateLabel(w http.ResponseWriter, r *http.Request) {
	if !s.hasRepo(w, r) {
		return
	}
	user, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	if !s.canManageRepository(r, user) {
		writeError(w, 403, "无权管理仓库标签。 ")
		return
	}
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var input forge.Label
	if !decodeJSON(w, r, &input) {
		return
	}
	item, err := s.forge.UpdateLabel(r.Context(), repoKey(r), id, input.Name, input.Color, input.Description)
	if errors.Is(err, forge.ErrNotFound) {
		writeError(w, 404, "未找到标签。")
	} else if err != nil {
		writeError(w, 400, err.Error())
	} else {
		writeJSON(w, 200, item)
	}
}
func (s *Server) deleteLabel(w http.ResponseWriter, r *http.Request) {
	if !s.hasRepo(w, r) {
		return
	}
	user, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	if !s.canManageRepository(r, user) {
		writeError(w, 403, "无权管理仓库标签。 ")
		return
	}
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if err := s.forge.DeleteLabel(r.Context(), repoKey(r), id); errors.Is(err, forge.ErrNotFound) {
		writeError(w, 404, "未找到标签。")
	} else if err != nil {
		writeError(w, 500, "无法删除标签。")
	} else {
		w.WriteHeader(204)
	}
}
func (s *Server) setIssueLabels(w http.ResponseWriter, r *http.Request) {
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
		LabelIDs []int64 `json:"labelIds"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	if err := s.forge.SetIssueLabels(r.Context(), repoKey(r), id, input.LabelIDs); errors.Is(err, forge.ErrNotFound) {
		writeError(w, 404, "未找到议题或标签。")
	} else if err != nil {
		writeError(w, 500, "无法更新议题标签。")
	} else {
		w.WriteHeader(204)
	}
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
		Title    string  `json:"title"`
		Body     string  `json:"body"`
		LabelIDs []int64 `json:"labelIds"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	item, err := s.forge.CreateIssue(r.Context(), repoKey(r), user, input.Title, input.Body)
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	if len(input.LabelIDs) > 0 {
		if err := s.forge.SetIssueLabels(r.Context(), repoKey(r), item.ID, input.LabelIDs); err != nil {
			writeError(w, 400, "无法设置议题标签。")
			return
		}
		item.Labels, _ = s.forge.IssueLabels(r.Context(), item.ID)
	}
	s.notifyRepositoryOwner(r, user, "issue", user.Username+" opened an issue", item.Title)
	writeJSON(w, 201, item)
}
func (s *Server) issue(w http.ResponseWriter, r *http.Request) {
	if !s.hasRepo(w, r) {
		return
	}
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	item, err := s.forge.Issue(r.Context(), repoKey(r), id)
	if errors.Is(err, forge.ErrNotFound) {
		writeError(w, http.StatusNotFound, "Issue not found.")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Could not load issue.")
		return
	}
	writeJSON(w, http.StatusOK, item)
}
func (s *Server) listIssueComments(w http.ResponseWriter, r *http.Request) {
	if !s.hasRepo(w, r) {
		return
	}
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if _, err := s.forge.Issue(r.Context(), repoKey(r), id); errors.Is(err, forge.ErrNotFound) {
		writeError(w, http.StatusNotFound, "Issue not found.")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "Could not load issue comments.")
		return
	}
	items, err := s.forge.ListIssueComments(r.Context(), repoKey(r), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Could not load issue comments.")
		return
	}
	writeJSON(w, http.StatusOK, items)
}
func (s *Server) createIssueComment(w http.ResponseWriter, r *http.Request) {
	if !s.hasRepo(w, r) {
		return
	}
	user, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var input struct {
		Body string `json:"body"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	comment, err := s.forge.CreateIssueComment(r.Context(), repoKey(r), id, user, input.Body)
	if errors.Is(err, forge.ErrNotFound) {
		writeError(w, http.StatusNotFound, "Issue not found.")
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, comment)
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
func (s *Server) pullRequest(w http.ResponseWriter, r *http.Request) {
	if !s.hasRepo(w, r) {
		return
	}
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	item, err := s.forge.PullRequest(r.Context(), repoKey(r), id)
	if errors.Is(err, forge.ErrNotFound) {
		writeError(w, 404, "Pull request not found.")
	} else if err != nil {
		writeError(w, 500, "Could not load pull request.")
	} else {
		writeJSON(w, 200, item)
	}
}
func (s *Server) pullRequestComments(w http.ResponseWriter, r *http.Request) {
	if !s.hasRepo(w, r) {
		return
	}
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	items, err := s.forge.ListPullRequestComments(r.Context(), repoKey(r), id)
	if err != nil {
		writeError(w, 500, "Could not load pull request comments.")
		return
	}
	writeJSON(w, 200, items)
}
func (s *Server) createPullRequestComment(w http.ResponseWriter, r *http.Request) {
	if !s.hasRepo(w, r) {
		return
	}
	user, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var input struct {
		Body string `json:"body"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	item, err := s.forge.CreatePullRequestComment(r.Context(), repoKey(r), id, user, input.Body)
	if errors.Is(err, forge.ErrNotFound) {
		writeError(w, 404, "Pull request not found.")
	} else if err != nil {
		writeError(w, 400, err.Error())
	} else {
		writeJSON(w, 201, item)
	}
}
func (s *Server) pullRequestFiles(w http.ResponseWriter, r *http.Request) {
	repo, ok := s.requireRepo(w, r)
	if !ok {
		return
	}
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	pull, err := s.forge.PullRequest(r.Context(), repoKey(r), id)
	if errors.Is(err, forge.ErrNotFound) {
		writeError(w, 404, "Pull request not found.")
		return
	}
	if err != nil {
		writeError(w, 500, "Could not load pull request.")
		return
	}
	files, err := s.repos.PullRequestDiff(r.Context(), repo, pull.SourceBranch, pull.TargetBranch)
	if errors.Is(err, repository.ErrNotFound) {
		writeError(w, 400, "Pull request branches are invalid.")
	} else if err != nil {
		writeError(w, 400, err.Error())
	} else {
		writeJSON(w, 200, files)
	}
}
func (s *Server) mergePullRequest(w http.ResponseWriter, r *http.Request) {
	repo, ok := s.requireRepo(w, r)
	if !ok {
		return
	}
	user, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	if !s.canManageRepository(r, user) {
		writeError(w, 403, "无权合并此拉取请求。")
		return
	}
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	pull, err := s.forge.PullRequest(r.Context(), repoKey(r), id)
	if errors.Is(err, forge.ErrNotFound) {
		writeError(w, 404, "Pull request not found.")
		return
	}
	if err != nil || pull.State != "open" {
		writeError(w, 400, "Only open pull requests can be merged.")
		return
	}
	commit, err := s.repos.MergePullRequest(r.Context(), repo, pull.SourceBranch, pull.TargetBranch, user.Username, user.Email)
	if err != nil {
		writeError(w, 409, err.Error())
		return
	}
	if err := s.forge.UpdatePullRequestState(r.Context(), repoKey(r), id, "merged"); err != nil {
		writeError(w, 500, "Could not update pull request state.")
		return
	}
	s.autoClosePullIssues(r, pull)
	writeJSON(w, 200, commit)
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
	if input.State == "merged" {
		writeError(w, 400, "Use the merge endpoint to merge a pull request.")
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
func (s *Server) autoClosePullIssues(r *http.Request, pull forge.PullRequest) {
	if settings, err := s.repos.Settings(r.Context(), repoKey(r)); err == nil && settings.AutoCloseIssues {
		for _, match := range issueCloseReference.FindAllStringSubmatch(pull.Title+"\n"+pull.Body, -1) {
			if issueID, err := strconv.ParseInt(match[1], 10, 64); err == nil {
				_ = s.forge.UpdateIssueState(r.Context(), repoKey(r), issueID, "closed")
			}
		}
	}
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
	writeJSON(w, 200, s.withReleaseAssetURLs(items))
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
		TagName   string `json:"tagName"`
		Title     string `json:"title"`
		Notes     string `json:"notes"`
		CreateTag bool   `json:"createTag"`
		TargetRef string `json:"targetRef"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	repo, ok := s.requireRepo(w, r)
	if !ok {
		return
	}
	tags, err := s.repos.Tags(r.Context(), repo)
	if err != nil {
		writeError(w, 500, "无法读取标签。")
		return
	}
	exists := false
	for _, tag := range tags {
		if tag.Name == input.TagName {
			exists = true
			break
		}
	}
	if !exists {
		if !input.CreateTag {
			writeError(w, 400, "请选择已有标签或创建新标签。")
			return
		}
		if err := s.repos.CreateTag(r.Context(), repo, input.TagName, valueOr(input.TargetRef, "HEAD")); err != nil {
			writeError(w, 400, "无法创建标签："+err.Error())
			return
		}
	}
	item, err := s.forge.CreateRelease(r.Context(), repoKey(r), user, input.TagName, input.Title, input.Notes)
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	writeJSON(w, 201, item)
}
func (s *Server) uploadReleaseAsset(w http.ResponseWriter, r *http.Request) {
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
	items, err := s.forge.ListReleases(r.Context(), repoKey(r))
	if err != nil {
		writeError(w, 500, "无法读取发布版本。")
		return
	}
	found := false
	for _, item := range items {
		if item.ID == id {
			found = true
			break
		}
	}
	if !found {
		writeError(w, 404, "未找到发布版本。")
		return
	}
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		writeError(w, 400, "发布文件过大。")
		return
	}
	file, header, err := r.FormFile("asset")
	if err != nil {
		writeError(w, 400, "请选择发布文件。")
		return
	}
	defer file.Close()
	content, err := io.ReadAll(io.LimitReader(file, 32<<20))
	if err != nil {
		writeError(w, 400, "无法读取发布文件。")
		return
	}
	name := filepath.Base(header.Filename)
	if name == "." || name == "" {
		writeError(w, 400, "无效的文件名。")
		return
	}
	storage := fmt.Sprintf("%d-%d-%s", id, time.Now().UnixNano(), name)
	if err := os.WriteFile(filepath.Join(s.releaseDir, storage), content, 0o644); err != nil {
		writeError(w, 500, "无法保存发布文件。")
		return
	}
	asset, err := s.forge.AddReleaseAsset(r.Context(), id, name, storage, int64(len(content)))
	if err != nil {
		writeError(w, 500, "无法记录发布文件。")
		return
	}
	asset.URL = "/uploads/releases/" + storage
	writeJSON(w, 201, asset)
}
func (s *Server) withReleaseAssetURLs(items []forge.Release) []forge.Release {
	for i := range items {
		for j := range items[i].Assets {
			items[i].Assets[j].URL = "/uploads/releases/" + items[i].Assets[j].StorageName
		}
	}
	return items
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

func (s *Server) archive(w http.ResponseWriter, r *http.Request) {
	repo, ok := s.requireRepo(w, r)
	if !ok {
		return
	}
	format := valueOr(r.URL.Query().Get("format"), "zip")
	content, contentType, err := s.repos.ArchiveFormat(r.Context(), repo, valueOr(r.URL.Query().Get("ref"), "HEAD"), format)
	if errors.Is(err, repository.ErrNotFound) {
		writeError(w, 404, "未找到可下载的提交。")
		return
	}
	if err != nil {
		writeError(w, 500, "无法生成 ZIP 文件。 ")
		return
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", `attachment; filename="`+repo.Name+"-"+valueOr(r.URL.Query().Get("ref"), "HEAD")+"."+format+`"`)
	w.WriteHeader(200)
	_, _ = w.Write(content)
}
func (s *Server) writeFile(w http.ResponseWriter, r *http.Request) {
	repo, ok := s.requireRepo(w, r)
	if !ok {
		return
	}
	user, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	allowed, err := s.forge.CanUseScope(r.Context(), user, repo.Scope)
	if err != nil || !allowed {
		writeError(w, 403, "无权修改此仓库。")
		return
	}
	var input struct {
		Path    string `json:"path"`
		Content string `json:"content"`
		Branch  string `json:"branch"`
		Message string `json:"message"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	commit, err := s.repos.WriteFile(r.Context(), repo, valueOr(input.Branch, "main"), input.Path, input.Content, user.Username, user.Email, input.Message)
	if errors.Is(err, repository.ErrNotFound) {
		writeError(w, 400, "文件路径或分支无效。 ")
		return
	}
	if err != nil {
		writeError(w, 500, "无法创建文件提交："+err.Error())
		return
	}
	writeJSON(w, 201, commit)
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
func (s *Server) fileTree(w http.ResponseWriter, r *http.Request) {
	repo, ok := s.requireRepo(w, r)
	if !ok {
		return
	}
	items, err := s.repos.FileTree(r.Context(), repo, valueOr(r.URL.Query().Get("ref"), "HEAD"))
	if errors.Is(err, repository.ErrNotFound) {
		writeError(w, 404, "Tree not found.")
	} else if err != nil {
		writeError(w, 500, "Could not browse repository.")
	} else {
		writeJSON(w, 200, items)
	}
}
func (s *Server) languages(w http.ResponseWriter, r *http.Request) {
	repo, ok := s.requireRepo(w, r)
	if !ok {
		return
	}
	items, err := s.repos.Languages(r.Context(), repo, valueOr(r.URL.Query().Get("ref"), "HEAD"))
	if errors.Is(err, repository.ErrNotFound) {
		writeError(w, 404, "Repository ref not found.")
	} else if err != nil {
		writeError(w, 500, "Could not calculate languages.")
	} else {
		writeJSON(w, 200, items)
	}
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
func (s *Server) rawFile(w http.ResponseWriter, r *http.Request) {
	repo, ok := s.requireRepo(w, r)
	if !ok {
		return
	}
	path := r.URL.Query().Get("path")
	content, err := s.repos.RawFile(r.Context(), repo, valueOr(r.URL.Query().Get("ref"), "HEAD"), path)
	if errors.Is(err, repository.ErrNotFound) {
		writeError(w, 404, "File not found.")
		return
	}
	if err != nil {
		writeError(w, 500, "Could not read file.")
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", `attachment; filename="`+filepath.Base(path)+`"`)
	_, _ = w.Write(content)
}
func (s *Server) fileCommit(w http.ResponseWriter, r *http.Request) {
	repo, ok := s.requireRepo(w, r)
	if !ok {
		return
	}
	item, err := s.repos.FileCommit(r.Context(), repo, valueOr(r.URL.Query().Get("ref"), "HEAD"), r.URL.Query().Get("path"))
	if errors.Is(err, repository.ErrNotFound) {
		writeError(w, 404, "File history not found.")
	} else if err != nil {
		writeError(w, 500, "Could not load file history.")
	} else {
		writeJSON(w, 200, item)
	}
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
func (s *Server) commit(w http.ResponseWriter, r *http.Request) {
	repo, ok := s.requireRepo(w, r)
	if !ok {
		return
	}
	item, err := s.repos.Commit(r.Context(), repo, r.PathValue("hash"))
	if errors.Is(err, repository.ErrNotFound) {
		writeError(w, 404, "未找到提交记录。")
		return
	}
	if err != nil {
		writeError(w, 500, "无法读取提交详情。")
		return
	}
	writeJSON(w, 200, item)
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
func (s *Server) repositoryLicense(w http.ResponseWriter, r *http.Request) {
	repo, ok := s.requireRepo(w, r)
	if !ok {
		return
	}
	writeJSON(w, 200, map[string]string{"license": s.repos.DetectLicense(r.Context(), repo, valueOr(r.URL.Query().Get("ref"), "HEAD"))})
}
func (s *Server) updateRepositorySettings(w http.ResponseWriter, r *http.Request) {
	if !s.hasRepo(w, r) {
		return
	}
	user, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	if !s.canManageRepository(r, user) {
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

func (s *Server) updateRepositoryVisibility(w http.ResponseWriter, r *http.Request) {
	if !s.hasRepo(w, r) {
		return
	}
	user, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	if !s.canManageRepository(r, user) {
		writeError(w, 403, "No permission to manage this repository.")
		return
	}
	var input struct {
		Visibility  string `json:"visibility"`
		ConfirmName string `json:"confirmName"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	if input.ConfirmName != repoKey(r) {
		writeError(w, 400, "请输入完整仓库名称以确认。 ")
		return
	}
	if err := s.repos.UpdateVisibility(r.Context(), repoKey(r), input.Visibility); err != nil {
		writeError(w, 400, err.Error())
		return
	}
	value, _ := s.repos.Settings(r.Context(), repoKey(r))
	writeJSON(w, 200, value)
}
func (s *Server) collaborators(w http.ResponseWriter, r *http.Request) {
	if !s.hasRepo(w, r) {
		return
	}
	items, err := s.repos.Collaborators(r.Context(), repoKey(r))
	if err != nil {
		writeError(w, 500, "无法读取协作者。 ")
		return
	}
	writeJSON(w, 200, items)
}
func (s *Server) setCollaborator(w http.ResponseWriter, r *http.Request) {
	if !s.hasRepo(w, r) {
		return
	}
	user, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	if !s.canManageRepository(r, user) {
		writeError(w, 403, "无权管理协作者。 ")
		return
	}
	var input struct {
		Permission string `json:"permission"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	if err := s.repos.SetCollaborator(r.Context(), repoKey(r), r.PathValue("username"), input.Permission); err != nil {
		writeError(w, 400, err.Error())
		return
	}
	w.WriteHeader(204)
}
func (s *Server) removeCollaborator(w http.ResponseWriter, r *http.Request) {
	if !s.hasRepo(w, r) {
		return
	}
	user, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	if !s.canManageRepository(r, user) {
		writeError(w, 403, "无权管理协作者。 ")
		return
	}
	if err := s.repos.RemoveCollaborator(r.Context(), repoKey(r), r.PathValue("username")); errors.Is(err, repository.ErrNotFound) {
		writeError(w, 404, "未找到协作者。")
	} else if err != nil {
		writeError(w, 500, "无法移除协作者。")
	} else {
		w.WriteHeader(204)
	}
}
func (s *Server) protectedBranches(w http.ResponseWriter, r *http.Request) {
	if !s.hasRepo(w, r) {
		return
	}
	items, err := s.repos.ProtectedBranches(r.Context(), repoKey(r))
	if err != nil {
		writeError(w, 500, "无法读取保护分支。 ")
		return
	}
	writeJSON(w, 200, items)
}
func (s *Server) setProtectedBranch(w http.ResponseWriter, r *http.Request) {
	if !s.hasRepo(w, r) {
		return
	}
	user, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	if !s.canManageRepository(r, user) {
		writeError(w, 403, "无权管理分支保护。 ")
		return
	}
	var input repository.ProtectedBranch
	if !decodeJSON(w, r, &input) {
		return
	}
	input.Branch = r.PathValue("branch")
	if err := s.repos.SetProtectedBranch(r.Context(), repoKey(r), input); err != nil {
		writeError(w, 400, err.Error())
		return
	}
	writeJSON(w, 200, input)
}
func (s *Server) removeProtectedBranch(w http.ResponseWriter, r *http.Request) {
	if !s.hasRepo(w, r) {
		return
	}
	user, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	if !s.canManageRepository(r, user) {
		writeError(w, 403, "无权管理分支保护。 ")
		return
	}
	if err := s.repos.RemoveProtectedBranch(r.Context(), repoKey(r), r.PathValue("branch")); err != nil {
		writeError(w, 500, "无法移除分支保护。 ")
		return
	}
	w.WriteHeader(204)
}
func (s *Server) transferRepository(w http.ResponseWriter, r *http.Request) {
	repo, ok := s.requireRepo(w, r)
	if !ok {
		return
	}
	user, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	if !s.canManageRepository(r, user) {
		writeError(w, 403, "无权转移仓库。 ")
		return
	}
	var input struct {
		TargetScope string `json:"targetScope"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	allowed, err := s.forge.CanUseScope(r.Context(), user, strings.TrimSpace(input.TargetScope))
	if err != nil || !allowed {
		writeError(w, 403, "请选择你拥有权限的目标空间。 ")
		return
	}
	next, err := s.repos.Transfer(r.Context(), repo, strings.TrimSpace(input.TargetScope))
	if errors.Is(err, repository.ErrExists) {
		writeError(w, 409, "目标空间已有同名仓库。")
	} else if err != nil {
		writeError(w, 400, err.Error())
	} else {
		writeJSON(w, 200, next)
	}
}
func (s *Server) renameRepository(w http.ResponseWriter, r *http.Request) {
	repo, ok := s.requireRepo(w, r)
	if !ok {
		return
	}
	user, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	if !s.canManageRepository(r, user) {
		writeError(w, 403, "无权重命名仓库。")
		return
	}
	var input struct {
		NewName     string `json:"newName"`
		ConfirmName string `json:"confirmName"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	if input.ConfirmName != repo.FullName {
		writeError(w, 400, "请输入当前完整仓库名称以确认。")
		return
	}
	next, err := s.repos.Rename(r.Context(), repo, strings.TrimSpace(input.NewName))
	if errors.Is(err, repository.ErrExists) {
		writeError(w, 409, "当前空间已有同名仓库。")
	} else if err != nil {
		writeError(w, 400, err.Error())
	} else {
		writeJSON(w, 200, next)
	}
}
func (s *Server) deleteRepository(w http.ResponseWriter, r *http.Request) {
	repo, ok := s.requireRepo(w, r)
	if !ok {
		return
	}
	user, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	if !s.canManageRepository(r, user) {
		writeError(w, 403, "无权删除仓库。 ")
		return
	}
	var input struct {
		ConfirmName string `json:"confirmName"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	if input.ConfirmName != repo.FullName {
		writeError(w, 400, "请输入完整仓库名称以确认。 ")
		return
	}
	if err := s.repos.Delete(r.Context(), repo); err != nil {
		writeError(w, 500, "无法删除仓库。 ")
		return
	}
	w.WriteHeader(204)
}
func (s *Server) canManageRepository(r *http.Request, user forge.User) bool {
	if allowed, err := s.forge.CanUseScope(r.Context(), user, r.PathValue("scope")); err == nil && allowed {
		return true
	}
	permission, err := s.repos.CollaboratorPermission(r.Context(), repoKey(r), user.Username)
	return err == nil && (permission == "maintain" || permission == "admin")
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

var issueCloseReference = regexp.MustCompile(`(?i)(?:close[sd]?|fix(?:e[sd])?|resolve[sd]?)\s+#(\d+)`)

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
