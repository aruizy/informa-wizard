package skills

import (
	"fmt"
	"io/fs"
	"log"
	"path/filepath"
	"strings"

	"gitlab.informa.tools/ai/wizard/informa-wizard/internal/agents"
	"gitlab.informa.tools/ai/wizard/informa-wizard/internal/assets"
	"gitlab.informa.tools/ai/wizard/informa-wizard/internal/components/filemerge"
	"gitlab.informa.tools/ai/wizard/informa-wizard/internal/model"
)

// isSDDSkill reports whether a skill ID belongs to the SDD orchestrator suite.
// SDD skills are installed by the SDD component; the skills component skips
// them to prevent duplicate writes when both components are selected.
func isSDDSkill(id model.SkillID) bool {
	return strings.HasPrefix(string(id), "sdd-")
}

type InjectionResult struct {
	Changed bool
	Files   []string
	Skipped []model.SkillID
}

// Inject writes the embedded SKILL.md files for each requested skill
// to the correct directory for the given agent adapter.
//
// The skills directory is determined by adapter.SkillsDir(), removing
// the need for any agent-specific switch statements.
//
// SDD skills (those whose IDs begin with "sdd-") are intentionally skipped
// here because the SDD component installs them as part of its own injection.
// This prevents a write conflict when both components are selected together.
//
// Individual skill failures (e.g., missing embedded asset) are logged
// and skipped rather than aborting the entire operation.
func Inject(homeDir string, adapter agents.Adapter, skillIDs []model.SkillID) (InjectionResult, error) {
	if !adapter.SupportsSkills() {
		return InjectionResult{Skipped: skillIDs}, nil
	}

	skillDir := adapter.SkillsDir(homeDir)
	if skillDir == "" {
		return InjectionResult{Skipped: skillIDs}, nil
	}

	paths := make([]string, 0, len(skillIDs))
	skipped := make([]model.SkillID, 0)
	changed := false

	for _, id := range skillIDs {
		// SDD skills are written by the SDD component — skip to avoid conflicts.
		if isSDDSkill(id) {
			continue
		}

		assetDir := "skills/" + string(id)
		if _, statErr := fs.Stat(assets.FS, assetDir); statErr != nil {
			log.Printf("skills: skipping %q — embedded directory not found: %v", id, statErr)
			skipped = append(skipped, id)
			continue
		}

		destDir := filepath.Join(skillDir, string(id))

		walkErr := fs.WalkDir(assets.FS, assetDir, func(assetPath string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if d.IsDir() {
				return nil
			}

			data, readErr := fs.ReadFile(assets.FS, assetPath)
			if readErr != nil {
				return fmt.Errorf("skill %q: read file %q failed: %w", id, assetPath, readErr)
			}
			if len(data) == 0 && d.Name() == "SKILL.md" {
				return fmt.Errorf("skill %q: SKILL.md exists but is empty — build may be corrupt", id)
			}

			// Compute path relative to assetDir so subdirectory structure is preserved.
			relPath, relErr := filepath.Rel(assetDir, filepath.FromSlash(assetPath))
			if relErr != nil {
				return fmt.Errorf("skill %q: resolve relative path %q failed: %w", id, assetPath, relErr)
			}

			path := filepath.Join(destDir, relPath)
			writeResult, writeErr := filemerge.WriteFileAtomic(path, data, 0o644)
			if writeErr != nil {
				return fmt.Errorf("skill %q: write %q failed: %w", id, relPath, writeErr)
			}

			changed = changed || writeResult.Changed
			paths = append(paths, path)
			return nil
		})
		if walkErr != nil {
			return InjectionResult{}, walkErr
		}
	}

	return InjectionResult{Changed: changed, Files: paths, Skipped: skipped}, nil
}

// SkillPathForAgent returns the filesystem path where a skill file would be written.
func SkillPathForAgent(homeDir string, adapter agents.Adapter, id model.SkillID) string {
	skillDir := adapter.SkillsDir(homeDir)
	if skillDir == "" {
		return ""
	}
	return filepath.Join(skillDir, string(id), "SKILL.md")
}
