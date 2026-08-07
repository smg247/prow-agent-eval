package report

import (
	"testing"

	"github.com/smg247/prow-agent-eval/pkg/judge"
)

func TestTallyResults(t *testing.T) {
	results := []judge.Result{
		{Name: "a", Passed: true},
		{Name: "b", Passed: false},
		{Name: "c", Error: "x"},
	}
	got := TallyResults(results)
	if got.Passed != 1 || got.Failed != 1 || got.Errors != 1 || got.Total != 3 {
		t.Errorf("TallyResults = %+v", got)
	}
}
