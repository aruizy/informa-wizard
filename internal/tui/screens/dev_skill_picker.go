package screens

import (
	"strings"

	"gitlab.informa.tools/ai/wizard/informa-wizard/internal/components/devskills"
	"gitlab.informa.tools/ai/wizard/informa-wizard/internal/tui/styles"
)

// RenderDevSkillPicker renders the dev-skill selection screen.
// skills is the list of discovered skills, checked is the per-skill checked state,
// and cursor is the currently focused row.
func RenderDevSkillPicker(skills []devskills.DiscoveredSkill, checked []bool, cursor int, cloneErr string) string {
	var b strings.Builder

	b.WriteString(styles.TitleStyle.Render("Select Dev Skills"))
	b.WriteString("\n\n")

	if cloneErr != "" {
		b.WriteString(styles.WarningStyle.Render("Failed to clone dev-skills repository:"))
		b.WriteString("\n")
		b.WriteString(styles.WarningStyle.Render(cloneErr))
		b.WriteString("\n\n")
		b.WriteString(styles.SubtextStyle.Render("Check your network connection and repository access permissions."))
		b.WriteString("\n")
		b.WriteString(styles.SubtextStyle.Render("Press enter to skip or esc to go back."))
		b.WriteString("\n\n")
	} else {
		b.WriteString(styles.SubtextStyle.Render("Toggle skills with space. Press enter to confirm."))
		b.WriteString("\n\n")
	}

	if len(skills) == 0 && cloneErr == "" {
		b.WriteString(styles.SubtextStyle.Render("No skills found in the repository."))
		b.WriteString("\n\n")
	}

	for idx, skill := range skills {
		isChecked := idx < len(checked) && checked[idx]
		focused := idx == cursor

		label := skill.Name
		if skill.Description != "" {
			label = skill.Name + "  " + skill.Description
		}
		b.WriteString(renderCheckbox(label, isChecked, focused))
	}

	b.WriteString("\n")
	b.WriteString(styles.HelpStyle.Render("j/k: navigate • space: toggle • enter: confirm • esc: back"))

	return b.String()
}
