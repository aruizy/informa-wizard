package devskills

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"gitlab.informa.tools/ai/wizard/informa-wizard/internal/components/filemerge"
)

// DefaultRepoURL is the default HTTPS git URL for the dev-skills repository.
const DefaultRepoURL = "https://gitlab.informa.tools/ai/skills/dev-skills.git"

// Config holds the dev-skills component configuration stored in
// ~/.informa-wizard/dev-skills.json.
type Config struct {
	RepoURL         string   `json:"repo_url"`
	InstalledSkills []string `json:"installed_skills"`
}

// ReadConfig reads the dev-skills config from homeDir.
// If the file does not exist, returns (Config{}, nil) — callers detect absence via empty RepoURL.
// If the file exists but is malformed JSON, returns a non-nil error.
func ReadConfig(homeDir string) (Config, error) {
	path := filepath.Join(homeDir, ".informa-wizard", "dev-skills.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Config{}, nil
		}
		return Config{}, fmt.Errorf("dev-skills.json: %w", err)
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("dev-skills.json: %w", err)
	}
	return cfg, nil
}

// WriteConfig serializes cfg to JSON and writes it atomically to
// ~/.informa-wizard/dev-skills.json under homeDir.
func WriteConfig(homeDir string, cfg Config) error {
	dir := filepath.Join(homeDir, ".informa-wizard")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("dev-skills.json: create directory: %w", err)
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("dev-skills.json: marshal: %w", err)
	}

	path := filepath.Join(dir, "dev-skills.json")
	if _, err := filemerge.WriteFileAtomic(path, data, 0o644); err != nil {
		return fmt.Errorf("dev-skills.json: write: %w", err)
	}
	return nil
}
