package report

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/smg247/prow-agent-eval/pkg/judge"
	"gopkg.in/yaml.v3"
)

type CaseReport struct {
	Case                 string            `yaml:"case"`
	JiraKey              string            `yaml:"jira_key,omitempty"`
	ClaudeBranch         string            `yaml:"claude_branch,omitempty"`
	BaseBranch           string            `yaml:"base_branch,omitempty"`
	ExpectedBranch       string            `yaml:"expected_branch,omitempty"`
	PRNumber             string            `yaml:"pr_number,omitempty"`
	PRURL                string            `yaml:"pr_url,omitempty"`
	DiffURL              string            `yaml:"diff_url,omitempty"`
	Checks               map[string]string `yaml:"checks"`
	Scores               map[string]string `yaml:"scores,omitempty"`
	ClaudeFilesChanged   []string          `yaml:"claude_files_changed,omitempty"`
	ExpectedFilesChanged []string          `yaml:"expected_files_changed,omitempty"`
	ClaudeDiffLines      int               `yaml:"claude_diff_lines,omitempty"`
	ExpectedDiffLines    int               `yaml:"expected_diff_lines,omitempty"`
}

type ResultReport struct {
	Name    string `yaml:"name"`
	Passed  bool   `yaml:"passed"`
	Message string `yaml:"message,omitempty"`
	Error   string `yaml:"error,omitempty"`
}

type SummaryReport struct {
	CasesRun          int               `yaml:"cases_run"`
	TotalChecksPassed int               `yaml:"total_checks_passed"`
	TotalChecks       int               `yaml:"total_checks"`
	Thresholds        []ThresholdReport `yaml:"thresholds,omitempty"`
}

type ThresholdReport struct {
	Name     string  `yaml:"name"`
	Met      bool    `yaml:"met"`
	Actual   float64 `yaml:"actual"`
	Required float64 `yaml:"required"`
}

var scoreRe = regexp.MustCompile(`:\s*([\d.]+)`)

func extractScore(msg string) string {
	if m := scoreRe.FindStringSubmatch(msg); m != nil {
		return m[1]
	}
	if strings.Contains(msg, "N/A") {
		return "N/A"
	}
	return ""
}

func WriteCaseYAML(dir, caseName string, results []judge.Result, outputs judge.Outputs) error {
	report := CaseReport{
		Case:   caseName,
		Checks: make(map[string]string),
	}

	gh, _ := outputs["github"].(map[string]any)
	if gh != nil {
		report.ClaudeBranch, _ = gh["agent_branch"].(string)
		if report.ClaudeBranch == "" {
			report.ClaudeBranch = "none"
		}
		report.ClaudeFilesChanged = toStringSlice(gh["changed_files"])
		report.ExpectedFilesChanged = toStringSlice(gh["expected_changed_files"])
		if n, ok := gh["agent_diff_lines"].(int); ok {
			report.ClaudeDiffLines = n
		}
		if n, ok := gh["expected_diff_lines"].(int); ok {
			report.ExpectedDiffLines = n
		}
		repo, _ := gh["repo"].(string)
		baseBranch, _ := gh["base_branch"].(string)
		expectedBranch, _ := gh["expected_branch"].(string)
		report.BaseBranch = baseBranch
		report.ExpectedBranch = expectedBranch
		if n, ok := gh["pr_number"].(int); ok && n > 0 {
			report.PRNumber = strconv.Itoa(n)
			if repo != "" {
				report.PRURL = fmt.Sprintf("https://github.com/%s/pull/%d", repo, n)
			}
		} else {
			report.PRNumber = "none"
		}
		if repo != "" && baseBranch != "" && expectedBranch != "" {
			report.DiffURL = fmt.Sprintf("https://github.com/%s/compare/%s...%s", repo, baseBranch, expectedBranch)
		}
	}

	passed := 0
	total := 0
	scores := make(map[string]string)
	for _, r := range results {
		total++
		if r.Error != "" {
			report.Checks[r.Name] = "error"
		} else if r.Passed {
			report.Checks[r.Name] = "pass"
			passed++
		} else {
			report.Checks[r.Name] = "fail"
		}
		if s := extractScore(r.Message); s != "" {
			scores[r.Name] = s
		}
	}
	report.Checks["passed"] = strconv.Itoa(passed)
	report.Checks["total"] = strconv.Itoa(total)

	if len(scores) > 0 {
		report.Scores = scores
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
		CasesRun:          totalCases,
		TotalChecksPassed: tally.Passed,
		TotalChecks:       tally.Total,
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

func toStringSlice(v any) []string {
	switch t := v.(type) {
	case []string:
		return t
	case []any:
		out := make([]string, 0, len(t))
		for _, item := range t {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}
