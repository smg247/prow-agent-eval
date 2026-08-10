package judge

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/smg247/prow-agent-eval/pkg/config"
	"github.com/smg247/prow-agent-eval/pkg/git"
	ghclient "github.com/smg247/prow-agent-eval/pkg/github"
	"github.com/smg247/prow-agent-eval/pkg/shared"
)

type CollectOptions struct {
	Config    *config.EvalConfig
	Case      *config.Case
	Meta      *shared.CaseMetadata
	Client    *ghclient.Client
	Token     string
	CloneDir  string
	SharedDir string
}

func validateCollectOptions(opts CollectOptions) error {
	if opts.Config == nil {
		return fmt.Errorf("collect: config is required")
	}
	if opts.Case == nil {
		return fmt.Errorf("collect: case is required")
	}
	if opts.Meta == nil {
		return fmt.Errorf("collect: metadata is required")
	}
	cfg := opts.Config.Collect
	if cfg.BotReplies && opts.Meta.BotLogin == "" {
		return fmt.Errorf("bot_replies collect requires bot login in metadata")
	}
	if cfg.ExpectedBranchDiff && opts.Case.Input.ExpectedBranch == "" {
		return fmt.Errorf("expected_branch_diff requires case input expected_branch")
	}
	if cfg.CommentMap && opts.SharedDir == "" {
		return fmt.Errorf("comment_map collect requires shared dir")
	}
	return nil
}

func Collect(ctx context.Context, opts CollectOptions) (Outputs, error) {
	if err := validateCollectOptions(opts); err != nil {
		return nil, err
	}

	outputs := make(Outputs)
	if opts.Case.Annotations != nil {
		outputs["annotations"] = opts.Case.Annotations
	}

	repo, err := git.Clone(ctx, opts.Client.CloneURL(), opts.CloneDir, opts.Token)
	if err != nil {
		return nil, fmt.Errorf("cloning repo: %w", err)
	}

	headBranch, prNumber, err := resolveHead(ctx, opts)
	if err != nil {
		return nil, err
	}

	if err := repo.Fetch(ctx, "origin", headBranch); err != nil {
		return nil, fmt.Errorf("fetching head branch: %w", err)
	}
	if err := repo.Checkout(ctx, "origin/"+headBranch); err != nil {
		return nil, fmt.Errorf("checking out head branch: %w", err)
	}

	fixtureSHA, err := resolveFixtureSHA(ctx, repo, opts.Meta)
	if err != nil {
		return nil, err
	}

	ghData, err := collectGitHubData(ctx, opts, repo, headBranch, fixtureSHA, prNumber)
	if err != nil {
		return nil, err
	}
	outputs["github"] = ghData

	cfg := opts.Config.Collect
	if cfg.BuildResult {
		outputs["build_result"] = runMake(ctx, opts.CloneDir, "build")
	}
	if cfg.TestResult {
		outputs["test_result"] = runMake(ctx, opts.CloneDir, "test")
	}

	return outputs, nil
}

func resolveHead(ctx context.Context, opts CollectOptions) (headBranch string, prNumber int, err error) {
	headBranch = opts.Meta.HeadBranch
	prNumber = opts.Meta.PRNumber

	if headBranch == "" && prNumber > 0 {
		pr, err := opts.Client.GetPR(ctx, prNumber)
		if err != nil {
			return "", 0, fmt.Errorf("fetching PR #%d to discover head branch: %w", prNumber, err)
		}
		headBranch = pr.GetHead().GetRef()
		slog.Info("discovered head branch from PR", "pr", prNumber, "branch", headBranch)
	}
	if headBranch == "" {
		return "", 0, fmt.Errorf("no head branch available: set eval-head-branch, claude-branch, or pr-number in metadata")
	}
	return headBranch, prNumber, nil
}

func resolveFixtureSHA(ctx context.Context, repo *git.Repo, meta *shared.CaseMetadata) (string, error) {
	if meta.FixtureHeadSHA != "" {
		return meta.FixtureHeadSHA, nil
	}
	if err := repo.Fetch(ctx, "origin", meta.BaseBranch); err != nil {
		return "", fmt.Errorf("fetching base branch for fixture SHA: %w", err)
	}
	fixtureSHA, err := repo.RevParse(ctx, "origin/"+meta.BaseBranch)
	if err != nil {
		return "", fmt.Errorf("resolving base branch SHA: %w", err)
	}
	slog.Info("resolved fixture SHA from base branch", "base", meta.BaseBranch, "sha", fixtureSHA[:8])
	return fixtureSHA, nil
}

func collectGitHubData(ctx context.Context, opts CollectOptions, repo *git.Repo, headBranch, fixtureSHA string, prNumber int) (map[string]any, error) {
	diff, err := repo.DiffAgainst(ctx, fixtureSHA)
	if err != nil {
		return nil, fmt.Errorf("diff against fixture SHA: %w", err)
	}

	ghData := map[string]any{
		"changed_files":    diff.ChangedFiles,
		"full_diff":        diff.FullDiff,
		"agent_branch":     headBranch,
		"agent_diff_lines": countDiffLines(diff.FullDiff),
		"repo":             opts.Client.Owner() + "/" + opts.Client.Repo(),
		"base_branch":      opts.Meta.BaseBranch,
	}

	prNumber = resolvePRNumber(ctx, opts.Client, headBranch, prNumber)
	ghData["pr_number"] = prNumber

	if err := collectPRDetails(ctx, opts, ghData, prNumber); err != nil {
		return nil, err
	}
	if err := collectOptionalGitHubData(ctx, opts, repo, ghData, fixtureSHA, prNumber); err != nil {
		return nil, err
	}
	return ghData, nil
}

func resolvePRNumber(ctx context.Context, client *ghclient.Client, headBranch string, prNumber int) int {
	if prNumber != 0 {
		return prNumber
	}
	pr, err := client.FindPRByHead(ctx, headBranch)
	if err != nil {
		if !errors.Is(err, ghclient.ErrNotFound) {
			slog.Warn("could not search for agent-created PR", "error", err)
		}
		return 0
	}
	slog.Info("discovered agent-created PR", "pr", pr.GetNumber())
	return pr.GetNumber()
}

func collectPRDetails(ctx context.Context, opts CollectOptions, ghData map[string]any, prNumber int) error {
	if prNumber > 0 {
		pr, err := opts.Client.GetPR(ctx, prNumber)
		if err != nil {
			return fmt.Errorf("fetching PR #%d: %w", prNumber, err)
		}
		ghData["pr_state"] = pr.GetState()
		ghData["pr_body"] = pr.GetBody()
	} else {
		ghData["pr_state"] = "none"
	}

	descPath := filepath.Join(opts.SharedDir, opts.Meta.CaseName+".pr-description.md")
	if data, err := os.ReadFile(descPath); err == nil && len(strings.TrimSpace(string(data))) > 0 {
		ghData["pr_description_file"] = true
	}
	return nil
}

func collectOptionalGitHubData(ctx context.Context, opts CollectOptions, repo *git.Repo, ghData map[string]any, fixtureSHA string, prNumber int) error {
	cfg := opts.Config.Collect

	if cfg.BotReplies {
		replies, err := collectBotReplies(ctx, opts.Client, prNumber, opts.Meta.BotLogin)
		if err != nil {
			return err
		}
		ghData["bot_replies"] = replies
	}

	if cfg.CommentMap {
		commentMap, err := loadCommentMap(opts.SharedDir, opts.Meta.CaseName)
		if err != nil {
			return err
		}
		ghData["comment_map"] = commentMap
	}

	if cfg.ExpectedBranchDiff {
		if err := collectExpectedDiff(ctx, repo, opts.Case.Input.ExpectedBranch, fixtureSHA, ghData); err != nil {
			return err
		}
	}
	return nil
}

func collectBotReplies(ctx context.Context, client *ghclient.Client, prNumber int, botLogin string) ([]map[string]any, error) {
	if prNumber <= 0 {
		return nil, fmt.Errorf("bot_replies collect requires a PR number")
	}
	issueComments, err := client.ListIssueComments(ctx, prNumber)
	if err != nil {
		return nil, fmt.Errorf("listing issue comments: %w", err)
	}
	prComments, err := client.ListPRReviewComments(ctx, prNumber)
	if err != nil {
		return nil, fmt.Errorf("listing PR review comments: %w", err)
	}

	var botReplies []map[string]any
	for _, c := range issueComments {
		if c.GetUser().GetLogin() == botLogin {
			botReplies = append(botReplies, map[string]any{
				"id":         c.GetID(),
				"body":       c.GetBody(),
				"created_at": c.GetCreatedAt().String(),
				"type":       "issue",
			})
		}
	}
	for _, c := range prComments {
		if c.GetUser().GetLogin() == botLogin {
			botReplies = append(botReplies, map[string]any{
				"id":         c.GetID(),
				"body":       c.GetBody(),
				"created_at": c.GetCreatedAt().String(),
				"path":       c.GetPath(),
				"type":       "review",
			})
		}
	}
	return botReplies, nil
}

func loadCommentMap(sharedDir, caseName string) (map[string]any, error) {
	mapPath := filepath.Join(sharedDir, caseName+".comment-map.json")
	data, err := os.ReadFile(mapPath)
	if err != nil {
		return nil, fmt.Errorf("reading comment map: %w", err)
	}
	var commentMap map[string]any
	if err := json.Unmarshal(data, &commentMap); err != nil {
		return nil, fmt.Errorf("parsing comment map: %w", err)
	}
	return commentMap, nil
}

func collectExpectedDiff(ctx context.Context, repo *git.Repo, expectedBranch, fixtureSHA string, ghData map[string]any) error {
	if err := repo.Fetch(ctx, "origin", expectedBranch); err != nil {
		return fmt.Errorf("fetching expected branch %s: %w", expectedBranch, err)
	}
	expectedDiff, err := repo.DiffBranches(ctx, fixtureSHA, "origin/"+expectedBranch)
	if err != nil {
		return fmt.Errorf("diffing fixture against expected branch: %w", err)
	}
	ghData["expected_changed_files"] = expectedDiff.ChangedFiles
	ghData["expected_diff_lines"] = countDiffLines(expectedDiff.FullDiff)
	ghData["expected_full_diff"] = expectedDiff.FullDiff
	ghData["expected_branch"] = expectedBranch
	return nil
}

func countDiffLines(diff string) int {
	count := 0
	for _, line := range strings.Split(diff, "\n") {
		if len(line) > 0 && (line[0] == '+' || line[0] == '-') && !strings.HasPrefix(line, "+++") && !strings.HasPrefix(line, "---") {
			count++
		}
	}
	return count
}

func runMake(ctx context.Context, dir, target string) map[string]any {
	cmd := exec.CommandContext(ctx, "make", target)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	output := string(out)
	const maxOutput = 10000
	if len(output) > maxOutput {
		output = output[len(output)-maxOutput:]
	}
	passed := err == nil
	result := map[string]any{
		"passed": passed,
		"output": strings.TrimSpace(output),
	}
	if err != nil {
		result["error"] = err.Error()
	}
	return result
}
