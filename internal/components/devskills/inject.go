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

// InjectSkills copies the entire skill directory (SKILL.md + all reference files)
// from the cloned dev-skills repository to the agent's skill directory.
//
// The source directory for each skill is:
//
//	~/.informa-wizard/dev-skills/skills/<skillID>/
//
// The destination directory is adapter.SkillsDir(homeDir)/<skillID>/.
//
// If adapter.SupportsSkills() is false, returns an empty result without error.
// If a skill directory is not found in the repo, a warning is logged and the
// skill is skipped (no error).
func InjectSkills(homeDir string, adapter agents.Adapter, skillIDs []string) (InjectionResult, error) {
	if !adapter.SupportsSkills() {
		return InjectionResult{}, nil
	}

	repoDir := filepath.Join(homeDir, ".informa-wizard", "dev-skills")
	var files []string
	changed := false

	for _, skillID := range skillIDs {
		sourceDir := filepath.Join(repoDir, "skills", skillID)
		entries, err := os.ReadDir(sourceDir)
		if err != nil {
			if os.IsNotExist(err) {
				log.Printf("devskills: skipping %q — directory not found in repo", skillID)
				continue
			}
			return InjectionResult{}, fmt.Errorf("skill %s: read dir failed: %w", skillID, err)
		}

		destDir := filepath.Join(adapter.SkillsDir(homeDir), skillID)
		if err := os.MkdirAll(destDir, 0o755); err != nil {
			return InjectionResult{}, fmt.Errorf("skill %s: create dir failed: %w", skillID, err)
		}

		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			content, readErr := os.ReadFile(filepath.Join(sourceDir, entry.Name()))
			if readErr != nil {
				return InjectionResult{}, fmt.Errorf("skill %s/%s: read failed: %w", skillID, entry.Name(), readErr)
			}

			destPath := filepath.Join(destDir, entry.Name())
			result, writeErr := filemerge.WriteFileAtomic(destPath, content, 0o644)
			if writeErr != nil {
				return InjectionResult{}, fmt.Errorf("skill %s/%s: write failed: %w", skillID, entry.Name(), writeErr)
			}

			changed = changed || result.Changed
			files = append(files, destPath)
		}
	}

	return InjectionResult{Changed: changed, Files: files}, nil
}
