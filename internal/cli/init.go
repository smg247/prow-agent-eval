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
	if err := validateCaseInput(cfg, c); err != nil {
		return err
	}

	repo, err := resolveRepo(cfg.Init.Repo, c.Input.Repo)
	if err != nil {
		return err
	}

	slog.Info("initializing case", "case", caseName, "repo", repo, "mode", initMode)

	meta := &shared.CaseMetadata{
		CaseName:     caseName,
		BaseBranch:   c.Input.BaseBranch,
		JiraIssueKey: c.Input.JiraKey,
		Repo:         repo,
	}

	if err := writeCaseFiles(initFlags.sharedDir, c); err != nil {
		return fmt.Errorf("writing case files: %w", err)
	}

	if initMode == "solve" && c.Input.HeadBranch == "" {
		if err := shared.WriteCaseMetadata(initFlags.sharedDir, meta); err != nil {
			return fmt.Errorf("writing metadata: %w", err)
		}
		slog.Info("case done (metadata only)", "case", caseName)
		return nil
	}
	if c.Input.HeadBranch == "" {
		return fmt.Errorf("case input missing head_branch (required for followup mode)")
	}

	if err := setupEvalBranch(ctx, cfg, c, meta, token); err != nil {
		return err
	}

	if err := shared.WriteCaseMetadata(initFlags.sharedDir, meta); err != nil {
		return fmt.Errorf("writing metadata: %w", err)
	}

	if initMode == "followup" {
		if err := seedCaseComments(ctx, c, meta, token); err != nil {
			return err
		}
	}

	if meta.PRNumber > 0 {
		slog.Info("case done", "case", caseName, "pr", meta.PRNumber, "branch", meta.HeadBranch, "sha", git.ShortSHA(meta.FixtureHeadSHA, 8))
	} else {
		slog.Info("case done", "case", caseName, "branch", meta.HeadBranch, "sha", git.ShortSHA(meta.FixtureHeadSHA, 8))
	}
	return nil
}

func validateCaseInput(cfg *config.EvalConfig, c *config.Case) error {
	if c.Input.BaseBranch == "" {
		return fmt.Errorf("case input missing base_branch")
	}
	if cfg.Collect.ExpectedBranchDiff && c.Input.ExpectedBranch == "" {
		return fmt.Errorf("expected_branch_diff is enabled but case input missing expected_branch")
	}
	return nil
}

func setupEvalBranch(ctx context.Context, cfg *config.EvalConfig, c *config.Case, meta *shared.CaseMetadata, token string) error {
	client, err := deps.NewGitHubClient(token, meta.Repo)
	if err != nil {
		return err
	}

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

	branchPrefix := strings.ReplaceAll(c.Name, "/", "-")
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

	botLogin := readBotLoginFromSharedDir(initFlags.sharedDir)
	if botLogin == "" {
		botLogin, err = client.GetBotLogin(ctx)
		if err != nil {
			if cfg.Collect.BotReplies {
				return fmt.Errorf("getting bot login (required for bot_replies): %w", err)
			}
			slog.Warn("could not get bot login", "case", c.Name, "error", err)
		}
	}

	meta.HeadBranch = evalBranch
	meta.FixtureHeadSHA = fixtureHeadSHA
	meta.BotLogin = botLogin

	if initMode != "followup" {
		return nil
	}

	prTitle := fmt.Sprintf("[eval] %s", c.Name)
	prBody := fmt.Sprintf("Automated eval PR for case: %s\nJira: %s", c.Name, c.Input.JiraKey)
	prNumber, err := client.CreatePR(ctx, evalBranch, c.Input.BaseBranch, prTitle, prBody)
	if err != nil {
		return fmt.Errorf("creating PR: %w", err)
	}
	meta.PRNumber = prNumber
	slog.Info("created PR", "case", c.Name, "pr", prNumber, "head", evalBranch, "base", c.Input.BaseBranch)
	return nil
}

func seedCaseComments(ctx context.Context, c *config.Case, meta *shared.CaseMetadata, token string) error {
	commentsPath := filepath.Join(c.Dir, "comments.json")
	st, err := os.Stat(commentsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("stat comments.json: %w", err)
	}
	if st.IsDir() {
		return nil
	}

	client, err := deps.NewGitHubClient(token, meta.Repo)
	if err != nil {
		return err
	}

	comments, err := ghclient.LoadSeededComments(commentsPath)
	if err != nil {
		return fmt.Errorf("loading seeded comments: %w", err)
	}

	posted, err := client.SeedComments(ctx, meta.PRNumber, comments)
	if err != nil {
		return fmt.Errorf("seeding comments: %w", err)
	}

	mapData, err := json.Marshal(posted)
	if err != nil {
		return fmt.Errorf("marshaling comment map: %w", err)
	}
	if err := shared.WriteFile(initFlags.sharedDir, c.Name+".comment-map.json", string(mapData)); err != nil {
		return fmt.Errorf("writing comment map: %w", err)
	}

	slog.Info("seeded comments", "case", c.Name, "count", len(comments))
	return nil
}

// writeCaseFiles copies per-case supporting files (jira-issue.json, expected branch, case name)
// to SHARED_DIR so downstream steps can find them.
func writeCaseFiles(sharedDir string, c *config.Case) error {
	prefix := c.Name + "."

	if err := shared.WriteFile(sharedDir, prefix+"eval-case", c.Name); err != nil {
		return err
	}

	if c.Input.ExpectedBranch != "" {
		if err := shared.WriteFile(sharedDir, prefix+"eval-expected-branch", c.Input.ExpectedBranch); err != nil {
			return err
		}
	}

	jiraPath := filepath.Join(c.Dir, "jira-issue.json")
	if data, err := os.ReadFile(jiraPath); err == nil {
		if err := shared.WriteFile(sharedDir, prefix+"jira-issue.json", string(data)); err != nil {
			return err
		}
	}

	return nil
}

func readBotLoginFromSharedDir(sharedDir string) string {
	data, err := os.ReadFile(filepath.Join(sharedDir, "gh-app-bot-login"))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}
