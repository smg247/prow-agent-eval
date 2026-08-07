package github

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	gh "github.com/google/go-github/v67/github"
)

const defaultHTTPTimeout = 30 * time.Second

type Client struct {
	gh    *gh.Client
	owner string
	repo  string
}

func NewClient(token, repoFullName string) (*Client, error) {
	if token == "" {
		token = os.Getenv("GITHUB_TOKEN")
	}
	if token == "" {
		return nil, fmt.Errorf("GITHUB_TOKEN not set and --token not provided")
	}

	parts := strings.SplitN(repoFullName, "/", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid repo format %q, expected owner/repo", repoFullName)
	}

	httpClient := &http.Client{Timeout: defaultHTTPTimeout}
	client := gh.NewClient(httpClient).WithAuthToken(token)

	return &Client{
		gh:    client,
		owner: parts[0],
		repo:  parts[1],
	}, nil
}

func (c *Client) Owner() string { return c.owner }
func (c *Client) Repo() string  { return c.repo }

func (c *Client) CloneURL() string {
	return fmt.Sprintf("https://github.com/%s/%s.git", c.owner, c.repo)
}

func (c *Client) GetBotLogin(ctx context.Context) (string, error) {
	user, _, err := c.gh.Users.Get(ctx, "")
	if err != nil {
		return "", fmt.Errorf("getting authenticated user: %w", err)
	}
	return user.GetLogin(), nil
}
