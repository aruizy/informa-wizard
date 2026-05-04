package cli

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"gitlab.informa.tools/ai/wizard/informa-wizard/internal/components/devagents"
	"gitlab.informa.tools/ai/wizard/informa-wizard/internal/components/devskills"
	"gitlab.informa.tools/ai/wizard/informa-wizard/internal/logger"
	"gitlab.informa.tools/ai/wizard/informa-wizard/internal/model"
	"gitlab.informa.tools/ai/wizard/informa-wizard/internal/state"
)

// UninstallComponent removes a specific component: deletes its files and updates state.json.
// Returns an error if state.json cannot be read or written.
func UninstallComponent(homeDir string, componentID model.ComponentID) error {
	st, err := state.Read(homeDir)
	if err != nil && !os.IsNotExist(err) {
		// Tolerate corrupt state by treating uninstall as a recovery path:
		// proceed with an empty state so the user can still clean files and
		// reset persisted metadata. Only hard-fail on unexpected read errors.
		if errors.Is(err, state.ErrInvalidState) {
			logger.Warn("uninstall: state.json is invalid (%v); proceeding with empty state", err)
			st = state.InstallState{}
		} else {
			return fmt.Errorf("read state: %w", err)
		}
	}

	// Verify the component is actually installed.
	if !hasComponentInState(st.InstalledComponents, string(componentID)) {
		return fmt.Errorf("component %q is not installed", componentID)
	}

	// Resolve agent adapters for path lookup.
	agentAdapters := resolveAdapters(agentIDsFromStrings(st.InstalledAgents))

	// Build the selection needed by componentPaths.
	sel := buildUninstallSelection(homeDir, st)

	// Delete files belonging to this component for each installed agent.
	// Collect any failures but ALWAYS update state.json after the loop —
	// otherwise state.json would say a component is still installed when many
	// files have already been deleted, leaving the system inconsistent. The
	// joined error (if any) is still returned so the caller can surface which
	// files require manual cleanup.
	paths := componentPaths(homeDir, sel, agentAdapters, componentID)
	var removeErrs []error
	for _, p := range paths {
		if err := removeIfExists(p); err != nil {
			logger.Warn("uninstall %s: remove %s: %v", componentID, p, err)
			removeErrs = append(removeErrs, fmt.Errorf("remove %s: %w", p, err))
		}
	}

	// For dev-skills and dev-agents, also delete their companion JSON config files
	// and the cloned repo directory.
	switch componentID {
	case model.ComponentDevSkills:
		jsonPath := filepath.Join(homeDir, ".informa-wizard", "dev-skills.json")
		_ = removeIfExists(jsonPath)
		repoDir := filepath.Join(homeDir, ".informa-wizard", "dev-skills")
		_ = os.RemoveAll(repoDir)
	case model.ComponentDevAgents:
		jsonPath := filepath.Join(homeDir, ".informa-wizard", "dev-agents.json")
		_ = removeIfExists(jsonPath)
		repoDir := filepath.Join(homeDir, ".informa-wizard", "dev-agents")
		_ = os.RemoveAll(repoDir)
	}

	// For components with multi-file footprints under per-agent skill roots,
	// prune any leftover empty subdirectories so users do not see stale folders.
	// Components with a flat single-file footprint (Permission, Theme, Context7,
	// MCP-style components) are skipped because they don't own a directory tree.
	if componentID == model.ComponentDevSkills || componentID == model.ComponentSkills {
		for _, adapter := range agentAdapters {
			if !adapter.SupportsSkills() {
				continue
			}
			skillsRoot := adapter.SkillsDir(homeDir)
			if skillsRoot == "" {
				continue
			}
			pruneEmptyDirs(skillsRoot)
		}
	}

	// Remove the component from the installed list and persist.
	updated := removeComponent(st.InstalledComponents, string(componentID))
	if writeErr := state.Write(homeDir, st.InstalledAgents, updated, st.InstalledSkills, st.InstalledPreset, st.InstalledClaudePreset); writeErr != nil {
		// State write failed — return both errors so the caller sees the full picture.
		if len(removeErrs) > 0 {
			return fmt.Errorf("uninstall %s: %w; additionally, state write failed: %v", componentID, errors.Join(removeErrs...), writeErr)
		}
		return writeErr
	}

	if len(removeErrs) > 0 {
		return fmt.Errorf("uninstall %s: %d file(s) could not be removed: %w — state has been updated, manual cleanup may be required", componentID, len(removeErrs), errors.Join(removeErrs...))
	}
	return nil
}

// pruneEmptyDirs walks root and removes empty subdirectories. The root itself
// is left in place (other components may share it). Removal failures are
// silently ignored — directories with content stay where they are.
func pruneEmptyDirs(root string) {
	if root == "" {
		return
	}
	if _, err := os.Stat(root); err != nil {
		return
	}
	var dirs []string
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() && path != root {
			dirs = append(dirs, path)
		}
		return nil
	})
	// Deepest first so children are removed before parents.
	sort.Slice(dirs, func(i, j int) bool { return len(dirs[i]) > len(dirs[j]) })
	for _, d := range dirs {
		_ = os.Remove(d) // fails (and is ignored) if not empty
	}
}

// removeIfExists removes a file if it exists; a missing file is not an error.
func removeIfExists(path string) error {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// removeComponent returns a new slice with every occurrence of target removed.
func removeComponent(components []string, target string) []string {
	result := make([]string, 0, len(components))
	for _, c := range components {
		if c != target {
			result = append(result, c)
		}
	}
	return result
}

// agentIDsFromStrings converts a slice of string agent IDs to []model.AgentID.
func agentIDsFromStrings(ids []string) []model.AgentID {
	result := make([]model.AgentID, 0, len(ids))
	for _, id := range ids {
		result = append(result, model.AgentID(id))
	}
	return result
}

// buildUninstallSelection builds a model.Selection from persisted state with enough
// detail for componentPaths to return the correct file list during uninstall.
func buildUninstallSelection(homeDir string, st state.InstallState) model.Selection {
	components := make([]model.ComponentID, 0, len(st.InstalledComponents))
	for _, c := range st.InstalledComponents {
		components = append(components, model.ComponentID(c))
	}
	agentIDs := agentIDsFromStrings(st.InstalledAgents)

	// Populate dev-skill / dev-agent selections from companion JSON configs so
	// that componentPaths returns the correct per-skill and per-agent file paths.
	var devSkillSelections []string
	if dsc, err := devskills.ReadConfig(homeDir); err == nil {
		devSkillSelections = dsc.InstalledSkills
	}
	var devAgentSelections []string
	if dac, err := devagents.ReadConfig(homeDir); err == nil {
		devAgentSelections = dac.InstalledAgents
	}

	return model.Selection{
		Agents:             agentIDs,
		Components:         components,
		DevSkillSelections: devSkillSelections,
		DevAgentSelections: devAgentSelections,
	}
}
