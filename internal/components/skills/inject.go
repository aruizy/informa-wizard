package skills

import (
	"fmt"
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
		entries, readDirErr := assets.FS.ReadDir(assetDir)
		if readDirErr != nil {
			log.Printf("skills: skipping %q — embedded directory not found: %v", id, readDirErr)
			skipped = append(skipped, id)
			continue
		}

		destDir := filepath.Join(skillDir, string(id))

		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}

			assetPath := assetDir + "/" + entry.Name()
			data, readErr := assets.FS.ReadFile(assetPath)
			if readErr != nil {
				return InjectionResult{}, fmt.Errorf("skill %q: read file %q failed: %w", id, entry.Name(), readErr)
			}
			if len(data) == 0 && entry.Name() == "SKILL.md" {
				return InjectionResult{}, fmt.Errorf("skill %q: SKILL.md exists but is empty — build may be corrupt", id)
			}

			path := filepath.Join(destDir, entry.Name())
			writeResult, writeErr := filemerge.WriteFileAtomic(path, data, 0o644)
			if writeErr != nil {
				return InjectionResult{}, fmt.Errorf("skill %q: write %q failed: %w", id, entry.Name(), writeErr)
			}

			changed = changed || writeResult.Changed
			paths = append(paths, path)
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
