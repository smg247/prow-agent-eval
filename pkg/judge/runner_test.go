package judge

import (
	"testing"

	"github.com/smg247/prow-agent-eval/pkg/config"
)

func TestApplyThresholds(t *testing.T) {
	results := []Result{
		{Name: "check_a", Passed: true},
		{Name: "check_a", Passed: true},
		{Name: "check_a", Passed: false},
		{Name: "check_b", Passed: true},
	}

	rate1 := 1.0
	rate06 := 0.6

	thresholds := map[string]config.Threshold{
		"check_a": {MinPassRate: &rate1},
		"check_b": {MinPassRate: &rate06},
	}

	tResults := ApplyThresholds(results, thresholds)

	resultMap := make(map[string]ThresholdResult)
	for _, tr := range tResults {
		resultMap[tr.Name] = tr
	}

	aResult, ok := resultMap["check_a/pass_rate"]
	if !ok {
		t.Fatal("missing check_a/pass_rate result")
	}
	if aResult.Met {
		t.Error("check_a should not meet threshold (2/3 < 1.0)")
	}
	if aResult.Actual < 0.66 || aResult.Actual > 0.67 {
		t.Errorf("check_a actual = %.2f, want ~0.67", aResult.Actual)
	}

	bResult, ok := resultMap["check_b/pass_rate"]
	if !ok {
		t.Fatal("missing check_b/pass_rate result")
	}
	if !bResult.Met {
		t.Error("check_b should meet threshold (1.0 >= 0.6)")
	}
}

func TestApplyThresholdsZeroSamples(t *testing.T) {
	rate := 1.0
	thresholds := map[string]config.Threshold{
		"missing": {MinPassRate: &rate},
	}
	tResults := ApplyThresholds(nil, thresholds)
	for _, tr := range tResults {
		if tr.Met {
			t.Errorf("%s should not be met with zero samples", tr.Name)
		}
	}
}

func TestApplyThresholdsSkipsErrors(t *testing.T) {
	rate := 1.0
	results := []Result{
		{Name: "check_a", Passed: true},
		{Name: "check_a", Error: "boom"},
	}
	thresholds := map[string]config.Threshold{
		"check_a": {MinPassRate: &rate},
	}
	tResults := ApplyThresholds(results, thresholds)
	if len(tResults) != 1 {
		t.Fatalf("len = %d", len(tResults))
	}
	if !tResults[0].Met {
		t.Error("only non-error result should count; 1/1 should meet")
	}
	if tResults[0].Actual != 1.0 {
		t.Errorf("Actual = %v, want 1.0", tResults[0].Actual)
	}
}

func TestEvaluateGate(t *testing.T) {
	ok, _ := EvaluateGate([]ThresholdResult{{Met: true}}, 0)
	if !ok {
		t.Error("expected ok")
	}
	ok, _ = EvaluateGate([]ThresholdResult{{Met: true}}, 2)
	if !ok {
		t.Error("case errors should not cause hard failure (synthetic results flow through thresholds)")
	}
	ok, reason := EvaluateGate([]ThresholdResult{{Met: false, Name: "x"}}, 0)
	if ok || reason == "" {
		t.Errorf("expected threshold failure, ok=%v reason=%q", ok, reason)
	}
}

func TestRunBuiltinByType(t *testing.T) {
	results, err := Run([]config.JudgeConfig{
		{Name: "compiles", Type: "build_passed"},
		{Name: "unknown", Type: "nope"},
	}, Outputs{
		"build_result": map[string]any{"passed": true, "output": "ok"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("len=%d", len(results))
	}
	if !results[0].Passed {
		t.Errorf("build_passed failed: %s", results[0].Message)
	}
	if results[1].Error == "" {
		t.Error("expected error for unknown type")
	}
}
