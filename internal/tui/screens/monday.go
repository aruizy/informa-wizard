package screens

import (
	"strings"

	"gitlab.informa.tools/ai/wizard/informa-wizard/internal/tui/styles"
)

// MondayField indicates which input field is active.
type MondayField int

const (
	MondayFieldToken   MondayField = 0
	MondayFieldBoardID MondayField = 1
)

// RenderMonday renders the Monday.com configuration screen with two text inputs.
func RenderMonday(token, boardID string, activeField MondayField, cursorPos int) string {
	var b strings.Builder

	b.WriteString(styles.TitleStyle.Render("Monday.com Configuration"))
	b.WriteString("\n\n")
	b.WriteString(styles.SubtextStyle.Render("Configure your Monday.com integration for task management."))
	b.WriteString("\n\n")

	// Token field
	tokenLabel := "API Token:"
	if activeField == MondayFieldToken {
		tokenLabel = "▸ API Token:"
	} else {
		tokenLabel = "  API Token:"
	}
	b.WriteString(styles.HeadingStyle.Render(tokenLabel))
	b.WriteString("\n")
	b.WriteString(styles.SubtextStyle.Render("  Get yours at: https://informadb.monday.com/apps/manage/tokens"))
	b.WriteString("\n")
	b.WriteString(renderTextInput(token, activeField == MondayFieldToken, cursorPos, true))
	b.WriteString("\n\n")

	// Board ID field
	boardLabel := "Board ID:"
	if activeField == MondayFieldBoardID {
		boardLabel = "▸ Board ID:"
	} else {
		boardLabel = "  Board ID:"
	}
	b.WriteString(styles.HeadingStyle.Render(boardLabel))
	b.WriteString("\n")
	b.WriteString(styles.SubtextStyle.Render("  From board URL: https://informadb.monday.com/boards/{BOARD_ID}/views/..."))
	b.WriteString("\n")
	b.WriteString(renderTextInput(boardID, activeField == MondayFieldBoardID, cursorPos, false))
	b.WriteString("\n\n")

	if token == "" {
		b.WriteString(styles.WarningStyle.Render("Token is required for Monday integration."))
		b.WriteString("\n\n")
	}

	b.WriteString(styles.HelpStyle.Render("tab: switch field • enter: continue • esc: back"))

	return b.String()
}

// renderTextInput renders a single-line text input with cursor.
// When masked is true and the field is unfocused, the value is replaced with asterisks.
func renderTextInput(value string, focused bool, cursorPos int, masked bool) string {
	if !focused {
		display := value
		if display == "" {
			display = "(empty)"
		} else if masked {
			display = strings.Repeat("*", len([]rune(display)))
		}
		return styles.UnselectedStyle.Render("  " + display)
	}

	runes := []rune(value)
	var b strings.Builder
	b.WriteString("  > ")
	for i, r := range runes {
		if i == cursorPos {
			b.WriteString(styles.SelectedStyle.Render("|"))
		}
		b.WriteRune(r)
	}
	if cursorPos >= len(runes) {
		b.WriteString(styles.SelectedStyle.Render("|"))
	}
	return b.String()
}
