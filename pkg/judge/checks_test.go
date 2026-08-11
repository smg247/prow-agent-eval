package judge

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/smg247/prow-agent-eval/pkg/config"
)

type checkOutcome struct {
	OK  bool
	Msg string
}

func TestCheckBranchCreated(t *testing.T) {
	tests := []struct {
		name     string
		evidence CaseEvidence
		want     bool
	}{
		{
			name:     "agent branch set",
			evidence: CaseEvidence{GitHub: GitHubData{AgentBranch: "feature-x"}},
			want:     true,
		},
		{
			name:     "main fails",
			evidence: CaseEvidence{GitHub: GitHubData{AgentBranch: "main"}},
			want:     false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _ := checkBranchCreated(tt.evidence)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("checkBranchCreated() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestCheckPRExists(t *testing.T) {
	tests := []struct {
		name     string
		evidence CaseEvidence
		want     bool
	}{
		{
			name:     "pr number set",
			evidence: CaseEvidence{GitHub: GitHubData{PRNumber: 12}},
			want:     true,
		},
		{
			name:     "zero pr number",
			evidence: CaseEvidence{GitHub: GitHubData{PRNumber: 0}},
			want:     false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _ := checkPRExists(tt.evidence)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("checkPRExists() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestCheckFileOverlap(t *testing.T) {
	tests := []struct {
		name     string
		evidence CaseEvidence
		want     bool
	}{
		{
			name: "identical files",
			evidence: CaseEvidence{GitHub: GitHubData{
				ChangedFiles:         []string{"a.go", "b.go"},
				ExpectedChangedFiles: []string{"a.go", "b.go"},
			}},
			want: true,
		},
		{
			name: "jaccard about 0.33 passes",
			evidence: CaseEvidence{GitHub: GitHubData{
				ChangedFiles:         []string{"a.go", "b.go"},
				ExpectedChangedFiles: []string{"a.go", "c.go"},
			}},
			want: true,
		},
		{
			name: "low overlap fails",
			evidence: CaseEvidence{GitHub: GitHubData{
				ChangedFiles:         []string{"a.go"},
				ExpectedChangedFiles: []string{"b.go", "c.go", "d.go"},
			}},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _ := checkFileOverlap(tt.evidence)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("checkFileOverlap() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestCheckExpectedFilesChanged(t *testing.T) {
	tests := []struct {
		name     string
		evidence CaseEvidence
		want     bool
	}{
		{
			name: "expected file present",
			evidence: CaseEvidence{
				Annotations: config.CaseAnnotations{
					"expected_files": map[string]any{
						"c1": []any{"pkg/a.go"},
					},
				},
				GitHub: GitHubData{ChangedFiles: []string{"pkg/a.go", "pkg/b.go"}},
			},
			want: true,
		},
		{
			name: "expected file missing",
			evidence: CaseEvidence{
				Annotations: config.CaseAnnotations{
					"expected_files": map[string]any{
						"c1": []any{"pkg/missing.go"},
					},
				},
				GitHub: GitHubData{ChangedFiles: []string{"pkg/a.go"}},
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _ := checkExpectedFilesChanged(tt.evidence)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("checkExpectedFilesChanged() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestCheckPRDescriptionExists(t *testing.T) {
	tests := []struct {
		name     string
		evidence CaseEvidence
		want     bool
	}{
		{
			name:     "pr body set",
			evidence: CaseEvidence{GitHub: GitHubData{PRBody: "fixes the bug"}},
			want:     true,
		},
		{
			name:     "empty pr body",
			evidence: CaseEvidence{GitHub: GitHubData{PRBody: ""}},
			want:     false,
		},
		{
			name:     "pr description file flag",
			evidence: CaseEvidence{GitHub: GitHubData{PRDescriptionFile: true}},
			want:     true,
		},
		{
			name:     "no description",
			evidence: CaseEvidence{},
			want:     false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _ := checkPRDescriptionExists(tt.evidence)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("checkPRDescriptionExists() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestCheckDiffSizeRatio(t *testing.T) {
	tests := []struct {
		name     string
		evidence CaseEvidence
		want     checkOutcome
	}{
		{
			name: "equal sizes",
			evidence: CaseEvidence{GitHub: GitHubData{
				AgentDiffLines:    50,
				ExpectedDiffLines: 50,
				HasExpectedDiff:   true,
			}},
			want: checkOutcome{OK: true, Msg: "Diff size ratio: 1.00"},
		},
		{
			name: "ratio too small",
			evidence: CaseEvidence{GitHub: GitHubData{
				AgentDiffLines:    1,
				ExpectedDiffLines: 100,
				HasExpectedDiff:   true,
			}},
			want: checkOutcome{OK: false},
		},
		{
			name: "no expected diff",
			evidence: CaseEvidence{GitHub: GitHubData{
				AgentDiffLines: 10,
			}},
			want: checkOutcome{OK: true, Msg: "N/A (no expected diff)"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ok, msg := checkDiffSizeRatio(tt.evidence)
			got := checkOutcome{OK: ok, Msg: msg}
			opts := []cmp.Option{}
			if tt.want.Msg == "" {
				opts = append(opts, cmp.FilterPath(func(p cmp.Path) bool {
					return p.String() == "Msg"
				}, cmp.Ignore()))
			}
			if diff := cmp.Diff(tt.want, got, opts...); diff != "" {
				t.Errorf("checkDiffSizeRatio() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestCheckFunctionOverlap(t *testing.T) {
	diff := `@@ -10,3 +10,5 @@ func Foo
+some code
@@ -20,3 +20,5 @@ func Bar
+more code`

	tests := []struct {
		name     string
		evidence CaseEvidence
		want     bool
	}{
		{
			name: "identical diffs",
			evidence: CaseEvidence{GitHub: GitHubData{
				FullDiff:         diff,
				ExpectedFullDiff: diff,
				HasExpectedDiff:  true,
			}},
			want: true,
		},
		{
			name: "zero overlap",
			evidence: CaseEvidence{GitHub: GitHubData{
				FullDiff:         `@@ -10,3 +10,5 @@ func Foo` + "\n+code",
				ExpectedFullDiff: `@@ -10,3 +10,5 @@ func Bar` + "\n+code",
				HasExpectedDiff:  true,
			}},
			want: false,
		},
		{
			name: "no expected diff",
			evidence: CaseEvidence{GitHub: GitHubData{
				FullDiff: "just a change",
			}},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _ := checkFunctionOverlap(tt.evidence)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("checkFunctionOverlap() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestExtractFunctions(t *testing.T) {
	tests := []struct {
		name string
		diff string
		want map[string]bool
	}{
		{
			name: "named functions",
			diff: `@@ -10,3 +10,5 @@ func Foo
+line
@@ -20,3 +20,5 @@ func (r *Receiver) Bar
+line
@@ -30,3 +30,5 @@ something else
+line`,
			want: map[string]bool{"Foo": true, "Bar": true},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractFunctions(tt.diff)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("extractFunctions() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestCheckMakeResult(t *testing.T) {
	tests := []struct {
		name   string
		result MakeResult
		label  string
		want   checkOutcome
	}{
		{
			name: "passed cleans message",
			result: MakeResult{
				Collected: true,
				Passed:    true,
				Output:    "go: writing stat cache: permission denied\nok",
			},
			label: "build_result",
			want:  checkOutcome{OK: true, Msg: "passed"},
		},
		{
			name: "failed includes error",
			result: MakeResult{
				Collected: true,
				Passed:    false,
				Error:     "exit status 1",
				Output:    "compilation error on line 42",
			},
			label: "build_result",
			want:  checkOutcome{OK: false, Msg: "failed: exit status 1"},
		},
		{
			name:   "missing result",
			result: MakeResult{},
			label:  "build_result",
			want:   checkOutcome{OK: false, Msg: "build_result not collected"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ok, msg := checkMakeResult(tt.result, tt.label)
			got := checkOutcome{OK: ok, Msg: msg}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("checkMakeResult() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestCheckNoSecrets(t *testing.T) {
	tests := []struct {
		name     string
		evidence CaseEvidence
		want     bool
	}{
		{
			name: "clean content",
			evidence: CaseEvidence{GitHub: GitHubData{
				BotReplies: []BotReply{{Body: "looks fine"}},
				FullDiff:   "diff --git a/x",
			}},
			want: true,
		},
		{
			name: "github token",
			evidence: CaseEvidence{GitHub: GitHubData{
				FullDiff: "token ghp_abcdefghijklmnopqrstuv",
			}},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _ := checkNoSecrets(tt.evidence)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("checkNoSecrets() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
