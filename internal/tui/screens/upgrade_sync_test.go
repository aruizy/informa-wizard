package screens

import (
	"fmt"
	"strings"
	"testing"

	"gitlab.informa.tools/ai/wizard/informa-wizard/internal/cli"
	"gitlab.informa.tools/ai/wizard/informa-wizard/internal/update"
	"gitlab.informa.tools/ai/wizard/informa-wizard/internal/update/upgrade"
)

// noPreview is a helper for tests that don't exercise the preview state.
var noPreview = cli.SyncPreview{}

// ─── RenderUpgradeSync states ──────────────────────────────────────────────

// TestRenderUpgradeSync_ConfirmState verifies the default confirmation screen
// (not running, no results) shows the two-step description.
func TestRenderUpgradeSync_ConfirmState(t *testing.T) {
	out := RenderUpgradeSync(nil, nil, 0, nil, nil, false /*operationRunning*/, true /*updateCheckDone*/, 0, 0, false, 0, noPreview)

	lower := strings.ToLower(out)
	// Must mention both operations.
	if !strings.Contains(lower, "update") {
		t.Errorf("RenderUpgradeSync(confirm) should mention 'update'; got:\n%s", out)
	}
	if !strings.Contains(lower, "sync") {
		t.Errorf("RenderUpgradeSync(confirm) should mention 'sync'; got:\n%s", out)
	}
	// Must show a prompt.
	if !strings.Contains(lower, "enter") && !strings.Contains(lower, "begin") {
		t.Errorf("RenderUpgradeSync(confirm) should show enter/begin prompt; got:\n%s", out)
	}
}

// TestRenderUpgradeSync_RunningUpgradePhase verifies that while the update is
// running (operationRunning=true, upgradeReport=nil), the screen shows an
// "updating" indicator.
func TestRenderUpgradeSync_RunningUpgradePhase(t *testing.T) {
	out := RenderUpgradeSync(nil, nil, 0, nil, nil, true /*operationRunning*/, true, 0, 0, false, 0, noPreview)

	lower := strings.ToLower(out)
	if !strings.Contains(lower, "updating") && !strings.Contains(lower, "please wait") {
		t.Errorf("RenderUpgradeSync(updating) should show progress; got:\n%s", out)
	}
}

// TestRenderUpgradeSync_RunningSyncPhase verifies that when update is done but
// sync is still running (operationRunning=true, upgradeReport!=nil), the screen
// shows the update complete indicator and sync progress.
func TestRenderUpgradeSync_RunningSyncPhase(t *testing.T) {
	report := &upgrade.UpgradeReport{
		Results: []upgrade.ToolUpgradeResult{
			{ToolName: "informa-wizard", OldVersion: "v1.0.0", NewVersion: "v2.0.0", Status: upgrade.UpgradeSucceeded},
		},
	}

	out := RenderUpgradeSync(nil, report, 0, nil, nil, true /*operationRunning*/, true, 0, 0, false, 2, noPreview)

	lower := strings.ToLower(out)
	// Update done indicator.
	if !strings.Contains(lower, "update complete") {
		t.Errorf("RenderUpgradeSync(sync phase) should show 'update complete'; got:\n%s", out)
	}
	// Sync in progress.
	if !strings.Contains(lower, "syncing") && !strings.Contains(lower, "please wait") {
		t.Errorf("RenderUpgradeSync(sync phase) should show sync progress; got:\n%s", out)
	}
}

// TestRenderUpgradeSync_CombinedResult verifies that when both operations are
// done (operationRunning=false, upgradeReport!=nil), the screen shows both
// update and sync results.
func TestRenderUpgradeSync_CombinedResult(t *testing.T) {
	report := &upgrade.UpgradeReport{
		Results: []upgrade.ToolUpgradeResult{
			{ToolName: "informa-wizard", OldVersion: "v1.0.0", NewVersion: "v2.0.0", Status: upgrade.UpgradeSucceeded},
		},
	}
	const syncFilesChanged = 3

	out := RenderUpgradeSync(nil, report, syncFilesChanged, nil, nil, false /*operationRunning*/, true, 0, 0, false, 3, noPreview)

	// Must mention both result sections.
	if !strings.Contains(out, "Update Results") {
		t.Errorf("RenderUpgradeSync(combined) should show 'Update Results'; got:\n%s", out)
	}
	if !strings.Contains(out, "Sync Results") {
		t.Errorf("RenderUpgradeSync(combined) should show 'Sync Results'; got:\n%s", out)
	}
	// Sync file count.
	if !strings.Contains(out, "3") {
		t.Errorf("RenderUpgradeSync(combined) should show sync file count '3'; got:\n%s", out)
	}
}

// TestRenderUpgradeSync_CombinedResultWithSyncError verifies that a sync error
// is shown in the combined result.
func TestRenderUpgradeSync_CombinedResultWithSyncError(t *testing.T) {
	report := &upgrade.UpgradeReport{
		Results: []upgrade.ToolUpgradeResult{},
	}
	syncErr := fmt.Errorf("permission denied writing config")

	out := RenderUpgradeSync(nil, report, 0, nil, syncErr, false, true, 0, 0, false, 3, noPreview)

	lower := strings.ToLower(out)
	if !strings.Contains(lower, "fail") && !strings.Contains(lower, "error") {
		t.Errorf("RenderUpgradeSync(sync error) should show failure indicator; got:\n%s", out)
	}
	if !strings.Contains(out, syncErr.Error()) {
		t.Errorf("RenderUpgradeSync(sync error) should show error text; got:\n%s", out)
	}
}

// TestRenderUpgradeSync_CombinedResultWithUpgradeError verifies that an
// update error is shown in the combined result (upgradeErr != nil, report nil).
func TestRenderUpgradeSync_CombinedResultWithUpgradeError(t *testing.T) {
	upgradeErr := fmt.Errorf("network timeout during upgrade")

	out := RenderUpgradeSync(nil, nil, 2, upgradeErr, nil, false, true, 0, 0, false, 3, noPreview)

	if !strings.Contains(out, "Update Results") {
		t.Errorf("RenderUpgradeSync(upgradeErr) should show 'Update Results'; got:\n%s", out)
	}
	if !strings.Contains(out, upgradeErr.Error()) {
		t.Errorf("RenderUpgradeSync(upgradeErr) should show error text %q; got:\n%s", upgradeErr.Error(), out)
	}
	if !strings.Contains(out, "Sync Results") {
		t.Errorf("RenderUpgradeSync(upgradeErr) should show 'Sync Results'; got:\n%s", out)
	}
}

// TestRenderUpgradeSync_TitleAlwaysPresent verifies the screen title is shown.
func TestRenderUpgradeSync_TitleAlwaysPresent(t *testing.T) {
	states := []struct {
		name             string
		report           *upgrade.UpgradeReport
		operationRunning bool
		updateCheckDone  bool
		phase            int
	}{
		{"confirm", nil, false, true, 0},
		{"updating", nil, true, true, 0},
	}

	for _, s := range states {
		t.Run(s.name, func(t *testing.T) {
			out := RenderUpgradeSync(nil, s.report, 0, nil, nil, s.operationRunning, s.updateCheckDone, 0, 0, false, s.phase, noPreview)
			if !strings.Contains(out, "Update + Sync") {
				t.Errorf("RenderUpgradeSync state=%q should contain 'Update + Sync'; got:\n%s", s.name, out)
			}
		})
	}
}

// TestRenderUpgradeSync_ConfirmState_NoUpdateCheckWait verifies the confirm screen
// shows immediately without waiting for an update check (updateCheckDone=false).
func TestRenderUpgradeSync_ConfirmState_NoUpdateCheckWait(t *testing.T) {
	results := []update.UpdateResult{}
	out := RenderUpgradeSync(results, nil, 0, nil, nil, false, false /*updateCheckDone=false*/, 0, 0, false, 0, noPreview)

	lower := strings.ToLower(out)
	// Should show the confirmation screen (not a "checking" spinner).
	if !strings.Contains(lower, "update") {
		t.Errorf("RenderUpgradeSync(updateCheckDone=false) should show confirm screen; got:\n%s", out)
	}
}

// ─── Preview state ──────────────────────────────────────────────────────────

// TestRenderUpgradeSync_PreviewState_Empty verifies that an empty preview
// (no files would change) shows an appropriate message.
func TestRenderUpgradeSync_PreviewState_Empty(t *testing.T) {
	out := RenderUpgradeSync(nil, nil, 0, nil, nil, false, true, 0, 0, false, 1, cli.SyncPreview{})

	lower := strings.ToLower(out)
	if !strings.Contains(lower, "preview") {
		t.Errorf("RenderUpgradeSync(preview,empty) should show 'preview'; got:\n%s", out)
	}
	if !strings.Contains(lower, "no files") {
		t.Errorf("RenderUpgradeSync(preview,empty) should mention 'no files'; got:\n%s", out)
	}
}

// TestRenderUpgradeSync_PreviewState_WithComponents verifies that a non-empty
// preview lists the components and shows the correct prompt.
func TestRenderUpgradeSync_PreviewState_WithComponents(t *testing.T) {
	preview := cli.SyncPreview{
		Components: []cli.ComponentPreview{
			{
				ID:            "sdd",
				Files:         []string{"/home/user/.claude/skills/sdd-init/SKILL.md", "/home/user/.claude/agents/sdd-init.md"},
				NewFiles:      1,
				ModifiedFiles: 1,
			},
			{
				ID:            "skills",
				Files:         []string{"/home/user/.claude/skills/commitpush/SKILL.md"},
				NewFiles:      0,
				ModifiedFiles: 1,
			},
		},
	}

	out := RenderUpgradeSync(nil, nil, 0, nil, nil, false, true, 0, 0, false, 1, preview)

	lower := strings.ToLower(out)
	if !strings.Contains(lower, "preview") {
		t.Errorf("RenderUpgradeSync(preview) should show 'preview'; got:\n%s", out)
	}
	if !strings.Contains(out, "sdd") {
		t.Errorf("RenderUpgradeSync(preview) should list 'sdd' component; got:\n%s", out)
	}
	if !strings.Contains(out, "skills") {
		t.Errorf("RenderUpgradeSync(preview) should list 'skills' component; got:\n%s", out)
	}
	if !strings.Contains(lower, "enter") {
		t.Errorf("RenderUpgradeSync(preview) should show enter prompt; got:\n%s", out)
	}
	if !strings.Contains(lower, "esc") {
		t.Errorf("RenderUpgradeSync(preview) should show esc/cancel prompt; got:\n%s", out)
	}
	// Total should be reported.
	if !strings.Contains(out, "3") {
		t.Errorf("RenderUpgradeSync(preview) should show total file count; got:\n%s", out)
	}
}

// TestRenderUpgradeSync_PreviewState_Truncation verifies that large file lists
// are truncated to previewMaxFilesPerComponent + "... and N more".
func TestRenderUpgradeSync_PreviewState_Truncation(t *testing.T) {
	files := make([]string, 10)
	for i := range files {
		files[i] = fmt.Sprintf("/home/user/.claude/skills/sdd-%d/SKILL.md", i)
	}
	preview := cli.SyncPreview{
		Components: []cli.ComponentPreview{
			{ID: "sdd", Files: files, ModifiedFiles: 10},
		},
	}

	out := RenderUpgradeSync(nil, nil, 0, nil, nil, false, true, 0, 0, false, 1, preview)

	// Should contain truncation message for 10 - previewMaxFilesPerComponent remaining.
	remaining := 10 - previewMaxFilesPerComponent
	truncMsg := fmt.Sprintf("and %d more", remaining)
	if !strings.Contains(out, truncMsg) {
		t.Errorf("RenderUpgradeSync(preview,truncation) should show %q; got:\n%s", truncMsg, out)
	}
}
