package cli

import (
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:          "prow-agent-eval",
	Short:        "Reusable agentic eval CLI for Prow",
	Long:         "prow-agent-eval handles the full eval lifecycle (init, judge, cleanup) for agentic Prow CI jobs.",
	SilenceUsage: true,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		setupLogging()
	},
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(judgeCmd)
	rootCmd.AddCommand(cleanupCmd)
}
