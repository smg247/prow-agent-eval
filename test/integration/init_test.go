//go:build integration

package integration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/smg247/prow-agent-eval/internal/cli"
	ghclient "github.com/smg247/prow-agent-eval/pkg/github"
	"github.com/smg247/prow-agent-eval/pkg/shared"
)

func TestInit_Followup(t *testing.T) {
	cli.ResetDeps()
	t.Cleanup(cli.ResetDeps)

	bare := setupBareOrigin(t, map[string]branchFiles{
		"main": {
			"README.md":         "# widget\n",
			"pkg/api/server.go": "package api\n\nfunc Serve() {}\n",
		},
		"fixture-head": {
			"README.md":         "# widget\n",
			"pkg/api/server.go": "package api\n\nfunc Serve() { /* fixture */ }\n",
		},
	})
	gh := startGitHubMock(t)
	gh.prBody = "Automated eval PR for case: case-001\nJira: TRT-9001"

	cli.SetDeps(cli.Deps{
		NewGitHubClient: func(token, repo string) (*ghclient.Client, error) {
			c, err := ghclient.NewClientWithHTTP(gh.server.Client(), gh.URL(), token, repo)
			if err != nil {
				return nil, err
			}
			c.SetCloneURL(bare.Path)
			return c, nil
		},
	})

	sharedDir := t.TempDir()
	evalYAML := filepath.Join(fixtureDir(t, "followup"), "eval.yaml")

	err := cli.ExecuteArgs([]string{
		"init",
		"--config", evalYAML,
		"--shared-dir", sharedDir,
		"--mode", "followup",
		"--case", "case-001",
		"--token", "test-token",
	})
	if err != nil {
		t.Fatalf("init: %v", err)
	}

	meta, err := shared.ReadCaseMetadata(sharedDir, "case-001")
	if err != nil {
		t.Fatalf("ReadCaseMetadata: %v", err)
	}
	if meta.Repo != "acme/widget" {
		t.Errorf("Repo = %q, want acme/widget", meta.Repo)
	}
	if meta.BaseBranch != "main" {
		t.Errorf("BaseBranch = %q, want main", meta.BaseBranch)
	}
	if meta.JiraIssueKey != "TRT-9001" {
		t.Errorf("JiraIssueKey = %q, want TRT-9001", meta.JiraIssueKey)
	}
	if meta.BotLogin != "test-bot" {
		t.Errorf("BotLogin = %q, want test-bot", meta.BotLogin)
	}
	if meta.PRNumber != 42 {
		t.Errorf("PRNumber = %d, want 42", meta.PRNumber)
	}
	if !strings.HasPrefix(meta.HeadBranch, "case-001-eval-") {
		t.Errorf("HeadBranch %q missing case-001-eval- prefix", meta.HeadBranch)
	}
	if meta.FixtureHeadSHA == "" {
		t.Error("FixtureHeadSHA is empty")
	}
	if !remoteHasBranch(t, bare.Path, meta.HeadBranch) {
		t.Errorf("remote missing pushed branch %s", meta.HeadBranch)
	}

	cases, err := shared.ReadCaseList(sharedDir)
	if err != nil {
		t.Fatalf("ReadCaseList: %v", err)
	}
	if len(cases) != 1 || cases[0] != "case-001" {
		t.Errorf("cases = %v, want [case-001]", cases)
	}

	if _, err := os.Stat(filepath.Join(sharedDir, "case-001.comment-map.json")); err != nil {
		t.Errorf("missing comment-map.json: %v", err)
	}
	if _, err := os.Stat(filepath.Join(sharedDir, "case-001.jira-issue.json")); err != nil {
		t.Errorf("missing jira-issue.json: %v", err)
	}
	if !gh.hasCall(httpMethodPost, "/pulls") {
		t.Error("expected CreatePR call")
	}
}

func TestInit_Solve(t *testing.T) {
	cli.ResetDeps()
	t.Cleanup(cli.ResetDeps)

	gh := startGitHubMock(t)
	var clientCreated bool
	cli.SetDeps(cli.Deps{
		NewGitHubClient: func(token, repo string) (*ghclient.Client, error) {
			clientCreated = true
			return ghclient.NewClientWithHTTP(gh.server.Client(), gh.URL(), token, repo)
		},
	})

	sharedDir := t.TempDir()
	evalYAML := filepath.Join(fixtureDir(t, "solve"), "eval.yaml")

	err := cli.ExecuteArgs([]string{
		"init",
		"--config", evalYAML,
		"--shared-dir", sharedDir,
		"--mode", "solve",
		"--case", "case-001",
		"--token", "test-token",
	})
	if err != nil {
		t.Fatalf("init: %v", err)
	}

	if clientCreated {
		t.Error("solve metadata-only init should not create a GitHub client")
	}

	meta, err := shared.ReadCaseMetadata(sharedDir, "case-001")
	if err != nil {
		t.Fatalf("ReadCaseMetadata: %v", err)
	}
	if meta.CaseName != "case-001" {
		t.Errorf("CaseName = %q, want case-001", meta.CaseName)
	}
	if meta.BaseBranch != "main" {
		t.Errorf("BaseBranch = %q, want main", meta.BaseBranch)
	}
	if meta.JiraIssueKey != "TRT-9002" {
		t.Errorf("JiraIssueKey = %q, want TRT-9002", meta.JiraIssueKey)
	}
	if meta.Repo != "acme/widget" {
		t.Errorf("Repo = %q, want acme/widget", meta.Repo)
	}
	if meta.HeadBranch != "" || meta.PRNumber != 0 {
		t.Errorf("solve init should leave HeadBranch/PRNumber empty; got head=%q pr=%d", meta.HeadBranch, meta.PRNumber)
	}

	gotCase, err := shared.ReadFile(sharedDir, "case-001.eval-case")
	if err != nil {
		t.Fatalf("eval-case: %v", err)
	}
	if gotCase != "case-001" {
		t.Errorf("eval-case = %q, want case-001", gotCase)
	}
	gotExpected, err := shared.ReadFile(sharedDir, "case-001.eval-expected-branch")
	if err != nil {
		t.Fatalf("eval-expected-branch: %v", err)
	}
	if gotExpected != "golden-fix" {
		t.Errorf("eval-expected-branch = %q, want golden-fix", gotExpected)
	}
}

const httpMethodPost = "POST"
