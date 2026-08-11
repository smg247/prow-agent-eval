package github

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	gh "github.com/google/go-github/v67/github"
)

type SeededComment struct {
	Body     string `json:"body"`
	Category string `json:"category"`
	Path     string `json:"path"`
	Line     int    `json:"line"`
	Side     string `json:"side"`
}

type PostedComment struct {
	GitHubID  int64  `json:"github_id"`
	Category  string `json:"category"`
	CreatedAt string `json:"created_at"`
}

func LoadSeededComments(path string) ([]SeededComment, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading comments file: %w", err)
	}
	var comments []SeededComment
	if err := json.Unmarshal(data, &comments); err != nil {
		return nil, fmt.Errorf("parsing comments file: %w", err)
	}
	return comments, nil
}

func (c *Client) PostIssueComment(ctx context.Context, number int, body string) (*gh.IssueComment, error) {
	comment, _, err := c.gh.Issues.CreateComment(ctx, c.owner, c.repo, number, &gh.IssueComment{
		Body: gh.String(body),
	})
	if err != nil {
		return nil, fmt.Errorf("posting issue comment: %w", err)
	}
	return comment, nil
}

func (c *Client) PostPRReviewComment(ctx context.Context, number int, body, path string, line int, side string) (*gh.PullRequestComment, error) {
	if side == "" {
		side = "RIGHT"
	}
	comment, _, err := c.gh.PullRequests.CreateComment(ctx, c.owner, c.repo, number, &gh.PullRequestComment{
		Body: gh.String(body),
		Path: gh.String(path),
		Line: gh.Int(line),
		Side: gh.String(side),
	})
	if err != nil {
		return nil, fmt.Errorf("posting PR review comment: %w", err)
	}
	return comment, nil
}

func (c *Client) SeedComments(ctx context.Context, prNumber int, comments []SeededComment) (map[string]PostedComment, error) {
	posted := make(map[string]PostedComment)

	for i, sc := range comments {
		key := fmt.Sprintf("comment-%03d", i+1)
		if sc.Path != "" && sc.Line > 0 {
			rc, err := c.PostPRReviewComment(ctx, prNumber, sc.Body, sc.Path, sc.Line, sc.Side)
			if err != nil {
				return nil, fmt.Errorf("seeding inline comment %d: %w", i, err)
			}
			posted[key] = PostedComment{
				GitHubID:  rc.GetID(),
				Category:  sc.Category,
				CreatedAt: rc.GetCreatedAt().String(),
			}
		} else {
			ic, err := c.PostIssueComment(ctx, prNumber, sc.Body)
			if err != nil {
				return nil, fmt.Errorf("seeding issue comment %d: %w", i, err)
			}
			posted[key] = PostedComment{
				GitHubID:  ic.GetID(),
				Category:  sc.Category,
				CreatedAt: ic.GetCreatedAt().String(),
			}
		}
	}

	return posted, nil
}

func (c *Client) ListIssueComments(ctx context.Context, number int) ([]*gh.IssueComment, error) {
	var all []*gh.IssueComment
	err := paginate(ctx, func(page int) ([]*gh.IssueComment, *gh.Response, error) {
		return c.gh.Issues.ListComments(ctx, c.owner, c.repo, number, &gh.IssueListCommentsOptions{
			ListOptions: gh.ListOptions{Page: page, PerPage: 100},
		})
	}, &all)
	if err != nil {
		return nil, fmt.Errorf("listing issue comments: %w", err)
	}
	return all, nil
}

func (c *Client) ListPRReviewComments(ctx context.Context, number int) ([]*gh.PullRequestComment, error) {
	var all []*gh.PullRequestComment
	err := paginate(ctx, func(page int) ([]*gh.PullRequestComment, *gh.Response, error) {
		return c.gh.PullRequests.ListComments(ctx, c.owner, c.repo, number, &gh.PullRequestListCommentsOptions{
			ListOptions: gh.ListOptions{Page: page, PerPage: 100},
		})
	}, &all)
	if err != nil {
		return nil, fmt.Errorf("listing PR review comments: %w", err)
	}
	return all, nil
}

func paginate[T any](ctx context.Context, fetch func(page int) ([]T, *gh.Response, error), dest *[]T) error {
	page := 1
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		items, resp, err := fetch(page)
		if err != nil {
			return err
		}
		*dest = append(*dest, items...)
		if resp == nil || resp.NextPage == 0 {
			return nil
		}
		page = resp.NextPage
	}
}
