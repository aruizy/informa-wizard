package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gitlab.informa.tools/ai/wizard/informa-wizard/internal/backup"
	"gitlab.informa.tools/ai/wizard/informa-wizard/internal/model"
	"gitlab.informa.tools/ai/wizard/informa-wizard/internal/state"
)

// TestListBackupsNewestFirst verifies that ListBackups returns manifests sorted
// newest-first by CreatedAt timestamp, matching the spec "newest first" ordering.
func TestListBackupsNewestFirst(t *testing.T) {
	home := t.TempDir()
	backupRoot := filepath.Join(home, ".informa-wizard", "backups")

	older := backup.Manifest{
		ID:        "older",
		CreatedAt: time.Date(2026, 3, 20, 10, 0, 0, 0, time.UTC),
		RootDir:   filepath.Join(backupRoot, "older"),
		Entries:   []backup.ManifestEntry{},
	}
	newer := backup.Manifest{
		ID:        "newer",
		CreatedAt: time.Date(2026, 3, 22, 15, 4, 5, 0, time.UTC),
		RootDir:   filepath.Join(backupRoot, "newer"),
		Entries:   []backup.ManifestEntry{},
	}

	// Write older backup first.
	for _, m := range []backup.Manifest{older, newer} {
		dir := filepath.Join(backupRoot, m.ID)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
		if err := backup.WriteManifest(filepath.Join(dir, backup.ManifestFilename), m); err != nil {
			t.Fatalf("WriteManifest: %v", err)
		}
	}

	// Temporarily override home dir resolution for ListBackups.
	origHomeDir := os.Getenv("HOME")
	t.Cleanup(func() { os.Setenv("HOME", origHomeDir) })
	os.Setenv("HOME", home)

	manifests := ListBackups()

	if len(manifests) != 2 {
		t.Fatalf("ListBackups() returned %d manifests, want 2", len(manifests))
	}

	// Newest must be first.
	if manifests[0].ID != "newer" {
		t.Errorf("ListBackups()[0].ID = %q, want %q (newest first)", manifests[0].ID, "newer")
	}
	if manifests[1].ID != "older" {
		t.Errorf("ListBackups()[1].ID = %q, want %q", manifests[1].ID, "older")
	}
}

// TestListBackupsWithSourceMetadata verifies that ListBackups returns manifests
// with Source metadata intact, so display labels can use the source field.
func TestListBackupsWithSourceMetadata(t *testing.T) {
	home := t.TempDir()
	backupRoot := filepath.Join(home, ".informa-wizard", "backups")

	m := backup.Manifest{
		ID:          "test-with-source",
		CreatedAt:   time.Now().UTC(),
		RootDir:     filepath.Join(backupRoot, "test-with-source"),
		Source:      backup.BackupSourceInstall,
		Description: "pre-install snapshot",
		Entries:     []backup.ManifestEntry{},
	}

	dir := filepath.Join(backupRoot, m.ID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := backup.WriteManifest(filepath.Join(dir, backup.ManifestFilename), m); err != nil {
		t.Fatalf("WriteManifest: %v", err)
	}

	origHome := os.Getenv("HOME")
	t.Cleanup(func() { os.Setenv("HOME", origHome) })
	os.Setenv("HOME", home)

	manifests := ListBackups()

	if len(manifests) != 1 {
		t.Fatalf("ListBackups() returned %d manifests, want 1", len(manifests))
	}

	got := manifests[0]
	if got.Source != backup.BackupSourceInstall {
		t.Errorf("Source = %q, want %q", got.Source, backup.BackupSourceInstall)
	}
	if got.Description != "pre-install snapshot" {
		t.Errorf("Description = %q, want %q", got.Description, "pre-install snapshot")
	}
}

// TestRunArgsRestoreListIsDispatched verifies that `informa-wizard restore --list`
// is correctly dispatched through RunArgs and produces a meaningful response
// (either a backup list or a "no backups" message — never "unknown command").
func TestRunArgsRestoreListIsDispatched(t *testing.T) {
	home := t.TempDir()
	origHome := os.Getenv("HOME")
	t.Cleanup(func() { os.Setenv("HOME", origHome) })
	os.Setenv("HOME", home)

	var buf bytes.Buffer
	err := RunArgs([]string{"restore", "--list"}, &buf)
	if err != nil {
		t.Fatalf("RunArgs(restore --list) error = %v", err)
	}

	out := buf.String()
	if out == "" {
		t.Fatalf("restore --list produced no output")
	}

	// Must not produce "unknown command".
	if strings.Contains(out, "unknown command") {
		t.Errorf("restore is not registered in RunArgs; got: %s", out)
	}
}

// TestRunArgsRestoreByIDWithYes verifies end-to-end wiring of `restore <id> --yes`
// through app.RunArgs.
func TestRunArgsRestoreByIDWithYes(t *testing.T) {
	home := t.TempDir()
	backupRoot := filepath.Join(home, ".informa-wizard", "backups")

	// Create a backup with a real file entry so restore can succeed.
	sourceFile := filepath.Join(home, "config.md")
	if err := os.WriteFile(sourceFile, []byte("original\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	snapshotDir := filepath.Join(backupRoot, "test-backup-001")
	snapshotFile := filepath.Join(snapshotDir, "files", "config.md")
	if err := os.MkdirAll(filepath.Dir(snapshotFile), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(snapshotFile, []byte("backup-content\n"), 0o644); err != nil {
		t.Fatalf("WriteFile snapshot: %v", err)
	}

	m := backup.Manifest{
		ID:        "test-backup-001",
		CreatedAt: time.Now().UTC(),
		RootDir:   snapshotDir,
		Source:    backup.BackupSourceInstall,
		Entries: []backup.ManifestEntry{
			{OriginalPath: sourceFile, SnapshotPath: snapshotFile, Existed: true, Mode: 0o644},
		},
	}
	if err := backup.WriteManifest(filepath.Join(snapshotDir, backup.ManifestFilename), m); err != nil {
		t.Fatalf("WriteManifest: %v", err)
	}

	origHome := os.Getenv("HOME")
	t.Cleanup(func() { os.Setenv("HOME", origHome) })
	os.Setenv("HOME", home)

	var buf bytes.Buffer
	err := RunArgs([]string{"restore", "test-backup-001", "--yes"}, &buf)
	if err != nil {
		t.Fatalf("RunArgs(restore test-backup-001 --yes) error = %v", err)
	}

	out := buf.String()
	if !strings.Contains(strings.ToLower(out), "restor") {
		t.Errorf("restore output should confirm restoration; got:\n%s", out)
	}
}

// TestRunArgsRestoreUnknownIDReturnsError verifies that an unknown backup ID
// is surfaced as an error from RunArgs.
func TestRunArgsRestoreUnknownIDReturnsError(t *testing.T) {
	home := t.TempDir()
	origHome := os.Getenv("HOME")
	t.Cleanup(func() { os.Setenv("HOME", origHome) })
	os.Setenv("HOME", home)

	var buf bytes.Buffer
	err := RunArgs([]string{"restore", "no-such-backup", "--yes"}, &buf)
	if err == nil {
		t.Fatalf("RunArgs(restore no-such-backup) expected error")
	}
	if strings.Contains(err.Error(), "unknown command") {
		t.Errorf("restore returned 'unknown command' — not dispatched: %v", err)
	}
}

// TestListBackupsFallsBackGracefullyForOldManifests verifies that old manifests
// without Source/Description are still returned (not skipped) and can be displayed
// via DisplayLabel without panicking.
func TestListBackupsFallsBackGracefullyForOldManifests(t *testing.T) {
	_ = fmt.Sprintf // Ensure fmt is used.
	home := t.TempDir()
	backupRoot := filepath.Join(home, ".informa-wizard", "backups")

	// Write a manifest with no Source/Description.
	m := backup.Manifest{
		ID:        "old-backup",
		CreatedAt: time.Now().UTC(),
		RootDir:   filepath.Join(backupRoot, "old-backup"),
		Entries:   []backup.ManifestEntry{},
		// Source and Description intentionally omitted — simulates old manifest.
	}

	dir := filepath.Join(backupRoot, m.ID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := backup.WriteManifest(filepath.Join(dir, backup.ManifestFilename), m); err != nil {
		t.Fatalf("WriteManifest: %v", err)
	}

	origHome := os.Getenv("HOME")
	t.Cleanup(func() { os.Setenv("HOME", origHome) })
	os.Setenv("HOME", home)

	manifests := ListBackups()

	if len(manifests) != 1 {
		t.Fatalf("ListBackups() returned %d manifests, want 1", len(manifests))
	}

	// Must not panic — DisplayLabel should handle empty Source gracefully.
	label := manifests[0].DisplayLabel()
	if label == "" {
		t.Errorf("DisplayLabel() returned empty string, want non-empty fallback label")
	}
}

// ─── BUG 3: SyncOverrides.StrictTDD never read in tuiSync ───────────────────

// TestTuiSyncAppliesStrictTDDOverride verifies that applyOverrides correctly
// merges SyncOverrides.StrictTDD into the selection.
// Previously, the field was declared on SyncOverrides but never applied.
func TestTuiSyncAppliesStrictTDDOverride(t *testing.T) {
	sel := boolPtr(true)
	overrides := &model.SyncOverrides{StrictTDD: sel}

	selection := model.Selection{StrictTDD: false}
	applyOverrides(&selection, overrides)

	if !selection.StrictTDD {
		t.Fatalf("Selection.StrictTDD = false after applyOverrides with StrictTDD=true override; field is not being applied")
	}
}

// TestTuiSyncAppliesStrictTDDOverrideFalse verifies the override correctly sets
// StrictTDD to false when the pointer points to false.
func TestTuiSyncAppliesStrictTDDOverrideFalse(t *testing.T) {
	sel := boolPtr(false)
	overrides := &model.SyncOverrides{StrictTDD: sel}

	selection := model.Selection{StrictTDD: true}
	applyOverrides(&selection, overrides)

	if selection.StrictTDD {
		t.Fatalf("Selection.StrictTDD = true after applyOverrides with StrictTDD=false override")
	}
}

// TestTuiSyncStrictTDDNilOverrideNoChange verifies that when StrictTDD override
// is nil, the selection's existing value is preserved.
func TestTuiSyncStrictTDDNilOverrideNoChange(t *testing.T) {
	overrides := &model.SyncOverrides{StrictTDD: nil}

	selection := model.Selection{StrictTDD: true}
	applyOverrides(&selection, overrides)

	if !selection.StrictTDD {
		t.Fatalf("Selection.StrictTDD changed unexpectedly; nil override should not modify the field")
	}
}

func boolPtr(b bool) *bool { return &b }

// ─── ClaudeModelPreset override propagation ────────────────────────────────

// TestApplyOverrides_AppliesClaudeModelPreset verifies that a non-empty
// ClaudeModelPreset override is merged into the selection, while an empty
// override leaves the selection's existing value unchanged.
func TestApplyOverrides_AppliesClaudeModelPreset(t *testing.T) {
	cases := []struct {
		name             string
		startingPreset   string
		overridePreset   string
		wantPreset       string
	}{
		{
			name:           "empty override preserves existing",
			startingPreset: "balanced",
			overridePreset: "",
			wantPreset:     "balanced",
		},
		{
			name:           "balanced override applied",
			startingPreset: "",
			overridePreset: "balanced",
			wantPreset:     "balanced",
		},
		{
			name:           "performance override applied",
			startingPreset: "balanced",
			overridePreset: "performance",
			wantPreset:     "performance",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			selection := model.Selection{ClaudeModelPreset: tc.startingPreset}
			overrides := &model.SyncOverrides{ClaudeModelPreset: tc.overridePreset}

			applyOverrides(&selection, overrides)

			if selection.ClaudeModelPreset != tc.wantPreset {
				t.Errorf("ClaudeModelPreset = %q, want %q", selection.ClaudeModelPreset, tc.wantPreset)
			}
		})
	}
}

// ─── persistClaudePreset state preservation ─────────────────────────────────

// TestPersistClaudePreset_PreservesExistingFields verifies that calling
// persistClaudePreset on a state.json with populated agents/components/skills
// only mutates the claude_preset field — all other install metadata is kept.
func TestPersistClaudePreset_PreservesExistingFields(t *testing.T) {
	home := t.TempDir()

	// Seed a populated state.json that passes Validate.
	if err := state.Write(
		home,
		[]string{"claude-code", "opencode"},
		[]string{"engram", "sdd"},
		[]string{"go-testing"},
		"full",
		"balanced",
	); err != nil {
		t.Fatalf("seed state.Write: %v", err)
	}

	persistClaudePreset(home, "performance")

	got, err := state.Read(home)
	if err != nil {
		t.Fatalf("state.Read after persist: %v", err)
	}

	// claude_preset must be the new value.
	if got.InstalledClaudePreset != "performance" {
		t.Errorf("InstalledClaudePreset = %q, want %q", got.InstalledClaudePreset, "performance")
	}
	// All other fields must be untouched.
	if !equalStrings(got.InstalledAgents, []string{"claude-code", "opencode"}) {
		t.Errorf("InstalledAgents = %v, want preserved [claude-code opencode]", got.InstalledAgents)
	}
	if !equalStrings(got.InstalledComponents, []string{"engram", "sdd"}) {
		t.Errorf("InstalledComponents = %v, want preserved [engram sdd]", got.InstalledComponents)
	}
	if !equalStrings(got.InstalledSkills, []string{"go-testing"}) {
		t.Errorf("InstalledSkills = %v, want preserved [go-testing]", got.InstalledSkills)
	}
	if got.InstalledPreset != "full" {
		t.Errorf("InstalledPreset = %q, want preserved %q", got.InstalledPreset, "full")
	}
}

// TestPersistClaudePreset_DoesNotWipeOnReadError verifies the critical
// invariant: if state.Read returns an error (here triggered by an invalid
// installed_preset that fails Validate), persistClaudePreset must NOT
// overwrite the existing state.json. Otherwise the user's installed agent
// metadata would be silently wiped.
func TestPersistClaudePreset_DoesNotWipeOnReadError(t *testing.T) {
	home := t.TempDir()

	stateDir := filepath.Join(home, ".informa-wizard")
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	statePath := filepath.Join(stateDir, "state.json")

	// Write a state.json that decodes but fails Validate (unknown preset).
	// The fields must be otherwise plausible so we can detect overwrite.
	originalJSON := `{
  "installed_agents": ["claude-code"],
  "installed_components": ["engram"],
  "installed_skills": ["go-testing"],
  "installed_preset": "bogus-preset-that-will-fail-validate",
  "installed_claude_preset": "balanced"
}
`
	if err := os.WriteFile(statePath, []byte(originalJSON), 0o644); err != nil {
		t.Fatalf("write seed: %v", err)
	}

	// Confirm Read fails (invariant for the test setup).
	if _, err := state.Read(home); err == nil {
		t.Fatalf("expected state.Read to fail validation, got nil")
	}

	persistClaudePreset(home, "performance")

	// Raw bytes must be UNCHANGED — function must refuse to overwrite.
	got, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("read state.json: %v", err)
	}
	if string(got) != originalJSON {
		t.Fatalf("state.json was modified despite read error.\noriginal:\n%s\nafter:\n%s", originalJSON, string(got))
	}

	// Sanity: parse and check no field was wiped.
	var parsed map[string]any
	if err := json.Unmarshal(got, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if agents, ok := parsed["installed_agents"].([]any); !ok || len(agents) != 1 {
		t.Errorf("installed_agents wiped or missing; got: %v", parsed["installed_agents"])
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// TestVersionBeforeSystemGuards verifies that `informa-wizard version` returns the
// version string without going through system detection or platform guards.
func TestVersionBeforeSystemGuards(t *testing.T) {
	var buf bytes.Buffer
	err := RunArgs([]string{"version"}, &buf)
	if err != nil {
		t.Fatalf("version should not fail: %v", err)
	}
	if !strings.Contains(buf.String(), "informa-wizard") {
		t.Error("version output should contain 'informa-wizard'")
	}
}

// TestHelpCommand verifies that help, --help, and -h all print USAGE and COMMANDS
// without triggering system detection or platform guards.
func TestHelpCommand(t *testing.T) {
	for _, arg := range []string{"help", "--help", "-h"} {
		t.Run(arg, func(t *testing.T) {
			var buf bytes.Buffer
			err := RunArgs([]string{arg}, &buf)
			if err != nil {
				t.Fatalf("help should not fail: %v", err)
			}
			if !strings.Contains(buf.String(), "USAGE") {
				t.Errorf("help output for %q should contain USAGE", arg)
			}
			if !strings.Contains(buf.String(), "COMMANDS") {
				t.Errorf("help output for %q should contain COMMANDS", arg)
			}
		})
	}
}

// TestUnknownCommandSuggestsHelp verifies that an unrecognised command returns
// an error whose message suggests running 'informa-wizard help'.
func TestUnknownCommandSuggestsHelp(t *testing.T) {
	var buf bytes.Buffer
	err := RunArgs([]string{"notacommand"}, &buf)
	if err == nil {
		t.Fatal("unknown command should return error")
	}
	if !strings.Contains(err.Error(), "informa-wizard help") {
		t.Error("unknown command error should suggest 'informa-wizard help'")
	}
}
