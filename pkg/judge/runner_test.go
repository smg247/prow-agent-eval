package judge

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/smg247/prow-agent-eval/pkg/config"
)

func TestApplyThresholds(t *testing.T) {
	rate1 := 1.0
	rate06 := 0.6

	tests := []struct {
		name       string
		results    []Result
		thresholds map[string]config.Threshold
		want       map[string]ThresholdResult
	}{
		{
			name: "pass rates against thresholds",
			results: []Result{
				{Name: "check_a", Passed: true},
				{Name: "check_a", Passed: true},
				{Name: "check_a", Passed: false},
				{Name: "check_b", Passed: true},
			},
			thresholds: map[string]config.Threshold{
				"check_a": {MinPassRate: &rate1},
				"check_b": {MinPassRate: &rate06},
			},
			want: map[string]ThresholdResult{
				"check_a/pass_rate": {Name: "check_a/pass_rate", Met: false, Actual: 2.0 / 3.0, Required: 1.0, MetricType: "pass_rate"},
				"check_b/pass_rate": {Name: "check_b/pass_rate", Met: true, Actual: 1.0, Required: 0.6, MetricType: "pass_rate"},
			},
		},
		{
			name:    "zero samples not met",
			results: nil,
			thresholds: map[string]config.Threshold{
				"missing": {MinPassRate: &rate1},
			},
			want: map[string]ThresholdResult{
				"missing/pass_rate": {Name: "missing/pass_rate", Met: false, Actual: 0, Required: 1.0, MetricType: "pass_rate"},
			},
		},
		{
			name: "skips errors",
			results: []Result{
				{Name: "check_a", Passed: true},
				{Name: "check_a", Error: "boom"},
			},
			thresholds: map[string]config.Threshold{
				"check_a": {MinPassRate: &rate1},
			},
			want: map[string]ThresholdResult{
				"check_a/pass_rate": {Name: "check_a/pass_rate", Met: true, Actual: 1.0, Required: 1.0, MetricType: "pass_rate"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotSlice := ApplyThresholds(tt.results, tt.thresholds)
			got := make(map[string]ThresholdResult, len(gotSlice))
			for _, tr := range gotSlice {
				got[tr.Name] = tr
			}
			if diff := cmp.Diff(tt.want, got, cmpopts.EquateApprox(0, 1e-9)); diff != "" {
				t.Errorf("ApplyThresholds() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestEvaluateGate(t *testing.T) {
	tests := []struct {
		name       string
		thresholds []ThresholdResult
		caseErrors int
		wantOK     bool
		wantReason string
	}{
		{
			name:       "all met",
			thresholds: []ThresholdResult{{Met: true}},
			wantOK:     true,
		},
		{
			name:       "case errors do not hard-fail",
			thresholds: []ThresholdResult{{Met: true}},
			caseErrors: 2,
			wantOK:     true,
		},
		{
			name:       "threshold failure",
			thresholds: []ThresholdResult{{Met: false, Name: "x"}},
			wantOK:     false,
			wantReason: "one or more thresholds not met",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotOK, gotReason := EvaluateGate(tt.thresholds, tt.caseErrors)
			got := struct {
				OK     bool
				Reason string
			}{gotOK, gotReason}
			want := struct {
				OK     bool
				Reason string
			}{tt.wantOK, tt.wantReason}
			if diff := cmp.Diff(want, got); diff != "" {
				t.Errorf("EvaluateGate() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestRunBuiltinByType(t *testing.T) {
	tests := []struct {
		name   string
		judges []config.JudgeConfig
		outs   Outputs
		want   []Result
	}{
		{
			name: "known and unknown types",
			judges: []config.JudgeConfig{
				{Name: "compiles", Type: "build_passed"},
				{Name: "unknown", Type: "nope"},
			},
			outs: Outputs{
				"build_result": map[string]any{"passed": true, "output": "ok"},
			},
			want: []Result{
				{Name: "compiles", Passed: true, Message: "passed"},
				{Name: "unknown", Error: `unknown judge type "nope"`},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Run(tt.judges, tt.outs)
			if err != nil {
				t.Fatalf("Run() unexpected error: %v", err)
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("Run() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
