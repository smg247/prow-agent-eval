package cli

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/smg247/prow-agent-eval/pkg/config"
	ghclient "github.com/smg247/prow-agent-eval/pkg/github"
	"github.com/smg247/prow-agent-eval/pkg/judge"
	"github.com/smg247/prow-agent-eval/pkg/report"
	"github.com/smg247/prow-agent-eval/pkg/shared"
	"github.com/spf13/cobra"
)

var (
	judgeFlags       sharedFlags
	judgeArtifactDir string
)

var judgeCmd = &cobra.Command{
	Use:   "judge",
	Short: "Collect post-agent state, run judges, and emit reports",
	Args:  cobra.NoArgs,
	RunE:  runJudge,
}

func init() {
	judgeFlags.addCommon(judgeCmd)
	judgeCmd.Flags().StringVar(&judgeArtifactDir, "artifact-dir", "", "Directory for output artifacts (JUnit XML, reports)")
	mustMarkRequired(judgeCmd, "config", "shared-dir", "artifact-dir")
}

func runJudge(cmd *cobra.Command, args []string) error {
	ctx, stop := commandContext(cmd)
	defer stop()

	token := resolveToken(judgeFlags.token)

	cfg, err := config.Load(judgeFlags.configPath)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	configDir := filepath.Dir(judgeFlags.configPath)

	var caseNames []string
	if judgeFlags.caseName != "" {
		caseNames = []string{judgeFlags.caseName}
	} else {
		caseNames, err = shared.ReadCaseList(judgeFlags.sharedDir)
		if err != nil {
			return fmt.Errorf("reading case list: %w", err)
		}
	}

	if len(caseNames) == 0 {
		return fmt.Errorf("no cases found")
	}

	if err := os.MkdirAll(judgeArtifactDir, 0755); err != nil {
		return fmt.Errorf("creating artifact dir: %w", err)
	}

	slog.Info("judging cases", "count", len(caseNames))

	var allResults []judge.Result
	caseErrors := 0
	for _, caseName := range caseNames {
		results, err := judgeCase(ctx, cfg, configDir, caseName, token)
		if err != nil {
			slog.Error("case error", "case", caseName, "error", err)
			caseErrors++
			allResults = append(allResults, judge.Result{
				Name:  caseName,
				Error: err.Error(),
			})
			continue
		}

		if err := report.WriteCaseYAML(judgeArtifactDir, caseName, results); err != nil {
			slog.Warn("failed to write case YAML", "case", caseName, "error", err)
		}

		allResults = append(allResults, results...)
	}

	thresholds := judge.ApplyThresholds(allResults, cfg.Thresholds)

	if err := report.WriteJUnit(judgeArtifactDir, cfg.Name, allResults); err != nil {
		return fmt.Errorf("writing JUnit XML: %w", err)
	}
	if err := report.WriteSummaryYAML(judgeArtifactDir, cfg.Name, len(caseNames), allResults, thresholds); err != nil {
		slog.Warn("failed to write summary YAML", "error", err)
	}
	if err := report.WriteHTML(judgeArtifactDir, cfg.Name, allResults, thresholds); err != nil {
		slog.Warn("failed to write HTML report", "error", err)
	}

	for _, t := range thresholds {
		if !t.Met {
			slog.Warn("threshold missed", "name", t.Name, "actual", t.Actual, "required", t.Required)
		} else {
			slog.Info("threshold met", "name", t.Name, "actual", t.Actual, "required", t.Required)
		}
	}

	tally := report.TallyResults(allResults)
	slog.Info("judge complete",
		"passed", tally.Passed,
		"failed", tally.Failed,
		"errors", tally.Errors,
		"cases", len(caseNames),
		"case_errors", caseErrors,
	)

	if ok, reason := judge.EvaluateGate(thresholds, caseErrors); !ok {
		return fmt.Errorf("%s", reason)
	}

	return nil
}

func judgeCase(ctx context.Context, cfg *config.EvalConfig, configDir, caseName, token string) ([]judge.Result, error) {
	meta, err := shared.ReadCaseMetadata(judgeFlags.sharedDir, caseName)
	if err != nil {
		return nil, fmt.Errorf("reading metadata: %w", err)
	}

	c, err := config.LoadCase(configDir, cfg.Dataset.Path, caseName)
	if err != nil {
		return nil, fmt.Errorf("loading case: %w", err)
	}

	repo, err := resolveRepo(cfg.Init.Repo, meta.Repo)
	if err != nil {
		return nil, err
	}

	client, err := ghclient.NewClient(token, repo)
	if err != nil {
		return nil, err
	}

	slog.Info("collecting data", "case", caseName)

	cloneDir, err := os.MkdirTemp("", "prow-agent-eval-judge-*")
	if err != nil {
		return nil, fmt.Errorf("creating temp dir: %w", err)
	}
	defer os.RemoveAll(cloneDir)

	outputs, err := judge.Collect(ctx, judge.CollectOptions{
		Config:    cfg,
		Case:      c,
		Meta:      meta,
		Client:    client,
		Token:     token,
		CloneDir:  cloneDir,
		SharedDir: judgeFlags.sharedDir,
	})
	if err != nil {
		return nil, fmt.Errorf("collecting data: %w", err)
	}

	slog.Info("running judges", "case", caseName)

	results, err := judge.Run(cfg.Judges, outputs)
	if err != nil {
		return nil, fmt.Errorf("running judges: %w", err)
	}

	for _, r := range results {
		status := "PASS"
		if r.Error != "" {
			status = "ERROR"
		} else if !r.Passed {
			status = "FAIL"
		}
		slog.Info("judge status", "case", caseName, "judge", r.Name, "status", status, "message", r.Message)
	}

	return results, nil
}
