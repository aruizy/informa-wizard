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
	MondayFieldScope   MondayField = 2
)

// RenderMonday renders the Monday.com configuration screen with two text inputs,
// a save-scope toggle, and an optional validation error.
func RenderMonday(token, boardID string, activeField MondayField, cursorPos int, validationErr error, saveScope string) string {
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

	// Save scope toggle
	scopeLabel := "Save to:"
	if activeField == MondayFieldScope {
		scopeLabel = "▸ Save to:"
	} else {
		scopeLabel = "  Save to:"
	}
	b.WriteString(styles.HeadingStyle.Render(scopeLabel))
	b.WriteString("\n")
	globalMark := "[ ]"
	workspaceMark := "[ ]"
	if saveScope == "workspace" {
		workspaceMark = "[x]"
	} else {
		globalMark = "[x]"
	}
	b.WriteString("  " + globalMark + " global  " + workspaceMark + " this workspace")
	b.WriteString("\n\n")

	// Warnings
	if validationErr != nil {
		b.WriteString(styles.WarningStyle.Render("Token validation failed: " + validationErr.Error()))
		b.WriteString("\n\n")
	} else if token == "" {
		b.WriteString(styles.WarningStyle.Render("Token is required for Monday integration."))
		b.WriteString("\n\n")
	}

	b.WriteString(styles.HelpStyle.Render("tab: next field • space: toggle scope (when on Save to) • enter: continue • esc: back"))

	return b.String()
}

// renderTextInput renders a single-line text input with cursor.
// When masked is true and the field is unfocused, the value is replaced with asterisks.
// Long inputs scroll horizontally so the cursor stays visible.
func renderTextInput(value string, focused bool, cursorPos int, masked bool) string {
	const windowWidth = 60 // visible character window

	if !focused {
		display := value
		if display == "" {
			display = "(empty)"
		} else if masked {
			display = strings.Repeat("*", len([]rune(display)))
		}
		// Truncate the unfocused view too if very long.
		runes := []rune(display)
		if len(runes) > windowWidth {
			display = "…" + string(runes[len(runes)-windowWidth+1:])
		}
		return styles.UnselectedStyle.Render("  " + display)
	}

	runes := []rune(value)
	// Compute scroll offset so the cursor stays inside [offset, offset+windowWidth].
	offset := 0
	if len(runes) > windowWidth {
		// Keep cursor about 5 chars from the right edge when typing forward.
		desired := cursorPos - (windowWidth - 5)
		if desired > offset {
			offset = desired
		}
		if offset > len(runes)-windowWidth {
			offset = len(runes) - windowWidth
		}
		if offset < 0 {
			offset = 0
		}
	}
	end := offset + windowWidth
	if end > len(runes) {
		end = len(runes)
	}

	var b strings.Builder
	b.WriteString("  > ")
	if offset > 0 {
		b.WriteString("…")
	}
	for i := offset; i < end; i++ {
		if i == cursorPos {
			b.WriteString(styles.SelectedStyle.Render("|"))
		}
		b.WriteRune(runes[i])
	}
	if cursorPos >= end {
		b.WriteString(styles.SelectedStyle.Render("|"))
	}
	if end < len(runes) {
		b.WriteString("…")
	}
	return b.String()
}
