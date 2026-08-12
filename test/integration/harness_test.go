//go:build integration

package integration

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

type ghCall struct {
	Method string
	Path   string
}

type githubMock struct {
	mu    sync.Mutex
	calls []ghCall
	server *httptest.Server

	botLogin string
	prNumber int
	prBody   string
	prHead   string
	prState  string

	nextCommentID int64
	issueComments []map[string]any
}

func startGitHubMock(t *testing.T) *githubMock {
	t.Helper()
	m := &githubMock{
		botLogin:      "test-bot",
		prNumber:      42,
		prBody:        "Automated eval PR for case: case-001\nJira: TRT-9001",
		prHead:        "case-001-eval-branch",
		prState:       "open",
		nextCommentID: 1000,
	}
	m.server = httptest.NewServer(http.HandlerFunc(m.serve))
	t.Cleanup(m.server.Close)
	return m
}

func (m *githubMock) URL() string { return m.server.URL }

func (m *githubMock) Calls() []ghCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]ghCall, len(m.calls))
	copy(out, m.calls)
	return out
}

func (m *githubMock) hasCall(method, pathSubstr string) bool {
	for _, c := range m.Calls() {
		if c.Method == method && strings.Contains(c.Path, pathSubstr) {
			return true
		}
	}
	return false
}

func (m *githubMock) serve(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	m.calls = append(m.calls, ghCall{Method: r.Method, Path: r.URL.Path})
	m.mu.Unlock()

	path := r.URL.Path
	switch {
	case r.Method == http.MethodGet && path == "/user":
		writeJSON(w, map[string]any{"login": m.botLogin})

	case r.Method == http.MethodPost && strings.HasSuffix(path, "/pulls"):
		writeJSON(w, map[string]any{
			"number": m.prNumber,
			"html_url": fmt.Sprintf("https://github.com/acme/widget/pull/%d", m.prNumber),
			"state": m.prState,
			"body": m.prBody,
			"head": map[string]any{"ref": m.prHead},
		})

	case r.Method == http.MethodGet && strings.Contains(path, "/pulls/") && !strings.Contains(path, "/comments"):
		writeJSON(w, map[string]any{
			"number": m.prNumber,
			"state":  m.prState,
			"body":   m.prBody,
			"head":   map[string]any{"ref": m.prHead},
		})

	case r.Method == http.MethodGet && strings.HasSuffix(path, "/pulls"):
		// FindPRByHead
		writeJSON(w, []map[string]any{{
			"number": m.prNumber,
			"state":  m.prState,
			"body":   m.prBody,
			"head":   map[string]any{"ref": m.prHead},
		}})

	case r.Method == http.MethodPost && strings.Contains(path, "/issues/") && strings.HasSuffix(path, "/comments"):
		m.mu.Lock()
		m.nextCommentID++
		id := m.nextCommentID
		m.mu.Unlock()
		writeJSON(w, map[string]any{
			"id":         id,
			"body":       "seeded",
			"created_at": "2026-01-01T00:00:00Z",
			"user":       map[string]any{"login": m.botLogin},
		})

	case r.Method == http.MethodPost && strings.Contains(path, "/pulls/") && strings.HasSuffix(path, "/comments"):
		m.mu.Lock()
		m.nextCommentID++
		id := m.nextCommentID
		m.mu.Unlock()
		writeJSON(w, map[string]any{
			"id":         id,
			"body":       "seeded inline",
			"path":       "pkg/api/server.go",
			"created_at": "2026-01-01T00:00:00Z",
			"user":       map[string]any{"login": m.botLogin},
		})

	case r.Method == http.MethodGet && strings.Contains(path, "/issues/") && strings.HasSuffix(path, "/comments"):
		if m.issueComments != nil {
			writeJSON(w, m.issueComments)
		} else {
			writeJSON(w, []map[string]any{{
				"id":         2001,
				"body":       "bot reply looks fine",
				"created_at": "2026-01-01T00:00:00Z",
				"user":       map[string]any{"login": m.botLogin},
			}})
		}

	case r.Method == http.MethodGet && strings.Contains(path, "/pulls/") && strings.HasSuffix(path, "/comments"):
		writeJSON(w, []any{})

	case r.Method == http.MethodPatch && strings.Contains(path, "/pulls/"):
		writeJSON(w, map[string]any{"number": m.prNumber, "state": "closed"})

	case r.Method == http.MethodDelete && strings.Contains(path, "/git/refs/"):
		w.WriteHeader(http.StatusNoContent)

	default:
		http.Error(w, "unexpected "+r.Method+" "+path, http.StatusNotFound)
	}
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

// bareRepo holds a local bare git origin with named branches.
type bareRepo struct {
	Path string
	SHAs map[string]string // branch -> tip SHA
}

type branchFiles map[string]string // path -> content

// setupBareOrigin creates a bare repo with the given branches.
// "main" should be included. Each branch is created from main with the listed files added/updated.
func setupBareOrigin(t *testing.T, branches map[string]branchFiles) *bareRepo {
	t.Helper()
	root := t.TempDir()
	work := filepath.Join(root, "work")
	bare := filepath.Join(root, "origin.git")
	mustRun(t, "", "git", "init", "-b", "main", work)
	mustRun(t, work, "git", "config", "user.email", "test@example.com")
	mustRun(t, work, "git", "config", "user.name", "Test")

	shas := map[string]string{}

	// Seed main first.
	mainFiles := branches["main"]
	if mainFiles == nil {
		mainFiles = branchFiles{"README.md": "# widget\n"}
	}
	writeFiles(t, work, mainFiles)
	mustRun(t, work, "git", "add", ".")
	mustRun(t, work, "git", "commit", "-m", "initial main")
	shas["main"] = strings.TrimSpace(mustOutput(t, work, "git", "rev-parse", "HEAD"))

	for name, files := range branches {
		if name == "main" {
			continue
		}
		mustRun(t, work, "git", "checkout", "-b", name, "main")
		writeFiles(t, work, files)
		mustRun(t, work, "git", "add", ".")
		mustRun(t, work, "git", "commit", "-m", "commit on "+name)
		shas[name] = strings.TrimSpace(mustOutput(t, work, "git", "rev-parse", "HEAD"))
		mustRun(t, work, "git", "checkout", "main")
	}

	mustRun(t, "", "git", "clone", "--bare", work, bare)
	// Allow pushes to bare.
	mustRun(t, bare, "git", "config", "receive.denyCurrentBranch", "ignore")
	return &bareRepo{Path: bare, SHAs: shas}
}

func writeFiles(t *testing.T, root string, files branchFiles) {
	t.Helper()
	for path, content := range files {
		full := filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func mustRun(t *testing.T, dir string, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %v: %v\n%s", name, args, err, out)
	}
}

func mustOutput(t *testing.T, dir string, name string, args ...string) string {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("%s %v: %v", name, args, err)
	}
	return string(out)
}

func remoteHasBranch(t *testing.T, barePath, branch string) bool {
	t.Helper()
	cmd := exec.Command("git", "--git-dir", barePath, "rev-parse", "--verify", branch)
	return cmd.Run() == nil
}

func fixtureDir(t *testing.T, name string) string {
	t.Helper()
	// test/integration/testdata/<name>
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Join(wd, "testdata", name)
}
