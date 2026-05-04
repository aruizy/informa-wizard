package screens

// Note: this file is intentionally named sync_screen.go instead of sync.go
// because sync.go would conflict with the Go standard library "sync" package name.

import (
	"fmt"
	"path/filepath"
	"strings"

	"gitlab.informa.tools/ai/wizard/informa-wizard/internal/tui/styles"
)

// RenderSync handles all states of the sync screen.
//
// State logic:
//  1. operationRunning → "Syncing configurations..." with spinner
//  2. hasSyncRun && (filesChanged > 0 || syncErr != nil) → show result
//  3. Otherwise → show confirmation screen
//
// width is the current terminal width (m.Width). When 0 or negative, falls
// back to a sensible default (defaultErrorWidth).
func RenderSync(filesChanged int, syncErr error, operationRunning bool, hasSyncRun bool, spinnerFrame int, width int) string {
	var b strings.Builder

	b.WriteString(styles.TitleStyle.Render("Sync Configurations"))
	b.WriteString("\n\n")

	// State 1: sync is running
	if operationRunning {
		b.WriteString(styles.WarningStyle.Render(SpinnerChar(spinnerFrame) + "  Syncing configurations..."))
		b.WriteString("\n\n")
		b.WriteString(styles.HelpStyle.Render("Please wait..."))
		return b.String()
	}

	// State 2: sync has run — show result
	if hasSyncRun {
		b.WriteString(renderSyncResult(filesChanged, syncErr, width))
		return b.String()
	}

	// State 3: confirmation screen
	b.WriteString(renderSyncConfirm())
	return b.String()
}

func renderSyncConfirm() string {
	var b strings.Builder

	b.WriteString(styles.UnselectedStyle.Render("Sync will re-apply your dotfile configurations"))
	b.WriteString("\n")
	b.WriteString(styles.UnselectedStyle.Render("to all detected AI agents on this machine."))
	b.WriteString("\n\n")

	b.WriteString(styles.SubtextStyle.Render("This operation:"))
	b.WriteString("\n")
	b.WriteString(styles.SubtextStyle.Render("  • Reads your current agent selections"))
	b.WriteString("\n")
	b.WriteString(styles.SubtextStyle.Render("  • Re-writes agent config files from templates"))
	b.WriteString("\n")
	b.WriteString(styles.SubtextStyle.Render("  • Does not modify your global dotfiles"))
	b.WriteString("\n\n")

	b.WriteString(styles.HeadingStyle.Render("Press enter to sync"))
	b.WriteString("\n\n")
	b.WriteString(styles.HelpStyle.Render("enter: confirm • esc: back • q: quit"))

	return b.String()
}

func renderSyncResult(filesChanged int, syncErr error, width int) string {
	var b strings.Builder

	if syncErr != nil {
		b.WriteString(styles.ErrorStyle.Render("✗ Sync failed"))
		b.WriteString("\n\n")
		for _, line := range compactErrorLines(syncErr.Error(), width) {
			b.WriteString(styles.SubtextStyle.Render(line))
			b.WriteString("\n")
		}
		b.WriteString("\n")
		b.WriteString(styles.HelpStyle.Render("Check your configuration and try again."))
	} else if filesChanged == 0 {
		b.WriteString(styles.SuccessStyle.Render("✓ Sync complete"))
		b.WriteString("\n\n")
		b.WriteString(styles.SubtextStyle.Render("No agents detected or no files needed updating."))
	} else {
		b.WriteString(styles.SuccessStyle.Render("✓ Sync complete"))
		b.WriteString("\n\n")
		b.WriteString(fmt.Sprintf("%s %s", styles.HeadingStyle.Render(fmt.Sprintf("%d file(s)", filesChanged)), styles.UnselectedStyle.Render("synchronized")))
	}

	b.WriteString("\n\n")
	b.WriteString(styles.HelpStyle.Render("enter: return • esc: back • q: quit"))

	return b.String()
}

// maxFailureLinesShown bounds how many [!!]/[??] entries we surface on screen.
// The full report is still in ~/.informa-wizard/logs/wizard.log for diagnostics.
const maxFailureLinesShown = 12

// defaultErrorWidth is the assumed terminal width when the model has not yet
// reported one (e.g. very early in the session before the first WindowSizeMsg).
// It also serves as the lower bound for very narrow terminals.
const defaultErrorWidth = 80

// minErrorWidth is the smallest width below which we use defaultErrorWidth so
// very narrow terminals still get truncation rather than broken layout
// (narrow-but-truncated is more usable than narrow-and-overflowed).
const minErrorWidth = 40

// verifyIDPrefixes lists the known verify Check ID prefixes whose tail is a
// filesystem path (and whose tail is what we want to surface to the user).
// Order matters: longer/more-specific prefixes must come first so we strip the
// most informative one. Keep this in sync with the IDs emitted in
// internal/cli/sync.go and internal/cli/run.go.
//
// Path-bearing prefixes only — non-path verify IDs (like "verify:engram:binary"
// or "verify:antigravity:rules-collision") are intentionally surfaced raw on
// the marker line because their suffix isn't a filesystem path.
var verifyIDPrefixes = []string{
	"verify:sync:file:",
	"verify:file:",
}

// compactErrorLines turns a verbose post-sync error (which may carry a full
// verify report with hundreds of [ok] lines) into a short, screen-friendly
// summary. For non-verify errors (no [ok]/[!!]/[??] markers) the original
// lines are returned unchanged.
//
// For verify reports it:
//  1. Drops [ok] and [--] noise entirely.
//  2. Caps failed/warning rows to maxFailureLinesShown, with a "… and N more"
//     hint pointing at the log.
//  3. Reformats each failure into a 2- or 3-line block:
//        [!!] <basename>
//             ↳ <error reason>
//             ↳ <full path, middle-truncated to fit width>
//
//  The basename is what the user scans first; the error reason and path are
//  indented continuation lines so even narrow terminals stay readable.
func compactErrorLines(msg string, width int) []string {
	if width < minErrorWidth {
		width = defaultErrorWidth
	}

	all := strings.Split(strings.TrimRight(msg, "\n"), "\n")

	hasReport := false
	for _, line := range all {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[ok]") || strings.HasPrefix(trimmed, "[!!]") || strings.HasPrefix(trimmed, "[??]") || strings.HasPrefix(trimmed, "[--]") {
			hasReport = true
			break
		}
	}
	if !hasReport {
		return all
	}

	// Single-pass to preserve original line order. Trailing prose like
	// "Installation completed with verification issues..." MUST appear AFTER
	// the failures it refers to, not before — bucketing failures separately
	// would invert the order.
	var out []string
	failuresShown := 0
	hidden := 0
	for _, line := range all {
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, "[ok]") || strings.HasPrefix(trimmed, "[--]"):
			// drop noise
		case strings.HasPrefix(trimmed, "[!!]") || strings.HasPrefix(trimmed, "[??]"):
			if failuresShown < maxFailureLinesShown {
				out = append(out, formatFailureLine(line, width)...)
				failuresShown++
			} else {
				hidden++
			}
		default:
			out = append(out, line)
		}
	}
	if hidden > 0 {
		out = append(out, fmt.Sprintf("… and %d more failed check(s) — see ~/.informa-wizard/logs/wizard.log for the full report", hidden))
	}

	// Defensive fallback: if a verify report came through but every check was
	// [ok]/[--] AND there was no prose either, we'd return empty and the user
	// would see "✗ Sync failed" with no body. Synthesize a single explanatory
	// line rather than dumping the (filtered) [ok] noise back to the user —
	// the original report is still in ~/.informa-wizard/logs/wizard.log.
	if len(out) == 0 {
		return []string{"verify report contained only passing checks — see ~/.informa-wizard/logs/wizard.log for the full report"}
	}
	return out
}

// formatFailureLine reflows a single verify failure/warning line into a
// readable multi-line block, returning the lines (no trailing newlines).
// Lines that don't match the expected shape are returned as-is (single line)
// so we never silently drop information.
//
// Parses without a regex so that paths containing legitimate " - " (e.g.
// "Resume - 2024.md", "OneDrive - Informa\file.md") are preserved intact:
//   1. Strip the [!!] / [??] marker.
//   2. If the line ends with " (...)", peel that off as the error message.
//   3. Split id from description on the LAST " - " (descriptions emitted by
//      verify/report.go never contain " - ", so the last occurrence is safe;
//      file paths may contain " - " freely).
//   4. Strip a known verify: prefix from id to recover the displayable path.
func formatFailureLine(line string, width int) []string {
	trimmed := strings.TrimSpace(line)
	var marker string
	switch {
	case strings.HasPrefix(trimmed, "[!!]"):
		marker = "[!!]"
	case strings.HasPrefix(trimmed, "[??]"):
		marker = "[??]"
	default:
		// Unrecognised shape — fall back to middle-truncating the whole line.
		return []string{truncateMiddle(line, width)}
	}

	rest := strings.TrimSpace(trimmed[len(marker):])

	// Peel an optional trailing " (error message)" — only when the line ends
	// with ")" so we don't misread an unbalanced paren in the path itself.
	errMsg := ""
	if strings.HasSuffix(rest, ")") {
		if i := strings.LastIndex(rest, " ("); i != -1 {
			errMsg = rest[i+2 : len(rest)-1]
			rest = strings.TrimSpace(rest[:i])
		}
	}

	// Split id from description on the LAST " - " (descriptions don't contain
	// it; paths can). Whole rest is the id when there's no description.
	id := rest
	if i := strings.LastIndex(rest, " - "); i != -1 {
		id = strings.TrimSpace(rest[:i])
	}

	// Extract the user-meaningful tail of the ID. Splitting on the last ":"
	// is wrong: on Windows it would strip the drive letter from "C:\..." paths,
	// and on Linux it would break paths that legitimately contain ":". Strip a
	// known fixed prefix instead. Non-path IDs fall through unchanged.
	displayID := id
	for _, p := range verifyIDPrefixes {
		if strings.HasPrefix(id, p) {
			displayID = strings.TrimPrefix(id, p)
			break
		}
	}

	basename := filepath.Base(displayID)
	if basename == "" || basename == "." || basename == "/" {
		basename = displayID
	}

	lines := []string{fmt.Sprintf("%s %s", marker, basename)}
	if errMsg != "" {
		lines = append(lines, "     ↳ "+truncateMiddle(errMsg, width-7))
	}
	// Only show the full path on a continuation line if it adds info beyond
	// the basename (i.e. the displayID has a directory component).
	if displayID != basename {
		lines = append(lines, "     ↳ "+truncateMiddle(displayID, width-7))
	}
	return lines
}

// truncateMiddle shortens s to fit within max runes by replacing the middle
// with "…", preserving head and tail. Useful for long absolute paths where
// both the drive root and the filename matter. Returns s unchanged when it
// already fits or when max is too small to be useful.
func truncateMiddle(s string, max int) string {
	r := []rune(s)
	if max <= 0 || len(r) <= max {
		return s
	}
	if max < 5 {
		return string(r[:max])
	}
	keep := max - 1 // 1 rune for the ellipsis
	head := keep / 2
	tail := keep - head
	return string(r[:head]) + "…" + string(r[len(r)-tail:])
}
