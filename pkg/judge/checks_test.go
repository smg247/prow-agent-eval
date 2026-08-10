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
		name string
		outs Outputs
		want bool
	}{
		{
			name: "agent branch set",
			outs: Outputs{"github": map[string]any{"agent_branch": "feature-x"}},
			want: true,
		},
		{
			name: "main fails",
			outs: Outputs{"github": map[string]any{"agent_branch": "main"}},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _ := checkBranchCreated(tt.outs)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("checkBranchCreated() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestCheckPRExists(t *testing.T) {
	tests := []struct {
		name string
		outs Outputs
		want bool
	}{
		{
			name: "pr number set",
			outs: Outputs{"github": map[string]any{"pr_number": 12}},
			want: true,
		},
		{
			name: "zero pr number",
			outs: Outputs{"github": map[string]any{"pr_number": 0}},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _ := checkPRExists(tt.outs)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("checkPRExists() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestCheckFileOverlap(t *testing.T) {
	tests := []struct {
		name string
		outs Outputs
		want bool
	}{
		{
			name: "identical files",
			outs: Outputs{"github": map[string]any{
				"changed_files":          []string{"a.go", "b.go"},
				"expected_changed_files": []string{"a.go", "b.go"},
			}},
			want: true,
		},
		{
			name: "jaccard about 0.33 passes",
			outs: Outputs{"github": map[string]any{
				"changed_files":          []string{"a.go", "b.go"},
				"expected_changed_files": []string{"a.go", "c.go"},
			}},
			want: true,
		},
		{
			name: "low overlap fails",
			outs: Outputs{"github": map[string]any{
				"changed_files":          []string{"a.go"},
				"expected_changed_files": []string{"b.go", "c.go", "d.go"},
			}},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _ := checkFileOverlap(tt.outs)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("checkFileOverlap() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestCheckExpectedFilesChanged(t *testing.T) {
	tests := []struct {
		name string
		outs Outputs
		want bool
	}{
		{
			name: "expected file present",
			outs: Outputs{
				"annotations": config.CaseAnnotations{
					"expected_files": map[string]any{
						"c1": []any{"pkg/a.go"},
					},
				},
				"github": map[string]any{"changed_files": []string{"pkg/a.go", "pkg/b.go"}},
			},
			want: true,
		},
		{
			name: "expected file missing",
			outs: Outputs{
				"annotations": config.CaseAnnotations{
					"expected_files": map[string]any{
						"c1": []any{"pkg/missing.go"},
					},
				},
				"github": map[string]any{"changed_files": []string{"pkg/a.go"}},
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _ := checkExpectedFilesChanged(tt.outs)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("checkExpectedFilesChanged() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestCheckPRDescriptionExists(t *testing.T) {
	tests := []struct {
		name string
		outs Outputs
		want bool
	}{
		{
			name: "pr body set",
			outs: Outputs{"github": map[string]any{"pr_body": "fixes the bug"}},
			want: true,
		},
		{
			name: "empty pr body",
			outs: Outputs{"github": map[string]any{"pr_body": ""}},
			want: false,
		},
		{
			name: "pr description file flag",
			outs: Outputs{"github": map[string]any{"pr_description_file": true}},
			want: true,
		},
		{
			name: "no description",
			outs: Outputs{"github": map[string]any{}},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _ := checkPRDescriptionExists(tt.outs)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("checkPRDescriptionExists() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestCheckDiffSizeRatio(t *testing.T) {
	tests := []struct {
		name string
		outs Outputs
		want checkOutcome
	}{
		{
			name: "equal sizes",
			outs: Outputs{"github": map[string]any{
				"agent_diff_lines":    50,
				"expected_diff_lines": 50,
			}},
			want: checkOutcome{OK: true, Msg: "Diff size ratio: 1.00"},
		},
		{
			name: "ratio too small",
			outs: Outputs{"github": map[string]any{
				"agent_diff_lines":    1,
				"expected_diff_lines": 100,
			}},
			want: checkOutcome{OK: false},
		},
		{
			name: "no expected diff",
			outs: Outputs{"github": map[string]any{
				"agent_diff_lines": 10,
			}},
			want: checkOutcome{OK: true, Msg: "N/A (no expected diff)"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ok, msg := checkDiffSizeRatio(tt.outs)
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
		name string
		outs Outputs
		want bool
	}{
		{
			name: "identical diffs",
			outs: Outputs{"github": map[string]any{
				"full_diff":          diff,
				"expected_full_diff": diff,
			}},
			want: true,
		},
		{
			name: "zero overlap",
			outs: Outputs{"github": map[string]any{
				"full_diff":          `@@ -10,3 +10,5 @@ func Foo` + "\n+code",
				"expected_full_diff": `@@ -10,3 +10,5 @@ func Bar` + "\n+code",
			}},
			want: false,
		},
		{
			name: "no expected diff",
			outs: Outputs{"github": map[string]any{
				"full_diff":          "just a change",
				"expected_full_diff": "",
			}},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _ := checkFunctionOverlap(tt.outs)
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
		name string
		outs Outputs
		key  string
		want checkOutcome
	}{
		{
			name: "passed cleans message",
			outs: Outputs{"build_result": map[string]any{
				"passed": true,
				"output": "go: writing stat cache: permission denied\nok",
			}},
			key:  "build_result",
			want: checkOutcome{OK: true, Msg: "passed"},
		},
		{
			name: "failed includes error",
			outs: Outputs{"build_result": map[string]any{
				"passed": false,
				"error":  "exit status 1",
				"output": "compilation error on line 42",
			}},
			key:  "build_result",
			want: checkOutcome{OK: false, Msg: "failed: exit status 1"},
		},
		{
			name: "missing result",
			outs: Outputs{},
			key:  "build_result",
			want: checkOutcome{OK: false, Msg: "build_result not collected"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ok, msg := checkMakeResult(tt.outs, tt.key)
			got := checkOutcome{OK: ok, Msg: msg}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("checkMakeResult() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestCheckNoSecrets(t *testing.T) {
	tests := []struct {
		name string
		outs Outputs
		want bool
	}{
		{
			name: "clean content",
			outs: Outputs{"github": map[string]any{
				"bot_replies": []map[string]any{{"body": "looks fine"}},
				"full_diff":   "diff --git a/x",
			}},
			want: true,
		},
		{
			name: "github token",
			outs: Outputs{"github": map[string]any{
				"full_diff": "token ghp_abcdefghijklmnopqrstuv",
			}},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _ := checkNoSecrets(tt.outs)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("checkNoSecrets() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
