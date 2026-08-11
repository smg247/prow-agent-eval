package report

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/smg247/prow-agent-eval/pkg/judge"
)

func containsMap(html string, needles []string) map[string]bool {
	got := make(map[string]bool, len(needles))
	for _, n := range needles {
		got[n] = strings.Contains(html, n)
	}
	return got
}

func wantContains(needles []string) map[string]bool {
	want := make(map[string]bool, len(needles))
	for _, n := range needles {
		want[n] = true
	}
	return want
}

func TestWriteHTML(t *testing.T) {
	tests := []struct {
		name         string
		evalName     string
		caseResults  map[string][]judge.Result
		thresholds   []judge.ThresholdResult
		caseEvidence map[string]judge.CaseEvidence
		wantNeedles  []string
	}{
		{
			name:     "basic summary",
			evalName: "test-eval",
			caseResults: map[string][]judge.Result{
				"case-001": {
					{Name: "check_files", Passed: true, Message: "Jaccard overlap: 0.80"},
					{Name: "no_secrets", Passed: false, Message: "found pattern"},
				},
			},
			thresholds: []judge.ThresholdResult{
				{Name: "check_files/pass_rate", Met: true, Actual: 1.0, Required: 1.0},
			},
			wantNeedles: []string{
				"test-eval",
				"check_files",
				"PASS",
				"FAIL",
				"MET",
				"case-001",
				"<details",
			},
		},
		{
			name:     "with links",
			evalName: "test-eval",
			caseResults: map[string][]judge.Result{
				"case-001": {
					{Name: "pr_exists", Passed: true, Message: "PR #42"},
					{Name: "file_overlap", Passed: true, Message: "Jaccard overlap: 0.80"},
				},
			},
			caseEvidence: map[string]judge.CaseEvidence{
				"case-001": {
					GitHub: judge.GitHubData{
						Repo:           "myorg/myrepo",
						PRNumber:       42,
						BaseBranch:     "main",
						ExpectedBranch: "expected-fix",
						AgentBranch:    "claude/fix-123",
					},
				},
			},
			wantNeedles: []string{
				"https://github.com/myorg/myrepo/pull/42",
				"https://github.com/myorg/myrepo/compare/main...expected-fix",
				"https://github.com/myorg/myrepo/compare/main...claude/fix-123",
				"expected diff",
				"agent diff",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			var err error
			if tt.caseEvidence != nil {
				err = WriteHTML(dir, tt.evalName, tt.caseResults, tt.thresholds, tt.caseEvidence)
			} else {
				err = WriteHTML(dir, tt.evalName, tt.caseResults, tt.thresholds)
			}
			if err != nil {
				t.Fatalf("WriteHTML: %v", err)
			}
			data, err := os.ReadFile(filepath.Join(dir, "eval-summary.html"))
			if err != nil {
				t.Fatal(err)
			}
			html := string(data)
			if diff := cmp.Diff(wantContains(tt.wantNeedles), containsMap(html, tt.wantNeedles)); diff != "" {
				t.Errorf("HTML contents mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
