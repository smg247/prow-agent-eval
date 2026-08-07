package cli

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
)

type sharedFlags struct {
	configPath string
	sharedDir  string
	token      string
	caseName   string
}

func (f *sharedFlags) addCommon(cmd *cobra.Command) {
	cmd.Flags().StringVar(&f.configPath, "config", "", "Path to eval config YAML")
	cmd.Flags().StringVar(&f.sharedDir, "shared-dir", "", "Shared directory for metadata exchange")
	cmd.Flags().StringVar(&f.token, "token", "", "GitHub token (defaults to GITHUB_TOKEN env)")
	cmd.Flags().StringVar(&f.caseName, "case", "", "Case name (runs all cases if empty)")
}

func mustMarkRequired(cmd *cobra.Command, names ...string) {
	for _, name := range names {
		if err := cmd.MarkFlagRequired(name); err != nil {
			panic(fmt.Sprintf("marking flag %q required: %v", name, err))
		}
	}
}

func resolveToken(flagToken string) string {
	if flagToken != "" {
		return flagToken
	}
	return os.Getenv("GITHUB_TOKEN")
}

func commandContext(cmd *cobra.Command) (context.Context, context.CancelFunc) {
	return signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
}

func resolveRepo(cfgRepo, override string) (string, error) {
	repo := cfgRepo
	if override != "" {
		repo = override
	}
	if repo == "" {
		return "", fmt.Errorf("no repo specified in config or case")
	}
	return repo, nil
}

func setupLogging() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))
}
