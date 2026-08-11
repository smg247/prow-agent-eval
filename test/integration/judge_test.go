//go:build integration

package integration

import (
	"encoding/json"
	"encoding/xml"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/smg247/prow-agent-eval/internal/cli"
	ghclient "github.com/smg247/prow-agent-eval/pkg/github"
	"github.com/smg247/prow-agent-eval/pkg/report"
	"github.com/smg247/prow-agent-eval/pkg/shared"
	"gopkg.in/yaml.v3"
)

func TestJudge_Followup(t *testing.T) {
	cli.ResetDeps()
	t.Cleanup(cli.ResetDeps)

	bare := setupBareOrigin(t, map[string]branchFiles{
		"main": {
			"README.md":         "# widget\n",
			"pkg/api/server.go": "package api\n\nfunc Serve() {}\n",
		},
		"agent-work": {
			"README.md":         "# widget\n",
			"pkg/api/server.go": "package api\n\nfunc Serve() { /* retries */ }\n",
		},
	})

	gh := startGitHubMock(t)
	gh.prNumber = 42
	gh.prBody = "Fixes review feedback on retries"
	gh.prHead = "agent-work"
	gh.prState = "open"

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
	artifactDir := t.TempDir()
	seedFollowupShared(t, sharedDir, bare.SHAs["main"])

	evalYAML := filepath.Join(fixtureDir(t, "followup"), "eval.yaml")
	err := cli.ExecuteArgs([]string{
		"judge",
		"--config", evalYAML,
		"--shared-dir", sharedDir,
		"--artifact-dir", artifactDir,
		"--case", "case-001",
		"--token", "test-token",
	})
	if err != nil {
		t.Fatalf("judge: %v", err)
	}

	assertArtifactsExist(t, artifactDir, "followup-eval", "case-001")

	data, err := os.ReadFile(filepath.Join(artifactDir, "eval-case-001.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var caseReport report.CaseReport
	if err := yaml.Unmarshal(data, &caseReport); err != nil {
		t.Fatal(err)
	}
	if got := caseReport.Checks["pr_exists"]; got != "pass" {
		t.Errorf("pr_exists = %q, want pass", got)
	}
	if got := caseReport.Checks["branch_created"]; got != "pass" {
		t.Errorf("branch_created = %q, want pass", got)
	}
	if got := caseReport.Checks["pr_description_exists"]; got != "pass" {
		t.Errorf("pr_description_exists = %q, want pass", got)
	}
	if !strings.Contains(caseReport.PRURL, "/pull/42") {
		t.Errorf("PRURL %q missing /pull/42", caseReport.PRURL)
	}

	junitPath := filepath.Join(artifactDir, "junit_followup-eval.xml")
	junitData, err := os.ReadFile(junitPath)
	if err != nil {
		t.Fatal(err)
	}
	var suites report.JUnitTestSuites
	if err := xml.Unmarshal(junitData, &suites); err != nil {
		t.Fatalf("parse junit: %v", err)
	}
	if len(suites.Suites) != 1 {
		t.Fatalf("suites len = %d, want 1", len(suites.Suites))
	}
	if suites.Suites[0].Failures != 0 || suites.Suites[0].Errors != 0 {
		t.Errorf("unexpected junit failures=%d errors=%d", suites.Suites[0].Failures, suites.Suites[0].Errors)
	}
}

func TestJudge_Solve(t *testing.T) {
	cli.ResetDeps()
	t.Cleanup(cli.ResetDeps)

	reconcileFix := "package controller\n\nfunc Reconcile() { /* fixed */ }\n"
	bare := setupBareOrigin(t, map[string]branchFiles{
		"main": {
			"README.md":                   "# widget\n",
			"pkg/controller/reconcile.go": "package controller\n\nfunc Reconcile() {}\n",
		},
		"golden-fix": {
			"README.md":                   "# widget\n",
			"pkg/controller/reconcile.go": reconcileFix,
		},
		"claude/fix-trt-9002": {
			"README.md":                   "# widget\n",
			"pkg/controller/reconcile.go": reconcileFix,
		},
	})

	gh := startGitHubMock(t)
	gh.prNumber = 7
	gh.prBody = "Solve TRT-9002"
	gh.prHead = "claude/fix-trt-9002"

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
	artifactDir := t.TempDir()
	seedSolveShared(t, sharedDir, bare.SHAs["main"])

	evalYAML := filepath.Join(fixtureDir(t, "solve"), "eval.yaml")
	err := cli.ExecuteArgs([]string{
		"judge",
		"--config", evalYAML,
		"--shared-dir", sharedDir,
		"--artifact-dir", artifactDir,
		"--case", "case-001",
		"--token", "test-token",
	})
	if err != nil {
		t.Fatalf("judge: %v", err)
	}

	assertArtifactsExist(t, artifactDir, "solve-eval", "case-001")

	data, err := os.ReadFile(filepath.Join(artifactDir, "eval-case-001.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var caseReport report.CaseReport
	if err := yaml.Unmarshal(data, &caseReport); err != nil {
		t.Fatal(err)
	}
	if got := caseReport.Checks["file_overlap"]; got != "pass" {
		t.Errorf("file_overlap = %q, want pass", got)
	}
	if got := caseReport.Checks["expected_files_changed"]; got != "pass" {
		t.Errorf("expected_files_changed = %q, want pass", got)
	}
	if got := caseReport.Checks["branch_created"]; got != "pass" {
		t.Errorf("branch_created = %q, want pass", got)
	}
}

func seedFollowupShared(t *testing.T, sharedDir, fixtureSHA string) {
	t.Helper()
	if err := shared.WriteCaseList(sharedDir, []string{"case-001"}); err != nil {
		t.Fatal(err)
	}
	meta := &shared.CaseMetadata{
		CaseName:       "case-001",
		PRNumber:       42,
		HeadBranch:     "agent-work",
		BaseBranch:     "main",
		FixtureHeadSHA: fixtureSHA,
		JiraIssueKey:   "TRT-9001",
		Repo:           "acme/widget",
		BotLogin:       "test-bot",
	}
	if err := shared.WriteCaseMetadata(sharedDir, meta); err != nil {
		t.Fatal(err)
	}
	posted := map[string]ghclient.PostedComment{
		"comment-001": {GitHubID: 1001, Category: "quality", CreatedAt: "2026-01-01T00:00:00Z"},
	}
	data, err := json.Marshal(posted)
	if err != nil {
		t.Fatal(err)
	}
	if err := shared.WriteFile(sharedDir, "case-001.comment-map.json", string(data)); err != nil {
		t.Fatal(err)
	}
	if err := shared.WriteFile(sharedDir, "case-001.eval-case", "case-001"); err != nil {
		t.Fatal(err)
	}
}

func seedSolveShared(t *testing.T, sharedDir, fixtureSHA string) {
	t.Helper()
	if err := shared.WriteCaseList(sharedDir, []string{"case-001"}); err != nil {
		t.Fatal(err)
	}
	meta := &shared.CaseMetadata{
		CaseName:       "case-001",
		PRNumber:       7,
		HeadBranch:     "claude/fix-trt-9002",
		BaseBranch:     "main",
		FixtureHeadSHA: fixtureSHA,
		JiraIssueKey:   "TRT-9002",
		Repo:           "acme/widget",
	}
	if err := shared.WriteCaseMetadata(sharedDir, meta); err != nil {
		t.Fatal(err)
	}
	if err := shared.WriteFile(sharedDir, "case-001.eval-case", "case-001"); err != nil {
		t.Fatal(err)
	}
	if err := shared.WriteFile(sharedDir, "case-001.eval-expected-branch", "golden-fix"); err != nil {
		t.Fatal(err)
	}
}

func assertArtifactsExist(t *testing.T, artifactDir, evalName, caseName string) {
	t.Helper()
	for _, name := range []string{
		"junit_" + evalName + ".xml",
		"eval-" + caseName + ".yaml",
		"eval-summary.yaml",
		"eval-summary.html",
	} {
		if _, err := os.Stat(filepath.Join(artifactDir, name)); err != nil {
			t.Errorf("missing artifact %s: %v", name, err)
		}
	}
}
