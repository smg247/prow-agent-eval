package judge

import (
	"github.com/smg247/prow-agent-eval/pkg/config"
	ghclient "github.com/smg247/prow-agent-eval/pkg/github"
)

// CaseEvidence is the collected evidence about what the agent did for one case.
// Judges and reporters consume it after Collect runs.
type CaseEvidence struct {
	GitHub      GitHubData
	Annotations config.CaseAnnotations
	BuildResult MakeResult
	TestResult  MakeResult
}

// MakeResult is the outcome of a make target run during collect.
type MakeResult struct {
	Collected bool // false = collect skipped / not run
	Passed    bool
	Output    string
	Error     string
}

// GitHubData holds PR, branch, and diff evidence gathered from GitHub/git.
type GitHubData struct {
	Repo                 string
	BaseBranch           string
	AgentBranch          string
	ExpectedBranch       string
	PRNumber             int
	PRState              string
	PRBody               string
	PRDescriptionFile    bool
	ChangedFiles         []string
	ExpectedChangedFiles []string
	FullDiff             string
	ExpectedFullDiff     string
	AgentDiffLines       int
	ExpectedDiffLines    int
	HasExpectedDiff      bool // set when expected_branch_diff collect ran
	BotReplies           []BotReply
	PostedComments       map[string]ghclient.PostedComment
}

// BotReply is a comment attributed to the configured bot login.
type BotReply struct {
	ID        int64
	Body      string
	CreatedAt string
	Path      string
	Type      string // "issue" | "review"
}
