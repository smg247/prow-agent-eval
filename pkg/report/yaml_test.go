package report

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/smg247/prow-agent-eval/pkg/judge"
	"gopkg.in/yaml.v3"
)

func TestWriteCaseYAML(t *testing.T) {
	tests := []struct {
		name     string
		caseName string
		results  []judge.Result
		outputs  judge.Outputs
		want     CaseReport
	}{
		{
			name:     "checks and scores",
			caseName: "case-001",
			results: []judge.Result{
				{Name: "check_files", Passed: true, Message: "Jaccard overlap: 0.75"},
				{Name: "no_secrets", Passed: false, Message: "found secret"},
			},
			outputs: judge.Outputs{
				"github": map[string]any{
					"agent_branch":  "fix-branch",
					"changed_files": []string{"a.go", "b.go"},
					"pr_number":     42,
				},
			},
			want: CaseReport{
				Case:         "case-001",
				ClaudeBranch: "fix-branch",
				PRNumber:     "42",
				Checks: map[string]string{
					"check_files": "pass",
					"no_secrets":  "fail",
					"passed":      "1",
					"total":       "2",
				},
				Scores: map[string]string{
					"check_files": "0.75",
				},
				ClaudeFilesChanged: []string{"a.go", "b.go"},
			},
		},
		{
			name:     "with urls",
			caseName: "case-001",
			results: []judge.Result{
				{Name: "pr_exists", Passed: true, Message: "PR #42"},
			},
			outputs: judge.Outputs{
				"github": map[string]any{
					"repo":            "myorg/myrepo",
					"pr_number":       42,
					"agent_branch":    "fix-branch",
					"base_branch":     "main",
					"expected_branch": "expected-fix",
				},
			},
			want: CaseReport{
				Case:           "case-001",
				ClaudeBranch:   "fix-branch",
				BaseBranch:     "main",
				ExpectedBranch: "expected-fix",
				PRNumber:       "42",
				PRURL:          "https://github.com/myorg/myrepo/pull/42",
				DiffURL:        "https://github.com/myorg/myrepo/compare/main...expected-fix",
				Checks: map[string]string{
					"pr_exists": "pass",
					"passed":    "1",
					"total":     "1",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := WriteCaseYAML(dir, tt.caseName, tt.results, tt.outputs); err != nil {
				t.Fatalf("WriteCaseYAML: %v", err)
			}
			data, err := os.ReadFile(filepath.Join(dir, "eval-"+tt.caseName+".yaml"))
			if err != nil {
				t.Fatal(err)
			}
			var got CaseReport
			if err := yaml.Unmarshal(data, &got); err != nil {
				t.Fatalf("parsing YAML: %v", err)
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("CaseReport mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestWriteSummaryYAML(t *testing.T) {
	tests := []struct {
		name       string
		evalName   string
		casesRun   int
		results    []judge.Result
		thresholds []judge.ThresholdResult
		want       SummaryReport
	}{
		{
			name:     "summary counts",
			evalName: "test-eval",
			casesRun: 2,
			results: []judge.Result{
				{Name: "a", Passed: true, Message: "ok"},
				{Name: "b", Passed: false, Message: "fail"},
				{Name: "c", Error: "boom"},
			},
			thresholds: []judge.ThresholdResult{
				{Name: "a/pass_rate", Met: true, Actual: 1.0, Required: 1.0},
			},
			want: SummaryReport{
				CasesRun:          2,
				TotalChecksPassed: 1,
				TotalChecks:       3,
				Thresholds: []ThresholdReport{
					{Name: "a/pass_rate", Met: true, Actual: 1.0, Required: 1.0},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := WriteSummaryYAML(dir, tt.evalName, tt.casesRun, tt.results, tt.thresholds); err != nil {
				t.Fatalf("WriteSummaryYAML: %v", err)
			}
			data, err := os.ReadFile(filepath.Join(dir, "eval-summary.yaml"))
			if err != nil {
				t.Fatal(err)
			}
			var got SummaryReport
			if err := yaml.Unmarshal(data, &got); err != nil {
				t.Fatalf("parsing YAML: %v", err)
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("SummaryReport mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
