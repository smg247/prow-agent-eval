package shared

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestWriteReadCaseMetadata(t *testing.T) {
	tests := []struct {
		name string
		meta *CaseMetadata
	}{
		{
			name: "round trip",
			meta: &CaseMetadata{
				CaseName:       "case-001",
				PRNumber:       42,
				HeadBranch:     "eval-case-001-20260807-120000",
				BaseBranch:     "main",
				FixtureHeadSHA: "abc123def456",
				JiraIssueKey:   "TRT-1234",
				Repo:           "openshift-trt/sippy-eval",
				BotLogin:       "test-bot",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := WriteCaseMetadata(dir, tt.meta); err != nil {
				t.Fatalf("WriteCaseMetadata: %v", err)
			}
			got, err := ReadCaseMetadata(dir, tt.meta.CaseName)
			if err != nil {
				t.Fatalf("ReadCaseMetadata: %v", err)
			}
			if diff := cmp.Diff(tt.meta, got); diff != "" {
				t.Errorf("CaseMetadata mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestWriteReadCaseList(t *testing.T) {
	tests := []struct {
		name  string
		cases []string
	}{
		{
			name:  "three cases",
			cases: []string{"case-001", "case-002", "case-003"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := WriteCaseList(dir, tt.cases); err != nil {
				t.Fatalf("WriteCaseList: %v", err)
			}
			got, err := ReadCaseList(dir)
			if err != nil {
				t.Fatalf("ReadCaseList: %v", err)
			}
			if diff := cmp.Diff(tt.cases, got); diff != "" {
				t.Errorf("CaseList mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestWriteReadFile(t *testing.T) {
	tests := []struct {
		name    string
		file    string
		content string
	}{
		{
			name:    "simple content",
			file:    "test-file",
			content: "hello world",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := WriteFile(dir, tt.file, tt.content); err != nil {
				t.Fatalf("WriteFile: %v", err)
			}
			got, err := ReadFile(dir, tt.file)
			if err != nil {
				t.Fatalf("ReadFile: %v", err)
			}
			if diff := cmp.Diff(tt.content, got); diff != "" {
				t.Errorf("ReadFile mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestWriteCaseMetadataClearsStale(t *testing.T) {
	tests := []struct {
		name   string
		first  *CaseMetadata
		second *CaseMetadata
		want   *CaseMetadata
	}{
		{
			name: "clears zero and empty fields",
			first: &CaseMetadata{
				CaseName:       "case-001",
				PRNumber:       42,
				HeadBranch:     "eval-head",
				BaseBranch:     "main",
				FixtureHeadSHA: "abc123",
				BotLogin:       "bot",
				Repo:           "o/r",
			},
			second: &CaseMetadata{
				CaseName:       "case-001",
				PRNumber:       0,
				HeadBranch:     "eval-head-2",
				BaseBranch:     "main",
				FixtureHeadSHA: "def456",
				BotLogin:       "",
				Repo:           "o/r",
			},
			want: &CaseMetadata{
				CaseName:       "case-001",
				PRNumber:       0,
				HeadBranch:     "eval-head-2",
				BaseBranch:     "main",
				FixtureHeadSHA: "def456",
				BotLogin:       "",
				Repo:           "o/r",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := WriteCaseMetadata(dir, tt.first); err != nil {
				t.Fatalf("WriteCaseMetadata first: %v", err)
			}
			if err := WriteCaseMetadata(dir, tt.second); err != nil {
				t.Fatalf("WriteCaseMetadata second: %v", err)
			}
			got, err := ReadCaseMetadata(dir, tt.want.CaseName)
			if err != nil {
				t.Fatalf("ReadCaseMetadata: %v", err)
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("CaseMetadata mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestEnsureCaseInList(t *testing.T) {
	tests := []struct {
		name       string
		initial    []string
		ensure     []string
		want       []string
		createOnly bool
	}{
		{
			name:    "append new and ignore duplicate",
			initial: []string{"case-001", "case-002"},
			ensure:  []string{"case-003", "case-001"},
			want:    []string{"case-001", "case-002", "case-003"},
		},
		{
			name:       "creates list when missing",
			createOnly: true,
			ensure:     []string{"only"},
			want:       []string{"only"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			if !tt.createOnly {
				if err := WriteCaseList(dir, tt.initial); err != nil {
					t.Fatalf("WriteCaseList: %v", err)
				}
			}
			for _, name := range tt.ensure {
				if err := EnsureCaseInList(dir, name); err != nil {
					t.Fatalf("EnsureCaseInList(%q): %v", name, err)
				}
			}
			got, err := ReadCaseList(dir)
			if err != nil {
				t.Fatalf("ReadCaseList: %v", err)
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("case list mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
