package report

import (
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"sort"

	"github.com/smg247/prow-agent-eval/pkg/judge"
)

const htmlTemplate = `<!DOCTYPE html>
<html>
<head>
<title>Eval Report: {{.EvalName}}</title>
<style>
body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; max-width: 1100px; margin: 40px auto; padding: 0 20px 200px; background: #1e1e1e; color: #e0e0e0; }
h1 { color: #ffffff; }
h2 { color: #cccccc; margin-top: 32px; }
.summary { background: #2a2a2a; padding: 16px; border-radius: 8px; margin-bottom: 24px; font-size: 18px; }
.pass { color: #4caf50; }
.fail { color: #f44336; }
.error { color: #ff9800; }
table { width: 100%; border-collapse: collapse; margin-bottom: 24px; }
th, td { text-align: left; padding: 8px 12px; border-bottom: 1px solid #444; }
th { background: #2a2a2a; font-weight: 600; color: #e0e0e0; }
details { margin-bottom: 16px; border: 1px solid #444; border-radius: 6px; }
details > summary { padding: 12px 16px; cursor: pointer; background: #2a2a2a; border-radius: 6px; font-weight: 600; color: #e0e0e0; }
details[open] > summary { border-bottom: 1px solid #444; border-radius: 6px 6px 0 0; }
details .content { padding: 16px; }
.icon-pass::before { content: "✅ "; }
.icon-fail::before { content: "❌ "; }
.icon-error::before { content: "⚠️ "; }
.threshold-met { color: #4caf50; }
.threshold-missed { color: #f44336; }
a { color: #64b5f6; text-decoration: none; }
a:hover { text-decoration: underline; }
.links { margin-top: 8px; font-size: 14px; }
.links a { margin-right: 16px; }
</style>
</head>
<body>
<h1>Eval Report: {{.EvalName}}</h1>
<div class="summary">
  <span class="pass">{{.TotalPassed}}/{{.TotalChecks}}</span> checks passed across {{.CaseCount}} cases
</div>

<h2>Overview</h2>
<table>
<thead><tr><th>Case</th><th>Checks</th><th>File Overlap</th><th>Diff Ratio</th><th>Func Overlap</th><th>PR</th><th>Links</th></tr></thead>
<tbody>
{{range .Cases}}
<tr>
  <td>{{.Name}}</td>
  <td>{{if eq .Failed 0}}<span class="pass">{{.Passed}}/{{.Total}}</span>{{else}}<span class="fail">{{.Passed}}/{{.Total}}</span>{{end}}</td>
  <td>{{.FileOverlap}}</td>
  <td>{{.DiffRatio}}</td>
  <td>{{.FuncOverlap}}</td>
  <td>{{if .PRURL}}<a href="{{.PRURL}}" target="_blank">#{{.PRNumber}}</a>{{else}}—{{end}}</td>
  <td>{{if .ExpectedDiffURL}}<a href="{{.ExpectedDiffURL}}" target="_blank">expected diff</a>{{end}}</td>
</tr>
{{end}}
</tbody>
</table>

<h2>Case Details</h2>
{{range .Cases}}
<details>
<summary>{{if eq .Failed 0}}{{.Name}} — <span class="pass">{{.Passed}}/{{.Total}} passed</span>{{else}}{{.Name}} — <span class="fail">{{.Passed}}/{{.Total}} passed</span>{{end}}</summary>
<div class="content">
{{if .HasLinks}}
<div class="links">
{{if .PRURL}}<a href="{{.PRURL}}" target="_blank">#{{.PRNumber}}</a>{{end}}
{{if .ExpectedDiffURL}}<a href="{{.ExpectedDiffURL}}" target="_blank">expected diff</a>{{end}}
{{if .AgentDiffURL}}<a href="{{.AgentDiffURL}}" target="_blank">agent diff</a>{{end}}
</div>
{{end}}
<table>
<thead><tr><th>Check</th><th>Status</th><th>Message</th></tr></thead>
<tbody>
{{range .Results}}
<tr>
  <td>{{.Name}}</td>
  <td class="{{.IconClass}}">{{.Status}}</td>
  <td>{{.Message}}</td>
</tr>
{{end}}
</tbody>
</table>
</div>
</details>
{{end}}

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
	EvalName    string
	TotalPassed int
	TotalChecks int
	CaseCount   int
	Cases       []htmlCase
	Thresholds  []judge.ThresholdResult
}

type htmlCase struct {
	Name            string
	Passed          int
	Failed          int
	Total           int
	FileOverlap     string
	DiffRatio       string
	FuncOverlap     string
	PR              string
	PRNumber        int
	PRURL           string
	ExpectedDiffURL string
	AgentDiffURL    string
	HasLinks        bool
	Results         []htmlResult
}

type htmlResult struct {
	Name      string
	Status    string
	Message   string
	IconClass string
}

func WriteHTML(dir, evalName string, caseResults map[string][]judge.Result, thresholds []judge.ThresholdResult, caseEvidence ...map[string]judge.CaseEvidence) error {
	var evidenceMap map[string]judge.CaseEvidence
	if len(caseEvidence) > 0 {
		evidenceMap = caseEvidence[0]
	}

	caseNames := make([]string, 0, len(caseResults))
	for name := range caseResults {
		caseNames = append(caseNames, name)
	}
	sort.Strings(caseNames)

	totalPassed := 0
	totalChecks := 0
	var cases []htmlCase

	for _, caseName := range caseNames {
		results := caseResults[caseName]
		hc := htmlCase{
			Name:        caseName,
			FileOverlap: "—",
			DiffRatio:   "—",
			FuncOverlap: "—",
			PR:          "—",
		}

		for _, r := range results {
			hc.Total++
			totalChecks++
			hr := htmlResult{Name: r.Name}
			switch {
			case r.Error != "":
				hr.Status = "ERROR"
				hr.Message = r.Error
				hr.IconClass = "icon-error"
				hc.Failed++
			case r.Passed:
				hr.Status = "PASS"
				hr.Message = r.Message
				hr.IconClass = "icon-pass"
				hc.Passed++
				totalPassed++
			default:
				hr.Status = "FAIL"
				hr.Message = r.Message
				hr.IconClass = "icon-fail"
				hc.Failed++
			}

			switch r.Name {
			case "file_overlap":
				if s := extractScore(r.Message); s != "" {
					hc.FileOverlap = s
				}
			case "diff_size_ratio":
				if s := extractScore(r.Message); s != "" {
					hc.DiffRatio = s
				}
			case "function_overlap":
				if s := extractScore(r.Message); s != "" {
					hc.FuncOverlap = s
				}
			case "pr_exists":
				hc.PR = r.Message
			}

			hc.Results = append(hc.Results, hr)
		}

		if evidenceMap != nil {
			if evidence, ok := evidenceMap[caseName]; ok {
				populateCaseLinks(&hc, evidence)
			}
		}

		cases = append(cases, hc)
	}

	data := htmlData{
		EvalName:    evalName,
		TotalPassed: totalPassed,
		TotalChecks: totalChecks,
		CaseCount:   len(caseNames),
		Cases:       cases,
		Thresholds:  thresholds,
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

func populateCaseLinks(hc *htmlCase, evidence judge.CaseEvidence) {
	gh := evidence.GitHub
	if gh.Repo != "" && gh.PRNumber > 0 {
		hc.PRNumber = gh.PRNumber
		hc.PRURL = fmt.Sprintf("https://github.com/%s/pull/%d", gh.Repo, gh.PRNumber)
		hc.HasLinks = true
	}
	if gh.Repo != "" && gh.BaseBranch != "" && gh.ExpectedBranch != "" {
		hc.ExpectedDiffURL = fmt.Sprintf("https://github.com/%s/compare/%s...%s", gh.Repo, gh.BaseBranch, gh.ExpectedBranch)
		hc.HasLinks = true
	}
	if gh.Repo != "" && gh.BaseBranch != "" && gh.AgentBranch != "" {
		hc.AgentDiffURL = fmt.Sprintf("https://github.com/%s/compare/%s...%s", gh.Repo, gh.BaseBranch, gh.AgentBranch)
		hc.HasLinks = true
	}
}
