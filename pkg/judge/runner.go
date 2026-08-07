package judge

import (
	"fmt"
	"log/slog"

	"github.com/smg247/prow-agent-eval/pkg/config"
)

type Result struct {
	Name        string
	Description string
	Passed      bool
	Message     string
	Error       string
}

type Outputs map[string]any

func Run(judges []config.JudgeConfig, outputs Outputs) ([]Result, error) {
	var results []Result

	for _, j := range judges {
		result := Result{
			Name:        j.Name,
			Description: j.Description,
		}

		typ := j.Type
		if typ == "" {
			typ = j.Name
		}
		if typ == "" {
			result.Error = "judge has no type or name"
		} else {
			passed, msg, err := runBuiltin(typ, outputs)
			if err != nil {
				result.Error = err.Error()
				slog.Error("judge errored", "judge", j.Name, "error", err)
			} else {
				result.Passed = passed
				result.Message = msg
			}
		}

		slog.Info("judge result", "judge", j.Name, "passed", result.Passed, "message", result.Message)
		results = append(results, result)
	}

	return results, nil
}

type ThresholdResult struct {
	Name       string
	Met        bool
	Actual     float64
	Required   float64
	MetricType string
}

func ApplyThresholds(results []Result, thresholds map[string]config.Threshold) []ThresholdResult {
	passCounts := make(map[string]int)
	totalCounts := make(map[string]int)

	for _, r := range results {
		if r.Error != "" {
			continue
		}
		totalCounts[r.Name]++
		if r.Passed {
			passCounts[r.Name]++
		}
	}

	var tResults []ThresholdResult
	for name, thresh := range thresholds {
		if thresh.MinPassRate == nil {
			continue
		}
		total := totalCounts[name]
		passed := passCounts[name]
		var rate float64
		met := false
		if total > 0 {
			rate = float64(passed) / float64(total)
			met = rate >= *thresh.MinPassRate
		}
		tResults = append(tResults, ThresholdResult{
			Name:       fmt.Sprintf("%s/pass_rate", name),
			Met:        met,
			Actual:     rate,
			Required:   *thresh.MinPassRate,
			MetricType: "pass_rate",
		})
	}

	return tResults
}

// EvaluateGate returns whether the overall judge run should succeed.
func EvaluateGate(thresholds []ThresholdResult, caseErrors int) (ok bool, reason string) {
	if caseErrors > 0 {
		return false, fmt.Sprintf("%d case error(s)", caseErrors)
	}
	for _, t := range thresholds {
		if !t.Met {
			return false, "one or more thresholds not met"
		}
	}
	return true, ""
}
