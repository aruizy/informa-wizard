package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"gitlab.informa.tools/ai/wizard/informa-wizard/internal/backup"
	"gitlab.informa.tools/ai/wizard/informa-wizard/internal/cli"
	"gitlab.informa.tools/ai/wizard/informa-wizard/internal/lock"
	"gitlab.informa.tools/ai/wizard/informa-wizard/internal/logger"
	"gitlab.informa.tools/ai/wizard/informa-wizard/internal/model"
	"gitlab.informa.tools/ai/wizard/informa-wizard/internal/pipeline"
	"gitlab.informa.tools/ai/wizard/informa-wizard/internal/planner"
	"gitlab.informa.tools/ai/wizard/informa-wizard/internal/state"
	"gitlab.informa.tools/ai/wizard/informa-wizard/internal/system"
	"gitlab.informa.tools/ai/wizard/informa-wizard/internal/tui"
	"gitlab.informa.tools/ai/wizard/informa-wizard/internal/update"
	"gitlab.informa.tools/ai/wizard/informa-wizard/internal/update/upgrade"
	"gitlab.informa.tools/ai/wizard/informa-wizard/internal/verify"
)

// Version is set from main via ldflags at build time.
var Version = "dev"

var (
	updateCheckAll      = update.CheckAll
	updateCheckFiltered = update.CheckFiltered
	upgradeExecute      = upgrade.Execute

	// lockAcquireFn is the lock acquisition function.
	// Tests can replace this with a no-op to avoid using the real user home directory.
	lockAcquireFn = lock.Acquire
)

func Run() error {
	return RunArgs(os.Args[1:], os.Stdout)
}

func RunArgs(args []string, stdout io.Writer) error {
	// Propagate the build-time version to the CLI and upgrade layers so backup
	// manifests record which version of informa-wizard created them.
	cli.AppVersion = Version
	upgrade.AppVersion = Version

	// Info commands: no system detection, no self-update, no platform validation.
	if len(args) > 0 {
		switch args[0] {
		case "version", "--version", "-v":
			_, _ = fmt.Fprintf(stdout, "informa-wizard %s\n", Version)
			return nil
		case "help", "--help", "-h":
			printHelp(stdout, Version)
			return nil
		}
	}

	if err := system.EnsureCurrentOSSupported(); err != nil {
		return err
	}

	// Determine the home directory once; used for lock, logger, and TUI.
	// On failure we continue without locking/logging (non-fatal for these subsystems).
	homeDir, homeDirErr := os.UserHomeDir()

	// Persistent logging: open ~/.informa-wizard/logs/wizard.log in append mode.
	// Errors are silently ignored so logging never blocks the user.
	if homeDirErr == nil {
		_ = logger.Init(homeDir)
		defer func() { _ = logger.Close() }()
	}

	// Lock: prevent two simultaneous wizard instances from stepping on each other.
	// Read-only commands (status, doctor, help, version) bypass the lock.
	// If home dir is unavailable, we skip the lock entirely (graceful degradation).
	isReadOnly := len(args) > 0 && (args[0] == "status" || args[0] == "doctor")
	if homeDirErr == nil && !isReadOnly {
		lk, lockErr := lockAcquireFn(homeDir)
		if lockErr != nil {
			// main() prints returned errors to stderr — let it surface this one
			// rather than printing here AND returning a wrapped error.
			return lockErr
		}
		defer func() { _ = lk.Release() }()
	}

	result, err := system.Detect(context.Background())
	if err != nil {
		return fmt.Errorf("detect system: %w", err)
	}

	if !result.System.Supported {
		return system.EnsureSupportedPlatform(result.System.Profile)
	}

	// Self-update: check for a newer informa-wizard release and apply it before
	// CLI/TUI dispatch. Errors are non-fatal — logged and swallowed.
	profile := cli.ResolveInstallProfile(result)
	if err := selfUpdate(context.Background(), Version, profile, stdout); err != nil {
		_, _ = fmt.Fprintf(stdout, "Warning: self-update failed: %v\n", err)
	}

	if len(args) == 0 {
		if homeDirErr != nil {
			return fmt.Errorf("resolve user home directory: %w", homeDirErr)
		}

		m := tui.NewModel(result, Version)
		m.ExecuteFn = tuiExecute
		m.RestoreFn = tuiRestore
		m.DeleteBackupFn = func(manifest backup.Manifest) error {
			return backup.DeleteBackup(manifest)
		}
		m.RenameBackupFn = func(manifest backup.Manifest, newDesc string) error {
			return backup.RenameBackup(manifest, newDesc)
		}
		m.TogglePinFn = func(manifest backup.Manifest) error {
			return backup.TogglePin(manifest)
		}
		m.ListBackupsFn = ListBackups
		m.Backups = ListBackups()
		m.UpgradeFn = tuiUpgrade(profile, homeDir)
		m.SyncFn = tuiSync(homeDir)
		p := tea.NewProgram(m, tea.WithAltScreen())
		_, err := p.Run()
		return err
	}

	switch args[0] {
	case "update":
		profile := cli.ResolveInstallProfile(result)
		return runUpdate(context.Background(), Version, profile, stdout)
	case "upgrade":
		return runUpgrade(context.Background(), args[1:], result, stdout)
	case "install":
		logger.Info("install start: args=%v", args[1:])
		installResult, err := cli.RunInstall(args[1:], result)
		if err != nil {
			logger.Error("install failed: %v", err)
			return err
		}

		if installResult.DryRun {
			_, _ = fmt.Fprintln(stdout, cli.RenderDryRun(installResult))
		} else {
			logger.Info("install complete: agents=%v components=%v", installResult.Selection.Agents, installResult.Selection.Components)
			_, _ = fmt.Fprint(stdout, verify.RenderReport(installResult.Verify))
		}

		return nil
	case "sync":
		logger.Info("sync start: args=%v", args[1:])
		syncResult, err := cli.RunSync(args[1:])
		if err != nil {
			logger.Error("sync failed: %v", err)
			return err
		}

		logger.Info("sync complete: filesChanged=%d", syncResult.FilesChanged)
		_, _ = fmt.Fprintln(stdout, cli.RenderSyncReport(syncResult))
		return nil
	case "status":
		if homeDirErr != nil {
			return fmt.Errorf("resolve user home directory: %w", homeDirErr)
		}
		return cli.RunStatus(homeDir)
	case "doctor":
		if homeDirErr != nil {
			return fmt.Errorf("resolve user home directory: %w", homeDirErr)
		}
		cli.RunHealthCLI(homeDir)
		return nil
	case "restore":
		return cli.RunRestore(args[1:], stdout)
	default:
		return fmt.Errorf("unknown command %q — run 'informa-wizard help' for available commands", args[0])
	}
}

func runUpdate(ctx context.Context, currentVersion string, profile system.PlatformProfile, stdout io.Writer) error {
	results := updateCheckAll(ctx, currentVersion, profile)
	_, _ = fmt.Fprint(stdout, update.RenderCLI(results))
	return updateCheckError(results)
}

// runUpgrade handles the `informa-wizard upgrade [--dry-run] [tool...]` command.
//
// This command:
//   - Checks for available updates for managed tools (informa-wizard, engram, gga)
//   - Snapshots agent config paths before execution (config preservation by design)
//   - Executes binary-only upgrades; does NOT invoke install or sync pipelines
//   - Skips informa-wizard itself when running as a dev build (version="dev")
//   - Falls back to manual guidance for unsafe platforms (Windows binary self-replace)
func runUpgrade(ctx context.Context, args []string, detection system.DetectionResult, stdout io.Writer) error {
	dryRun := false
	var toolFilter []string

	for _, arg := range args {
		switch {
		case arg == "--dry-run" || arg == "-n":
			dryRun = true
		case !strings.HasPrefix(arg, "-"):
			toolFilter = append(toolFilter, arg)
		}
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("resolve home directory: %w", err)
	}

	profile := cli.ResolveInstallProfile(detection)

	// Check for available updates (filtered to requested tools if specified).
	sp := upgrade.NewSpinner(stdout, "Checking for updates")
	checkResults := updateCheckFiltered(ctx, Version, profile, toolFilter)
	checkErr := updateCheckError(checkResults)
	sp.Finish(checkErr == nil)
	if checkErr != nil {
		_, _ = fmt.Fprint(stdout, update.RenderCLI(checkResults))
		return checkErr
	}

	// Execute upgrades (no-op if nothing is UpdateAvailable).
	report := upgradeExecute(ctx, checkResults, profile, homeDir, dryRun, stdout)

	_, _ = fmt.Fprint(stdout, upgrade.RenderUpgradeReport(report))

	// Return error only if any tool failed (not for skipped/manual).
	var errs []error
	for _, r := range report.Results {
		if r.Status == upgrade.UpgradeFailed && r.Err != nil {
			errs = append(errs, fmt.Errorf("upgrade failed for %q: %w", r.ToolName, r.Err))
		}
	}

	return errors.Join(errs...)
}

func updateCheckError(results []update.UpdateResult) error {
	failed := update.CheckFailures(results)
	if len(failed) == 0 {
		return nil
	}

	return fmt.Errorf("update check failed for: %s", strings.Join(failed, ", "))
}

// tuiExecute creates a real install runtime and runs the pipeline with progress reporting.
func tuiExecute(
	selection model.Selection,
	resolved planner.ResolvedPlan,
	detection system.DetectionResult,
	onProgress pipeline.ProgressFunc,
) pipeline.ExecutionResult {
	restoreCommandOutput := cli.SetCommandOutputStreaming(false)
	defer restoreCommandOutput()

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return pipeline.ExecutionResult{Err: fmt.Errorf("resolve user home directory: %w", err)}
	}

	profile := cli.ResolveInstallProfile(detection)
	resolved.PlatformDecision = planner.PlatformDecisionFromProfile(profile)

	stagePlan, err := cli.BuildRealStagePlan(homeDir, selection, resolved, profile)
	if err != nil {
		return pipeline.ExecutionResult{Err: fmt.Errorf("build stage plan: %w", err)}
	}

	orchestrator := pipeline.NewOrchestrator(
		pipeline.DefaultRollbackPolicy(),
		pipeline.WithFailurePolicy(pipeline.ContinueOnError),
		pipeline.WithProgressFunc(onProgress),
	)

	execResult := orchestrator.Execute(stagePlan)
	if execResult.Err == nil {
		// Persist the user's agent and component selection so that future `sync`
		// runs target only what the user actually installed.
		agentIDs := make([]string, 0, len(selection.Agents))
		for _, a := range selection.Agents {
			agentIDs = append(agentIDs, string(a))
		}
		componentIDs := make([]string, 0, len(selection.Components))
		for _, c := range selection.Components {
			componentIDs = append(componentIDs, string(c))
		}
		skillIDs := make([]string, 0, len(selection.Skills))
		for _, s := range selection.Skills {
			skillIDs = append(skillIDs, string(s))
		}
		// Non-fatal: a state write failure must not break an otherwise successful install.
		_ = state.Write(homeDir, agentIDs, componentIDs, skillIDs, string(selection.Preset), string(selection.ClaudeModelPreset))

		// Persist source repo dir for Update+Sync.
		if wd, wdErr := os.Getwd(); wdErr == nil {
			if _, gitErr := os.Stat(filepath.Join(wd, ".git")); gitErr == nil {
				sourceDirFile := filepath.Join(homeDir, ".informa-wizard", "source-dir")
				_ = os.MkdirAll(filepath.Dir(sourceDirFile), 0o755)
				_ = os.WriteFile(sourceDirFile, []byte(wd+"\n"), 0o644)
			}
		}
	}

	return execResult
}

// tuiRestore restores a backup from its manifest.
func tuiRestore(manifest backup.Manifest) error {
	return backup.RestoreService{}.Restore(manifest)
}

// tuiUpgrade returns a tui.UpgradeFunc that wraps upgrade.Execute.
// The profile and homeDir are captured from the call site so the closure
// is self-contained and requires no extra parameters at call time.
func tuiUpgrade(profile system.PlatformProfile, homeDir string) tui.UpgradeFunc {
	return func(ctx context.Context, results []update.UpdateResult) upgrade.UpgradeReport {
		return upgradeExecute(ctx, results, profile, homeDir, false)
	}
}

// tuiSync returns a tui.SyncFunc that performs a full managed-asset sync.
// It mirrors the RunSync CLI path: discovers installed agents from persisted
// state (or filesystem fallback), builds the default sync selection, and
// delegates to RunSyncWithSelection.
//
// When overrides is non-nil, model assignments are merged into the selection
// so that the "Configure Models" TUI flow persists its choices to disk.
func tuiSync(homeDir string) tui.SyncFunc {
	return func(overrides *model.SyncOverrides) (int, error) {
		presetForLog := ""
		if overrides != nil {
			presetForLog = overrides.ClaudeModelPreset
		}
		logger.Info("tui sync start: claude_preset_override=%q", presetForLog)

		agentIDs := cli.DiscoverAgents(homeDir)
		selection := cli.BuildSyncSelection(cli.SyncFlags{}, agentIDs, homeDir)

		applyOverrides(&selection, overrides)

		result, err := cli.RunSyncWithSelection(homeDir, selection)
		// Persist Claude preset to state.json only when the pipeline actually
		// wrote files reflecting the new preset. RunSyncWithSelection enforces
		// `FilesChanged == 0 ⇒ NoOp == true` on success (sync.go:812), so this
		// single condition correctly excludes both NoOp paths and partial-failure
		// paths where rollback left no files written.
		shouldPersist := overrides != nil && overrides.ClaudeModelPreset != "" && result.FilesChanged > 0
		if shouldPersist {
			logger.Info("tui sync persisting claude preset: %s (sync err=%v)", overrides.ClaudeModelPreset, err)
			persistClaudePreset(homeDir, overrides.ClaudeModelPreset)
		}
		if err != nil {
			logger.Error("tui sync RunSyncWithSelection failed: %v", err)
			return result.FilesChanged, err
		}
		return result.FilesChanged, nil
	}
}

// persistClaudePreset rewrites state.json with the new Claude preset, leaving
// all other fields untouched. Read errors are handled defensively:
//   - state.json missing → write a fresh state with just the preset (install repopulates).
//   - state.json exists but failed to read/validate → DO NOT overwrite; preserve existing data on disk.
func persistClaudePreset(homeDir, preset string) {
	s, err := state.Read(homeDir)
	if err != nil {
		// If state.json exists but failed to read/validate, refuse to overwrite —
		// otherwise we'd wipe the user's installed agents/components/skills metadata.
		if !errors.Is(err, fs.ErrNotExist) && !os.IsNotExist(err) {
			logger.Warn("persistClaudePreset: state.Read failed (%v); refusing to overwrite existing state.json", err)
			return
		}
		logger.Info("persistClaudePreset: state.json not found; writing fresh state with preset only")
		s = state.InstallState{}
	}
	logger.Info("persistClaudePreset: writing state.json with claude_preset=%s (was %s)", preset, s.InstalledClaudePreset)
	if writeErr := state.Write(
		homeDir,
		s.InstalledAgents,
		s.InstalledComponents,
		s.InstalledSkills,
		s.InstalledPreset,
		preset,
	); writeErr != nil {
		logger.Warn("failed to persist Claude preset to state.json: %v", writeErr)
		return
	}
	logger.Info("persistClaudePreset: wrote state.json successfully")
}

// applyOverrides merges non-nil fields from overrides into selection.
// A nil overrides pointer is a no-op.
func applyOverrides(selection *model.Selection, overrides *model.SyncOverrides) {
	if overrides == nil {
		return
	}
	if overrides.ModelAssignments != nil {
		selection.ModelAssignments = overrides.ModelAssignments
	}
	if overrides.ClaudeModelAssignments != nil {
		selection.ClaudeModelAssignments = overrides.ClaudeModelAssignments
	}
	if overrides.ClaudeModelPreset != "" {
		selection.ClaudeModelPreset = overrides.ClaudeModelPreset
	}
	if overrides.SDDMode != "" {
		selection.SDDMode = overrides.SDDMode
	}
	if overrides.StrictTDD != nil {
		selection.StrictTDD = *overrides.StrictTDD
	}
	if len(overrides.Profiles) > 0 {
		selection.Profiles = overrides.Profiles
		// Profiles are an OpenCode multi-mode feature — if profiles are being
		// created/synced, SDDModeMulti is required so that WriteSharedPromptFiles
		// runs and the {file:...} prompt references resolve correctly.
		if selection.SDDMode == "" {
			selection.SDDMode = model.SDDModeMulti
		}
	}
}

// ListBackups returns all backup manifests from the backup directory.
func ListBackups() []backup.Manifest {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil
	}

	backupRoot := filepath.Join(homeDir, ".informa-wizard", "backups")
	entries, err := os.ReadDir(backupRoot)
	if err != nil {
		return nil
	}

	manifests := make([]backup.Manifest, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		manifestPath := filepath.Join(backupRoot, entry.Name(), backup.ManifestFilename)
		manifest, err := backup.ReadManifest(manifestPath)
		if err != nil {
			continue
		}
		manifests = append(manifests, manifest)
	}

	// Sort by creation time (newest first) — the IDs are timestamps.
	for i := 0; i < len(manifests); i++ {
		for j := i + 1; j < len(manifests); j++ {
			if manifests[j].CreatedAt.After(manifests[i].CreatedAt) {
				manifests[i], manifests[j] = manifests[j], manifests[i]
			}
		}
	}

	return manifests
}
