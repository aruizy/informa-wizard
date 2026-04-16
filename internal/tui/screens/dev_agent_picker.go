package screens

import (
	"strings"

	"gitlab.informa.tools/ai/wizard/informa-wizard/internal/components/devagents"
	"gitlab.informa.tools/ai/wizard/informa-wizard/internal/tui/styles"
)

// RenderDevAgentPicker renders the dev-agent selection screen.
// agents is the list of discovered agents, checked is the per-agent checked state,
// and cursor is the currently focused row.
func RenderDevAgentPicker(agents []devagents.DiscoveredAgent, checked []bool, cursor int) string {
	var b strings.Builder

	b.WriteString(styles.TitleStyle.Render("Select Dev Agents"))
	b.WriteString("\n\n")
	b.WriteString(styles.SubtextStyle.Render("Toggle agents with space. Press enter to confirm."))
	b.WriteString("\n\n")

	if len(agents) == 0 {
		b.WriteString(styles.SubtextStyle.Render("No agents found. Agents will be available after the repository is cloned during install."))
		b.WriteString("\n\n")
	}

	for idx, agent := range agents {
		isChecked := idx < len(checked) && checked[idx]
		focused := idx == cursor

		label := agent.Name
		if agent.Description != "" {
			label = agent.Name + "  " + agent.Description
		}
		b.WriteString(renderCheckbox(label, isChecked, focused))
	}

	b.WriteString("\n")
	b.WriteString(styles.HelpStyle.Render("j/k: navigate • space: toggle • enter: confirm • esc: back"))

	return b.String()
}
