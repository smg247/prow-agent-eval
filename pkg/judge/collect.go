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

func Collect(ctx context.Context, opts CollectOptions) (CaseEvidence, error) {
	if err := validateCollectOptions(opts); err != nil {
		return CaseEvidence{}, err
	}

	var evidence CaseEvidence
	if opts.Case.Annotations != nil {
		evidence.Annotations = opts.Case.Annotations
	}

	repo, err := git.Clone(ctx, opts.Client.CloneURL(), opts.CloneDir, opts.Token)
	if err != nil {
		return CaseEvidence{}, fmt.Errorf("cloning repo: %w", err)
	}

	headBranch, prNumber, err := resolveHead(ctx, opts)
	if err != nil {
		return CaseEvidence{}, err
	}

	if err := repo.Fetch(ctx, "origin", headBranch); err != nil {
		return CaseEvidence{}, fmt.Errorf("fetching head branch: %w", err)
	}
	if err := repo.Checkout(ctx, "origin/"+headBranch); err != nil {
		return CaseEvidence{}, fmt.Errorf("checking out head branch: %w", err)
	}

	fixtureSHA, err := resolveFixtureSHA(ctx, repo, opts.Meta)
	if err != nil {
		return CaseEvidence{}, err
	}

	ghData, err := collectGitHubData(ctx, opts, repo, headBranch, fixtureSHA, prNumber)
	if err != nil {
		return CaseEvidence{}, err
	}
	evidence.GitHub = ghData

	cfg := opts.Config.Collect
	if cfg.BuildResult {
		evidence.BuildResult = runMake(ctx, opts.CloneDir, "build")
	}
	if cfg.TestResult {
		evidence.TestResult = runMake(ctx, opts.CloneDir, "test")
	}

	return evidence, nil
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

func collectGitHubData(ctx context.Context, opts CollectOptions, repo *git.Repo, headBranch, fixtureSHA string, prNumber int) (GitHubData, error) {
	diff, err := repo.DiffAgainst(ctx, fixtureSHA)
	if err != nil {
		return GitHubData{}, fmt.Errorf("diff against fixture SHA: %w", err)
	}

	gh := GitHubData{
		ChangedFiles:   diff.ChangedFiles,
		FullDiff:       diff.FullDiff,
		AgentBranch:    headBranch,
		AgentDiffLines: countDiffLines(diff.FullDiff),
		Repo:           opts.Client.Owner() + "/" + opts.Client.Repo(),
		BaseBranch:     opts.Meta.BaseBranch,
	}

	gh.PRNumber = resolvePRNumber(ctx, opts.Client, headBranch, prNumber)

	if err := collectPRDetails(ctx, opts, &gh); err != nil {
		return GitHubData{}, err
	}
	if err := collectOptionalGitHubData(ctx, opts, repo, &gh, fixtureSHA); err != nil {
		return GitHubData{}, err
	}
	return gh, nil
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

func collectPRDetails(ctx context.Context, opts CollectOptions, gh *GitHubData) error {
	if gh.PRNumber > 0 {
		pr, err := opts.Client.GetPR(ctx, gh.PRNumber)
		if err != nil {
			return fmt.Errorf("fetching PR #%d: %w", gh.PRNumber, err)
		}
		gh.PRState = pr.GetState()
		gh.PRBody = pr.GetBody()
	} else {
		gh.PRState = "none"
	}

	descPath := filepath.Join(opts.SharedDir, opts.Meta.CaseName+".pr-description.md")
	if data, err := os.ReadFile(descPath); err == nil && len(strings.TrimSpace(string(data))) > 0 {
		gh.PRDescriptionFile = true
	}
	return nil
}

func collectOptionalGitHubData(ctx context.Context, opts CollectOptions, repo *git.Repo, gh *GitHubData, fixtureSHA string) error {
	cfg := opts.Config.Collect

	if cfg.BotReplies {
		replies, err := collectBotReplies(ctx, opts.Client, gh.PRNumber, opts.Meta.BotLogin)
		if err != nil {
			return err
		}
		gh.BotReplies = replies
	}

	if cfg.CommentMap {
		posted, err := loadPostedComments(opts.SharedDir, opts.Meta.CaseName)
		if err != nil {
			return err
		}
		gh.PostedComments = posted
	}

	if cfg.ExpectedBranchDiff {
		if err := collectExpectedDiff(ctx, repo, opts.Case.Input.ExpectedBranch, fixtureSHA, gh); err != nil {
			return err
		}
	}
	return nil
}

func collectBotReplies(ctx context.Context, client *ghclient.Client, prNumber int, botLogin string) ([]BotReply, error) {
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

	var botReplies []BotReply
	for _, c := range issueComments {
		if c.GetUser().GetLogin() == botLogin {
			botReplies = append(botReplies, BotReply{
				ID:        c.GetID(),
				Body:      c.GetBody(),
				CreatedAt: c.GetCreatedAt().String(),
				Type:      "issue",
			})
		}
	}
	for _, c := range prComments {
		if c.GetUser().GetLogin() == botLogin {
			botReplies = append(botReplies, BotReply{
				ID:        c.GetID(),
				Body:      c.GetBody(),
				CreatedAt: c.GetCreatedAt().String(),
				Path:      c.GetPath(),
				Type:      "review",
			})
		}
	}
	return botReplies, nil
}

func loadPostedComments(sharedDir, caseName string) (map[string]ghclient.PostedComment, error) {
	mapPath := filepath.Join(sharedDir, caseName+".comment-map.json")
	data, err := os.ReadFile(mapPath)
	if err != nil {
		return nil, fmt.Errorf("reading comment map: %w", err)
	}
	var posted map[string]ghclient.PostedComment
	if err := json.Unmarshal(data, &posted); err != nil {
		return nil, fmt.Errorf("parsing comment map: %w", err)
	}
	return posted, nil
}

func collectExpectedDiff(ctx context.Context, repo *git.Repo, expectedBranch, fixtureSHA string, gh *GitHubData) error {
	if err := repo.Fetch(ctx, "origin", expectedBranch); err != nil {
		return fmt.Errorf("fetching expected branch %s: %w", expectedBranch, err)
	}
	expectedDiff, err := repo.DiffBranches(ctx, fixtureSHA, "origin/"+expectedBranch)
	if err != nil {
		return fmt.Errorf("diffing fixture against expected branch: %w", err)
	}
	gh.ExpectedChangedFiles = expectedDiff.ChangedFiles
	gh.ExpectedDiffLines = countDiffLines(expectedDiff.FullDiff)
	gh.ExpectedFullDiff = expectedDiff.FullDiff
	gh.ExpectedBranch = expectedBranch
	gh.HasExpectedDiff = true
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

func runMake(ctx context.Context, dir, target string) MakeResult {
	cmd := exec.CommandContext(ctx, "make", target)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	output := string(out)
	const maxOutput = 10000
	if len(output) > maxOutput {
		output = output[len(output)-maxOutput:]
	}
	result := MakeResult{
		Collected: true,
		Passed:    err == nil,
		Output:    strings.TrimSpace(output),
	}
	if err != nil {
		result.Error = err.Error()
	}
	return result
}
