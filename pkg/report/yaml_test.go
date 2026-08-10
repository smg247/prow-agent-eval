package report

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/smg247/prow-agent-eval/pkg/judge"
	"gopkg.in/yaml.v3"
)

func TestWriteCaseYAML(t *testing.T) {
	dir := t.TempDir()

	results := []judge.Result{
		{Name: "check_files", Passed: true, Message: "Jaccard overlap: 0.75"},
		{Name: "no_secrets", Passed: false, Message: "found secret"},
	}
	outputs := judge.Outputs{
		"github": map[string]any{
			"agent_branch":  "fix-branch",
			"changed_files": []string{"a.go", "b.go"},
			"pr_number":     42,
		},
	}

	if err := WriteCaseYAML(dir, "case-001", results, outputs); err != nil {
		t.Fatalf("WriteCaseYAML: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "eval-case-001.yaml"))
	if err != nil {
		t.Fatal(err)
	}

	var report CaseReport
	if err := yaml.Unmarshal(data, &report); err != nil {
		t.Fatalf("parsing YAML: %v", err)
	}

	if report.Case != "case-001" {
		t.Errorf("Case = %q, want %q", report.Case, "case-001")
	}
	if report.Checks["check_files"] != "pass" {
		t.Errorf("Checks[check_files] = %q, want pass", report.Checks["check_files"])
	}
	if report.Checks["no_secrets"] != "fail" {
		t.Errorf("Checks[no_secrets] = %q, want fail", report.Checks["no_secrets"])
	}
	if report.Scores["check_files"] != "0.75" {
		t.Errorf("Scores[check_files] = %q, want 0.75", report.Scores["check_files"])
	}
	if report.ClaudeBranch != "fix-branch" {
		t.Errorf("ClaudeBranch = %q, want fix-branch", report.ClaudeBranch)
	}
}

func TestWriteCaseYAMLWithURLs(t *testing.T) {
	dir := t.TempDir()

	results := []judge.Result{
		{Name: "pr_exists", Passed: true, Message: "PR #42"},
	}
	outputs := judge.Outputs{
		"github": map[string]any{
			"repo":            "myorg/myrepo",
			"pr_number":       42,
			"agent_branch":    "fix-branch",
			"base_branch":     "main",
			"expected_branch": "expected-fix",
		},
	}

	if err := WriteCaseYAML(dir, "case-001", results, outputs); err != nil {
		t.Fatalf("WriteCaseYAML: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "eval-case-001.yaml"))
	if err != nil {
		t.Fatal(err)
	}

	var report CaseReport
	if err := yaml.Unmarshal(data, &report); err != nil {
		t.Fatalf("parsing YAML: %v", err)
	}

	if report.PRURL != "https://github.com/myorg/myrepo/pull/42" {
		t.Errorf("PRURL = %q, want GitHub PR URL", report.PRURL)
	}
	if report.DiffURL != "https://github.com/myorg/myrepo/compare/main...expected-fix" {
		t.Errorf("DiffURL = %q, want GitHub compare URL", report.DiffURL)
	}
	if report.BaseBranch != "main" {
		t.Errorf("BaseBranch = %q, want main", report.BaseBranch)
	}
	if report.ExpectedBranch != "expected-fix" {
		t.Errorf("ExpectedBranch = %q, want expected-fix", report.ExpectedBranch)
	}
}

func TestWriteSummaryYAML(t *testing.T) {
	dir := t.TempDir()

	results := []judge.Result{
		{Name: "a", Passed: true, Message: "ok"},
		{Name: "b", Passed: false, Message: "fail"},
		{Name: "c", Error: "boom"},
	}

	thresholds := []judge.ThresholdResult{
		{Name: "a/pass_rate", Met: true, Actual: 1.0, Required: 1.0},
	}

	if err := WriteSummaryYAML(dir, "test-eval", 2, results, thresholds); err != nil {
		t.Fatalf("WriteSummaryYAML: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "eval-summary.yaml"))
	if err != nil {
		t.Fatal(err)
	}

	var summary SummaryReport
	if err := yaml.Unmarshal(data, &summary); err != nil {
		t.Fatalf("parsing YAML: %v", err)
	}

	if summary.CasesRun != 2 {
		t.Errorf("CasesRun = %d, want 2", summary.CasesRun)
	}
	if summary.TotalChecksPassed != 1 {
		t.Errorf("TotalChecksPassed = %d, want 1", summary.TotalChecksPassed)
	}
	if summary.TotalChecks != 3 {
		t.Errorf("TotalChecks = %d, want 3", summary.TotalChecks)
	}
}
