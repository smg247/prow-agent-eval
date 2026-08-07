package shared

import (
	"testing"
)

func TestWriteReadCaseMetadata(t *testing.T) {
	dir := t.TempDir()

	original := &CaseMetadata{
		CaseName:       "case-001",
		PRNumber:       42,
		HeadBranch:     "eval-case-001-20260807-120000",
		BaseBranch:     "main",
		FixtureHeadSHA: "abc123def456",
		JiraIssueKey:   "TRT-1234",
		Repo:           "openshift-trt/sippy-eval",
		BotLogin:       "test-bot",
	}

	if err := WriteCaseMetadata(dir, original); err != nil {
		t.Fatalf("WriteCaseMetadata: %v", err)
	}

	got, err := ReadCaseMetadata(dir, "case-001")
	if err != nil {
		t.Fatalf("ReadCaseMetadata: %v", err)
	}

	if got.PRNumber != original.PRNumber {
		t.Errorf("PRNumber = %d, want %d", got.PRNumber, original.PRNumber)
	}
	if got.HeadBranch != original.HeadBranch {
		t.Errorf("HeadBranch = %q, want %q", got.HeadBranch, original.HeadBranch)
	}
	if got.BaseBranch != original.BaseBranch {
		t.Errorf("BaseBranch = %q, want %q", got.BaseBranch, original.BaseBranch)
	}
	if got.FixtureHeadSHA != original.FixtureHeadSHA {
		t.Errorf("FixtureHeadSHA = %q, want %q", got.FixtureHeadSHA, original.FixtureHeadSHA)
	}
	if got.JiraIssueKey != original.JiraIssueKey {
		t.Errorf("JiraIssueKey = %q, want %q", got.JiraIssueKey, original.JiraIssueKey)
	}
	if got.Repo != original.Repo {
		t.Errorf("Repo = %q, want %q", got.Repo, original.Repo)
	}
	if got.BotLogin != original.BotLogin {
		t.Errorf("BotLogin = %q, want %q", got.BotLogin, original.BotLogin)
	}
}

func TestWriteReadCaseList(t *testing.T) {
	dir := t.TempDir()

	cases := []string{"case-001", "case-002", "case-003"}
	if err := WriteCaseList(dir, cases); err != nil {
		t.Fatalf("WriteCaseList: %v", err)
	}

	got, err := ReadCaseList(dir)
	if err != nil {
		t.Fatalf("ReadCaseList: %v", err)
	}

	if len(got) != len(cases) {
		t.Fatalf("len(cases) = %d, want %d", len(got), len(cases))
	}
	for i, c := range got {
		if c != cases[i] {
			t.Errorf("case[%d] = %q, want %q", i, c, cases[i])
		}
	}
}

func TestWriteReadFile(t *testing.T) {
	dir := t.TempDir()

	if err := WriteFile(dir, "test-file", "hello world"); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	got, err := ReadFile(dir, "test-file")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	if got != "hello world" {
		t.Errorf("got %q, want %q", got, "hello world")
	}
}

func TestWriteCaseMetadataClearsStale(t *testing.T) {
	dir := t.TempDir()

	if err := WriteCaseMetadata(dir, &CaseMetadata{
		CaseName:       "case-001",
		PRNumber:       42,
		HeadBranch:     "eval-head",
		BaseBranch:     "main",
		FixtureHeadSHA: "abc123",
		BotLogin:       "bot",
		Repo:           "o/r",
	}); err != nil {
		t.Fatal(err)
	}

	if err := WriteCaseMetadata(dir, &CaseMetadata{
		CaseName:       "case-001",
		PRNumber:       0,
		HeadBranch:     "eval-head-2",
		BaseBranch:     "main",
		FixtureHeadSHA: "def456",
		BotLogin:       "",
		Repo:           "o/r",
	}); err != nil {
		t.Fatal(err)
	}

	got, err := ReadCaseMetadata(dir, "case-001")
	if err != nil {
		t.Fatal(err)
	}
	if got.PRNumber != 0 {
		t.Errorf("PRNumber = %d, want 0", got.PRNumber)
	}
	if got.BotLogin != "" {
		t.Errorf("BotLogin = %q, want empty", got.BotLogin)
	}
	if got.HeadBranch != "eval-head-2" {
		t.Errorf("HeadBranch = %q", got.HeadBranch)
	}
}

func TestEnsureCaseInList(t *testing.T) {
	dir := t.TempDir()

	if err := WriteCaseList(dir, []string{"case-001", "case-002"}); err != nil {
		t.Fatal(err)
	}
	if err := EnsureCaseInList(dir, "case-003"); err != nil {
		t.Fatal(err)
	}
	if err := EnsureCaseInList(dir, "case-001"); err != nil {
		t.Fatal(err)
	}

	got, err := ReadCaseList(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("len = %d, want 3: %v", len(got), got)
	}
	if got[0] != "case-001" || got[1] != "case-002" || got[2] != "case-003" {
		t.Errorf("got %v", got)
	}
}

func TestEnsureCaseInListCreates(t *testing.T) {
	dir := t.TempDir()
	if err := EnsureCaseInList(dir, "only"); err != nil {
		t.Fatal(err)
	}
	got, err := ReadCaseList(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != "only" {
		t.Errorf("got %v", got)
	}
}
