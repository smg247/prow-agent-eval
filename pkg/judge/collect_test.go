package judge

import (
	"strings"
	"testing"

	"github.com/smg247/prow-agent-eval/pkg/config"
	"github.com/smg247/prow-agent-eval/pkg/shared"
)

func TestValidateCollectOptionsBotLogin(t *testing.T) {
	err := validateCollectOptions(CollectOptions{
		Config: &config.EvalConfig{Collect: config.CollectConfig{BotReplies: true}},
		Case:   &config.Case{},
		Meta:   &shared.CaseMetadata{BotLogin: ""},
	})
	if err == nil || !strings.Contains(err.Error(), "bot login") {
		t.Fatalf("err = %v", err)
	}
}

func TestValidateCollectOptionsExpectedBranch(t *testing.T) {
	err := validateCollectOptions(CollectOptions{
		Config: &config.EvalConfig{Collect: config.CollectConfig{ExpectedBranchDiff: true}},
		Case:   &config.Case{Input: config.CaseInput{}},
		Meta:   &shared.CaseMetadata{FixtureHeadSHA: "abc"},
	})
	if err == nil || !strings.Contains(err.Error(), "expected_branch") {
		t.Fatalf("err = %v", err)
	}

	err = validateCollectOptions(CollectOptions{
		Config: &config.EvalConfig{Collect: config.CollectConfig{ExpectedBranchDiff: true}},
		Case:   &config.Case{Input: config.CaseInput{ExpectedBranch: "golden"}},
		Meta:   &shared.CaseMetadata{FixtureHeadSHA: ""},
	})
	if err == nil || !strings.Contains(err.Error(), "fixture head SHA") {
		t.Fatalf("err = %v", err)
	}
}

func TestValidateCollectOptionsOK(t *testing.T) {
	err := validateCollectOptions(CollectOptions{
		Config: &config.EvalConfig{Collect: config.CollectConfig{
			BotReplies:         true,
			ExpectedBranchDiff: true,
		}},
		Case: &config.Case{Input: config.CaseInput{ExpectedBranch: "golden"}},
		Meta: &shared.CaseMetadata{BotLogin: "bot", FixtureHeadSHA: "abc"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
