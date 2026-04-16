package devskills

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestReadConfig_MissingFile verifies that a missing dev-skills.json returns
// (Config{}, nil) — an empty config with no error.
func TestReadConfig_MissingFile(t *testing.T) {
	homeDir := t.TempDir()

	cfg, err := ReadConfig(homeDir)
	if err != nil {
		t.Fatalf("ReadConfig() error = %v, want nil", err)
	}
	if cfg.RepoURL != "" {
		t.Errorf("cfg.RepoURL = %q, want empty (file absent detection)", cfg.RepoURL)
	}
	if len(cfg.InstalledSkills) != 0 {
		t.Errorf("cfg.InstalledSkills = %v, want empty", cfg.InstalledSkills)
	}
}

// TestReadConfig_MalformedJSON verifies that a malformed JSON file returns a
// non-nil error whose message contains "dev-skills.json:".
func TestReadConfig_MalformedJSON(t *testing.T) {
	homeDir := t.TempDir()

	dir := filepath.Join(homeDir, ".informa-wizard")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "dev-skills.json"), []byte("{invalid json"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	_, err := ReadConfig(homeDir)
	if err == nil {
		t.Fatal("ReadConfig() error = nil, want non-nil for malformed JSON")
	}
	if !strings.Contains(err.Error(), "dev-skills.json:") {
		t.Errorf("ReadConfig() error = %q, want prefix %q", err.Error(), "dev-skills.json:")
	}
}

// TestReadConfig_ValidFile verifies round-trip read of a known config.
func TestReadConfig_ValidFile(t *testing.T) {
	homeDir := t.TempDir()

	dir := filepath.Join(homeDir, ".informa-wizard")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	raw := `{"repo_url":"https://example.com/skills.git","installed_skills":["java-development","go-testing"]}`
	if err := os.WriteFile(filepath.Join(dir, "dev-skills.json"), []byte(raw), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	cfg, err := ReadConfig(homeDir)
	if err != nil {
		t.Fatalf("ReadConfig() error = %v", err)
	}
	if cfg.RepoURL != "https://example.com/skills.git" {
		t.Errorf("cfg.RepoURL = %q, want https://example.com/skills.git", cfg.RepoURL)
	}
	if len(cfg.InstalledSkills) != 2 {
		t.Fatalf("len(InstalledSkills) = %d, want 2", len(cfg.InstalledSkills))
	}
	if cfg.InstalledSkills[0] != "java-development" {
		t.Errorf("InstalledSkills[0] = %q, want java-development", cfg.InstalledSkills[0])
	}
}

// TestWriteConfig_AtomicWrite verifies that WriteConfig creates a valid JSON
// file containing the installed_skills field.
func TestWriteConfig_AtomicWrite(t *testing.T) {
	homeDir := t.TempDir()

	cfg := Config{
		RepoURL:         "https://example.com/skills.git",
		InstalledSkills: []string{"java-development"},
	}
	if err := WriteConfig(homeDir, cfg); err != nil {
		t.Fatalf("WriteConfig() error = %v", err)
	}

	path := filepath.Join(homeDir, ".informa-wizard", "dev-skills.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "installed_skills") {
		t.Errorf("written file does not contain 'installed_skills'; content: %s", content)
	}
}

// TestWriteConfig_RoundTrip verifies that WriteConfig followed by ReadConfig
// returns the original Config with InstalledSkills intact.
func TestWriteConfig_RoundTrip(t *testing.T) {
	homeDir := t.TempDir()

	original := Config{
		RepoURL:         "https://gitlab.example.com/org/dev-skills.git",
		InstalledSkills: []string{"java-development", "go-testing", "human-documentation"},
	}

	if err := WriteConfig(homeDir, original); err != nil {
		t.Fatalf("WriteConfig() error = %v", err)
	}

	got, err := ReadConfig(homeDir)
	if err != nil {
		t.Fatalf("ReadConfig() error = %v", err)
	}

	if got.RepoURL != original.RepoURL {
		t.Errorf("RepoURL = %q, want %q", got.RepoURL, original.RepoURL)
	}
	if len(got.InstalledSkills) != len(original.InstalledSkills) {
		t.Fatalf("len(InstalledSkills) = %d, want %d", len(got.InstalledSkills), len(original.InstalledSkills))
	}
	for i, id := range original.InstalledSkills {
		if got.InstalledSkills[i] != id {
			t.Errorf("InstalledSkills[%d] = %q, want %q", i, got.InstalledSkills[i], id)
		}
	}
}

// TestReadConfig_DefaultRepoURL verifies the DefaultRepoURL constant value.
func TestReadConfig_DefaultRepoURL(t *testing.T) {
	const want = "https://gitlab.informa.tools/ai/skills/dev-skills.git"
	if DefaultRepoURL != want {
		t.Errorf("DefaultRepoURL = %q, want %q", DefaultRepoURL, want)
	}
}
