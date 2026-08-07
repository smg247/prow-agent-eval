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
