package judge

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/smg247/prow-agent-eval/pkg/config"
	"github.com/smg247/prow-agent-eval/pkg/shared"
)

func TestValidateCollectOptions(t *testing.T) {
	tests := []struct {
		name       string
		opts       CollectOptions
		wantErrSub string
	}{
		{
			name: "missing bot login",
			opts: CollectOptions{
				Config: &config.EvalConfig{Collect: config.CollectConfig{BotReplies: true}},
				Case:   &config.Case{},
				Meta:   &shared.CaseMetadata{BotLogin: ""},
			},
			wantErrSub: "bot login",
		},
		{
			name: "missing expected branch",
			opts: CollectOptions{
				Config: &config.EvalConfig{Collect: config.CollectConfig{ExpectedBranchDiff: true}},
				Case:   &config.Case{Input: config.CaseInput{}},
				Meta:   &shared.CaseMetadata{FixtureHeadSHA: "abc"},
			},
			wantErrSub: "expected_branch",
		},
		{
			name: "empty fixture head SHA ok when expected branch set",
			opts: CollectOptions{
				Config: &config.EvalConfig{Collect: config.CollectConfig{ExpectedBranchDiff: true}},
				Case:   &config.Case{Input: config.CaseInput{ExpectedBranch: "golden"}},
				Meta:   &shared.CaseMetadata{FixtureHeadSHA: ""},
			},
		},
		{
			name: "all required fields present",
			opts: CollectOptions{
				Config: &config.EvalConfig{Collect: config.CollectConfig{
					BotReplies:         true,
					ExpectedBranchDiff: true,
				}},
				Case: &config.Case{Input: config.CaseInput{ExpectedBranch: "golden"}},
				Meta: &shared.CaseMetadata{BotLogin: "bot", FixtureHeadSHA: "abc"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateCollectOptions(tt.opts)
			gotErr := ""
			if err != nil {
				gotErr = err.Error()
			}
			if tt.wantErrSub == "" {
				if diff := cmp.Diff("", gotErr); diff != "" {
					t.Errorf("error mismatch (-want +got):\n%s", diff)
				}
				return
			}
			if !strings.Contains(gotErr, tt.wantErrSub) {
				t.Errorf("error %q does not contain %q", gotErr, tt.wantErrSub)
			}
		})
	}
}
