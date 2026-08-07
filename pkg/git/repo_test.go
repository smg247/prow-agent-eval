package git

import (
	"testing"
)

func TestRedactSecrets(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "url credentials",
			input: "fatal: cloning https://x-access-token:ghs_secret123@github.com/o/r.git failed",
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
			if got != tt.want {
				t.Errorf("RedactSecrets() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestShortSHA(t *testing.T) {
	if got := ShortSHA("abcdef123456", 8); got != "abcdef12" {
		t.Errorf("ShortSHA long = %q", got)
	}
	if got := ShortSHA("abc", 8); got != "abc" {
		t.Errorf("ShortSHA short = %q", got)
	}
	if got := ShortSHA("", 8); got != "" {
		t.Errorf("ShortSHA empty = %q", got)
	}
}

func TestParseNameOnly(t *testing.T) {
	got := parseNameOnly("a.go\nb.go\n\n")
	if len(got) != 2 || got[0] != "a.go" || got[1] != "b.go" {
		t.Errorf("parseNameOnly = %#v", got)
	}
}

func TestEvalBranchName(t *testing.T) {
	name := EvalBranchName("case-001")
	if name == "" || len(name) < len("case-001-eval-") {
		t.Errorf("EvalBranchName = %q", name)
	}
}
