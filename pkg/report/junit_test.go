package report

import (
	"encoding/xml"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/smg247/prow-agent-eval/pkg/judge"
)

func TestWriteJUnit(t *testing.T) {
	dir := t.TempDir()

	caseResults := map[string][]judge.Result{
		"case-001": {
			{Name: "check_files", Passed: true, Message: "All files changed"},
			{Name: "no_secrets", Passed: false, Message: "Credential pattern found"},
		},
		"case-002": {
			{Name: "quality", Error: "Python error"},
		},
	}

	if err := WriteJUnit(dir, "test-eval", caseResults); err != nil {
		t.Fatalf("WriteJUnit: %v", err)
	}

	path := filepath.Join(dir, "junit_test-eval.xml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading JUnit XML: %v", err)
	}

	if !strings.Contains(string(data), "<?xml") {
		t.Error("missing XML header")
	}

	var suites JUnitTestSuites
	if err := xml.Unmarshal(data, &suites); err != nil {
		t.Fatalf("parsing JUnit XML: %v", err)
	}

	if len(suites.Suites) != 1 {
		t.Fatalf("len(Suites) = %d, want 1", len(suites.Suites))
	}

	suite := suites.Suites[0]
	if suite.Tests != 3 {
		t.Errorf("Tests = %d, want 3", suite.Tests)
	}
	if suite.Failures != 1 {
		t.Errorf("Failures = %d, want 1", suite.Failures)
	}
	if suite.Errors != 1 {
		t.Errorf("Errors = %d, want 1", suite.Errors)
	}

	if suite.Cases[0].ClassName != "test-eval.case-001" {
		t.Errorf("ClassName = %q, want %q", suite.Cases[0].ClassName, "test-eval.case-001")
	}
	if !strings.Contains(suite.Cases[0].Name, "[test-eval]") {
		t.Errorf("Name %q should contain [test-eval]", suite.Cases[0].Name)
	}
}
