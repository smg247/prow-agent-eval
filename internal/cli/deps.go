package cli

import (
	ghclient "github.com/smg247/prow-agent-eval/pkg/github"
)

// Deps holds overridable constructors for integration tests.
type Deps struct {
	NewGitHubClient func(token, repo string) (*ghclient.Client, error)
}

var deps = defaultDeps()

func defaultDeps() Deps {
	return Deps{
		NewGitHubClient: ghclient.NewClient,
	}
}

// SetDeps replaces runtime dependencies. Call ResetDeps when finished.
func SetDeps(d Deps) {
	if d.NewGitHubClient == nil {
		d.NewGitHubClient = ghclient.NewClient
	}
	deps = d
}

// ResetDeps restores production dependencies and clears command flags.
func ResetDeps() {
	deps = defaultDeps()
	resetFlags()
}

func resetFlags() {
	initFlags = sharedFlags{}
	initMode = ""
	judgeFlags = sharedFlags{}
	judgeArtifactDir = ""
	cleanupFlags = sharedFlags{}
}

// ExecuteArgs runs the root command with the given args (for tests).
func ExecuteArgs(args []string) error {
	rootCmd.SetArgs(args)
	err := rootCmd.Execute()
	rootCmd.SetArgs(nil)
	return err
}
