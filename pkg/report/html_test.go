package report

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/smg247/prow-agent-eval/pkg/judge"
)

func TestWriteHTML(t *testing.T) {
	dir := t.TempDir()

	caseResults := map[string][]judge.Result{
		"case-001": {
			{Name: "check_files", Passed: true, Message: "Jaccard overlap: 0.80"},
			{Name: "no_secrets", Passed: false, Message: "found pattern"},
		},
	}

	thresholds := []judge.ThresholdResult{
		{Name: "check_files/pass_rate", Met: true, Actual: 1.0, Required: 1.0},
	}

	if err := WriteHTML(dir, "test-eval", caseResults, thresholds); err != nil {
		t.Fatalf("WriteHTML: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "eval-summary.html"))
	if err != nil {
		t.Fatal(err)
	}

	html := string(data)
	if !strings.Contains(html, "test-eval") {
		t.Error("HTML missing eval name")
	}
	if !strings.Contains(html, "check_files") {
		t.Error("HTML missing judge name")
	}
	if !strings.Contains(html, "PASS") {
		t.Error("HTML missing PASS status")
	}
	if !strings.Contains(html, "FAIL") {
		t.Error("HTML missing FAIL status")
	}
	if !strings.Contains(html, "MET") {
		t.Error("HTML missing threshold status")
	}
	if !strings.Contains(html, "case-001") {
		t.Error("HTML missing case name")
	}
	if !strings.Contains(html, "<details") {
		t.Error("HTML missing expandable details")
	}
}

func TestWriteHTMLWithLinks(t *testing.T) {
	dir := t.TempDir()

	caseResults := map[string][]judge.Result{
		"case-001": {
			{Name: "pr_exists", Passed: true, Message: "PR #42"},
			{Name: "file_overlap", Passed: true, Message: "Jaccard overlap: 0.80"},
		},
	}

	caseOutputs := map[string]judge.Outputs{
		"case-001": {
			"github": map[string]any{
				"repo":            "myorg/myrepo",
				"pr_number":       42,
				"base_branch":     "main",
				"expected_branch": "expected-fix",
				"agent_branch":    "claude/fix-123",
			},
		},
	}

	if err := WriteHTML(dir, "test-eval", caseResults, nil, caseOutputs); err != nil {
		t.Fatalf("WriteHTML: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "eval-summary.html"))
	if err != nil {
		t.Fatal(err)
	}

	html := string(data)
	if !strings.Contains(html, "https://github.com/myorg/myrepo/pull/42") {
		t.Error("HTML missing PR link")
	}
	if !strings.Contains(html, "https://github.com/myorg/myrepo/compare/main...expected-fix") {
		t.Error("HTML missing expected diff link")
	}
	if !strings.Contains(html, "https://github.com/myorg/myrepo/compare/main...claude/fix-123") {
		t.Error("HTML missing agent diff link")
	}
	if !strings.Contains(html, "expected diff") {
		t.Error("HTML missing 'expected diff' link text")
	}
	if !strings.Contains(html, "agent diff") {
		t.Error("HTML missing 'agent diff' link text")
	}
}
