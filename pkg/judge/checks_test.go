package judge

import (
	"testing"

	"github.com/smg247/prow-agent-eval/pkg/config"
)

func TestCheckBranchCreated(t *testing.T) {
	ok, _ := checkBranchCreated(Outputs{"github": map[string]any{"agent_branch": "feature-x"}})
	if !ok {
		t.Fatal("expected pass")
	}
	ok, _ = checkBranchCreated(Outputs{"github": map[string]any{"agent_branch": "main"}})
	if ok {
		t.Fatal("expected fail on main")
	}
}

func TestCheckPRExists(t *testing.T) {
	ok, _ := checkPRExists(Outputs{"github": map[string]any{"pr_number": 12}})
	if !ok {
		t.Fatal("expected pass")
	}
	ok, _ = checkPRExists(Outputs{"github": map[string]any{"pr_number": 0}})
	if ok {
		t.Fatal("expected fail")
	}
}

func TestCheckFileOverlap(t *testing.T) {
	ok, msg := checkFileOverlap(Outputs{"github": map[string]any{
		"changed_files":          []string{"a.go", "b.go"},
		"expected_changed_files": []string{"a.go", "b.go"},
	}})
	if !ok {
		t.Fatalf("expected pass, got %s", msg)
	}
	// Jaccard 1/3 ≈ 0.33 >= 0.25
	ok, msg = checkFileOverlap(Outputs{"github": map[string]any{
		"changed_files":          []string{"a.go", "b.go"},
		"expected_changed_files": []string{"a.go", "c.go"},
	}})
	if !ok {
		t.Fatalf("expected pass at 0.33: %s", msg)
	}
	ok, _ = checkFileOverlap(Outputs{"github": map[string]any{
		"changed_files":          []string{"a.go"},
		"expected_changed_files": []string{"b.go", "c.go", "d.go"},
	}})
	if ok {
		t.Fatal("expected fail on low overlap")
	}
}

func TestCheckExpectedFilesChanged(t *testing.T) {
	ok, msg := checkExpectedFilesChanged(Outputs{
		"annotations": config.CaseAnnotations{
			"expected_files": map[string]any{
				"c1": []any{"pkg/a.go"},
			},
		},
		"github": map[string]any{"changed_files": []string{"pkg/a.go", "pkg/b.go"}},
	})
	if !ok {
		t.Fatalf("expected pass: %s", msg)
	}

	ok, _ = checkExpectedFilesChanged(Outputs{
		"annotations": config.CaseAnnotations{
			"expected_files": map[string]any{
				"c1": []any{"pkg/missing.go"},
			},
		},
		"github": map[string]any{"changed_files": []string{"pkg/a.go"}},
	})
	if ok {
		t.Fatal("expected fail")
	}
}

func TestCheckPRDescriptionExists(t *testing.T) {
	ok, _ := checkPRDescriptionExists(Outputs{"github": map[string]any{"pr_body": "fixes the bug"}})
	if !ok {
		t.Fatal("expected pass with pr_body")
	}
	ok, _ = checkPRDescriptionExists(Outputs{"github": map[string]any{"pr_body": ""}})
	if ok {
		t.Fatal("expected fail on empty pr_body")
	}
	ok, _ = checkPRDescriptionExists(Outputs{"github": map[string]any{"pr_description_file": true}})
	if !ok {
		t.Fatal("expected pass with pr_description_file")
	}
	ok, _ = checkPRDescriptionExists(Outputs{"github": map[string]any{}})
	if ok {
		t.Fatal("expected fail with no description")
	}
}

func TestCheckDiffSizeRatio(t *testing.T) {
	ok, msg := checkDiffSizeRatio(Outputs{"github": map[string]any{
		"agent_diff_lines":    50,
		"expected_diff_lines": 50,
	}})
	if !ok {
		t.Fatalf("expected pass: %s", msg)
	}
	if msg != "Diff size ratio: 1.00" {
		t.Errorf("unexpected message: %s", msg)
	}

	ok, _ = checkDiffSizeRatio(Outputs{"github": map[string]any{
		"agent_diff_lines":    1,
		"expected_diff_lines": 100,
	}})
	if ok {
		t.Fatal("expected fail on ratio 0.01")
	}

	ok, msg = checkDiffSizeRatio(Outputs{"github": map[string]any{
		"agent_diff_lines": 10,
	}})
	if !ok {
		t.Fatalf("expected pass when no expected diff: %s", msg)
	}
}

func TestCheckFunctionOverlap(t *testing.T) {
	diff := `@@ -10,3 +10,5 @@ func Foo
+some code
@@ -20,3 +20,5 @@ func Bar
+more code`

	ok, msg := checkFunctionOverlap(Outputs{"github": map[string]any{
		"full_diff":          diff,
		"expected_full_diff": diff,
	}})
	if !ok {
		t.Fatalf("expected pass on identical diffs: %s", msg)
	}

	ok, msg = checkFunctionOverlap(Outputs{"github": map[string]any{
		"full_diff":          `@@ -10,3 +10,5 @@ func Foo` + "\n+code",
		"expected_full_diff": `@@ -10,3 +10,5 @@ func Bar` + "\n+code",
	}})
	if ok {
		t.Fatalf("expected fail on zero overlap: %s", msg)
	}

	ok, msg = checkFunctionOverlap(Outputs{"github": map[string]any{
		"full_diff":          "just a change",
		"expected_full_diff": "",
	}})
	if !ok {
		t.Fatalf("expected pass when no expected diff: %s", msg)
	}
}

func TestExtractFunctions(t *testing.T) {
	diff := `@@ -10,3 +10,5 @@ func Foo
+line
@@ -20,3 +20,5 @@ func (r *Receiver) Bar
+line
@@ -30,3 +30,5 @@ something else
+line`
	funcs := extractFunctions(diff)
	if !funcs["Foo"] {
		t.Error("missing Foo")
	}
	if !funcs["Bar"] {
		t.Error("missing Bar")
	}
	if len(funcs) != 2 {
		t.Errorf("expected 2 functions, got %d", len(funcs))
	}
}

func TestCheckMakeResult(t *testing.T) {
	ok, msg := checkMakeResult(Outputs{"build_result": map[string]any{
		"passed": true,
		"output": "go: writing stat cache: permission denied\nok",
	}}, "build_result")
	if !ok {
		t.Fatal("expected pass")
	}
	if msg != "passed" {
		t.Errorf("expected clean 'passed' message, got %q", msg)
	}

	ok, msg = checkMakeResult(Outputs{"build_result": map[string]any{
		"passed": false,
		"error":  "exit status 1",
		"output": "compilation error on line 42",
	}}, "build_result")
	if ok {
		t.Fatal("expected fail")
	}
	if msg != "failed: exit status 1" {
		t.Errorf("unexpected message: %q", msg)
	}

	ok, msg = checkMakeResult(Outputs{}, "build_result")
	if ok {
		t.Fatal("expected fail on missing")
	}
}

func TestCheckNoSecrets(t *testing.T) {
	ok, _ := checkNoSecrets(Outputs{"github": map[string]any{
		"bot_replies": []map[string]any{{"body": "looks fine"}},
		"full_diff":   "diff --git a/x",
	}})
	if !ok {
		t.Fatal("expected pass")
	}
	ok, _ = checkNoSecrets(Outputs{"github": map[string]any{
		"full_diff": "token ghp_abcdefghijklmnopqrstuv",
	}})
	if ok {
		t.Fatal("expected fail on secret")
	}
}
