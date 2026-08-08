package server

import (
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path"
	"strings"

	"kohame/internal/config"
	"kohame/internal/database"
	"kohame/internal/repository"
	"kohame/internal/web"
)

type Server struct{ repos *repository.Store }

func New(cfg config.Config) (*Server, error) {
	if err := os.MkdirAll(cfg.Storage.RepositoryRoot, 0o755); err != nil {
		return nil, err
	}
	db, err := database.Open(cfg.Database)
	if err != nil {
		return nil, err
	}
	return &Server{repos: repository.NewStore(cfg.Storage.RepositoryRoot, db, cfg.Database.Driver)}, nil
}

func (s *Server) Close() error { return s.repos.Close() }

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/repos", s.listRepos)
	mux.HandleFunc("POST /api/repos", s.createRepo)
	mux.HandleFunc("GET /api/repos/{name}", s.getRepo)
	mux.HandleFunc("/git/{name}/{rest...}", s.gitHTTP)
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
	repo, err := s.repos.Get(r.PathValue("name"))
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
	var input struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "Provide a repository name.")
		return
	}
	repo, err := s.repos.Create(r.Context(), strings.TrimSpace(input.Name))
	switch {
	case errors.Is(err, repository.ErrInvalidName):
		writeError(w, http.StatusBadRequest, "Use 1–80 lowercase letters, numbers, dots, hyphens, or underscores.")
	case errors.Is(err, repository.ErrExists):
		writeError(w, http.StatusConflict, "That repository already exists.")
	case err != nil:
		writeError(w, http.StatusInternalServerError, "Could not create repository.")
	default:
		writeJSON(w, http.StatusCreated, repo)
	}
}

// gitHTTP delegates Git smart-HTTP protocol framing to the installed git executable.
func (s *Server) gitHTTP(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if _, err := s.repos.Get(name); err != nil {
		http.NotFound(w, r)
		return
	}
	rest := r.PathValue("rest")
	if strings.Contains(rest, "..") {
		http.NotFound(w, r)
		return
	}
	cmd := exec.CommandContext(r.Context(), "git", "http-backend")
	cmd.Env = append(os.Environ(),
		"GIT_HTTP_EXPORT_ALL=1",
		"GIT_PROJECT_ROOT="+s.reposRoot(),
		"PATH_INFO=/"+name+".git/"+rest,
		"REQUEST_METHOD="+r.Method,
		"QUERY_STRING="+r.URL.RawQuery,
		"CONTENT_TYPE="+r.Header.Get("Content-Type"),
		"CONTENT_LENGTH="+r.Header.Get("Content-Length"),
		"REMOTE_USER=kohame",
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
