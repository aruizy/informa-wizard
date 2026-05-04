package screens

import (
	"strings"

	"gitlab.informa.tools/ai/wizard/informa-wizard/internal/state"
	"gitlab.informa.tools/ai/wizard/informa-wizard/internal/tui/styles"
)

// InstallationViewData holds the data shown on the installation view screen.
type InstallationViewData struct {
	State                state.InstallState
	GlobalMondayToken    string
	GlobalMondayBoard    string
	WorkspaceMondayToken string
	WorkspaceMondayBoard string
	WorkspacePath        string // current working directory shown alongside workspace config
	DevSkills            []string
	DevAgents            []string
}

// RenderInstallationView shows what the user installed.
func RenderInstallationView(data InstallationViewData) string {
	var b strings.Builder

	b.WriteString(styles.TitleStyle.Render("Installation Summary"))
	b.WriteString("\n\n")

	// Preset
	if data.State.InstalledPreset != "" {
		b.WriteString(styles.HeadingStyle.Render("Preset"))
		b.WriteString("\n  " + data.State.InstalledPreset + "\n\n")
	}

	// Claude Model Preset
	claudePreset := data.State.InstalledClaudePreset
	if claudePreset == "" {
		claudePreset = "(not configured)"
	}
	b.WriteString(styles.HeadingStyle.Render("Claude Model Preset"))
	b.WriteString("\n  " + claudePreset + "\n\n")

	// Agents
	b.WriteString(styles.HeadingStyle.Render("Agents"))
	b.WriteString("\n")
	if len(data.State.InstalledAgents) == 0 {
		b.WriteString(styles.SubtextStyle.Render("  (none)") + "\n")
	} else {
		for _, a := range data.State.InstalledAgents {
			b.WriteString("  • " + a + "\n")
		}
	}
	b.WriteString("\n")

	// Components
	b.WriteString(styles.HeadingStyle.Render("Components"))
	b.WriteString("\n")
	if len(data.State.InstalledComponents) == 0 {
		b.WriteString(styles.SubtextStyle.Render("  (none)") + "\n")
	} else {
		for _, c := range data.State.InstalledComponents {
			b.WriteString("  • " + c + "\n")
		}
	}
	b.WriteString("\n")

	// Dev Skills (from dev-skills.json)
	b.WriteString(styles.HeadingStyle.Render("Dev Skills"))
	b.WriteString("\n")
	if len(data.DevSkills) == 0 {
		b.WriteString(styles.SubtextStyle.Render("  (none)") + "\n")
	} else {
		for _, s := range data.DevSkills {
			b.WriteString("  • " + s + "\n")
		}
	}
	b.WriteString("\n")

	// Dev Agents (from dev-agents.json)
	b.WriteString(styles.HeadingStyle.Render("Dev Agents"))
	b.WriteString("\n")
	if len(data.DevAgents) == 0 {
		b.WriteString(styles.SubtextStyle.Render("  (none)") + "\n")
	} else {
		for _, a := range data.DevAgents {
			b.WriteString("  • " + a + "\n")
		}
	}
	b.WriteString("\n")

	// Monday config — show both global and workspace.
	b.WriteString(styles.HeadingStyle.Render("Monday"))
	b.WriteString("\n")

	// Global
	b.WriteString("  " + styles.HeadingStyle.Render("Global"))
	b.WriteString("\n")
	if data.GlobalMondayToken == "" && data.GlobalMondayBoard == "" {
		b.WriteString(styles.SubtextStyle.Render("    (not configured)") + "\n")
	} else {
		token := "(empty)"
		if data.GlobalMondayToken != "" {
			token = strings.Repeat("*", 12)
		}
		b.WriteString("    Token: " + token + "\n")
		board := data.GlobalMondayBoard
		if board == "" {
			board = "(empty)"
		}
		b.WriteString("    Board: " + board + "\n")
	}
	b.WriteString("\n")

	// Workspace
	wsLabel := "Workspace"
	if data.WorkspacePath != "" {
		wsLabel = "Workspace (" + data.WorkspacePath + ")"
	}
	b.WriteString("  " + styles.HeadingStyle.Render(wsLabel))
	b.WriteString("\n")
	if data.WorkspaceMondayToken == "" && data.WorkspaceMondayBoard == "" {
		b.WriteString(styles.SubtextStyle.Render("    (not configured)") + "\n")
	} else {
		token := "(empty)"
		if data.WorkspaceMondayToken != "" {
			token = strings.Repeat("*", 12)
		}
		b.WriteString("    Token: " + token + "\n")
		board := data.WorkspaceMondayBoard
		if board == "" {
			board = "(empty)"
		}
		b.WriteString("    Board: " + board + "\n")
	}

	b.WriteString("\n")
	b.WriteString(styles.HelpStyle.Render("enter / esc: back"))
	return b.String()
}
