package report

import (
	"fmt"
	"html/template"
	"os"
	"path/filepath"

	"github.com/smg247/prow-agent-eval/pkg/judge"
)

const htmlTemplate = `<!DOCTYPE html>
<html>
<head>
<title>Eval Report: {{.EvalName}}</title>
<style>
body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; max-width: 900px; margin: 40px auto; padding: 0 20px; }
h1 { color: #1a1a1a; }
.summary { background: #f5f5f5; padding: 16px; border-radius: 8px; margin-bottom: 24px; }
.summary span { margin-right: 24px; }
.pass { color: #22863a; }
.fail { color: #cb2431; }
.error { color: #e36209; }
table { width: 100%; border-collapse: collapse; margin-bottom: 24px; }
th, td { text-align: left; padding: 8px 12px; border-bottom: 1px solid #e1e4e8; }
th { background: #f6f8fa; font-weight: 600; }
tr.passed td:first-child { border-left: 3px solid #22863a; }
tr.failed td:first-child { border-left: 3px solid #cb2431; }
tr.errored td:first-child { border-left: 3px solid #e36209; }
.threshold { margin-top: 24px; }
.threshold-met { color: #22863a; }
.threshold-missed { color: #cb2431; }
</style>
</head>
<body>
<h1>Eval Report: {{.EvalName}}</h1>
<div class="summary">
  <span class="pass">Passed: {{.Passed}}</span>
  <span class="fail">Failed: {{.Failed}}</span>
  <span class="error">Errors: {{.Errors}}</span>
  <span>Total: {{.Total}}</span>
</div>
<h2>Judge Results</h2>
<table>
<thead><tr><th>Judge</th><th>Status</th><th>Message</th></tr></thead>
<tbody>
{{range .Results}}
<tr class="{{.RowClass}}">
  <td>{{.Name}}</td>
  <td>{{.Status}}</td>
  <td>{{.Message}}</td>
</tr>
{{end}}
</tbody>
</table>
{{if .Thresholds}}
<h2>Thresholds</h2>
<table>
<thead><tr><th>Metric</th><th>Status</th><th>Actual</th><th>Required</th></tr></thead>
<tbody>
{{range .Thresholds}}
<tr>
  <td>{{.Name}}</td>
  <td class="{{if .Met}}threshold-met{{else}}threshold-missed{{end}}">{{if .Met}}MET{{else}}MISSED{{end}}</td>
  <td>{{printf "%.2f" .Actual}}</td>
  <td>{{printf "%.2f" .Required}}</td>
</tr>
{{end}}
</tbody>
</table>
{{end}}
</body>
</html>`

type htmlData struct {
	EvalName   string
	Passed     int
	Failed     int
	Errors     int
	Total      int
	Results    []htmlResult
	Thresholds []judge.ThresholdResult
}

type htmlResult struct {
	Name     string
	Status   string
	Message  string
	RowClass string
}

func WriteHTML(dir, evalName string, results []judge.Result, thresholds []judge.ThresholdResult) error {
	tally := TallyResults(results)
	data := htmlData{
		EvalName:   evalName,
		Thresholds: thresholds,
		Passed:     tally.Passed,
		Failed:     tally.Failed,
		Errors:     tally.Errors,
		Total:      tally.Total,
	}

	for _, r := range results {
		hr := htmlResult{Name: r.Name}
		switch {
		case r.Error != "":
			hr.Status = "ERROR"
			hr.Message = r.Error
			hr.RowClass = "errored"
		case r.Passed:
			hr.Status = "PASS"
			hr.Message = r.Message
			hr.RowClass = "passed"
		default:
			hr.Status = "FAIL"
			hr.Message = r.Message
			hr.RowClass = "failed"
		}
		data.Results = append(data.Results, hr)
	}

	tmpl, err := template.New("report").Parse(htmlTemplate)
	if err != nil {
		return fmt.Errorf("parsing HTML template: %w", err)
	}

	path := filepath.Join(dir, "eval-summary.html")
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("creating HTML report: %w", err)
	}
	defer f.Close()

	return tmpl.Execute(f, data)
}
