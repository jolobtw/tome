package notes

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Config struct {
	ActiveNotebook string          `json:"ActiveNotebook"`
	Notebooks      []NotebookEntry `json:"Notebooks"`
	SortMode       string          `json:"SortMode"` // "name" or "date"
}

type NotebookEntry struct {
	Name string `json:"Name"`
	Path string `json:"Path"`
}

// GetConfigPath returns the path to ~/.tome.json
func GetConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".tome.json")
}

// LoadConfig loads ~/.tome.json, migrating from ~/.terminal_notebook.json if needed
func LoadConfig() Config {
	home, _ := os.UserHomeDir()
	defaultNotesPath := filepath.Join(home, "Notes")

	defaultCfg := Config{
		ActiveNotebook: defaultNotesPath,
		Notebooks: []NotebookEntry{
			{Name: "Default", Path: defaultNotesPath},
		},
		SortMode: "name",
	}

	tomeCfgPath := GetConfigPath()
	if data, err := os.ReadFile(tomeCfgPath); err == nil {
		var cfg Config
		if err := json.Unmarshal(data, &cfg); err == nil && cfg.ActiveNotebook != "" {
			return cfg
		}
	}

	// Try migrating from legacy ~/.terminal_notebook.json
	legacyPath := filepath.Join(home, ".terminal_notebook.json")
	if data, err := os.ReadFile(legacyPath); err == nil {
		var cfg Config
		if err := json.Unmarshal(data, &cfg); err == nil && cfg.ActiveNotebook != "" {
			SaveConfig(cfg)
			return cfg
		}
	}

	// Ensure default directory exists
	_ = os.MkdirAll(defaultNotesPath, 0755)
	SaveConfig(defaultCfg)
	return defaultCfg
}

// SaveConfig writes config to ~/.tome.json
func SaveConfig(cfg Config) error {
	tomeCfgPath := GetConfigPath()
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(tomeCfgPath, data, 0644)
}
