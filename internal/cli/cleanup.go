package cli

import (
	"context"
	"log/slog"

	ghclient "github.com/smg247/prow-agent-eval/pkg/github"
	"github.com/smg247/prow-agent-eval/pkg/shared"
	"github.com/spf13/cobra"
)

var cleanupFlags sharedFlags

var cleanupCmd = &cobra.Command{
	Use:   "cleanup",
	Short: "Close PR and delete eval branch",
	Args:  cobra.NoArgs,
	RunE:  runCleanup,
}

func init() {
	cleanupFlags.addCommon(cleanupCmd)
	mustMarkRequired(cleanupCmd, "shared-dir")
}

func runCleanup(cmd *cobra.Command, args []string) error {
	ctx, stop := commandContext(cmd)
	defer stop()

	token := resolveToken(cleanupFlags.token)

	caseNames, err := shared.ReadCaseList(cleanupFlags.sharedDir)
	if err != nil {
		slog.Warn("could not read case list", "error", err)
		return nil
	}

	if cleanupFlags.caseName != "" {
		caseNames = []string{cleanupFlags.caseName}
	}

	slog.Info("cleaning up cases", "count", len(caseNames))

	for _, caseName := range caseNames {
		cleanupCase(ctx, caseName, token)
	}

	slog.Info("cleanup complete")
	return nil
}

func cleanupCase(ctx context.Context, caseName, token string) {
	meta, err := shared.ReadCaseMetadata(cleanupFlags.sharedDir, caseName)
	if err != nil {
		slog.Warn("could not read metadata", "case", caseName, "error", err)
		return
	}

	repo := meta.Repo
	if repo == "" {
		slog.Warn("no repo in metadata, skipping", "case", caseName)
		return
	}

	client, err := ghclient.NewClient(token, repo)
	if err != nil {
		slog.Warn("could not create GitHub client", "case", caseName, "error", err)
		return
	}

	if meta.PRNumber > 0 {
		if err := client.ClosePR(ctx, meta.PRNumber); err != nil {
			slog.Warn("could not close PR", "case", caseName, "pr", meta.PRNumber, "error", err)
		} else {
			slog.Info("closed PR", "case", caseName, "pr", meta.PRNumber)
		}
	}

	if meta.HeadBranch != "" {
		if err := client.DeleteBranch(ctx, meta.HeadBranch); err != nil {
			slog.Warn("could not delete branch", "case", caseName, "branch", meta.HeadBranch, "error", err)
		} else {
			slog.Info("deleted branch", "case", caseName, "branch", meta.HeadBranch)
		}
	}
}
