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
	if cfg.ExpectedBranchDiff {
		if opts.Case.Input.ExpectedBranch == "" {
			return fmt.Errorf("expected_branch_diff requires case input expected_branch")
		}
		if opts.Meta.FixtureHeadSHA == "" {
			return fmt.Errorf("expected_branch_diff requires fixture head SHA in metadata")
		}
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

	ghData := make(map[string]any)

	repo, err := git.Clone(ctx, opts.Client.CloneURL(), opts.CloneDir, opts.Token)
	if err != nil {
		return nil, fmt.Errorf("cloning repo: %w", err)
	}

	if err := repo.Fetch(ctx, "origin", opts.Meta.HeadBranch); err != nil {
		return nil, fmt.Errorf("fetching head branch: %w", err)
	}
	if err := repo.Checkout(ctx, "origin/"+opts.Meta.HeadBranch); err != nil {
		return nil, fmt.Errorf("checking out head branch: %w", err)
	}

	diff, err := repo.DiffAgainst(ctx, opts.Meta.FixtureHeadSHA)
	if err != nil {
		return nil, fmt.Errorf("diff against fixture SHA: %w", err)
	}
	ghData["changed_files"] = diff.ChangedFiles
	ghData["full_diff"] = diff.FullDiff
	ghData["agent_branch"] = opts.Meta.HeadBranch

	prNumber := opts.Meta.PRNumber
	if prNumber == 0 {
		pr, err := opts.Client.FindPRByHead(ctx, opts.Meta.HeadBranch)
		if err != nil && !errors.Is(err, ghclient.ErrNotFound) {
			slog.Warn("could not search for agent-created PR", "error", err)
		} else if err == nil {
			prNumber = pr.GetNumber()
			slog.Info("discovered agent-created PR", "pr", prNumber)
		}
	}
	ghData["pr_number"] = prNumber

	if prNumber > 0 {
		pr, err := opts.Client.GetPR(ctx, prNumber)
		if err != nil {
			return nil, fmt.Errorf("fetching PR #%d: %w", prNumber, err)
		}
		ghData["pr_state"] = pr.GetState()
	} else {
		ghData["pr_state"] = "none"
	}

	cfg := opts.Config.Collect

	if cfg.BotReplies {
		if prNumber <= 0 {
			return nil, fmt.Errorf("bot_replies collect requires a PR number")
		}
		botLogin := opts.Meta.BotLogin
		issueComments, err := opts.Client.ListIssueComments(ctx, prNumber)
		if err != nil {
			return nil, fmt.Errorf("listing issue comments: %w", err)
		}
		prComments, err := opts.Client.ListPRReviewComments(ctx, prNumber)
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
		ghData["bot_replies"] = botReplies
	}

	if cfg.CommentMap {
		mapPath := filepath.Join(opts.SharedDir, opts.Meta.CaseName+".comment-map.json")
		data, err := os.ReadFile(mapPath)
		if err != nil {
			return nil, fmt.Errorf("reading comment map: %w", err)
		}
		var commentMap map[string]any
		if err := json.Unmarshal(data, &commentMap); err != nil {
			return nil, fmt.Errorf("parsing comment map: %w", err)
		}
		ghData["comment_map"] = commentMap
	}

	if cfg.ExpectedBranchDiff {
		expectedBranch := opts.Case.Input.ExpectedBranch
		if err := repo.Fetch(ctx, "origin", expectedBranch); err != nil {
			return nil, fmt.Errorf("fetching expected branch %s: %w", expectedBranch, err)
		}
		expectedDiff, err := repo.DiffBranches(ctx, opts.Meta.FixtureHeadSHA, "origin/"+expectedBranch)
		if err != nil {
			return nil, fmt.Errorf("diffing fixture against expected branch: %w", err)
		}
		ghData["expected_changed_files"] = expectedDiff.ChangedFiles
	}

	outputs["github"] = ghData

	if cfg.BuildResult {
		outputs["build_result"] = runMake(ctx, opts.CloneDir, "build")
	}

	if cfg.TestResult {
		outputs["test_result"] = runMake(ctx, opts.CloneDir, "test")
	}

	return outputs, nil
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
