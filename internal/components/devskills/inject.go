package devskills

import (
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"

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
		if _, err := os.Stat(sourceDir); err != nil {
			if os.IsNotExist(err) {
				log.Printf("devskills: skipping %q — directory not found in repo", skillID)
				continue
			}
			return InjectionResult{}, fmt.Errorf("skill %s: stat dir failed: %w", skillID, err)
		}

		destDir := filepath.Join(adapter.SkillsDir(homeDir), skillID)
		if err := os.MkdirAll(destDir, 0o755); err != nil {
			return InjectionResult{}, fmt.Errorf("skill %s: create dir failed: %w", skillID, err)
		}

		walkErr := filepath.WalkDir(sourceDir, func(path string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if d.IsDir() {
				if path != sourceDir && strings.HasPrefix(d.Name(), ".") {
					return filepath.SkipDir
				}
				return nil
			}
			if d.Name() == ".DS_Store" || d.Name() == "Thumbs.db" {
				return nil
			}
			content, readErr := os.ReadFile(path)
			if readErr != nil {
				return fmt.Errorf("skill %s: read %s failed: %w", skillID, path, readErr)
			}
			relPath, relErr := filepath.Rel(sourceDir, path)
			if relErr != nil {
				return fmt.Errorf("skill %s: resolve relative path %s failed: %w", skillID, path, relErr)
			}
			destPath := filepath.Join(destDir, relPath)
			result, writeErr := filemerge.WriteFileAtomic(destPath, content, 0o644)
			if writeErr != nil {
				return fmt.Errorf("skill %s/%s: write failed: %w", skillID, relPath, writeErr)
			}
			changed = changed || result.Changed
			files = append(files, destPath)
			return nil
		})
		if walkErr != nil {
			return InjectionResult{}, walkErr
		}
	}

	return InjectionResult{Changed: changed, Files: files}, nil
}
