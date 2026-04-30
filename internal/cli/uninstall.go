package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"gitlab.informa.tools/ai/wizard/informa-wizard/internal/components/devagents"
	"gitlab.informa.tools/ai/wizard/informa-wizard/internal/components/devskills"
	"gitlab.informa.tools/ai/wizard/informa-wizard/internal/model"
	"gitlab.informa.tools/ai/wizard/informa-wizard/internal/state"
)

// UninstallComponent removes a specific component: deletes its files and updates state.json.
// Returns an error if state.json cannot be read or written.
func UninstallComponent(homeDir string, componentID model.ComponentID) error {
	st, err := state.Read(homeDir)
	if err != nil {
		return fmt.Errorf("read state: %w", err)
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
	paths := componentPaths(homeDir, sel, agentAdapters, componentID)
	for _, p := range paths {
		if err := removeIfExists(p); err != nil {
			// Non-fatal: log but continue to clean up remaining files.
			fmt.Fprintf(os.Stderr, "WARNING: uninstall %s: remove %s: %v\n", componentID, p, err)
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

	// Remove the component from the installed list and persist.
	updated := removeComponent(st.InstalledComponents, string(componentID))
	return state.Write(homeDir, st.InstalledAgents, updated, st.InstalledSkills, st.InstalledPreset, st.InstalledClaudePreset)
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
