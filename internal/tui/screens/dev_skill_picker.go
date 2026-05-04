package screens

import (
	"strings"

	"gitlab.informa.tools/ai/wizard/informa-wizard/internal/components/devskills"
	"gitlab.informa.tools/ai/wizard/informa-wizard/internal/tui/styles"
)

// RenderDevSkillPicker renders the dev-skill selection screen.
// skills is the list of discovered skills, checked is the per-skill checked state,
// cursor is the currently focused row (relative to the filtered list when filter is active),
// filter is the current search query, and searchMode indicates whether the search input is active.
func RenderDevSkillPicker(skills []devskills.DiscoveredSkill, checked []bool, cursor int, cloneErr string, filter string, searchMode bool) string {
	var b strings.Builder

	b.WriteString(styles.TitleStyle.Render("Select Dev Skills"))
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
		b.WriteString(styles.WarningStyle.Render("Failed to clone dev-skills repository:"))
		b.WriteString("\n")
		b.WriteString(styles.WarningStyle.Render(cloneErr))
		b.WriteString("\n\n")
		b.WriteString(styles.SubtextStyle.Render("Check your network connection and repository access permissions."))
		b.WriteString("\n")
		b.WriteString(styles.SubtextStyle.Render("Press enter to skip or esc to go back."))
		b.WriteString("\n\n")
	} else if !searchMode && filter == "" {
		b.WriteString(styles.SubtextStyle.Render("Toggle skills with space. Press enter to confirm."))
		b.WriteString("\n\n")
	}

	// Build the filtered view.
	filteredIndices := devSkillFilteredIndices(skills, filter)

	if len(filteredIndices) == 0 && cloneErr == "" {
		if filter != "" {
			b.WriteString(styles.SubtextStyle.Render("No skills match the search."))
		} else {
			b.WriteString(styles.SubtextStyle.Render("No skills found in the repository."))
		}
		b.WriteString("\n\n")
	}

	for filteredPos, realIdx := range filteredIndices {
		skill := skills[realIdx]
		isChecked := realIdx < len(checked) && checked[realIdx]
		focused := filteredPos == cursor

		label := skill.Name
		if skill.Description != "" {
			label = skill.Name + "  " + skill.Description
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

// devSkillFilteredIndices returns the real indices of skills that match the filter.
// When filter is empty, all indices are returned.
func devSkillFilteredIndices(skills []devskills.DiscoveredSkill, filter string) []int {
	if filter == "" {
		indices := make([]int, len(skills))
		for i := range skills {
			indices[i] = i
		}
		return indices
	}
	query := strings.ToLower(filter)
	var indices []int
	for i, s := range skills {
		if strings.Contains(strings.ToLower(s.Name), query) ||
			strings.Contains(strings.ToLower(s.ID), query) ||
			strings.Contains(strings.ToLower(s.Description), query) {
			indices = append(indices, i)
		}
	}
	return indices
}
