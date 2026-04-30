package screens

import (
	"strings"

	"gitlab.informa.tools/ai/wizard/informa-wizard/internal/tui/styles"
)

// RenderUninstall renders the uninstall component screen.
//
// First step (confirmStep=false): shows the list of installed components with a cursor.
// Second step (confirmStep=true): shows a confirmation prompt for the selected component.
func RenderUninstall(components []string, cursor int, confirmStep bool, selected string) string {
	var b strings.Builder

	b.WriteString(styles.TitleStyle.Render("Uninstall Component"))
	b.WriteString("\n\n")

	if confirmStep {
		b.WriteString(styles.WarningStyle.Render("Are you sure you want to uninstall " + selected + "?"))
		b.WriteString("\n\n")
		b.WriteString(styles.SubtextStyle.Render("This will delete its files and update state.json."))
		b.WriteString("\n\n")
		b.WriteString(renderOptions([]string{"Yes, uninstall", "Cancel"}, cursor))
		b.WriteString("\n")
		b.WriteString(styles.HelpStyle.Render("enter: confirm • esc: cancel"))
	} else {
		if len(components) == 0 {
			b.WriteString(styles.SubtextStyle.Render("No components are currently installed."))
			b.WriteString("\n")
		} else {
			b.WriteString(styles.SubtextStyle.Render("Select a component to uninstall:"))
			b.WriteString("\n\n")
			b.WriteString(renderOptions(components, cursor))
		}
		b.WriteString("\n")
		b.WriteString(styles.HelpStyle.Render("j/k: navigate • enter: select • esc: back"))
	}

	return b.String()
}
