package screens

import (
	"strings"

	"gitlab.informa.tools/ai/wizard/informa-wizard/internal/tui/styles"
)

// MondayField indicates which input field is active.
type MondayField int

const (
	MondayFieldToken MondayField = 0
	MondayFieldTabs  MondayField = 1
	MondayFieldBoard MondayField = 2
)

// RenderMonday renders the Monday.com configuration screen with:
//   - a shared token input
//   - a tab row to switch between global and workspace config
//   - a board ID input for the active tab
//   - status indicators per scope
//   - an optional validation error (token rejected by the API)
//   - an optional MCP injection error (token OK but writing into agent
//     configs failed). These are rendered with distinct prefixes so the
//     user can tell which step failed.
func RenderMonday(
	token string,
	globalBoard string,
	workspaceBoard string,
	activeField MondayField,
	activeTab string,
	tokenPos int,
	globalBoardPos int,
	workspaceBoardPos int,
	validationErr error,
	injectErr error,
) string {
	var b strings.Builder

	b.WriteString(styles.TitleStyle.Render("Monday.com Configuration"))
	b.WriteString("\n\n")

	// ── Token field ───────────────────────────────────────────────────────
	tokenLabel := "  API Token (shared between both configs):"
	if activeField == MondayFieldToken {
		tokenLabel = "▸ API Token (shared between both configs):"
	}
	b.WriteString(styles.HeadingStyle.Render(tokenLabel))
	b.WriteString("\n")
	b.WriteString(styles.SubtextStyle.Render("  Get yours at: https://informadb.monday.com/apps/manage/tokens"))
	b.WriteString("\n")
	b.WriteString(renderTextInput(token, activeField == MondayFieldToken, tokenPos, true))
	b.WriteString("\n\n")

	b.WriteString(styles.SubtextStyle.Render(strings.Repeat("─", 50)))
	b.WriteString("\n")

	// ── Tab row ───────────────────────────────────────────────────────────
	tabPrefix := "  "
	if activeField == MondayFieldTabs {
		tabPrefix = "▸ "
	}

	globalStatus := configStatus(token, globalBoard)
	workspaceStatus := configStatus(token, workspaceBoard)

	var globalTab, workspaceTab string
	if activeTab == "workspace" {
		globalTab = styles.SubtextStyle.Render("[Global]")
		workspaceTab = styles.SelectedStyle.Render("[Workspace]")
	} else {
		globalTab = styles.SelectedStyle.Render("[Global]")
		workspaceTab = styles.SubtextStyle.Render("[Workspace]")
	}

	b.WriteString(tabPrefix + globalTab + "       " + workspaceTab)
	b.WriteString("\n")

	// Status line under each tab label (aligned roughly).
	b.WriteString(styles.SubtextStyle.Render("  status: " + globalStatus))
	b.WriteString("     ")
	b.WriteString(styles.SubtextStyle.Render("status: " + workspaceStatus))
	b.WriteString("\n\n")

	// ── Board ID field for the active tab ─────────────────────────────────
	var boardLabel string
	var boardValue string
	var boardPos int
	if activeTab == "workspace" {
		boardLabel = "Board ID for Workspace config:"
		boardValue = workspaceBoard
		boardPos = workspaceBoardPos
	} else {
		boardLabel = "Board ID for Global config:"
		boardValue = globalBoard
		boardPos = globalBoardPos
	}
	fullBoardLabel := "  " + boardLabel
	if activeField == MondayFieldBoard {
		fullBoardLabel = "▸ " + boardLabel
	}
	b.WriteString(styles.HeadingStyle.Render(fullBoardLabel))
	b.WriteString("\n")
	b.WriteString(styles.SubtextStyle.Render("  From board URL: https://informadb.monday.com/boards/{BOARD_ID}/views/..."))
	b.WriteString("\n")
	b.WriteString(renderTextInput(boardValue, activeField == MondayFieldBoard, boardPos, false))
	b.WriteString("\n\n")

	b.WriteString(styles.SubtextStyle.Render("Workspace overrides global for this project when set."))
	b.WriteString("\n")
	b.WriteString(styles.SubtextStyle.Render(strings.Repeat("─", 50)))
	b.WriteString("\n\n")

	// ── Validation / warning ──────────────────────────────────────────────
	if validationErr != nil {
		b.WriteString(styles.WarningStyle.Render("Token validation failed: " + validationErr.Error()))
		b.WriteString("\n\n")
	} else if injectErr != nil {
		b.WriteString(styles.WarningStyle.Render("MCP injection failed: " + injectErr.Error()))
		b.WriteString("\n\n")
	} else if token == "" {
		b.WriteString(styles.WarningStyle.Render("Token is required for Monday integration."))
		b.WriteString("\n\n")
	}

	b.WriteString(styles.HelpStyle.Render("tab: next field • ←/→: switch tab (when on tab row) • enter: save • esc: discard"))

	return b.String()
}

// configStatus returns a short human-readable status string for one scope.
// "✓ configured" when both the shared token and the scope's board are set;
// "✗ not configured" otherwise.
func configStatus(token, board string) string {
	if token != "" && board != "" {
		return "✓ configured"
	}
	return "✗ not configured"
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
