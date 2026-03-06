package config

import "gopkg.in/yaml.v3"

type RepoConfig struct {
	Trigger  TriggerConfig  `yaml:"trigger"`
	LLM      RepoLLMConfig  `yaml:"llm"`
	Executor RepoExecConfig `yaml:"executor"`
	Pipeline PipelineConfig `yaml:"pipeline"`
	Limits   LimitsConfig   `yaml:"limits"`
}

type TriggerConfig struct {
	Label string `yaml:"label"`
}

type RepoLLMConfig struct {
	Provider string `yaml:"provider"`
	Model    string `yaml:"model"`
}

type RepoExecConfig struct {
	Type string `yaml:"type"`
}

type PipelineConfig struct {
	ArchitectureReview bool               `yaml:"architecture_review"`
	SecurityReview     bool               `yaml:"security_review"`
	Verification       VerificationConfig `yaml:"verification"`
	Compliance         ComplianceConfig   `yaml:"compliance"`
	CodeReview         bool               `yaml:"code_review"`
	AutoMerge          bool               `yaml:"auto_merge"`
	CIDeploy           bool               `yaml:"ci_deploy"`
}

type VerificationConfig struct {
	Command string `yaml:"command"`
}

type ComplianceConfig struct {
	MaxDiffLines    int      `yaml:"max_diff_lines"`
	BlockedLicenses []string `yaml:"blocked_licenses"`
}

type LimitsConfig struct {
	MaxFilesChanged int     `yaml:"max_files_changed"`
	TimeoutMinutes  int     `yaml:"timeout_minutes"`
	BudgetUSD       float64 `yaml:"budget_usd"`
}

type repoConfigWrapper struct {
	NeuralForge RepoConfig `yaml:"neuralforge"`
}

func ParseRepoConfig(data []byte) (RepoConfig, error) {
	cfg := RepoConfig{
		Trigger: TriggerConfig{Label: "neuralforge"},
		Limits: LimitsConfig{
			MaxFilesChanged: 50,
			TimeoutMinutes:  30,
			BudgetUSD:       5.0,
		},
	}

	if len(data) == 0 {
		return cfg, nil
	}

	var wrapper repoConfigWrapper
	if err := yaml.Unmarshal(data, &wrapper); err != nil {
		return cfg, err
	}

	merged := wrapper.NeuralForge
	if merged.Trigger.Label == "" {
		merged.Trigger.Label = cfg.Trigger.Label
	}
	if merged.Limits.BudgetUSD == 0 {
		merged.Limits.BudgetUSD = cfg.Limits.BudgetUSD
	}
	if merged.Limits.MaxFilesChanged == 0 {
		merged.Limits.MaxFilesChanged = cfg.Limits.MaxFilesChanged
	}
	if merged.Limits.TimeoutMinutes == 0 {
		merged.Limits.TimeoutMinutes = cfg.Limits.TimeoutMinutes
	}

	return merged, nil
}
