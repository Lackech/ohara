package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"go.yaml.in/yaml/v3"
)

// HubConfig is the .ohara.yaml that lives in the docs hub repo
type HubConfig struct {
	Name     string      `yaml:"name"`
	Repos    []RepoEntry `yaml:"repos"`
	DocsRepo *DocsRepo   `yaml:"docs_repo,omitempty"`
}

type RepoEntry struct {
	Name   string `yaml:"name"`
	Path   string `yaml:"path"`   // relative path to the code repo
	Remote string `yaml:"remote"` // optional git remote URL
}

type DocsRepo struct {
	Remote string `yaml:"remote,omitempty"` // optional git remote for the docs hub
}

const hubConfigFile = ".ohara.yaml"

// FindHubRoot walks up from dir looking for .ohara.yaml
func FindHubRoot(dir string) (string, error) {
	current, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}

	for {
		configPath := filepath.Join(current, hubConfigFile)
		if _, err := os.Stat(configPath); err == nil {
			return current, nil
		}

		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}

	// Also check common subdirectory names
	absDir, _ := filepath.Abs(dir)
	for _, sub := range []string{"ohara-docs", "docs"} {
		configPath := filepath.Join(absDir, sub, hubConfigFile)
		if _, err := os.Stat(configPath); err == nil {
			return filepath.Join(absDir, sub), nil
		}
	}

	return "", fmt.Errorf("no .ohara.yaml found (run 'ohara init' first)")
}

// LoadHubConfig reads .ohara.yaml from the given directory
func LoadHubConfig(hubRoot string) (*HubConfig, error) {
	data, err := os.ReadFile(filepath.Join(hubRoot, hubConfigFile))
	if err != nil {
		return nil, fmt.Errorf("failed to read .ohara.yaml: %w", err)
	}

	var config HubConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse .ohara.yaml: %w", err)
	}

	return &config, nil
}

// SaveHubConfig writes .ohara.yaml to the given directory
func SaveHubConfig(hubRoot string, config *HubConfig) error {
	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	return os.WriteFile(filepath.Join(hubRoot, hubConfigFile), data, 0644)
}

// ResolveRepoPath resolves a repo entry's path relative to the hub root
func ResolveRepoPath(hubRoot string, repo RepoEntry) string {
	if filepath.IsAbs(repo.Path) {
		return repo.Path
	}
	return filepath.Join(hubRoot, repo.Path)
}
