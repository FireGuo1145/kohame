package main

import (
	"archive/tar"
	"bytes"
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
	"kohame/internal/forge"
)

type executeRequest struct {
	Event      string               `json:"event"`
	Repository string               `json:"repository"`
	Workflow   string               `json:"workflow"`
	Workspace  []byte               `json:"workspace"`
	Steps      []forge.WorkflowStep `json:"steps"`
}

type executeResponse struct {
	Status string `json:"status"`
	Output string `json:"output"`
}

type actionDefinition struct {
	Runs struct {
		Using string               `yaml:"using"`
		Main  string               `yaml:"main"`
		Image string               `yaml:"image"`
		Steps []forge.WorkflowStep `yaml:"steps"`
	} `yaml:"runs"`
}

type runner struct{ token string }

func main() {
	addr := flag.String("addr", ":8090", "runner listen address")
	token := flag.String("token", os.Getenv("KOHAME_RUNNER_TOKEN"), "shared token used by Kohame")
	flag.Parse()
	handler := &runner{token: strings.TrimSpace(*token)}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	mux.HandleFunc("POST /v1/execute", handler.execute)
	server := &http.Server{Addr: *addr, Handler: requestLimit(mux)}
	if handler.token == "" {
		fmt.Fprintln(os.Stderr, "warning: runner token is empty; configure -token or KOHAME_RUNNER_TOKEN")
	}
	fmt.Printf("Kohame runner listening on http://localhost%s\n", *addr)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		panic(err)
	}
}

func requestLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.ContentLength > 64<<20 {
			http.Error(w, "request too large", http.StatusRequestEntityTooLarge)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (r *runner) execute(w http.ResponseWriter, request *http.Request) {
	if r.token != "" {
		provided := strings.TrimPrefix(request.Header.Get("Authorization"), "Bearer ")
		if subtle.ConstantTimeCompare([]byte(provided), []byte(r.token)) != 1 {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
	}
	var input executeRequest
	if err := json.NewDecoder(io.LimitReader(request.Body, 64<<20)).Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, executeResponse{Status: "failure", Output: err.Error()})
		return
	}
	status, output := executeWorkflow(request.Context(), input)
	writeJSON(w, http.StatusOK, executeResponse{Status: status, Output: output})
}

func executeWorkflow(ctx context.Context, input executeRequest) (string, string) {
	work, err := os.MkdirTemp("", "kohame-runner-")
	if err != nil {
		return "failure", err.Error()
	}
	defer os.RemoveAll(work)
	if err := extractTar(work, input.Workspace); err != nil {
		return "failure", err.Error()
	}
	var output strings.Builder
	failed := false
	baseEnv := []string{"GITHUB_WORKSPACE=" + work, "GITHUB_REPOSITORY=" + input.Repository, "GITHUB_EVENT_NAME=" + input.Event, "CI=true"}
	for _, step := range input.Steps {
		if failed {
			break
		}
		if strings.TrimSpace(step.Uses) != "" {
			err = runAction(ctx, work, step.Uses, step.With, baseEnv, &output, 0)
		} else {
			err = runCommand(ctx, work, step.Run, baseEnv, &output)
		}
		if err != nil {
			failed = true
			output.WriteString("\n" + err.Error())
		}
	}
	if failed {
		return "failure", output.String()
	}
	return "success", output.String()
}

func extractTar(root string, content []byte) error {
	reader := tar.NewReader(bytes.NewReader(content))
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("read workspace: %w", err)
		}
		name := filepath.Clean(filepath.FromSlash(header.Name))
		if name == "." || filepath.IsAbs(name) || strings.HasPrefix(name, ".."+string(os.PathSeparator)) || name == ".." {
			return errors.New("workspace archive contains an unsafe path")
		}
		target := filepath.Join(root, name)
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			file, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
			if err != nil {
				return err
			}
			_, copyErr := io.Copy(file, io.LimitReader(reader, 16<<20))
			closeErr := file.Close()
			if copyErr != nil {
				return copyErr
			}
			if closeErr != nil {
				return closeErr
			}
		}
	}
}

func runCommand(ctx context.Context, work, command string, env []string, output *strings.Builder) error {
	if strings.TrimSpace(command) == "" {
		return errors.New("empty run command")
	}
	process := exec.CommandContext(ctx, "sh", "-c", command)
	process.Dir = work
	process.Env = append(os.Environ(), env...)
	value, err := process.CombinedOutput()
	output.Write(value)
	return err
}

func runAction(ctx context.Context, work, uses string, with map[string]string, baseEnv []string, output *strings.Builder, depth int) error {
	if depth > 8 {
		return errors.New("action nesting is too deep")
	}
	actionDir, cleanup, err := actionDirectory(ctx, work, uses)
	if err != nil {
		return err
	}
	defer cleanup()
	content, err := os.ReadFile(filepath.Join(actionDir, "action.yml"))
	if os.IsNotExist(err) {
		content, err = os.ReadFile(filepath.Join(actionDir, "action.yaml"))
	}
	if err != nil {
		return fmt.Errorf("read action metadata: %w", err)
	}
	var definition actionDefinition
	if err := yaml.Unmarshal(content, &definition); err != nil {
		return fmt.Errorf("parse action metadata: %w", err)
	}
	env := append(append([]string{}, baseEnv...), actionInputs(with)...)
	switch strings.ToLower(definition.Runs.Using) {
	case "composite":
		for _, step := range definition.Runs.Steps {
			if step.Uses != "" {
				if err := runAction(ctx, work, step.Uses, step.With, baseEnv, output, depth+1); err != nil {
					return err
				}
			} else if err := runCommand(ctx, work, step.Run, env, output); err != nil {
				return err
			}
		}
	case "node16", "node20", "node24":
		if definition.Runs.Main == "" {
			return errors.New("node action has no main entrypoint")
		}
		process := exec.CommandContext(ctx, "node", definition.Runs.Main)
		process.Dir = actionDir
		process.Env = append(os.Environ(), env...)
		value, runErr := process.CombinedOutput()
		output.Write(value)
		return runErr
	default:
		return fmt.Errorf("unsupported action runtime %q; composite and node actions are supported", definition.Runs.Using)
	}
	return nil
}

func actionInputs(with map[string]string) []string {
	env := make([]string, 0, len(with))
	for key, value := range with {
		key = strings.NewReplacer("-", "_", " ", "_").Replace(strings.ToUpper(key))
		env = append(env, "INPUT_"+key+"="+value)
	}
	return env
}

func actionDirectory(ctx context.Context, work, uses string) (string, func(), error) {
	uses = strings.TrimSpace(uses)
	if strings.HasPrefix(uses, "./") {
		path := filepath.Join(work, filepath.FromSlash(strings.TrimPrefix(uses, "./")))
		if !strings.HasPrefix(filepath.Clean(path), filepath.Clean(work)+string(os.PathSeparator)) {
			return "", func() {}, errors.New("local action path is unsafe")
		}
		return path, func() {}, nil
	}
	parts := strings.SplitN(uses, "@", 2)
	if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
		return "", func() {}, errors.New("uses must use owner/repository/path@ref")
	}
	ref := parts[1]
	repoPath := parts[0]
	segments := strings.Split(repoPath, "/")
	if len(segments) < 2 || strings.ContainsAny(repoPath, `\\`) || strings.Contains(repoPath, "..") {
		return "", func() {}, errors.New("uses repository reference is invalid")
	}
	base, err := os.MkdirTemp("", "kohame-action-")
	if err != nil {
		return "", func() {}, err
	}
	cloneTarget := filepath.Join(base, "repo")
	args := []string{"clone", "--depth", "1", "--quiet", "--branch", ref, "https://github.com/" + segments[0] + "/" + segments[1] + ".git", cloneTarget}
	if output, cloneErr := exec.CommandContext(ctx, "git", args...).CombinedOutput(); cloneErr != nil {
		_ = os.RemoveAll(base)
		return "", func() {}, fmt.Errorf("clone action %s: %w: %s", uses, cloneErr, strings.TrimSpace(string(output)))
	}
	actionPath := cloneTarget
	if len(segments) > 2 {
		actionPath = filepath.Join(append([]string{cloneTarget}, segments[2:]...)...)
	}
	return actionPath, func() { _ = os.RemoveAll(base) }, nil
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
