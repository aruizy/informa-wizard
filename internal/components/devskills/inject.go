package devskills

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"gitlab.informa.tools/ai/wizard/informa-wizard/internal/agents"
	"gitlab.informa.tools/ai/wizard/informa-wizard/internal/components/filemerge"
)

// InjectionResult reports the outcome of an InjectSkills call.
type InjectionResult struct {
	Changed bool
	Files   []string
}

// InjectSkills copies SKILL.md files from the cloned dev-skills repository to
// the agent's skill directory for each skillID in skillIDs.
//
// The source path for each skill is:
//
//	~/.informa-wizard/dev-skills/skills/<skillID>/SKILL.md
//
// The destination path is determined by adapter.SkillsDir(homeDir).
//
// If adapter.SupportsSkills() is false, returns an empty result without error.
// If a skill's SKILL.md is not found in the repo, a warning is logged and the
// skill is skipped (no error).
func InjectSkills(homeDir string, adapter agents.Adapter, skillIDs []string) (InjectionResult, error) {
	if !adapter.SupportsSkills() {
		return InjectionResult{}, nil
	}

	repoDir := filepath.Join(homeDir, ".informa-wizard", "dev-skills")
	var files []string
	changed := false

	for _, skillID := range skillIDs {
		sourcePath := filepath.Join(repoDir, "skills", skillID, "SKILL.md")
		content, err := os.ReadFile(sourcePath)
		if err != nil {
			if os.IsNotExist(err) {
				log.Printf("devskills: skipping %q — SKILL.md not found in repo", skillID)
				continue
			}
			return InjectionResult{}, fmt.Errorf("skill %s: read failed: %w", skillID, err)
		}

		destPath := filepath.Join(adapter.SkillsDir(homeDir), skillID, "SKILL.md")
		result, writeErr := filemerge.WriteFileAtomic(destPath, content, 0o644)
		if writeErr != nil {
			return InjectionResult{}, fmt.Errorf("skill %s: write failed: %w", skillID, writeErr)
		}

		changed = changed || result.Changed
		files = append(files, destPath)
	}

	return InjectionResult{Changed: changed, Files: files}, nil
}
