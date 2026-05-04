package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gitlab.informa.tools/ai/wizard/informa-wizard/internal/components/devagents"
	"gitlab.informa.tools/ai/wizard/informa-wizard/internal/components/devskills"
	"gitlab.informa.tools/ai/wizard/informa-wizard/internal/state"
)

type mondayStatusJSON struct {
	Token   string `json:"token"`
	BoardID string `json:"board_id"`
}

// RunStatus prints the current installation state to stdout in plain text.
func RunStatus(homeDir string) error {
	fmt.Println("Informa Wizard — Installation Status")
	fmt.Println()

	// State: agents, components, skills, preset.
	s, stateErr := state.Read(homeDir)
	if stateErr != nil && !os.IsNotExist(stateErr) {
		fmt.Printf("Warning: could not read state.json: %v\n", stateErr)
	}

	// Preset
	preset := s.InstalledPreset
	if preset == "" {
		preset = "(none)"
	}
	fmt.Printf("Preset: %s\n", preset)

	// Claude model preset
	claudePreset := s.InstalledClaudePreset
	if claudePreset == "" {
		claudePreset = "(none)"
	}
	fmt.Printf("Claude model preset: %s\n", claudePreset)
	fmt.Println()

	// Agents
	printSection("Agents", s.InstalledAgents)

	// Components
	printSection("Components", s.InstalledComponents)

	// Dev Skills
	devSkillsCfg, _ := devskills.ReadConfig(homeDir)
	printSection("Dev Skills", devSkillsCfg.InstalledSkills)

	// Dev Agents
	devAgentsCfg, _ := devagents.ReadConfig(homeDir)
	printSection("Dev Agents", devAgentsCfg.InstalledAgents)

	// Monday config — show both global and workspace.
	fmt.Println("Monday:")
	printMondayScope("  Global", filepath.Join(homeDir, ".informa-wizard", "monday.json"))
	if cwd, err := os.Getwd(); err == nil {
		printMondayScope("  Workspace ("+cwd+")", filepath.Join(cwd, ".informa-wizard", "monday.json"))
	}
	fmt.Println()

	return nil
}

func printMondayScope(label, path string) {
	fmt.Printf("%s:\n", label)
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Println("    (not configured)")
		return
	}
	var mc mondayStatusJSON
	if jsonErr := json.Unmarshal(data, &mc); jsonErr != nil {
		fmt.Println("    (malformed config)")
		return
	}
	tokenDisplay := "(empty)"
	if mc.Token != "" {
		tokenDisplay = strings.Repeat("*", 12)
	}
	boardDisplay := mc.BoardID
	if boardDisplay == "" {
		boardDisplay = "(empty)"
	}
	fmt.Printf("    Token: %s\n", tokenDisplay)
	fmt.Printf("    Board: %s\n", boardDisplay)
}

func printSection(title string, items []string) {
	if len(items) == 0 {
		fmt.Printf("%s (0):\n  (none)\n", title)
	} else {
		fmt.Printf("%s (%d):\n", title, len(items))
		for _, item := range items {
			fmt.Printf("  - %s\n", item)
		}
	}
	fmt.Println()
}
