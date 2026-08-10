package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func TestLoad(t *testing.T) {
	rate := 1.0
	tests := []struct {
		name string
		yaml string
		want *EvalConfig
	}{
		{
			name: "full config",
			yaml: `
name: test-eval
description: Test evaluation
init:
  repo: owner/repo
dataset:
  path: cases/test
collect:
  bot_replies: true
  comment_map: true
  build_result: false
  test_result: false
  expected_branch_diff: false
judges:
  - name: build_passed
    description: Code compiles
    type: build_passed
thresholds:
  build_passed:
    min_pass_rate: 1.0
`,
			want: &EvalConfig{
				Name:        "test-eval",
				Description: "Test evaluation",
				Init:        InitConfig{Repo: "owner/repo"},
				Dataset:     DatasetConfig{Path: "cases/test"},
				Collect: CollectConfig{
					BotReplies: true,
					CommentMap: true,
				},
				Judges: []JudgeConfig{
					{Name: "build_passed", Description: "Code compiles", Type: "build_passed"},
				},
				Thresholds: map[string]Threshold{
					"build_passed": {MinPassRate: &rate},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "eval.yaml")
			if err := os.WriteFile(path, []byte(tt.yaml), 0o600); err != nil {
				t.Fatal(err)
			}
			got, err := Load(path)
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("Load() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestLoadCase(t *testing.T) {
	tests := []struct {
		name            string
		caseName        string
		inputYAML       string
		annotationsYAML string
		want            Case
	}{
		{
			name:     "with annotations",
			caseName: "case-001",
			inputYAML: `jira_key: TRT-1234
base_branch: main
head_branch: fix-trt-1234
expected_branch: fix-trt-1234-expected
`,
			annotationsYAML: `expected_files:
  comment-001:
    - pkg/api/foo.go
difficulty: easy
`,
			want: Case{
				Name: "case-001",
				Input: CaseInput{
					JiraKey:        "TRT-1234",
					BaseBranch:     "main",
					HeadBranch:     "fix-trt-1234",
					ExpectedBranch: "fix-trt-1234-expected",
				},
				Annotations: CaseAnnotations{
					"expected_files": CaseAnnotations{
						"comment-001": []any{"pkg/api/foo.go"},
					},
					"difficulty": "easy",
				},
			},
		},
		{
			name:     "missing annotations ok",
			caseName: "case-002",
			inputYAML: `jira_key: TRT-1
base_branch: main
head_branch: head
`,
			want: Case{
				Name: "case-002",
				Input: CaseInput{
					JiraKey:    "TRT-1",
					BaseBranch: "main",
					HeadBranch: "head",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			caseDir := filepath.Join(dir, "cases", "test", tt.caseName)
			if err := os.MkdirAll(caseDir, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(caseDir, "input.yaml"), []byte(tt.inputYAML), 0o600); err != nil {
				t.Fatal(err)
			}
			if tt.annotationsYAML != "" {
				if err := os.WriteFile(filepath.Join(caseDir, "annotations.yaml"), []byte(tt.annotationsYAML), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			got, err := LoadCase(dir, "cases/test", tt.caseName)
			if err != nil {
				t.Fatalf("LoadCase: %v", err)
			}
			want := tt.want
			want.Dir = caseDir
			if diff := cmp.Diff(&want, got); diff != "" {
				t.Errorf("LoadCase() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestListCases(t *testing.T) {
	tests := []struct {
		name      string
		dirs      []string
		withInput []string
		want      []string
	}{
		{
			name:      "only dirs with input.yaml",
			dirs:      []string{"case-001", "case-002", "not-a-case"},
			withInput: []string{"case-001", "case-002"},
			want:      []string{"case-001", "case-002"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			casesDir := filepath.Join(dir, "cases")
			inputSet := map[string]bool{}
			for _, name := range tt.withInput {
				inputSet[name] = true
			}
			for _, name := range tt.dirs {
				d := filepath.Join(casesDir, name)
				if err := os.MkdirAll(d, 0o755); err != nil {
					t.Fatal(err)
				}
				if inputSet[name] {
					if err := os.WriteFile(filepath.Join(d, "input.yaml"), []byte("jira_key: X"), 0o600); err != nil {
						t.Fatal(err)
					}
				}
			}
			got, err := ListCases(dir, "cases")
			if err != nil {
				t.Fatalf("ListCases: %v", err)
			}
			if diff := cmp.Diff(tt.want, got, cmpopts.SortSlices(func(a, b string) bool { return a < b })); diff != "" {
				t.Errorf("ListCases() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
