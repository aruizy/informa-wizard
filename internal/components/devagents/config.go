package devagents

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"gitlab.informa.tools/ai/wizard/informa-wizard/internal/components/filemerge"
)

// Config holds the dev-agents component configuration stored in
// ~/.informa-wizard/dev-agents.json.
type Config struct {
	RepoURL         string   `json:"repo_url"`
	InstalledAgents []string `json:"installed_agents"`
}

// ReadConfig reads the dev-agents config from homeDir.
// If the file does not exist, returns (Config{}, nil) — callers detect absence via empty RepoURL.
// If the file exists but is malformed JSON, returns a non-nil error.
func ReadConfig(homeDir string) (Config, error) {
	path := filepath.Join(homeDir, ".informa-wizard", "dev-agents.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Config{}, nil
		}
		return Config{}, fmt.Errorf("dev-agents.json: %w", err)
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("dev-agents.json: %w", err)
	}
	return cfg, nil
}

// WriteConfig serializes cfg to JSON and writes it atomically to
// ~/.informa-wizard/dev-agents.json under homeDir.
func WriteConfig(homeDir string, cfg Config) error {
	dir := filepath.Join(homeDir, ".informa-wizard")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("dev-agents.json: create directory: %w", err)
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("dev-agents.json: marshal: %w", err)
	}

	path := filepath.Join(dir, "dev-agents.json")
	if _, err := filemerge.WriteFileAtomic(path, data, 0o644); err != nil {
		return fmt.Errorf("dev-agents.json: write: %w", err)
	}
	return nil
}
