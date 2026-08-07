package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/smg247/prow-agent-eval/pkg/config"
	"github.com/smg247/prow-agent-eval/pkg/git"
	ghclient "github.com/smg247/prow-agent-eval/pkg/github"
	"github.com/smg247/prow-agent-eval/pkg/shared"
	"github.com/spf13/cobra"
)

var (
	initFlags sharedFlags
	initMode  string
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize eval environment: create branches, PRs, and seed comments",
	Args:  cobra.NoArgs,
	RunE:  runInit,
}

func init() {
	initFlags.addCommon(initCmd)
	initCmd.Flags().StringVar(&initMode, "mode", "", "Eval mode: 'solve' (branch only) or 'followup' (branch + PR + seed comments)")
	mustMarkRequired(initCmd, "config", "shared-dir", "mode")
}

func runInit(cmd *cobra.Command, args []string) error {
	if initMode != "solve" && initMode != "followup" {
		return fmt.Errorf("--mode must be 'solve' or 'followup', got %q", initMode)
	}

	ctx, stop := commandContext(cmd)
	defer stop()

	token := resolveToken(initFlags.token)

	cfg, err := config.Load(initFlags.configPath)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	configDir := filepath.Dir(initFlags.configPath)

	var caseNames []string
	if initFlags.caseName != "" {
		caseNames = []string{initFlags.caseName}
		if err := shared.EnsureCaseInList(initFlags.sharedDir, initFlags.caseName); err != nil {
			return fmt.Errorf("updating case list: %w", err)
		}
	} else {
		caseNames, err = config.ListCases(configDir, cfg.Dataset.Path)
		if err != nil {
			return fmt.Errorf("listing cases: %w", err)
		}
		if err := shared.WriteCaseList(initFlags.sharedDir, caseNames); err != nil {
			return fmt.Errorf("writing case list: %w", err)
		}
	}

	if len(caseNames) == 0 {
		return fmt.Errorf("no cases found")
	}

	slog.Info("initializing cases", "count", len(caseNames), "mode", initMode)

	for _, caseName := range caseNames {
		if err := initCase(ctx, cfg, configDir, caseName, token); err != nil {
			return fmt.Errorf("case %s: %w", caseName, err)
		}
	}

	slog.Info("init complete", "count", len(caseNames))
	return nil
}

func initCase(ctx context.Context, cfg *config.EvalConfig, configDir, caseName, token string) error {
	c, err := config.LoadCase(configDir, cfg.Dataset.Path, caseName)
	if err != nil {
		return fmt.Errorf("loading case: %w", err)
	}

	if c.Input.HeadBranch == "" {
		return fmt.Errorf("case input missing head_branch")
	}
	if c.Input.BaseBranch == "" {
		return fmt.Errorf("case input missing base_branch")
	}
	if cfg.Collect.ExpectedBranchDiff && c.Input.ExpectedBranch == "" {
		return fmt.Errorf("expected_branch_diff is enabled but case input missing expected_branch")
	}

	repo, err := resolveRepo(cfg.Init.Repo, c.Input.Repo)
	if err != nil {
		return err
	}

	client, err := ghclient.NewClient(token, repo)
	if err != nil {
		return err
	}

	slog.Info("initializing case", "case", caseName, "repo", repo)

	cloneDir, err := os.MkdirTemp("", "prow-agent-eval-*")
	if err != nil {
		return fmt.Errorf("creating temp dir: %w", err)
	}
	defer os.RemoveAll(cloneDir)

	gitRepo, err := git.Clone(ctx, client.CloneURL(), cloneDir, token)
	if err != nil {
		return fmt.Errorf("cloning: %w", err)
	}

	if err := gitRepo.Fetch(ctx, "origin", c.Input.HeadBranch); err != nil {
		return fmt.Errorf("fetching head branch %s: %w", c.Input.HeadBranch, err)
	}

	branchPrefix := strings.ReplaceAll(caseName, "/", "-")
	evalBranch := git.EvalBranchName(branchPrefix)

	if err := gitRepo.CreateBranch(ctx, evalBranch, "origin/"+c.Input.HeadBranch); err != nil {
		return fmt.Errorf("creating eval branch: %w", err)
	}

	fixtureHeadSHA, err := gitRepo.RevParse(ctx, "HEAD")
	if err != nil {
		return fmt.Errorf("getting fixture head SHA: %w", err)
	}

	if err := gitRepo.Push(ctx, "origin", evalBranch); err != nil {
		return fmt.Errorf("pushing eval branch: %w", err)
	}

	botLogin, err := client.GetBotLogin(ctx)
	if err != nil {
		if cfg.Collect.BotReplies {
			return fmt.Errorf("getting bot login (required for bot_replies): %w", err)
		}
		slog.Warn("could not get bot login", "case", caseName, "error", err)
	}

	meta := &shared.CaseMetadata{
		CaseName:       caseName,
		HeadBranch:     evalBranch,
		BaseBranch:     c.Input.BaseBranch,
		FixtureHeadSHA: fixtureHeadSHA,
		JiraIssueKey:   c.Input.JiraKey,
		Repo:           repo,
		BotLogin:       botLogin,
	}

	if initMode == "followup" {
		prTitle := fmt.Sprintf("[eval] %s", caseName)
		prBody := fmt.Sprintf("Automated eval PR for case: %s\nJira: %s", caseName, c.Input.JiraKey)
		prNumber, err := client.CreatePR(ctx, evalBranch, c.Input.BaseBranch, prTitle, prBody)
		if err != nil {
			return fmt.Errorf("creating PR: %w", err)
		}
		meta.PRNumber = prNumber
		slog.Info("created PR", "case", caseName, "pr", prNumber, "head", evalBranch, "base", c.Input.BaseBranch)
	}

	// Persist metadata before seeding so cleanup can reclaim orphans on later failure.
	if err := shared.WriteCaseMetadata(initFlags.sharedDir, meta); err != nil {
		return fmt.Errorf("writing metadata: %w", err)
	}

	if initMode == "followup" {
		commentsPath := filepath.Join(c.Dir, "comments.json")
		st, err := os.Stat(commentsPath)
		if err != nil {
			if !os.IsNotExist(err) {
				return fmt.Errorf("stat comments.json: %w", err)
			}
		} else if !st.IsDir() {
			comments, err := ghclient.LoadSeededComments(commentsPath)
			if err != nil {
				return fmt.Errorf("loading seeded comments: %w", err)
			}

			commentMap, err := client.SeedComments(ctx, meta.PRNumber, comments)
			if err != nil {
				return fmt.Errorf("seeding comments: %w", err)
			}

			mapData, err := json.Marshal(commentMap)
			if err != nil {
				return fmt.Errorf("marshaling comment map: %w", err)
			}
			if err := shared.WriteFile(initFlags.sharedDir, caseName+".comment-map.json", string(mapData)); err != nil {
				return fmt.Errorf("writing comment map: %w", err)
			}

			slog.Info("seeded comments", "case", caseName, "count", len(comments))
		}
	}

	if meta.PRNumber > 0 {
		slog.Info("case done", "case", caseName, "pr", meta.PRNumber, "branch", evalBranch, "sha", git.ShortSHA(fixtureHeadSHA, 8))
	} else {
		slog.Info("case done", "case", caseName, "branch", evalBranch, "sha", git.ShortSHA(fixtureHeadSHA, 8))
	}
	return nil
}
