package screens

import (
	"strings"

	"gitlab.informa.tools/ai/wizard/informa-wizard/internal/components/devagents"
	"gitlab.informa.tools/ai/wizard/informa-wizard/internal/tui/styles"
)

// RenderDevAgentPicker renders the dev-agent selection screen.
// agents is the list of discovered agents, checked is the per-agent checked state,
// cursor is the currently focused row (relative to the filtered list when filter is active),
// filter is the current search query, and searchMode indicates whether the search input is active.
func RenderDevAgentPicker(agents []devagents.DiscoveredAgent, checked []bool, cursor int, cloneErr string, filter string, searchMode bool) string {
	var b strings.Builder

	b.WriteString(styles.TitleStyle.Render("Select Dev Agents"))
	b.WriteString("\n\n")

	// Search input line.
	if searchMode {
		b.WriteString(styles.SubtextStyle.Render("Search: "+filter+"_"))
		b.WriteString("\n\n")
	} else if filter != "" {
		b.WriteString(styles.SubtextStyle.Render("Search: "+filter+" (enter to clear, / to refine)"))
		b.WriteString("\n\n")
	}

	if cloneErr != "" {
		b.WriteString(styles.WarningStyle.Render("Failed to clone dev-agents repository:"))
		b.WriteString("\n")
		b.WriteString(styles.WarningStyle.Render(cloneErr))
		b.WriteString("\n\n")
		b.WriteString(styles.SubtextStyle.Render("Check your network connection and repository access permissions."))
		b.WriteString("\n")
		b.WriteString(styles.SubtextStyle.Render("Press enter to skip or esc to go back."))
		b.WriteString("\n\n")
	} else if !searchMode && filter == "" {
		b.WriteString(styles.SubtextStyle.Render("Toggle agents with space. Press enter to confirm."))
		b.WriteString("\n\n")
	}

	// Build the filtered view.
	filteredIndices := devAgentFilteredIndices(agents, filter)

	if len(filteredIndices) == 0 && cloneErr == "" {
		if filter != "" {
			b.WriteString(styles.SubtextStyle.Render("No agents match the search."))
		} else {
			b.WriteString(styles.SubtextStyle.Render("No agents found in the repository."))
		}
		b.WriteString("\n\n")
	}

	for filteredPos, realIdx := range filteredIndices {
		agent := agents[realIdx]
		isChecked := realIdx < len(checked) && checked[realIdx]
		focused := filteredPos == cursor

		label := agent.Name
		if agent.Description != "" {
			label = agent.Name + "  " + agent.Description
		}
		b.WriteString(renderCheckbox(label, isChecked, focused))
	}

	b.WriteString("\n")
	if searchMode {
		b.WriteString(styles.HelpStyle.Render("enter: apply filter • esc: clear filter"))
	} else {
		b.WriteString(styles.HelpStyle.Render("j/k: navigate • space: toggle • a/A: select all/none • /: search • enter: confirm • esc: back"))
	}

	return b.String()
}

// devAgentFilteredIndices returns the real indices of agents that match the filter.
// When filter is empty, all indices are returned.
func devAgentFilteredIndices(agents []devagents.DiscoveredAgent, filter string) []int {
	if filter == "" {
		indices := make([]int, len(agents))
		for i := range agents {
			indices[i] = i
		}
		return indices
	}
	query := strings.ToLower(filter)
	var indices []int
	for i, a := range agents {
		if strings.Contains(strings.ToLower(a.Name), query) ||
			strings.Contains(strings.ToLower(a.ID), query) ||
			strings.Contains(strings.ToLower(a.Description), query) {
			indices = append(indices, i)
		}
	}
	return indices
}
