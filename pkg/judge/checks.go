package judge

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/smg247/prow-agent-eval/pkg/config"
)

// Builtin judges keyed by type (JudgeConfig.Type, or Name if Type is empty).
var builtins = map[string]func(Outputs) (bool, string){
	"branch_created":         checkBranchCreated,
	"pr_exists":              checkPRExists,
	"pr_description_exists":  checkPRDescriptionExists,
	"build_passed":           checkBuildPassed,
	"test_passed":            checkTestPassed,
	"file_overlap":           checkFileOverlap,
	"diff_size_ratio":        checkDiffSizeRatio,
	"function_overlap":       checkFunctionOverlap,
	"expected_files_changed": checkExpectedFilesChanged,
	"no_secrets":             checkNoSecrets,
}

func runBuiltin(typ string, outputs Outputs) (bool, string, error) {
	fn, ok := builtins[typ]
	if !ok {
		return false, "", fmt.Errorf("unknown judge type %q", typ)
	}
	passed, msg := fn(outputs)
	return passed, msg, nil
}

func githubMap(outputs Outputs) map[string]any {
	gh, _ := outputs["github"].(map[string]any)
	if gh == nil {
		return map[string]any{}
	}
	return gh
}

func annotationsMap(outputs Outputs) map[string]any {
	switch a := outputs["annotations"].(type) {
	case map[string]any:
		return a
	case config.CaseAnnotations:
		return map[string]any(a)
	default:
		return map[string]any{}
	}
}

func stringSlice(v any) []string {
	switch t := v.(type) {
	case []string:
		return t
	case []any:
		out := make([]string, 0, len(t))
		for _, item := range t {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}

func setFromStrings(ss []string) map[string]bool {
	m := make(map[string]bool, len(ss))
	for _, s := range ss {
		m[s] = true
	}
	return m
}

func checkBranchCreated(outputs Outputs) (bool, string) {
	branch, _ := githubMap(outputs)["agent_branch"].(string)
	if branch == "" || branch == "main" || branch == "master" {
		return false, fmt.Sprintf("Branch: %s", branch)
	}
	return true, fmt.Sprintf("Branch: %s", branch)
}

func checkPRExists(outputs Outputs) (bool, string) {
	gh := githubMap(outputs)
	switch n := gh["pr_number"].(type) {
	case int:
		if n > 0 {
			return true, fmt.Sprintf("PR #%d", n)
		}
	case int64:
		if n > 0 {
			return true, fmt.Sprintf("PR #%d", n)
		}
	case float64:
		if n > 0 {
			return true, fmt.Sprintf("PR #%d", int(n))
		}
	}
	return false, "No PR created"
}

func checkPRDescriptionExists(outputs Outputs) (bool, string) {
	gh := githubMap(outputs)
	body, _ := gh["pr_body"].(string)
	if strings.TrimSpace(body) != "" {
		return true, "PR description exists"
	}
	if _, ok := gh["pr_description_file"]; ok {
		return true, "PR description file exists"
	}
	return false, "No PR description"
}

func checkDiffSizeRatio(outputs Outputs) (bool, string) {
	gh := githubMap(outputs)
	agentLines, ok1 := gh["agent_diff_lines"].(int)
	expectedLines, ok2 := gh["expected_diff_lines"].(int)
	if !ok1 || !ok2 {
		return true, "N/A (no expected diff)"
	}
	if expectedLines == 0 {
		if agentLines == 0 {
			return true, "Diff size ratio: N/A (both empty)"
		}
		return true, fmt.Sprintf("Diff size ratio: N/A (expected empty, agent=%d)", agentLines)
	}
	ratio := float64(agentLines) / float64(expectedLines)
	passed := ratio >= 0.1
	return passed, fmt.Sprintf("Diff size ratio: %.2f", ratio)
}

func checkFunctionOverlap(outputs Outputs) (bool, string) {
	gh := githubMap(outputs)
	agentDiff, _ := gh["full_diff"].(string)
	expectedDiff, _ := gh["expected_full_diff"].(string)
	if expectedDiff == "" {
		return true, "N/A (no expected diff)"
	}
	agentFuncs := extractFunctions(agentDiff)
	expectedFuncs := extractFunctions(expectedDiff)
	if len(agentFuncs) == 0 && len(expectedFuncs) == 0 {
		return true, "Function overlap: N/A"
	}
	if len(agentFuncs) == 0 || len(expectedFuncs) == 0 {
		return false, fmt.Sprintf("Function overlap: 0.00 (agent=%d expected=%d)", len(agentFuncs), len(expectedFuncs))
	}
	inter := 0
	for f := range agentFuncs {
		if expectedFuncs[f] {
			inter++
		}
	}
	union := len(agentFuncs) + len(expectedFuncs) - inter
	overlap := float64(inter) / float64(union)
	return overlap >= 0.25, fmt.Sprintf("Function overlap: %.2f", overlap)
}

func extractFunctions(diff string) map[string]bool {
	funcs := make(map[string]bool)
	funcRe := regexp.MustCompile(`^@@.*@@\s+func\s+(\([^)]*\)\s+)?(\w+)`)
	for _, line := range strings.Split(diff, "\n") {
		if m := funcRe.FindStringSubmatch(line); m != nil {
			funcs[m[2]] = true
		}
	}
	return funcs
}

func checkBuildPassed(outputs Outputs) (bool, string) {
	return checkMakeResult(outputs, "build_result")
}

func checkTestPassed(outputs Outputs) (bool, string) {
	return checkMakeResult(outputs, "test_result")
}

func checkMakeResult(outputs Outputs, key string) (bool, string) {
	res, _ := outputs[key].(map[string]any)
	if res == nil {
		return false, key + " not collected"
	}
	passed, _ := res["passed"].(bool)
	output, _ := res["output"].(string)
	msg := output
	if len(msg) > 200 {
		msg = msg[len(msg)-200:]
	}
	if msg == "" {
		if passed {
			msg = "passed"
		} else {
			msg = "failed"
		}
	}
	return passed, msg
}

func checkFileOverlap(outputs Outputs) (bool, string) {
	gh := githubMap(outputs)
	changed := setFromStrings(stringSlice(gh["changed_files"]))
	expected := setFromStrings(stringSlice(gh["expected_changed_files"]))
	if len(changed) == 0 && len(expected) == 0 {
		return true, "Both empty (vacuous match)"
	}
	if len(changed) == 0 || len(expected) == 0 {
		return false, fmt.Sprintf("Overlap: 0 (changed=%d expected=%d)", len(changed), len(expected))
	}
	inter := 0
	for f := range changed {
		if expected[f] {
			inter++
		}
	}
	union := len(changed) + len(expected) - inter
	overlap := float64(inter) / float64(union)
	passed := overlap >= 0.25
	return passed, fmt.Sprintf("Jaccard overlap: %.2f", overlap)
}

func checkExpectedFilesChanged(outputs Outputs) (bool, string) {
	annotations := annotationsMap(outputs)
	changed := setFromStrings(stringSlice(githubMap(outputs)["changed_files"]))

	expectedRaw, ok := annotations["expected_files"]
	if !ok {
		return true, "No expected_files in annotations"
	}

	var missing []string
	switch expected := expectedRaw.(type) {
	case map[string]any:
		for commentID, filesVal := range expected {
			for _, f := range stringSlice(filesVal) {
				if !changed[f] {
					missing = append(missing, fmt.Sprintf("%s: %s", commentID, f))
				}
			}
		}
	case []any, []string:
		for _, f := range stringSlice(expected) {
			if !changed[f] {
				missing = append(missing, f)
			}
		}
	default:
		return false, "annotations.expected_files has unexpected type"
	}

	if len(missing) > 0 {
		return false, "Missing: " + strings.Join(missing, ", ")
	}
	return true, "All expected files changed"
}

var secretPatterns = []*regexp.Regexp{
	regexp.MustCompile(`ghp_[A-Za-z0-9]+`),
	regexp.MustCompile(`ghs_[A-Za-z0-9]+`),
	regexp.MustCompile(`ya29\.[A-Za-z0-9_-]+`),
}

func checkNoSecrets(outputs Outputs) (bool, string) {
	gh := githubMap(outputs)
	var b strings.Builder
	if replies, ok := gh["bot_replies"].([]map[string]any); ok {
		for _, m := range replies {
			if body, ok := m["body"].(string); ok {
				b.WriteString(body)
				b.WriteByte(' ')
			}
		}
	} else if replies, ok := gh["bot_replies"].([]any); ok {
		for _, r := range replies {
			if m, ok := r.(map[string]any); ok {
				if body, ok := m["body"].(string); ok {
					b.WriteString(body)
					b.WriteByte(' ')
				}
			}
		}
	}
	if diff, ok := gh["full_diff"].(string); ok {
		b.WriteString(diff)
	}
	text := b.String()
	for _, p := range secretPatterns {
		if p.MatchString(text) {
			return false, "Credential pattern found: " + p.String()
		}
	}
	return true, "No secrets leaked"
}
