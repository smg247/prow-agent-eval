package report

import (
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"

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

func WriteJUnit(dir, evalName string, results []judge.Result) error {
	var cases []JUnitTestCase
	tally := TallyResults(results)

	for _, r := range results {
		tc := JUnitTestCase{
			Name:      r.Name,
			ClassName: evalName,
		}
		if r.Error != "" {
			tc.Error = &JUnitError{
				Message: r.Error,
				Text:    r.Error,
			}
		} else if !r.Passed {
			tc.Failure = &JUnitFailure{
				Message: r.Message,
				Text:    r.Message,
			}
		}
		cases = append(cases, tc)
	}

	suites := JUnitTestSuites{
		Suites: []JUnitTestSuite{
			{
				Name:     evalName,
				Tests:    tally.Total,
				Failures: tally.Failed,
				Errors:   tally.Errors,
				Cases:    cases,
			},
		},
	}

	data, err := xml.MarshalIndent(suites, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling JUnit XML: %w", err)
	}

	path := filepath.Join(dir, fmt.Sprintf("junit_%s.xml", evalName))
	header := []byte(xml.Header)
	return os.WriteFile(path, append(header, data...), 0644)
}
