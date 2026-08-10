package report

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/smg247/prow-agent-eval/pkg/judge"
)

func TestTallyResults(t *testing.T) {
	tests := []struct {
		name    string
		results []judge.Result
		want    Tally
	}{
		{
			name: "passed failed and error",
			results: []judge.Result{
				{Name: "a", Passed: true},
				{Name: "b", Passed: false},
				{Name: "c", Error: "x"},
			},
			want: Tally{Passed: 1, Failed: 1, Errors: 1, Total: 3},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TallyResults(tt.results)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("TallyResults() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
