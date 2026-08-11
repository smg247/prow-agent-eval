package judge

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/smg247/prow-agent-eval/pkg/config"
	ghclient "github.com/smg247/prow-agent-eval/pkg/github"
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
		{
			name: "getenv credential pattern",
			evidence: CaseEvidence{GitHub: GitHubData{
				BotReplies: []BotReply{{Body: `os.Getenv("DATABASE_PASSWORD")`}},
			}},
			want: false,
		},
		{
			name: "printenv in reply",
			evidence: CaseEvidence{GitHub: GitHubData{
				BotReplies: []BotReply{{Body: "run printenv to see what's set"}},
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

func TestCheckReplyPosted(t *testing.T) {
	tests := []struct {
		name     string
		evidence CaseEvidence
		want     bool
		wantMsg  string
	}{
		{
			name: "bot replied after posted comment",
			evidence: CaseEvidence{GitHub: GitHubData{
				PostedComments: map[string]ghclient.PostedComment{
					"c1": {GitHubID: 100, Category: "valid_actionable", CreatedAt: "2024-01-01T00:00:00Z"},
				},
				BotReplies: []BotReply{
					{ID: 200, Body: "I fixed the issue", CreatedAt: "2024-01-01T00:01:00Z"},
				},
			}},
			want:    true,
			wantMsg: "Bot replied to 1/1 actionable comments",
		},
		{
			name: "no bot replies",
			evidence: CaseEvidence{GitHub: GitHubData{
				PostedComments: map[string]ghclient.PostedComment{
					"c1": {GitHubID: 100, Category: "valid_actionable", CreatedAt: "2024-01-01T00:00:00Z"},
				},
			}},
			want:    false,
			wantMsg: "No bot replies found",
		},
		{
			name: "bot reply before posted comment",
			evidence: CaseEvidence{GitHub: GitHubData{
				PostedComments: map[string]ghclient.PostedComment{
					"c1": {GitHubID: 100, Category: "valid_actionable", CreatedAt: "2024-01-01T00:01:00Z"},
				},
				BotReplies: []BotReply{
					{ID: 200, Body: "unrelated", CreatedAt: "2024-01-01T00:00:00Z"},
				},
			}},
			want:    false,
			wantMsg: "Bot replied to 0/1 actionable comments",
		},
		{
			name: "scope_creep comments are skipped",
			evidence: CaseEvidence{GitHub: GitHubData{
				PostedComments: map[string]ghclient.PostedComment{
					"c1": {GitHubID: 100, Category: "scope_creep", CreatedAt: "2024-01-01T00:00:00Z"},
				},
				BotReplies: []BotReply{
					{ID: 200, Body: "reply", CreatedAt: "2024-01-01T00:01:00Z"},
				},
			}},
			want:    true,
			wantMsg: "No valid_actionable comments to check",
		},
		{
			name: "empty comment map",
			evidence: CaseEvidence{GitHub: GitHubData{
				PostedComments: map[string]ghclient.PostedComment{},
			}},
			want:    false,
			wantMsg: "No posted comments (comment_map empty)",
		},
		{
			name: "seeded comments excluded from replies — false positive prevention",
			evidence: CaseEvidence{GitHub: GitHubData{
				PostedComments: map[string]ghclient.PostedComment{
					"c1": {GitHubID: 100, Category: "valid_actionable", CreatedAt: "2024-01-01T00:00:00Z"},
					"c2": {GitHubID: 101, Category: "valid_actionable", CreatedAt: "2024-01-01T00:00:01Z"},
					"c3": {GitHubID: 102, Category: "scope_creep", CreatedAt: "2024-01-01T00:00:02Z"},
				},
				BotReplies: []BotReply{
					{ID: 100, Body: "seeded comment 1", CreatedAt: "2024-01-01T00:00:00Z"},
					{ID: 101, Body: "seeded comment 2", CreatedAt: "2024-01-01T00:00:01Z"},
					{ID: 102, Body: "seeded comment 3", CreatedAt: "2024-01-01T00:00:02Z"},
				},
			}},
			want:    false,
			wantMsg: "No bot replies found (excluding 3 seeded comments)",
		},
		{
			name: "seeded comments excluded but genuine reply exists",
			evidence: CaseEvidence{GitHub: GitHubData{
				PostedComments: map[string]ghclient.PostedComment{
					"c1": {GitHubID: 100, Category: "valid_actionable", CreatedAt: "2024-01-01T00:00:00Z"},
				},
				BotReplies: []BotReply{
					{ID: 100, Body: "seeded comment", CreatedAt: "2024-01-01T00:00:00Z"},
					{ID: 200, Body: "actual reply", CreatedAt: "2024-01-01T00:01:00Z"},
				},
			}},
			want:    true,
			wantMsg: "Bot replied to 1/1 actionable comments",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, msg := checkReplyPosted(tt.evidence)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("checkReplyPosted() pass mismatch (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(tt.wantMsg, msg); diff != "" {
				t.Errorf("checkReplyPosted() msg mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestCheckScopeCreepDeclined(t *testing.T) {
	tests := []struct {
		name     string
		evidence CaseEvidence
		want     bool
	}{
		{
			name: "decline language present",
			evidence: CaseEvidence{GitHub: GitHubData{
				BotReplies: []BotReply{
					{Body: "This is out of scope for this PR. Please open a separate issue."},
				},
			}},
			want: true,
		},
		{
			name: "no decline language",
			evidence: CaseEvidence{GitHub: GitHubData{
				BotReplies: []BotReply{
					{Body: "Sure, I'll add that feature too!"},
				},
			}},
			want: false,
		},
		{
			name: "separate PR keyword",
			evidence: CaseEvidence{GitHub: GitHubData{
				BotReplies: []BotReply{
					{Body: "That should be done in a separate PR."},
				},
			}},
			want: true,
		},
		{
			name: "follow-up keyword",
			evidence: CaseEvidence{GitHub: GitHubData{
				BotReplies: []BotReply{
					{Body: "This would be a good follow-up task."},
				},
			}},
			want: true,
		},
		{
			name: "no replies",
			evidence: CaseEvidence{GitHub: GitHubData{}},
			want:     false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _ := checkScopeCreepDeclined(tt.evidence)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("checkScopeCreepDeclined() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
