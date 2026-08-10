package report

import (
	"encoding/xml"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/smg247/prow-agent-eval/pkg/judge"
)

func TestWriteJUnit(t *testing.T) {
	tests := []struct {
		name        string
		evalName    string
		caseResults map[string][]judge.Result
		wantSuite   JUnitTestSuite
		wantXMLHdr  bool
	}{
		{
			name:     "mixed pass fail error",
			evalName: "test-eval",
			caseResults: map[string][]judge.Result{
				"case-001": {
					{Name: "check_files", Passed: true, Message: "All files changed"},
					{Name: "no_secrets", Passed: false, Message: "Credential pattern found"},
				},
				"case-002": {
					{Name: "quality", Error: "Python error"},
				},
			},
			wantSuite: JUnitTestSuite{
				Name:     "test-eval",
				Tests:    3,
				Failures: 1,
				Errors:   1,
				Cases: []JUnitTestCase{
					{
						Name:      "[test-eval] case-001 check_files",
						ClassName: "test-eval.case-001",
					},
					{
						Name:      "[test-eval] case-001 no_secrets",
						ClassName: "test-eval.case-001",
						Failure:   &JUnitFailure{Message: "Credential pattern found", Text: "Credential pattern found"},
					},
					{
						Name:      "[test-eval] case-002 quality",
						ClassName: "test-eval.case-002",
						Error:     &JUnitError{Message: "Python error", Text: "Python error"},
					},
				},
			},
			wantXMLHdr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := WriteJUnit(dir, tt.evalName, tt.caseResults); err != nil {
				t.Fatalf("WriteJUnit: %v", err)
			}
			path := filepath.Join(dir, "junit_"+tt.evalName+".xml")
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("reading JUnit XML: %v", err)
			}
			hasHdr := strings.Contains(string(data), "<?xml")
			if diff := cmp.Diff(tt.wantXMLHdr, hasHdr); diff != "" {
				t.Errorf("XML header presence mismatch (-want +got):\n%s", diff)
			}

			var suites JUnitTestSuites
			if err := xml.Unmarshal(data, &suites); err != nil {
				t.Fatalf("parsing JUnit XML: %v", err)
			}
			if diff := cmp.Diff(1, len(suites.Suites)); diff != "" {
				t.Fatalf("len(Suites) mismatch (-want +got):\n%s", diff)
			}

			opts := []cmp.Option{
				cmpopts.IgnoreFields(JUnitTestSuite{}, "XMLName"),
				cmpopts.IgnoreFields(JUnitTestCase{}, "XMLName"),
			}
			if diff := cmp.Diff(tt.wantSuite, suites.Suites[0], opts...); diff != "" {
				t.Errorf("JUnitTestSuite mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
