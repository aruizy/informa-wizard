package screens

import (
	"fmt"
	"strings"
	"testing"
)

// ─── RenderSync states ─────────────────────────────────────────────────────

// TestRenderSync_ConfirmState verifies the default confirm state — no operation
// running, no result yet — shows sync description and a prompt.
func TestRenderSync_ConfirmState(t *testing.T) {
	out := RenderSync(0, nil, false /*operationRunning*/, false /*hasSyncRun*/, 0, 120)

	lower := strings.ToLower(out)
	if !strings.Contains(lower, "sync") {
		t.Errorf("RenderSync(confirm) should contain 'sync'; got:\n%s", out)
	}
	// Should show a prompt to press enter.
	if !strings.Contains(lower, "enter") && !strings.Contains(lower, "confirm") {
		t.Errorf("RenderSync(confirm) should show enter/confirm prompt; got:\n%s", out)
	}
}

// TestRenderSync_RunningState verifies that while sync is running the screen
// shows a spinner/progress indicator.
func TestRenderSync_RunningState(t *testing.T) {
	out := RenderSync(0, nil, true /*operationRunning*/, false, 0, 120)

	lower := strings.ToLower(out)
	if !strings.Contains(lower, "syncing") && !strings.Contains(lower, "please wait") {
		t.Errorf("RenderSync(running) should show 'syncing' or 'please wait'; got:\n%s", out)
	}
}

// TestRenderSync_ResultWithFilesChanged verifies that after a successful sync
// with changed files, the screen shows the file count.
func TestRenderSync_ResultWithFilesChanged(t *testing.T) {
	const filesChanged = 5
	out := RenderSync(filesChanged, nil, false, true /*hasSyncRun*/, 0, 120)

	if !strings.Contains(out, "5") {
		t.Errorf("RenderSync(filesChanged=5) should show '5'; got:\n%s", out)
	}
	lower := strings.ToLower(out)
	if !strings.Contains(lower, "sync") {
		t.Errorf("RenderSync(result) should mention 'sync'; got:\n%s", out)
	}
}

// TestRenderSync_ResultWithError verifies that a failed sync shows the error
// message.
func TestRenderSync_ResultWithError(t *testing.T) {
	syncErr := fmt.Errorf("connection refused: agent config dir not writable")
	out := RenderSync(0, syncErr, false, true /*hasSyncRun*/, 0, 120)

	lower := strings.ToLower(out)
	if !strings.Contains(lower, "fail") && !strings.Contains(lower, "error") {
		t.Errorf("RenderSync(error) should show failure indicator; got:\n%s", out)
	}
	if !strings.Contains(out, syncErr.Error()) {
		t.Errorf("RenderSync(error) should show error text %q; got:\n%s", syncErr.Error(), out)
	}
}

// TestRenderSync_TitleAlwaysPresent verifies the screen title is shown in all
// states.
func TestRenderSync_TitleAlwaysPresent(t *testing.T) {
	states := []struct {
		name             string
		filesChanged     int
		syncErr          error
		operationRunning bool
		hasSyncRun       bool
	}{
		{"confirm", 0, nil, false, false},
		{"running", 0, nil, true, false},
		{"success", 3, nil, false, true},
		{"error", 0, fmt.Errorf("fail"), false, true},
	}

	for _, s := range states {
		t.Run(s.name, func(t *testing.T) {
			out := RenderSync(s.filesChanged, s.syncErr, s.operationRunning, s.hasSyncRun, 0, 120)
			if !strings.Contains(out, "Sync") {
				t.Errorf("RenderSync state=%q should contain 'Sync'; got:\n%s", s.name, out)
			}
		})
	}
}

// TestRenderSync_ZeroFilesChangedWithNoError verifies the "nothing to update"
// case (hasSyncRun=true, filesChanged=0, no error) shows a completion message.
func TestRenderSync_ZeroFilesChangedWithNoError(t *testing.T) {
	out := RenderSync(0, nil, false, true /*hasSyncRun*/, 0, 120)

	lower := strings.ToLower(out)
	if !strings.Contains(lower, "sync complete") && !strings.Contains(lower, "complete") &&
		!strings.Contains(lower, "no agents") {
		t.Errorf("RenderSync(0 files, no error) should show completion; got:\n%s", out)
	}
}

// TestRenderSync_ResultWithVerifyError_DropsOkLines verifies that when the
// error carries a full verification report (hundreds of [ok] lines plus a
// few [!!] failures), the result screen surfaces only the failures and the
// summary, NOT the [ok] noise that would otherwise overflow the screen.
func TestRenderSync_ResultWithVerifyError_DropsOkLines(t *testing.T) {
	var raw strings.Builder
	raw.WriteString("post-sync verification failed:\n")
	raw.WriteString("Verification checks: 349 passed, 2 failed, 0 warnings, 0 skipped\n")
	for i := 0; i < 349; i++ {
		fmt.Fprintf(&raw, "[ok] verify:sync:file:/path/file-%d.md - synced file exists\n", i)
	}
	raw.WriteString("[!!] verify:sync:file:/path/missing-1.agent.md - synced file exists (file not found)\n")
	raw.WriteString("[!!] verify:sync:file:/path/missing-2.agent.md - synced file exists (file not found)\n")
	raw.WriteString("Installation completed with verification issues. Run repair on failed checks.\n")

	out := RenderSync(0, fmt.Errorf("%s", raw.String()), false, true, 0, 120)

	if strings.Contains(out, "[ok]") {
		t.Errorf("RenderSync should drop [ok] lines from a verify-style error; got:\n%s", out)
	}
	// New format: header line is "[!!] <basename>" followed by indented continuation rows.
	if !strings.Contains(out, "[!!] missing-1.agent.md") {
		t.Errorf("RenderSync should show the failed file's basename in the marker line; got:\n%s", out)
	}
	if !strings.Contains(out, "[!!] missing-2.agent.md") {
		t.Errorf("RenderSync should show every failed file (within cap); got:\n%s", out)
	}
	// Continuation row carries the error reason — not on the marker line.
	if !strings.Contains(out, "file not found") {
		t.Errorf("RenderSync should surface the error reason on a continuation row; got:\n%s", out)
	}
	if !strings.Contains(out, "Verification checks: 349 passed, 2 failed") {
		t.Errorf("RenderSync should keep the verification summary line; got:\n%s", out)
	}
}

// TestRenderSync_ResultWithVerifyError_NarrowTerminalTruncatesPath verifies
// that on a narrow terminal the long full-path continuation line is
// middle-truncated with an ellipsis so head and tail stay visible.
func TestRenderSync_ResultWithVerifyError_NarrowTerminalTruncatesPath(t *testing.T) {
	longPath := "/a/very/long/absolute/path/that/should/get/middle/truncated/file.md"
	var raw strings.Builder
	raw.WriteString("post-sync verification failed:\n")
	raw.WriteString("Verification checks: 0 passed, 1 failed, 0 warnings, 0 skipped\n")
	fmt.Fprintf(&raw, "[!!] verify:sync:file:%s - missing (cannot find the file specified)\n", longPath)

	out := RenderSync(0, fmt.Errorf("%s", raw.String()), false, true, 0, 50)

	if !strings.Contains(out, "[!!] file.md") {
		t.Errorf("narrow render should still show basename on marker line; got:\n%s", out)
	}
	if strings.Contains(out, longPath) {
		t.Errorf("narrow render should truncate the full path with an ellipsis, not include it whole; got:\n%s", out)
	}
	if !strings.Contains(out, "…") {
		t.Errorf("narrow render should use the … ellipsis to mark the truncation; got:\n%s", out)
	}
}

// TestRenderSync_ResultWithVerifyError_TruncatesManyFailures verifies that
// when there are more than maxFailureLinesShown failures, the screen shows
// the cap plus a "and N more" hint pointing the user to the full log.
func TestRenderSync_ResultWithVerifyError_TruncatesManyFailures(t *testing.T) {
	var raw strings.Builder
	raw.WriteString("post-sync verification failed:\n")
	raw.WriteString("Verification checks: 0 passed, 30 failed, 0 warnings, 0 skipped\n")
	for i := 0; i < 30; i++ {
		fmt.Fprintf(&raw, "[!!] verify:sync:file:/path/missing-%d.md - file missing\n", i)
	}

	out := RenderSync(0, fmt.Errorf("%s", raw.String()), false, true, 0, 120)

	if !strings.Contains(out, "and ") || !strings.Contains(out, "more failed check") {
		t.Errorf("RenderSync should show '… and N more failed check(s)' truncation hint; got:\n%s", out)
	}
}

// TestRenderSync_ResultWithVerifyError_AllOkBodyFallsBackToFullText verifies
// the defensive fallback: if a verify report came through with only [ok]/[--]
// lines and no prose either, the user must still see SOMETHING — we fall back
// to rendering the original lines rather than an empty body.
func TestRenderSync_ResultWithVerifyError_AllOkBodyFallsBackToFullText(t *testing.T) {
	var raw strings.Builder
	raw.WriteString("Verification checks: 5 passed, 0 failed, 0 warnings, 0 skipped\n")
	for i := 0; i < 5; i++ {
		fmt.Fprintf(&raw, "[ok] verify:sync:file:/path/file-%d.md - synced\n", i)
	}
	out := RenderSync(0, fmt.Errorf("%s", raw.String()), false, true, 0, 120)
	if !strings.Contains(out, "Verification checks: 5 passed") {
		t.Errorf("all-ok body should still be visible to user; got:\n%s", out)
	}
}

// TestRenderSync_ResultWithVerifyError_FinalNoteAppearsAfterFailures verifies
// the trailing recommendation line appears AFTER the failures it refers to,
// not before. Bucketing failures separately would invert the order.
func TestRenderSync_ResultWithVerifyError_FinalNoteAppearsAfterFailures(t *testing.T) {
	raw := "post-sync verification failed:\n" +
		"Verification checks: 0 passed, 1 failed, 0 warnings, 0 skipped\n" +
		"[!!] verify:sync:file:/p/missing.md - missing\n" +
		"Installation completed with verification issues. Run repair on failed checks.\n"
	out := RenderSync(0, fmt.Errorf("%s", raw), false, true, 0, 120)
	failIdx := strings.Index(out, "missing.md")
	finalIdx := strings.Index(out, "Installation completed")
	if failIdx == -1 || finalIdx == -1 {
		t.Fatalf("missing expected lines in output:\n%s", out)
	}
	if failIdx > finalIdx {
		t.Errorf("failure line should appear BEFORE the FinalNote; got fail@%d, final@%d", failIdx, finalIdx)
	}
}

// TestFormatFailureLine_PathWithSpaces verifies that paths containing spaces
// (realistic on Windows: "C:\Users\Aruiz Gonzalez\...") still parse and
// produce a basename row instead of falling back to the raw single-line form.
func TestFormatFailureLine_PathWithSpaces(t *testing.T) {
	line := `[!!] verify:sync:file:C:\Users\Aruiz Gonzalez\file.md - missing (cannot find file)`
	lines := formatFailureLine(line, 120)
	found := false
	for _, l := range lines {
		if strings.Contains(l, "file.md") && !strings.Contains(l, "verify:") {
			found = true
		}
	}
	if !found {
		t.Errorf("path-with-spaces should still produce a basename row; got %v", lines)
	}
}

// TestFormatFailureLine_WindowsDrivePath verifies that on Windows the drive
// letter is NOT stripped from displayID (the previous strings.LastIndex(":")
// logic returned "\Users\foo\bar.md" instead of the full path).
func TestFormatFailureLine_WindowsDrivePath(t *testing.T) {
	line := `[!!] verify:sync:file:C:\Users\foo\bar.md - missing (cannot find file)`
	lines := formatFailureLine(line, 120)
	// We expect the marker line + a path continuation row containing the full
	// drive-prefixed path.
	hasFullPath := false
	for _, l := range lines {
		if strings.Contains(l, `C:\Users\foo\bar.md`) {
			hasFullPath = true
		}
	}
	if !hasFullPath {
		t.Errorf("windows drive path should be preserved; got %v", lines)
	}
}

// TestCompactErrorLines_ExactlyMaxFailures verifies that exactly
// maxFailureLinesShown failures emits NO truncation hint (boundary).
func TestCompactErrorLines_ExactlyMaxFailures(t *testing.T) {
	var raw strings.Builder
	raw.WriteString("Verification checks: 0 passed, 12 failed, 0 warnings, 0 skipped\n")
	for i := 0; i < maxFailureLinesShown; i++ {
		fmt.Fprintf(&raw, "[!!] verify:sync:file:/p/f-%d.md - missing\n", i)
	}
	out := compactErrorLines(raw.String(), 120)
	joined := strings.Join(out, "\n")
	if strings.Contains(joined, "more failed check") {
		t.Errorf("exactly max failures should NOT show truncation hint; got:\n%s", joined)
	}
}

// TestCompactErrorLines_MaxPlusOneFailures verifies that maxFailureLinesShown+1
// failures produces the truncation hint with the right count.
func TestCompactErrorLines_MaxPlusOneFailures(t *testing.T) {
	var raw strings.Builder
	raw.WriteString("Verification checks: 0 passed, 13 failed, 0 warnings, 0 skipped\n")
	for i := 0; i < maxFailureLinesShown+1; i++ {
		fmt.Fprintf(&raw, "[!!] verify:sync:file:/p/f-%d.md - missing\n", i)
	}
	out := compactErrorLines(raw.String(), 120)
	joined := strings.Join(out, "\n")
	if !strings.Contains(joined, "and 1 more failed check") {
		t.Errorf("max+1 failures should show 'and 1 more failed check'; got:\n%s", joined)
	}
}

// TestCompactErrorLines_ZeroWidthFallsBackSafely verifies that width=0 falls
// back to defaultErrorWidth (so paths get truncated to fit ≤80 cols rather
// than overflowing on a width-less terminal).
func TestCompactErrorLines_ZeroWidthFallsBackSafely(t *testing.T) {
	// A path long enough that it MUST be truncated under defaultErrorWidth=80
	// (with a 7-char indent budget, truncation kicks in past ~73 chars).
	longPath := "/a/very/long/path/that/should/get/middle/truncated/when/width/is/zero/file.md"
	raw := "Verification checks: 0 passed, 1 failed, 0 warnings, 0 skipped\n" +
		"[!!] verify:sync:file:" + longPath + " - missing\n"

	out := compactErrorLines(raw, 0)
	if len(out) == 0 {
		t.Fatalf("width=0 should still produce output; got empty slice")
	}
	// Observable contract: with width=0 → defaultErrorWidth=80, the long path
	// should be middle-truncated (contain the ellipsis) rather than emitted in
	// full.
	joined := strings.Join(out, "\n")
	if strings.Contains(joined, longPath) {
		t.Errorf("width=0 should apply defaultErrorWidth truncation; full path leaked into output:\n%s", joined)
	}
	if !strings.Contains(joined, "…") {
		t.Errorf("width=0 should produce a truncation ellipsis; got:\n%s", joined)
	}
}

// TestFormatFailureLine_PathContainingDashSeparator verifies that a verify
// line whose path legitimately contains " - " (e.g. "Resume - 2024.md" or
// "OneDrive - Informa\file.md") is parsed correctly: the basename keeps its
// ` - ` portion and the description is split on the LAST " - ", not the first.
func TestFormatFailureLine_PathContainingDashSeparator(t *testing.T) {
	cases := []struct {
		name             string
		line             string
		wantBasename     string
		wantPathContains string
	}{
		{
			name:             "windows-onedrive-org-folder",
			line:             `[!!] verify:sync:file:C:\Users\foo\OneDrive - Informa\config.md - synced file exists (cannot find file)`,
			wantBasename:     "config.md",
			wantPathContains: "OneDrive - Informa",
		},
		{
			name:             "filename-with-dash-separator",
			line:             `[!!] verify:sync:file:/home/foo/Resume - 2024.md - synced file exists (cannot find file)`,
			wantBasename:     "Resume - 2024.md",
			wantPathContains: "Resume - 2024.md",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			lines := formatFailureLine(tc.line, 200)
			if len(lines) == 0 {
				t.Fatalf("formatFailureLine returned no lines for %q", tc.line)
			}
			if !strings.Contains(lines[0], tc.wantBasename) {
				t.Errorf("marker line %q should contain basename %q", lines[0], tc.wantBasename)
			}
			joined := strings.Join(lines, "\n")
			if !strings.Contains(joined, tc.wantPathContains) {
				t.Errorf("output should preserve %q; got:\n%s", tc.wantPathContains, joined)
			}
		})
	}
}

// TestTruncateMiddle_Boundaries documents the contract for truncateMiddle's
// edge cases (max=0, max<5, max>=len, multibyte).
func TestTruncateMiddle_Boundaries(t *testing.T) {
	tests := []struct {
		name string
		s    string
		max  int
		want string
	}{
		{"max-zero-returns-input", "hello", 0, "hello"},
		{"max-negative-returns-input", "hello", -1, "hello"},
		{"max-greater-than-len", "hi", 10, "hi"},
		{"max-equal-to-len", "hello", 5, "hello"},
{"max-4-no-ellipsis", "abcdef", 4, "abcd"},
		{"max-5-with-ellipsis", "abcdefgh", 5, "ab…gh"},
		// max=3 hits the early-return branch; the multibyte coverage that
		// matters is the ellipsis path with multibyte runes — added below.
		{"multibyte-early-return", "αβγδε", 3, "αβγ"},
		{"multibyte-with-ellipsis", "αβγδεζηθικ", 7, "αβγ…θικ"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := truncateMiddle(tt.s, tt.max); got != tt.want {
				t.Errorf("truncateMiddle(%q, %d) = %q, want %q", tt.s, tt.max, got, tt.want)
			}
		})
	}
}
