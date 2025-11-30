package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// --- Constants ---
const (
	configFileName = "colonsh.json"
)

// --- Struct Definitions (Exported for main.go access) ---

// Config holds the top-level configuration structure.
type Config struct {
	Aliases     []Alias      `json:"aliases"`
	ProjectDirs []ProjectDir `json:"project_dirs"`
	GitRepos    []GitRepo    `json:"git_repos"`
	OpenCmd     string       `json:"open_cmd,omitempty"`
}

// Alias defines a custom command alias.
type Alias struct {
	Name string `json:"name"`
	Cmd  string `json:"cmd"`
}

// ProjectDir defines a root directory to scan for Git repositories.
type ProjectDir struct {
	Path    string   `json:"path"`
	Exclude []string `json:"exclude"`
}

// GitRepo defines actions and specific settings for a repository identified by its slug.
type GitRepo struct {
	Slug    string       `json:"slug"`
	Name    string       `json:"name"`
	OpenCmd string       `json:"open_cmd,omitempty"`
	Actions []RepoAction `json:"actions"`
}

// RepoAction defines a single action available within a GitRepo.
type RepoAction struct {
	Name string `json:"name"`
	Cmd  string `json:"cmd"`
	Dir  string `json:"dir,omitempty"`
}

// --- Path and Loading Logic ---

// colonConfigPath returns the determined path to the colonsh config file (~/colonsh.json).
func colonConfigPath() (string, error) {
	// Feature removed: No environment variable check. Path is strictly ~/colonsh.json.

	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, configFileName), nil
}

// loadOrInitConfig loads the config file or creates a default one if it doesn't exist.
func loadOrInitConfig(path string) (*Config, error) {
	if _, err := os.Stat(path); err == nil {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		var cfg Config
		if err := json.Unmarshal(data, &cfg); err != nil {
			return nil, fmt.Errorf("failed to parse %s: %w", path, err)
		}
		return &cfg, nil
	}

	// Create default config if not found
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	cfg := defaultConfig()
	data, err := json.MarshalIndent(cfg, "", "    ")
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return nil, err
	}

	fmt.Println("colonsh: no config found, created new one at", path)
	fmt.Println("colonsh: edit the file to add your projects and actions.")
	return cfg, nil
}

// defaultConfig generates a basic, example Config structure.
func defaultConfig() *Config {
	return &Config{
		OpenCmd: "code .",
		Aliases: []Alias{
			{
				Name: "config",
				Cmd:  fmt.Sprintf("code ~/%s", configFileName),
			},
			{Name: "c", Cmd: "code ."},
			{Name: "source", Cmd: "source ~/.zshrc"},
		},
		ProjectDirs: []ProjectDir{
			{
				Path: "~/MyProjects",
				Exclude: []string{
					"bin",
					"notes",
				},
			},
		},
		GitRepos: []GitRepo{
			{
				Slug: "octocat/Hello-World",
				Name: "Hello-World",
				Actions: []RepoAction{
					{Name: "PRs", Cmd: "open https://github.com/octocat/Hello-World/pulls"},
				},
			},
		},
	}
}
