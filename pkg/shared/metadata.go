package shared

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type CaseMetadata struct {
	CaseName       string
	PRNumber       int
	HeadBranch     string
	BaseBranch     string
	FixtureHeadSHA string
	JiraIssueKey   string
	Repo           string
	BotLogin       string
}

func safeJoin(dir, name string) (string, error) {
	if name == "" {
		return "", fmt.Errorf("empty file name")
	}
	if filepath.IsAbs(name) {
		return "", fmt.Errorf("absolute path not allowed: %s", name)
	}
	clean := filepath.Clean(name)
	if clean == ".." || strings.HasPrefix(clean, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("path escapes directory: %s", name)
	}
	return filepath.Join(dir, clean), nil
}

func WriteCaseMetadata(dir string, m *CaseMetadata) error {
	prefix := m.CaseName + "."
	writes := map[string]string{
		prefix + "pr-number":        strconv.Itoa(m.PRNumber),
		prefix + "eval-head-branch": m.HeadBranch,
		prefix + "eval-base-branch": m.BaseBranch,
		prefix + "fixture-head-sha": m.FixtureHeadSHA,
		prefix + "jira-issue-key":   m.JiraIssueKey,
		prefix + "eval-repo":        m.Repo,
		prefix + "bot-login":        m.BotLogin,
	}
	for name, val := range writes {
		path, err := safeJoin(dir, name)
		if err != nil {
			return err
		}
		if val == "" || val == "0" {
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("removing stale %s: %w", name, err)
			}
			continue
		}
		if err := os.WriteFile(path, []byte(val), 0644); err != nil {
			return fmt.Errorf("writing %s: %w", name, err)
		}
	}
	return nil
}

func ReadCaseMetadata(dir, caseName string) (*CaseMetadata, error) {
	m := &CaseMetadata{CaseName: caseName}
	prefix := caseName + "."
	var err error

	m.HeadBranch, err = readString(dir, prefix+"eval-head-branch")
	if err != nil {
		return nil, err
	}
	m.BaseBranch, err = readString(dir, prefix+"eval-base-branch")
	if err != nil {
		return nil, err
	}
	m.FixtureHeadSHA, err = readString(dir, prefix+"fixture-head-sha")
	if err != nil {
		return nil, err
	}

	if n, err := readInt(dir, prefix+"pr-number"); err == nil {
		m.PRNumber = n
	}
	m.JiraIssueKey, _ = readString(dir, prefix+"jira-issue-key")
	m.Repo, _ = readString(dir, prefix+"eval-repo")
	m.BotLogin, _ = readString(dir, prefix+"bot-login")

	return m, nil
}

func WriteCaseList(dir string, cases []string) error {
	content := strings.Join(cases, "\n") + "\n"
	return os.WriteFile(filepath.Join(dir, "eval-cases"), []byte(content), 0644)
}

// EnsureCaseInList adds caseName to eval-cases if missing, preserving existing entries.
func EnsureCaseInList(dir, caseName string) error {
	existing, err := ReadCaseList(dir)
	if err != nil {
		if _, statErr := os.Stat(filepath.Join(dir, "eval-cases")); os.IsNotExist(statErr) {
			existing = nil
		} else {
			return err
		}
	}
	for _, c := range existing {
		if c == caseName {
			return nil
		}
	}
	return WriteCaseList(dir, append(existing, caseName))
}

func ReadCaseList(dir string) ([]string, error) {
	data, err := os.ReadFile(filepath.Join(dir, "eval-cases"))
	if err != nil {
		return nil, fmt.Errorf("reading eval-cases: %w", err)
	}
	var cases []string
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if line != "" {
			cases = append(cases, line)
		}
	}
	return cases, nil
}

func WriteFile(dir, name, content string) error {
	path, err := safeJoin(dir, name)
	if err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0644)
}

func ReadFile(dir, name string) (string, error) {
	return readString(dir, name)
}

func readString(dir, name string) (string, error) {
	path, err := safeJoin(dir, name)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("reading %s: %w", name, err)
	}
	return strings.TrimSpace(string(data)), nil
}

func readInt(dir, name string) (int, error) {
	s, err := readString(dir, name)
	if err != nil {
		return 0, err
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("parsing %s as int: %w", name, err)
	}
	return n, nil
}
