package screens

import (
	"strings"

	"gitlab.informa.tools/ai/wizard/informa-wizard/internal/state"
	"gitlab.informa.tools/ai/wizard/informa-wizard/internal/tui/styles"
)

// RenderInstallOverwriteWarn renders the warning screen shown when a previous
// installation is detected and the user tries to start a new install.
func RenderInstallOverwriteWarn(s state.InstallState, cursor int) string {
	var b strings.Builder

	b.WriteString(styles.TitleStyle.Render("Existing installation detected"))
	b.WriteString("\n\n")
	b.WriteString(styles.WarningStyle.Render("Last installed:"))
	b.WriteString("\n")

	if s.InstalledPreset != "" {
		b.WriteString("  Preset: " + s.InstalledPreset + "\n")
	}
	if len(s.InstalledAgents) > 0 {
		b.WriteString("  Agents: " + strings.Join(s.InstalledAgents, ", ") + "\n")
	}
	if len(s.InstalledComponents) > 0 {
		b.WriteString("  Components: " + strings.Join(s.InstalledComponents, ", ") + "\n")
	}

	b.WriteString("\n")
	b.WriteString(styles.WarningStyle.Render("Continuing will OVERWRITE the current configuration."))
	b.WriteString("\n\n")
	b.WriteString(styles.SubtextStyle.Render("What would you like to do?"))
	b.WriteString("\n\n")

	options := []string{
		"Continue (overwrite)",
		"View current installation",
		"Back",
	}
	b.WriteString(renderOptions(options, cursor))

	b.WriteString("\n")
	b.WriteString(styles.HelpStyle.Render("j/k: navigate • enter: select • esc: back"))

	return b.String()
}
