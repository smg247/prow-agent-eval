package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad(t *testing.T) {
	yaml := `
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
`
	dir := t.TempDir()
	path := filepath.Join(dir, "eval.yaml")
	if err := os.WriteFile(path, []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.Name != "test-eval" {
		t.Errorf("Name = %q, want %q", cfg.Name, "test-eval")
	}
	if cfg.Init.Repo != "owner/repo" {
		t.Errorf("Init.Repo = %q, want %q", cfg.Init.Repo, "owner/repo")
	}
	if !cfg.Collect.BotReplies {
		t.Error("Collect.BotReplies = false, want true")
	}
	if cfg.Collect.BuildResult {
		t.Error("Collect.BuildResult = true, want false")
	}
	if len(cfg.Judges) != 1 {
		t.Fatalf("len(Judges) = %d, want 1", len(cfg.Judges))
	}
	if cfg.Judges[0].Name != "build_passed" {
		t.Errorf("Judges[0].Name = %q, want %q", cfg.Judges[0].Name, "build_passed")
	}
	if cfg.Judges[0].Type != "build_passed" {
		t.Errorf("Judges[0].Type = %q, want %q", cfg.Judges[0].Type, "build_passed")
	}

	thresh, ok := cfg.Thresholds["build_passed"]
	if !ok {
		t.Fatal("missing threshold for build_passed")
	}
	if thresh.MinPassRate == nil || *thresh.MinPassRate != 1.0 {
		t.Errorf("build_passed threshold = %v, want 1.0", thresh.MinPassRate)
	}
}

func TestLoadCase(t *testing.T) {
	dir := t.TempDir()
	caseDir := filepath.Join(dir, "cases", "test", "case-001")
	if err := os.MkdirAll(caseDir, 0755); err != nil {
		t.Fatal(err)
	}

	inputYAML := `jira_key: TRT-1234
base_branch: main
head_branch: fix-trt-1234
expected_branch: fix-trt-1234-expected
`
	if err := os.WriteFile(filepath.Join(caseDir, "input.yaml"), []byte(inputYAML), 0644); err != nil {
		t.Fatal(err)
	}

	annotationsYAML := `expected_files:
  comment-001:
    - pkg/api/foo.go
difficulty: easy
`
	if err := os.WriteFile(filepath.Join(caseDir, "annotations.yaml"), []byte(annotationsYAML), 0644); err != nil {
		t.Fatal(err)
	}

	c, err := LoadCase(dir, "cases/test", "case-001")
	if err != nil {
		t.Fatalf("LoadCase: %v", err)
	}

	if c.Name != "case-001" {
		t.Errorf("Name = %q, want %q", c.Name, "case-001")
	}
	if c.Input.JiraKey != "TRT-1234" {
		t.Errorf("JiraKey = %q, want %q", c.Input.JiraKey, "TRT-1234")
	}
	if c.Input.BaseBranch != "main" {
		t.Errorf("BaseBranch = %q, want %q", c.Input.BaseBranch, "main")
	}
	if c.Input.HeadBranch != "fix-trt-1234" {
		t.Errorf("HeadBranch = %q, want %q", c.Input.HeadBranch, "fix-trt-1234")
	}
	if c.Input.ExpectedBranch != "fix-trt-1234-expected" {
		t.Errorf("ExpectedBranch = %q, want %q", c.Input.ExpectedBranch, "fix-trt-1234-expected")
	}
	if c.Annotations == nil {
		t.Fatal("Annotations is nil")
	}
	if _, ok := c.Annotations["expected_files"]; !ok {
		t.Error("Annotations missing expected_files")
	}
}

func TestLoadCaseMissingAnnotationsOK(t *testing.T) {
	dir := t.TempDir()
	caseDir := filepath.Join(dir, "cases", "test", "case-002")
	if err := os.MkdirAll(caseDir, 0755); err != nil {
		t.Fatal(err)
	}
	inputYAML := `jira_key: TRT-1
base_branch: main
head_branch: head
`
	if err := os.WriteFile(filepath.Join(caseDir, "input.yaml"), []byte(inputYAML), 0644); err != nil {
		t.Fatal(err)
	}
	c, err := LoadCase(dir, "cases/test", "case-002")
	if err != nil {
		t.Fatalf("LoadCase: %v", err)
	}
	if c.Annotations != nil && len(c.Annotations) != 0 {
		t.Errorf("expected empty annotations, got %#v", c.Annotations)
	}
}

func TestListCases(t *testing.T) {
	dir := t.TempDir()
	casesDir := filepath.Join(dir, "cases")

	for _, name := range []string{"case-001", "case-002", "not-a-case"} {
		d := filepath.Join(casesDir, name)
		os.MkdirAll(d, 0755)
		if name != "not-a-case" {
			os.WriteFile(filepath.Join(d, "input.yaml"), []byte("jira_key: X"), 0644)
		}
	}

	cases, err := ListCases(dir, "cases")
	if err != nil {
		t.Fatalf("ListCases: %v", err)
	}
	if len(cases) != 2 {
		t.Errorf("len(cases) = %d, want 2", len(cases))
	}
}
