package devskills

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"gitlab.informa.tools/ai/wizard/informa-wizard/internal/agents/claude"
	"gitlab.informa.tools/ai/wizard/informa-wizard/internal/model"
	"gitlab.informa.tools/ai/wizard/informa-wizard/internal/system"
)

// writeRepoSkill creates skills/<id>/SKILL.md inside the dev-skills repo at repoDir.
func writeRepoSkill(t *testing.T, repoDir, id, content string) {
	t.Helper()
	dir := filepath.Join(repoDir, "skills", id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll(%q): %v", dir, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile SKILL.md for %q: %v", id, err)
	}
}

// setupRepo creates the dev-skills repository layout under homeDir and returns
// the repo directory path: <homeDir>/.informa-wizard/dev-skills.
func setupRepo(t *testing.T, homeDir string, skillIDs []string) string {
	t.Helper()
	repoDir := filepath.Join(homeDir, ".informa-wizard", "dev-skills")
	for _, id := range skillIDs {
		writeRepoSkill(t, repoDir, id, "# "+id+"\nsome skill content\n")
	}
	return repoDir
}

// noSkillsAdapter is an adapter whose SupportsSkills() returns false.
type noSkillsAdapter struct{}

func (a noSkillsAdapter) Agent() model.AgentID    { return "no-skills" }
func (a noSkillsAdapter) Tier() model.SupportTier { return model.TierFull }
func (a noSkillsAdapter) Detect(_ context.Context, _ string) (bool, string, string, bool, error) {
	return false, "", "", false, nil
}
func (a noSkillsAdapter) SupportsAutoInstall() bool { return false }
func (a noSkillsAdapter) InstallCommand(_ system.PlatformProfile) ([][]string, error) {
	return nil, nil
}
func (a noSkillsAdapter) GlobalConfigDir(_ string) string  { return "" }
func (a noSkillsAdapter) SystemPromptDir(_ string) string  { return "" }
func (a noSkillsAdapter) SystemPromptFile(_ string) string { return "" }
func (a noSkillsAdapter) SkillsDir(_ string) string        { return "" }
func (a noSkillsAdapter) SettingsPath(_ string) string     { return "" }
func (a noSkillsAdapter) SystemPromptStrategy() model.SystemPromptStrategy {
	return model.StrategyFileReplace
}
func (a noSkillsAdapter) MCPStrategy() model.MCPStrategy          { return model.StrategyMergeIntoSettings }
func (a noSkillsAdapter) MCPConfigPath(_ string, _ string) string { return "" }
func (a noSkillsAdapter) SupportsOutputStyles() bool              { return false }
func (a noSkillsAdapter) OutputStyleDir(_ string) string          { return "" }
func (a noSkillsAdapter) SupportsSlashCommands() bool             { return false }
func (a noSkillsAdapter) CommandsDir(_ string) string             { return "" }
func (a noSkillsAdapter) SupportsSkills() bool                    { return false }
func (a noSkillsAdapter) SupportsSystemPrompt() bool              { return false }
func (a noSkillsAdapter) SupportsMCP() bool                       { return false }

// TestInjectSkills_AdapterNoSkills verifies that an adapter with
// SupportsSkills() == false returns an empty result and writes nothing.
func TestInjectSkills_AdapterNoSkills(t *testing.T) {
	homeDir := t.TempDir()
	setupRepo(t, homeDir, []string{"java-development"})

	result, err := InjectSkills(homeDir, noSkillsAdapter{}, []string{"java-development"})
	if err != nil {
		t.Fatalf("InjectSkills() error = %v, want nil", err)
	}
	if result.Changed {
		t.Errorf("result.Changed = true, want false for adapter without skills")
	}
	if len(result.Files) != 0 {
		t.Errorf("result.Files = %v, want empty", result.Files)
	}
}

// TestInjectSkills_SingleSkill verifies that a single skill is written to the
// correct destination path under the adapter's SkillsDir.
func TestInjectSkills_SingleSkill(t *testing.T) {
	homeDir := t.TempDir()
	setupRepo(t, homeDir, []string{"java-development"})
	adapter := claude.NewAdapter()

	result, err := InjectSkills(homeDir, adapter, []string{"java-development"})
	if err != nil {
		t.Fatalf("InjectSkills() error = %v", err)
	}
	if !result.Changed {
		t.Errorf("result.Changed = false, want true after first injection")
	}
	if len(result.Files) != 1 {
		t.Fatalf("len(result.Files) = %d, want 1", len(result.Files))
	}

	expectedDest := filepath.Join(homeDir, ".claude", "skills", "java-development", "SKILL.md")
	if result.Files[0] != expectedDest {
		t.Errorf("result.Files[0] = %q, want %q", result.Files[0], expectedDest)
	}
	if _, err := os.Stat(expectedDest); err != nil {
		t.Errorf("destination file not found: %v", err)
	}
}

// TestInjectSkills_MissingSkillDirectoryWarnsAndContinues verifies that when a
// skill's SKILL.md is absent from the repo, it is skipped with a log warning
// but injection continues for other skills and no error is returned.
func TestInjectSkills_MissingSkillDirectoryWarnsAndContinues(t *testing.T) {
	homeDir := t.TempDir()
	// Only set up java-development; go-testing is absent from the repo.
	setupRepo(t, homeDir, []string{"java-development"})
	adapter := claude.NewAdapter()

	result, err := InjectSkills(homeDir, adapter, []string{"java-development", "go-testing"})
	if err != nil {
		t.Fatalf("InjectSkills() error = %v, want nil (missing skill should not cause error)", err)
	}
	// Only the present skill should be written.
	if len(result.Files) != 1 {
		t.Errorf("len(result.Files) = %d, want 1 (missing skill skipped)", len(result.Files))
	}
	if result.Files[0] != filepath.Join(homeDir, ".claude", "skills", "java-development", "SKILL.md") {
		t.Errorf("result.Files[0] = %q, unexpected", result.Files[0])
	}
}

// TestInjectSkills_MultipleSkills verifies that three skills are all written.
func TestInjectSkills_MultipleSkills(t *testing.T) {
	homeDir := t.TempDir()
	ids := []string{"java-development", "java-testing", "human-documentation"}
	setupRepo(t, homeDir, ids)
	adapter := claude.NewAdapter()

	result, err := InjectSkills(homeDir, adapter, ids)
	if err != nil {
		t.Fatalf("InjectSkills() error = %v", err)
	}
	if len(result.Files) != 3 {
		t.Errorf("len(result.Files) = %d, want 3", len(result.Files))
	}
}

// TestInjectSkills_Idempotent verifies that calling InjectSkills twice with the
// same inputs sets Changed=false on the second call (content unchanged).
func TestInjectSkills_Idempotent(t *testing.T) {
	homeDir := t.TempDir()
	setupRepo(t, homeDir, []string{"java-development"})
	adapter := claude.NewAdapter()

	first, err := InjectSkills(homeDir, adapter, []string{"java-development"})
	if err != nil {
		t.Fatalf("InjectSkills() first error = %v", err)
	}
	if !first.Changed {
		t.Errorf("first.Changed = false, want true")
	}

	second, err := InjectSkills(homeDir, adapter, []string{"java-development"})
	if err != nil {
		t.Fatalf("InjectSkills() second error = %v", err)
	}
	if second.Changed {
		t.Errorf("second.Changed = true, want false (idempotent — content unchanged)")
	}
}

// TestInjectSkills_EmptySkillList verifies that passing an empty skill list
// returns an empty InjectionResult with no files written and no error.
func TestInjectSkills_EmptySkillList(t *testing.T) {
	homeDir := t.TempDir()
	adapter := claude.NewAdapter()

	result, err := InjectSkills(homeDir, adapter, []string{})
	if err != nil {
		t.Fatalf("InjectSkills() error = %v, want nil", err)
	}
	if result.Changed {
		t.Errorf("result.Changed = true, want false for empty skill list")
	}
	if len(result.Files) != 0 {
		t.Errorf("result.Files = %v, want empty", result.Files)
	}
}
