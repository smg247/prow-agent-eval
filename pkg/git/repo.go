package git

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)

type Repo struct {
	Dir   string
	Token string
}

// Clone clones a repository using optional token auth via http.extraHeader
// (never embeds the token in the remote URL).
func Clone(ctx context.Context, url, dir, token string) (*Repo, error) {
	if err := gitRun(ctx, "", token, "clone", url, dir); err != nil {
		return nil, fmt.Errorf("cloning %s: %w", redactURL(url), err)
	}
	return &Repo{Dir: dir, Token: token}, nil
}

func Open(dir string) *Repo {
	return &Repo{Dir: dir}
}

func OpenValidated(ctx context.Context, dir string) (*Repo, error) {
	r := &Repo{Dir: dir}
	if _, err := gitOutput(ctx, r.Dir, r.Token, "rev-parse", "--git-dir"); err != nil {
		return nil, fmt.Errorf("not a git repository %s: %w", dir, err)
	}
	return r, nil
}

func (r *Repo) Fetch(ctx context.Context, remote string, refs ...string) error {
	args := append([]string{"fetch", remote}, refs...)
	return gitRun(ctx, r.Dir, r.Token, args...)
}

func (r *Repo) RevParse(ctx context.Context, ref string) (string, error) {
	out, err := gitOutput(ctx, r.Dir, r.Token, "rev-parse", ref)
	if err != nil {
		return "", fmt.Errorf("rev-parse %s: %w", ref, err)
	}
	return strings.TrimSpace(out), nil
}

func (r *Repo) SetConfig(ctx context.Context, key, value string) error {
	return gitRun(ctx, r.Dir, r.Token, "config", key, value)
}

func authHeaderArgs(token string) []string {
	if token == "" {
		return nil
	}
	cred := base64.StdEncoding.EncodeToString([]byte("x-access-token:" + token))
	return []string{"-c", "http.extraHeader=AUTHORIZATION: basic " + cred}
}

func gitRun(ctx context.Context, dir, token string, args ...string) error {
	cmdArgs := append(authHeaderArgs(token), args...)
	cmd := exec.CommandContext(ctx, "git", cmdArgs...)
	cmd.Dir = dir
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg != "" {
			return fmt.Errorf("%w: %s", err, redactSecrets(msg))
		}
		return fmt.Errorf("%s", redactSecrets(err.Error()))
	}
	return nil
}

func gitOutput(ctx context.Context, dir, token string, args ...string) (string, error) {
	cmdArgs := append(authHeaderArgs(token), args...)
	cmd := exec.CommandContext(ctx, "git", cmdArgs...)
	cmd.Dir = dir
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg != "" {
			return "", fmt.Errorf("%w: %s", err, redactSecrets(msg))
		}
		return "", fmt.Errorf("%s", redactSecrets(err.Error()))
	}
	return string(out), nil
}

var (
	urlCredPattern   = regexp.MustCompile(`://[^/@\s]+:[^/@\s]+@`)
	basicAuthPattern = regexp.MustCompile(`(?i)(AUTHORIZATION:\s*basic\s+)[A-Za-z0-9+/=]+`)
)

func redactURL(url string) string {
	return urlCredPattern.ReplaceAllString(url, "://***:***@")
}

func redactSecrets(s string) string {
	s = urlCredPattern.ReplaceAllString(s, "://***:***@")
	s = basicAuthPattern.ReplaceAllString(s, "${1}***")
	return s
}

// ShortSHA returns up to n characters of a SHA without panicking on short strings.
func ShortSHA(sha string, n int) string {
	if n <= 0 || sha == "" {
		return sha
	}
	if len(sha) < n {
		return sha
	}
	return sha[:n]
}

// RedactSecrets is exported for tests and callers that need to scrub error text.
func RedactSecrets(s string) string {
	return redactSecrets(s)
}
