//go:build integration

package integration

import (
	"testing"

	"github.com/smg247/prow-agent-eval/internal/cli"
	ghclient "github.com/smg247/prow-agent-eval/pkg/github"
	"github.com/smg247/prow-agent-eval/pkg/shared"
)

func TestCleanup_ClosesPRAndDeletesBranch(t *testing.T) {
	cli.ResetDeps()
	t.Cleanup(cli.ResetDeps)

	gh := startGitHubMock(t)
	gh.prNumber = 99

	cli.SetDeps(cli.Deps{
		NewGitHubClient: func(token, repo string) (*ghclient.Client, error) {
			return ghclient.NewClientWithHTTP(gh.server.Client(), gh.URL(), token, repo)
		},
	})

	sharedDir := t.TempDir()
	if err := shared.WriteCaseList(sharedDir, []string{"case-001"}); err != nil {
		t.Fatal(err)
	}
	meta := &shared.CaseMetadata{
		CaseName:   "case-001",
		PRNumber:   99,
		HeadBranch: "case-001-eval-20260101-120000",
		BaseBranch: "main",
		Repo:       "acme/widget",
	}
	if err := shared.WriteCaseMetadata(sharedDir, meta); err != nil {
		t.Fatal(err)
	}

	err := cli.ExecuteArgs([]string{
		"cleanup",
		"--shared-dir", sharedDir,
		"--case", "case-001",
		"--token", "test-token",
	})
	if err != nil {
		t.Fatalf("cleanup: %v", err)
	}

	if !gh.hasCall("PATCH", "/pulls/99") {
		t.Errorf("expected ClosePR PATCH; calls=%v", gh.Calls())
	}
	if !gh.hasCall("DELETE", "/git/refs/heads/case-001-eval-20260101-120000") {
		t.Errorf("expected DeleteBranch; calls=%v", gh.Calls())
	}
}
