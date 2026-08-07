package git

import (
	"context"
	"fmt"
	"time"
)

func (r *Repo) CreateBranch(ctx context.Context, name, startRef string) error {
	return gitRun(ctx, r.Dir, r.Token, "checkout", "-b", name, startRef)
}

func (r *Repo) Checkout(ctx context.Context, ref string) error {
	return gitRun(ctx, r.Dir, r.Token, "checkout", ref)
}

func (r *Repo) Push(ctx context.Context, remote, branch string) error {
	return gitRun(ctx, r.Dir, r.Token, "push", remote, branch)
}

func (r *Repo) DeleteRemoteBranch(ctx context.Context, remote, branch string) error {
	return gitRun(ctx, r.Dir, r.Token, "push", remote, "--delete", branch)
}

func EvalBranchName(prefix string) string {
	return fmt.Sprintf("%s-eval-%s", prefix, time.Now().Format("20060102-150405"))
}
