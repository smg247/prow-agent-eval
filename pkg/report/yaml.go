package report

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/smg247/prow-agent-eval/pkg/judge"
	"gopkg.in/yaml.v3"
)

type CaseReport struct {
	Case    string         `yaml:"case"`
	Results []ResultReport `yaml:"results"`
}

type ResultReport struct {
	Name    string `yaml:"name"`
	Passed  bool   `yaml:"passed"`
	Message string `yaml:"message,omitempty"`
	Error   string `yaml:"error,omitempty"`
}

type SummaryReport struct {
	EvalName   string            `yaml:"eval_name"`
	TotalCases int               `yaml:"total_cases"`
	Passed     int               `yaml:"passed"`
	Failed     int               `yaml:"failed"`
	Errors     int               `yaml:"errors"`
	Thresholds []ThresholdReport `yaml:"thresholds,omitempty"`
}

type ThresholdReport struct {
	Name     string  `yaml:"name"`
	Met      bool    `yaml:"met"`
	Actual   float64 `yaml:"actual"`
	Required float64 `yaml:"required"`
}

func WriteCaseYAML(dir, caseName string, results []judge.Result) error {
	report := CaseReport{Case: caseName}
	for _, r := range results {
		report.Results = append(report.Results, ResultReport{
			Name:    r.Name,
			Passed:  r.Passed,
			Message: r.Message,
			Error:   r.Error,
		})
	}

	data, err := yaml.Marshal(report)
	if err != nil {
		return fmt.Errorf("marshaling case YAML: %w", err)
	}

	path := filepath.Join(dir, fmt.Sprintf("eval-%s.yaml", caseName))
	return os.WriteFile(path, data, 0644)
}

func WriteSummaryYAML(dir, evalName string, totalCases int, results []judge.Result, thresholds []judge.ThresholdResult) error {
	tally := TallyResults(results)

	summary := SummaryReport{
		EvalName:   evalName,
		TotalCases: totalCases,
		Passed:     tally.Passed,
		Failed:     tally.Failed,
		Errors:     tally.Errors,
	}

	for _, t := range thresholds {
		summary.Thresholds = append(summary.Thresholds, ThresholdReport{
			Name:     t.Name,
			Met:      t.Met,
			Actual:   t.Actual,
			Required: t.Required,
		})
	}

	data, err := yaml.Marshal(summary)
	if err != nil {
		return fmt.Errorf("marshaling summary YAML: %w", err)
	}

	path := filepath.Join(dir, "eval-summary.yaml")
	return os.WriteFile(path, data, 0644)
}
