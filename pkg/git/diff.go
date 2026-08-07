package git

import (
	"context"
	"fmt"
	"strings"
)

type DiffResult struct {
	ChangedFiles []string
	FullDiff     string
}

func parseNameOnly(out string) []string {
	var files []string
	for _, f := range strings.Split(strings.TrimSpace(out), "\n") {
		if f != "" {
			files = append(files, f)
		}
	}
	return files
}

func (r *Repo) DiffAgainst(ctx context.Context, baseSHA string) (*DiffResult, error) {
	filesOut, err := gitOutput(ctx, r.Dir, r.Token, "diff", "--name-only", baseSHA)
	if err != nil {
		return nil, fmt.Errorf("diff --name-only against %s: %w", baseSHA, err)
	}

	fullDiff, err := gitOutput(ctx, r.Dir, r.Token, "diff", baseSHA)
	if err != nil {
		return nil, fmt.Errorf("diff against %s: %w", baseSHA, err)
	}

	return &DiffResult{
		ChangedFiles: parseNameOnly(filesOut),
		FullDiff:     fullDiff,
	}, nil
}

func (r *Repo) DiffBranches(ctx context.Context, branch1, branch2 string) (*DiffResult, error) {
	rangeSpec := branch1 + "..." + branch2
	filesOut, err := gitOutput(ctx, r.Dir, r.Token, "diff", "--name-only", rangeSpec)
	if err != nil {
		return nil, fmt.Errorf("diff --name-only %s: %w", rangeSpec, err)
	}

	fullDiff, err := gitOutput(ctx, r.Dir, r.Token, "diff", rangeSpec)
	if err != nil {
		return nil, fmt.Errorf("diff %s: %w", rangeSpec, err)
	}

	return &DiffResult{
		ChangedFiles: parseNameOnly(filesOut),
		FullDiff:     fullDiff,
	}, nil
}

func (r *Repo) CurrentBranch(ctx context.Context) (string, error) {
	out, err := gitOutput(ctx, r.Dir, r.Token, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", fmt.Errorf("getting current branch: %w", err)
	}
	return strings.TrimSpace(out), nil
}
