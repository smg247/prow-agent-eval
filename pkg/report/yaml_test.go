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
		{Name: "check_files", Passed: true, Message: "ok"},
		{Name: "no_secrets", Passed: false, Message: "found secret"},
	}

	if err := WriteCaseYAML(dir, "case-001", results); err != nil {
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
	if len(report.Results) != 2 {
		t.Fatalf("len(Results) = %d, want 2", len(report.Results))
	}
	if !report.Results[0].Passed {
		t.Error("Results[0].Passed = false, want true")
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

	if summary.TotalCases != 2 {
		t.Errorf("TotalCases = %d, want 2", summary.TotalCases)
	}
	if summary.Passed != 1 {
		t.Errorf("Passed = %d, want 1", summary.Passed)
	}
	if summary.Failed != 1 {
		t.Errorf("Failed = %d, want 1", summary.Failed)
	}
	if summary.Errors != 1 {
		t.Errorf("Errors = %d, want 1", summary.Errors)
	}
}
