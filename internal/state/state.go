package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gitlab.informa.tools/ai/wizard/informa-wizard/internal/model"
)

// ErrInvalidState is returned by Read when state.json decodes successfully but
// fails schema validation (e.g., unknown agent IDs, unknown components).
// Callers can check via errors.Is(err, ErrInvalidState) to distinguish a
// malformed state from os.IsNotExist.
var ErrInvalidState = errors.New("invalid state file")

const stateDir = ".informa-wizard"
const stateFile = "state.json"

// InstallState holds the persisted user selections from the last install run.
type InstallState struct {
	InstalledAgents      []string `json:"installed_agents"`
	InstalledComponents  []string `json:"installed_components"`
	InstalledSkills      []string `json:"installed_skills,omitempty"`
	InstalledPreset      string   `json:"installed_preset,omitempty"`
	InstalledClaudePreset string  `json:"installed_claude_preset,omitempty"`
}

// Path returns the absolute path to the state file for the given home directory.
func Path(homeDir string) string {
	return filepath.Join(homeDir, stateDir, stateFile)
}

// knownAgents contains all valid agent IDs that can appear in state.json.
var knownAgents = map[string]struct{}{
	string(model.AgentClaudeCode):    {},
	string(model.AgentOpenCode):      {},
	string(model.AgentGeminiCLI):     {},
	string(model.AgentCursor):        {},
	string(model.AgentVSCodeCopilot): {},
	string(model.AgentCodex):         {},
	string(model.AgentAntigravity):   {},
	string(model.AgentWindsurf):      {},
}

// knownComponents contains all valid component IDs that can appear in state.json.
var knownComponents = map[string]struct{}{
	string(model.ComponentEngram):     {},
	string(model.ComponentSDD):        {},
	string(model.ComponentSkills):     {},
	string(model.ComponentContext7):   {},
	string(model.ComponentPersona):    {},
	string(model.ComponentPermission): {},
	string(model.ComponentGGA):        {},
	string(model.ComponentTheme):      {},
	string(model.ComponentMonday):     {},
	string(model.ComponentDevSkills):  {},
	string(model.ComponentDevAgents):  {},
}

// knownPresets contains all valid preset values that can appear in state.json.
// An empty string is allowed (means no preset was recorded).
var knownPresets = map[string]struct{}{
	"":                             {},
	string(model.PresetFull):       {},
	string(model.PresetEcosystemOnly): {},
	string(model.PresetMinimal):    {},
	string(model.PresetCustom):     {},
}

// knownClaudePresets contains all valid ClaudeModelPreset values.
// An empty string is allowed (means no preset was recorded).
var knownClaudePresets = map[string]struct{}{
	"":            {},
	"balanced":    {},
	"performance": {},
	"economy":     {},
	"custom":      {},
}

// Validate checks that the state contains only known agent IDs, component IDs,
// and preset values. Unknown fields are tolerated (JSON forward-compatibility),
// but unknown values in known fields indicate a corrupted or tampered file.
func (s InstallState) Validate() error {
	for _, id := range s.InstalledAgents {
		if _, ok := knownAgents[id]; !ok {
			return fmt.Errorf("unknown agent ID %q", id)
		}
	}
	for _, id := range s.InstalledComponents {
		if _, ok := knownComponents[id]; !ok {
			return fmt.Errorf("unknown component ID %q", id)
		}
	}
	if _, ok := knownPresets[s.InstalledPreset]; !ok {
		return fmt.Errorf("unknown preset %q", s.InstalledPreset)
	}
	if _, ok := knownClaudePresets[s.InstalledClaudePreset]; !ok {
		return fmt.Errorf("unknown claude preset %q", s.InstalledClaudePreset)
	}
	return nil
}

// Read reads and unmarshals the state file from the given home directory.
// Returns an error if the file does not exist or cannot be decoded.
// When the file decodes successfully but fails schema validation, Read returns
// a zero-value InstallState wrapped with ErrInvalidState so callers can
// distinguish malformed state from a missing file via errors.Is.
func Read(homeDir string) (InstallState, error) {
	data, err := os.ReadFile(Path(homeDir))
	if err != nil {
		return InstallState{}, err
	}
	var s InstallState
	if err := json.Unmarshal(data, &s); err != nil {
		return InstallState{}, err
	}
	if err := s.Validate(); err != nil {
		return InstallState{}, fmt.Errorf("%w: %v", ErrInvalidState, err)
	}
	return s, nil
}

// Write persists the install state.
func Write(homeDir string, agents, components, skills []string, preset string, claudePreset string) error {
	dir := filepath.Join(homeDir, stateDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	s := InstallState{
		InstalledAgents:       agents,
		InstalledComponents:   components,
		InstalledSkills:       skills,
		InstalledPreset:       preset,
		InstalledClaudePreset: claudePreset,
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(Path(homeDir), append(data, '\n'), 0o644)
}
