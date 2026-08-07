package github

import (
	"context"
	"errors"
	"fmt"

	gh "github.com/google/go-github/v67/github"
)

var ErrNotFound = errors.New("not found")

func (c *Client) CreatePR(ctx context.Context, head, base, title, body string) (int, error) {
	pr, _, err := c.gh.PullRequests.Create(ctx, c.owner, c.repo, &gh.NewPullRequest{
		Title: gh.String(title),
		Head:  gh.String(head),
		Base:  gh.String(base),
		Body:  gh.String(body),
	})
	if err != nil {
		return 0, fmt.Errorf("creating PR: %w", err)
	}
	return pr.GetNumber(), nil
}

func (c *Client) ClosePR(ctx context.Context, number int) error {
	_, _, err := c.gh.PullRequests.Edit(ctx, c.owner, c.repo, number, &gh.PullRequest{
		State: gh.String("closed"),
	})
	if err != nil {
		return fmt.Errorf("closing PR #%d: %w", number, err)
	}
	return nil
}

func (c *Client) GetPR(ctx context.Context, number int) (*gh.PullRequest, error) {
	pr, _, err := c.gh.PullRequests.Get(ctx, c.owner, c.repo, number)
	if err != nil {
		return nil, fmt.Errorf("getting PR #%d: %w", number, err)
	}
	return pr, nil
}

func (c *Client) FindPRByHead(ctx context.Context, headBranch string) (*gh.PullRequest, error) {
	prs, _, err := c.gh.PullRequests.List(ctx, c.owner, c.repo, &gh.PullRequestListOptions{
		Head:        c.owner + ":" + headBranch,
		State:       "all",
		ListOptions: gh.ListOptions{PerPage: 1},
	})
	if err != nil {
		return nil, fmt.Errorf("listing PRs for head %s: %w", headBranch, err)
	}
	if len(prs) == 0 {
		return nil, ErrNotFound
	}
	return prs[0], nil
}

func (c *Client) DeleteBranch(ctx context.Context, branch string) error {
	_, err := c.gh.Git.DeleteRef(ctx, c.owner, c.repo, "heads/"+branch)
	if err != nil {
		return fmt.Errorf("deleting branch %s: %w", branch, err)
	}
	return nil
}
