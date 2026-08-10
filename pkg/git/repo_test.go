package git

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestRedactSecrets(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "url credentials",
			input: "fatal: cloning https://x-access-token:" + "ghs_secret123@github.com/o/r.git failed",
			want:  "fatal: cloning https://***:***@github.com/o/r.git failed",
		},
		{
			name:  "basic auth header",
			input: "AUTHORIZATION: basic YWJjMTIz",
			want:  "AUTHORIZATION: basic ***",
		},
		{
			name:  "plain text unchanged",
			input: "fatal: repository not found",
			want:  "fatal: repository not found",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RedactSecrets(tt.input)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("RedactSecrets() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestShortSHA(t *testing.T) {
	tests := []struct {
		name string
		sha  string
		n    int
		want string
	}{
		{name: "long", sha: "abcdef123456", n: 8, want: "abcdef12"},
		{name: "short", sha: "abc", n: 8, want: "abc"},
		{name: "empty", sha: "", n: 8, want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ShortSHA(tt.sha, tt.n)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("ShortSHA() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestParseNameOnly(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name:  "two files with blank line",
			input: "a.go\nb.go\n\n",
			want:  []string{"a.go", "b.go"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseNameOnly(tt.input)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("parseNameOnly() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestEvalBranchName(t *testing.T) {
	tests := []struct {
		name   string
		prefix string
	}{
		{name: "case prefix", prefix: "case-001"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EvalBranchName(tt.prefix)
			parts := strings.SplitN(got, "-eval-", 2)
			if diff := cmp.Diff([]string{tt.prefix}, parts[:1]); diff != "" {
				t.Errorf("EvalBranchName() prefix mismatch (-want +got):\n%s", diff)
			}
			if len(parts) != 2 || parts[1] == "" {
				t.Errorf("EvalBranchName() missing timestamp suffix: %q", got)
			}
		})
	}
}
