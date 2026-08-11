package github

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	gh "github.com/google/go-github/v67/github"
)

const defaultHTTPTimeout = 30 * time.Second

type Client struct {
	gh       *gh.Client
	owner    string
	repo     string
	cloneURL string
}

func NewClient(token, repoFullName string) (*Client, error) {
	if token == "" {
		token = os.Getenv("GITHUB_TOKEN")
	}
	if token == "" {
		return nil, fmt.Errorf("GITHUB_TOKEN not set and --token not provided")
	}

	httpClient := &http.Client{Timeout: defaultHTTPTimeout}
	client := gh.NewClient(httpClient).WithAuthToken(token)
	return makeClient(client, repoFullName, "")
}

// NewClientWithHTTP builds a client that talks to apiBaseURL (e.g. an httptest server).
// token may be empty for tests that do not require auth.
func NewClientWithHTTP(httpClient *http.Client, apiBaseURL, token, repoFullName string) (*Client, error) {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: defaultHTTPTimeout}
	}
	client := gh.NewClient(httpClient)
	if token != "" {
		client = client.WithAuthToken(token)
	}
	base, err := url.Parse(strings.TrimRight(apiBaseURL, "/") + "/")
	if err != nil {
		return nil, fmt.Errorf("parsing API base URL: %w", err)
	}
	client.BaseURL = base
	return makeClient(client, repoFullName, "")
}

func makeClient(ghClient *gh.Client, repoFullName, cloneURL string) (*Client, error) {
	parts := strings.SplitN(repoFullName, "/", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid repo format %q, expected owner/repo", repoFullName)
	}
	return &Client{
		gh:       ghClient,
		owner:    parts[0],
		repo:     parts[1],
		cloneURL: cloneURL,
	}, nil
}

func (c *Client) Owner() string { return c.owner }
func (c *Client) Repo() string  { return c.repo }

// SetCloneURL overrides the URL used for git clone (e.g. a local bare repo in tests).
func (c *Client) SetCloneURL(u string) { c.cloneURL = u }

func (c *Client) CloneURL() string {
	if c.cloneURL != "" {
		return c.cloneURL
	}
	return fmt.Sprintf("https://github.com/%s/%s.git", c.owner, c.repo)
}

func (c *Client) GetBotLogin(ctx context.Context) (string, error) {
	user, _, err := c.gh.Users.Get(ctx, "")
	if err != nil {
		return "", fmt.Errorf("getting authenticated user: %w", err)
	}
	return user.GetLogin(), nil
}
