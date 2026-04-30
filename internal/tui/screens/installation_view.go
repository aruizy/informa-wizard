package screens

import (
	"strings"

	"gitlab.informa.tools/ai/wizard/informa-wizard/internal/state"
	"gitlab.informa.tools/ai/wizard/informa-wizard/internal/tui/styles"
)

// InstallationViewData holds the data shown on the installation view screen.
type InstallationViewData struct {
	State       state.InstallState
	MondayToken string // masked when shown
	MondayBoard string
	DevSkills   []string
	DevAgents   []string
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

	// Skills
	b.WriteString(styles.HeadingStyle.Render("Skills"))
	b.WriteString("\n")
	if len(data.State.InstalledSkills) == 0 {
		b.WriteString(styles.SubtextStyle.Render("  (none)") + "\n")
	} else {
		for _, s := range data.State.InstalledSkills {
			b.WriteString("  • " + s + "\n")
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

	// Monday config
	b.WriteString(styles.HeadingStyle.Render("Monday"))
	b.WriteString("\n")
	if data.MondayToken == "" && data.MondayBoard == "" {
		b.WriteString(styles.SubtextStyle.Render("  (not configured)") + "\n")
	} else {
		mask := "(empty)"
		if data.MondayToken != "" {
			mask = strings.Repeat("*", 12)
		}
		b.WriteString("  Token: " + mask + "\n")
		b.WriteString("  Board: " + data.MondayBoard + "\n")
	}

	b.WriteString("\n")
	b.WriteString(styles.HelpStyle.Render("enter / esc: back"))
	return b.String()
}
