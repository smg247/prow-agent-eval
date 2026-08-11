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

func WriteCaseYAML(dir, caseName string, results []judge.Result, evidence judge.CaseEvidence) error {
	report := CaseReport{
		Case:   caseName,
		Checks: make(map[string]string),
	}

	gh := evidence.GitHub
	report.ClaudeBranch = gh.AgentBranch
	if report.ClaudeBranch == "" {
		report.ClaudeBranch = "none"
	}
	report.ClaudeFilesChanged = gh.ChangedFiles
	report.ExpectedFilesChanged = gh.ExpectedChangedFiles
	report.ClaudeDiffLines = gh.AgentDiffLines
	report.ExpectedDiffLines = gh.ExpectedDiffLines
	report.BaseBranch = gh.BaseBranch
	report.ExpectedBranch = gh.ExpectedBranch
	if gh.PRNumber > 0 {
		report.PRNumber = strconv.Itoa(gh.PRNumber)
		if gh.Repo != "" {
			report.PRURL = fmt.Sprintf("https://github.com/%s/pull/%d", gh.Repo, gh.PRNumber)
		}
	} else {
		report.PRNumber = "none"
	}
	if gh.Repo != "" && gh.BaseBranch != "" && gh.ExpectedBranch != "" {
		report.DiffURL = fmt.Sprintf("https://github.com/%s/compare/%s...%s", gh.Repo, gh.BaseBranch, gh.ExpectedBranch)
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
	return os.WriteFile(path, data, 0o600)
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
	return os.WriteFile(path, data, 0o600)
}
