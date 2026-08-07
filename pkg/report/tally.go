package report

import "github.com/smg247/prow-agent-eval/pkg/judge"

type Tally struct {
	Passed int
	Failed int
	Errors int
	Total  int
}

func TallyResults(results []judge.Result) Tally {
	var t Tally
	t.Total = len(results)
	for _, r := range results {
		switch {
		case r.Error != "":
			t.Errors++
		case r.Passed:
			t.Passed++
		default:
			t.Failed++
		}
	}
	return t
}
