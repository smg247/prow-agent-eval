package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type EvalConfig struct {
	Name        string               `yaml:"name"`
	Description string               `yaml:"description"`
	Init        InitConfig           `yaml:"init"`
	Dataset     DatasetConfig        `yaml:"dataset"`
	Collect     CollectConfig        `yaml:"collect"`
	Judges      []JudgeConfig        `yaml:"judges"`
	Thresholds  map[string]Threshold `yaml:"thresholds"`
}

type InitConfig struct {
	Repo string `yaml:"repo"`
}

type DatasetConfig struct {
	Path   string `yaml:"path"`
	Schema string `yaml:"schema"`
}

type CollectConfig struct {
	BotReplies         bool `yaml:"bot_replies"`
	CommentMap         bool `yaml:"comment_map"`
	BuildResult        bool `yaml:"build_result"`
	TestResult         bool `yaml:"test_result"`
	ExpectedBranchDiff bool `yaml:"expected_branch_diff"`
}

type JudgeConfig struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	// Type selects a builtin judge. If empty, Name is used as the type.
	Type string `yaml:"type"`
}

type Threshold struct {
	MinPassRate *float64 `yaml:"min_pass_rate"`
}

type CaseInput struct {
	JiraKey        string `yaml:"jira_key"`
	BaseBranch     string `yaml:"base_branch"`
	HeadBranch     string `yaml:"head_branch"`
	ExpectedBranch string `yaml:"expected_branch"`
	Repo           string `yaml:"repo"`
}

type CaseAnnotations map[string]any

type Case struct {
	Name        string
	Dir         string
	Input       CaseInput
	Annotations CaseAnnotations
}

func Load(path string) (*EvalConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config: %w", err)
	}
	var cfg EvalConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}
	return &cfg, nil
}

func datasetDir(configDir, datasetPath string) string {
	if filepath.IsAbs(datasetPath) {
		return datasetPath
	}
	return filepath.Join(configDir, datasetPath)
}

func LoadCase(configDir, datasetPath, caseName string) (*Case, error) {
	caseDir := filepath.Join(datasetDir(configDir, datasetPath), caseName)

	inputData, err := os.ReadFile(filepath.Join(caseDir, "input.yaml"))
	if err != nil {
		return nil, fmt.Errorf("reading input.yaml for case %s: %w", caseName, err)
	}
	var input CaseInput
	if err := yaml.Unmarshal(inputData, &input); err != nil {
		return nil, fmt.Errorf("parsing input.yaml for case %s: %w", caseName, err)
	}

	var annotations CaseAnnotations
	annotationsPath := filepath.Join(caseDir, "annotations.yaml")
	if data, err := os.ReadFile(annotationsPath); err == nil {
		if err := yaml.Unmarshal(data, &annotations); err != nil {
			return nil, fmt.Errorf("parsing annotations.yaml for case %s: %w", caseName, err)
		}
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("reading annotations.yaml for case %s: %w", caseName, err)
	}

	return &Case{
		Name:        caseName,
		Dir:         caseDir,
		Input:       input,
		Annotations: annotations,
	}, nil
}

func ListCases(configDir, datasetPath string) ([]string, error) {
	casesDir := datasetDir(configDir, datasetPath)
	entries, err := os.ReadDir(casesDir)
	if err != nil {
		return nil, fmt.Errorf("listing cases in %s: %w", casesDir, err)
	}
	var cases []string
	for _, e := range entries {
		if e.IsDir() {
			if _, err := os.Stat(filepath.Join(casesDir, e.Name(), "input.yaml")); err == nil {
				cases = append(cases, e.Name())
			}
		}
	}
	return cases, nil
}
