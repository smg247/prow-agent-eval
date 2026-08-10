package report

import (
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/smg247/prow-agent-eval/pkg/judge"
)

type JUnitTestSuites struct {
	XMLName xml.Name         `xml:"testsuites"`
	Suites  []JUnitTestSuite `xml:"testsuite"`
}

type JUnitTestSuite struct {
	XMLName  xml.Name        `xml:"testsuite"`
	Name     string          `xml:"name,attr"`
	Tests    int             `xml:"tests,attr"`
	Failures int             `xml:"failures,attr"`
	Errors   int             `xml:"errors,attr"`
	Cases    []JUnitTestCase `xml:"testcase"`
}

type JUnitTestCase struct {
	XMLName   xml.Name      `xml:"testcase"`
	Name      string        `xml:"name,attr"`
	ClassName string        `xml:"classname,attr"`
	Failure   *JUnitFailure `xml:"failure,omitempty"`
	Error     *JUnitError   `xml:"error,omitempty"`
}

type JUnitFailure struct {
	Message string `xml:"message,attr"`
	Text    string `xml:",chardata"`
}

type JUnitError struct {
	Message string `xml:"message,attr"`
	Text    string `xml:",chardata"`
}

func WriteJUnit(dir, evalName string, caseResults map[string][]judge.Result) error {
	var allCases []JUnitTestCase
	totalTests := 0
	totalFailures := 0
	totalErrors := 0

	caseNames := make([]string, 0, len(caseResults))
	for name := range caseResults {
		caseNames = append(caseNames, name)
	}
	sort.Strings(caseNames)

	for _, caseName := range caseNames {
		results := caseResults[caseName]
		for _, r := range results {
			totalTests++
			tc := JUnitTestCase{
				Name:      fmt.Sprintf("[%s] %s %s", evalName, caseName, r.Name),
				ClassName: fmt.Sprintf("%s.%s", evalName, caseName),
			}
			if r.Error != "" {
				tc.Error = &JUnitError{
					Message: r.Error,
					Text:    r.Error,
				}
				totalErrors++
			} else if !r.Passed {
				msg := r.Message
				if msg == "" {
					msg = r.Name + " check did not pass."
				}
				tc.Failure = &JUnitFailure{
					Message: msg,
					Text:    msg,
				}
				totalFailures++
			}
			allCases = append(allCases, tc)
		}
	}

	suites := JUnitTestSuites{
		Suites: []JUnitTestSuite{
			{
				Name:     evalName,
				Tests:    totalTests,
				Failures: totalFailures,
				Errors:   totalErrors,
				Cases:    allCases,
			},
		},
	}

	data, err := xml.MarshalIndent(suites, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling JUnit XML: %w", err)
	}

	path := filepath.Join(dir, fmt.Sprintf("junit_%s.xml", evalName))
	header := []byte(xml.Header)
	return os.WriteFile(path, append(header, data...), 0o600)
}
