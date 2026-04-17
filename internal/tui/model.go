package tui

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"gitlab.informa.tools/ai/wizard/informa-wizard/internal/agentbuilder"
	"gitlab.informa.tools/ai/wizard/informa-wizard/internal/backup"
	"gitlab.informa.tools/ai/wizard/informa-wizard/internal/catalog"
	"gitlab.informa.tools/ai/wizard/informa-wizard/internal/components/devagents"
	"gitlab.informa.tools/ai/wizard/informa-wizard/internal/components/devskills"
	"gitlab.informa.tools/ai/wizard/informa-wizard/internal/components/sdd"
	"gitlab.informa.tools/ai/wizard/informa-wizard/internal/model"
	"gitlab.informa.tools/ai/wizard/informa-wizard/internal/opencode"
	"gitlab.informa.tools/ai/wizard/informa-wizard/internal/pipeline"
	"gitlab.informa.tools/ai/wizard/informa-wizard/internal/planner"
	"gitlab.informa.tools/ai/wizard/informa-wizard/internal/system"
	"gitlab.informa.tools/ai/wizard/informa-wizard/internal/tui/screens"
	"gitlab.informa.tools/ai/wizard/informa-wizard/internal/update"
	"gitlab.informa.tools/ai/wizard/informa-wizard/internal/update/upgrade"
)

// osStatModelCache is a package-level variable so tests can override it to
// simulate a missing or present OpenCode model cache file.
var osStatModelCache = os.Stat

// readCurrentAssignmentsFn is a package-level variable so tests can override
// how current model assignments are read from opencode.json. It wraps
// sdd.ReadCurrentModelAssignments and is only called during ModelConfigMode.
var readCurrentAssignmentsFn = func(settingsPath string) (map[string]model.ModelAssignment, error) {
	return sdd.ReadCurrentModelAssignments(settingsPath)
}

// readProfilesFn is a package-level variable so tests can override how profiles
// are detected from opencode.json. It wraps sdd.DetectProfiles and is called
// on ScreenProfiles entry and after SyncDoneMsg to refresh the profile list.
var readProfilesFn = func(settingsPath string) ([]model.Profile, error) {
	return sdd.DetectProfiles(settingsPath)
}

// TickMsg drives the spinner animation on the installing screen.
type TickMsg time.Time

func tickCmd() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
		return TickMsg(t)
	})
}

// StepProgressMsg is sent from the pipeline goroutine when a step changes status.
type StepProgressMsg struct {
	StepID string
	Status pipeline.StepStatus
	Err    error
}

// PipelineDoneMsg is sent when the pipeline finishes execution.
type PipelineDoneMsg struct {
	Result pipeline.ExecutionResult
}

// BackupRestoreMsg is sent when a backup restore completes.
type BackupRestoreMsg struct {
	Err error
}

// UpdateCheckResultMsg is sent when the background update check completes.
type UpdateCheckResultMsg struct {
	Results []update.UpdateResult
}

// UpgradeDoneMsg is sent when the upgrade operation completes.
type UpgradeDoneMsg struct {
	Report upgrade.UpgradeReport
	Err    error
}

// SyncDoneMsg is sent when the sync operation completes.
type SyncDoneMsg struct {
	FilesChanged int
	Err          error
}

// UpgradePhaseCompletedMsg is sent by startUpgradeSync when the upgrade phase
// finishes (before the sync phase begins). This enables the intermediate "sync
// running" state to be displayed.
type UpgradePhaseCompletedMsg struct {
	Report upgrade.UpgradeReport
	Err    error
}

// AgentBuilderGeneratedMsg is sent when the AI generation goroutine completes.
type AgentBuilderGeneratedMsg struct {
	Agent *agentbuilder.GeneratedAgent
	Err   error
}

// AgentBuilderInstallDoneMsg is sent when the agent installation goroutine completes.
type AgentBuilderInstallDoneMsg struct {
	Results []agentbuilder.InstallResult
	Err     error
}

// AgentBuilderState holds all transient state for the agent-builder TUI flow.
type AgentBuilderState struct {
	AvailableEngines []model.AgentID
	SelectedEngine   model.AgentID
	Textarea         textarea.Model
	SDDMode          agentbuilder.SDDIntegrationMode
	SDDTargetPhase   string
	Generating       bool
	GenerationCancel context.CancelFunc
	Generated        *agentbuilder.GeneratedAgent
	GenerationErr    error
	ConflictWarning  string
	Installing       bool
	InstallResults   []agentbuilder.InstallResult
	InstallErr       error
	PreviewScroll    int
}

// UpgradeFunc is the signature of the function injected to perform tool upgrades.
type UpgradeFunc func(ctx context.Context, results []update.UpdateResult) upgrade.UpgradeReport

// SyncFunc is the signature of the function injected to perform config sync.
// When overrides is non-nil, the sync merges those model assignments into the
// selection before executing. Returns the number of files changed and any error.
type SyncFunc func(overrides *model.SyncOverrides) (int, error)

// ExecuteFunc builds and runs the installation pipeline. It receives a ProgressFunc
// callback to emit step-level progress events, and returns the ExecutionResult.
type ExecuteFunc func(
	selection model.Selection,
	resolved planner.ResolvedPlan,
	detection system.DetectionResult,
	onProgress pipeline.ProgressFunc,
) pipeline.ExecutionResult

// RestoreFunc restores a backup from a manifest.
type RestoreFunc func(manifest backup.Manifest) error

// DeleteBackupFunc deletes the entire backup directory.
type DeleteBackupFunc func(manifest backup.Manifest) error

// RenameBackupFunc updates the backup's Description field in its manifest file.
type RenameBackupFunc func(manifest backup.Manifest, newDescription string) error

// ListBackupsFn returns the current list of available backups.
// When nil, the backup list is not refreshed after restore.
type ListBackupsFn func() []backup.Manifest

type Screen int

const (
	ScreenUnknown Screen = iota
	ScreenWelcome
	ScreenDetection
	ScreenAgents
	ScreenPersona
	ScreenPreset
	ScreenClaudeModelPicker
	ScreenSDDMode
	ScreenStrictTDD
	ScreenDependencyTree
	ScreenSkillPicker
	ScreenDevSkillPicker
	ScreenDevAgentPicker
	ScreenReview
	ScreenInstalling
	ScreenModelPicker
	ScreenComplete
	ScreenBackups
	ScreenRestoreConfirm
	ScreenRestoreResult
	ScreenDeleteConfirm
	ScreenDeleteResult
	ScreenRenameBackup
	ScreenUpgrade
	ScreenSync
	ScreenUpgradeSync
	ScreenModelConfig
	ScreenProfiles
	ScreenProfileCreate
	ScreenProfileDelete
	ScreenAgentBuilderEngine
	ScreenAgentBuilderPrompt
	ScreenAgentBuilderSDD
	ScreenAgentBuilderSDDPhase
	ScreenAgentBuilderGenerating
	ScreenAgentBuilderPreview
	ScreenAgentBuilderInstalling
	ScreenAgentBuilderComplete
	ScreenMonday
)

type Model struct {
	Screen         Screen
	PreviousScreen Screen
	Width          int
	Height         int
	Cursor         int
	Version        string
	SpinnerFrame   int

	Selection         model.Selection
	Detection         system.DetectionResult
	DependencyPlan    planner.ResolvedPlan
	Review            planner.ReviewPayload
	Progress          ProgressState
	Execution         pipeline.ExecutionResult
	Backups           []backup.Manifest
	ModelPicker       screens.ModelPickerState
	ClaudeModelPicker screens.ClaudeModelPickerState
	SkillPicker       []model.SkillID
	DevSkills         []devskills.DiscoveredSkill
	DevSkillChecked   []bool
	DevSkillCursor    int
	DevAgents         []devagents.DiscoveredAgent
	DevAgentChecked   []bool
	DevAgentCursor    int
	Err               error

	// SelectedBackup holds the manifest chosen on ScreenBackups, used by the
	// restore confirmation and result screens.
	SelectedBackup backup.Manifest

	// RestoreErr holds the error from the most recent restore attempt.
	// Nil on success, non-nil on failure. Displayed on ScreenRestoreResult.
	RestoreErr error

	// DeleteErr holds the error from the most recent delete attempt.
	// Nil on success, non-nil on failure. Displayed on ScreenDeleteResult.
	DeleteErr error

	// PinErr holds the error from the most recent pin/unpin attempt.
	// Nil on success, non-nil on failure. Shown inline on ScreenBackups.
	PinErr error

	// BackupScroll is the scroll offset for the backup list.
	BackupScroll int

	// BackupRenameText is the text input buffer for rename operations.
	BackupRenameText string

	// BackupRenamePos is the cursor position within BackupRenameText.
	BackupRenamePos int

	// Monday.com configuration input state.
	MondayTokenInput   string
	MondayTokenPos     int
	MondayBoardInput   string
	MondayBoardPos     int
	MondayActiveField  screens.MondayField

	// ExecuteFn is called to run the real pipeline. When nil, the installing
	// screen falls back to manual step-through (useful for tests/development).
	ExecuteFn ExecuteFunc

	// RestoreFn is called to restore a backup. When nil, restore is a no-op.
	RestoreFn RestoreFunc

	// DeleteBackupFn is called to delete a backup directory.
	DeleteBackupFn DeleteBackupFunc

	// RenameBackupFn is called to rename (update description of) a backup.
	RenameBackupFn RenameBackupFunc

	// TogglePinFn toggles the Pinned field of a backup manifest.
	// When nil, pin/unpin is a no-op.
	TogglePinFn func(manifest backup.Manifest) error

	// ListBackupsFn refreshes the backup list (e.g. after a restore).
	// When nil, the backup list is not refreshed automatically.
	ListBackupsFn ListBackupsFn

	// UpdateResults holds the results of the background update check.
	UpdateResults []update.UpdateResult

	// UpdateCheckDone is true once the background update check has completed.
	UpdateCheckDone bool

	// pipelineRunning tracks whether the pipeline goroutine is active.
	pipelineRunning bool

	// TUI operations — set by startUpgrade / startSync / startUpgradeSync goroutines.

	// UpgradeReport holds the result of the last upgrade run.
	// nil means the upgrade has not been run yet or is currently running.
	UpgradeReport *upgrade.UpgradeReport

	// SyncFilesChanged holds the number of files changed during the last sync run.
	SyncFilesChanged int

	// SyncErr holds the error from the last sync run (nil on success).
	SyncErr error

	// UpgradeFn is injected at construction time and called to perform upgrades.
	UpgradeFn UpgradeFunc

	// SyncFn is injected at construction time and called to perform config sync.
	SyncFn SyncFunc

	// ModelConfigMode is true when the model pickers were reached via the
	// Model Config shortcut, so they return to ScreenWelcome instead of
	// continuing the install flow.
	ModelConfigMode bool

	// PendingSyncOverrides holds model assignments selected via the
	// "Configure Models" shortcut. When non-nil, the next sync run merges
	// these into the sync selection so the choices are persisted to disk.
	// Cleared after the sync completes (SyncDoneMsg handler).
	PendingSyncOverrides *model.SyncOverrides

	// OperationRunning is true while an upgrade/sync/upgrade-sync goroutine is
	// executing. Prevents concurrent operation launches.
	OperationRunning bool

	// OperationMode records which operation is running or was last run.
	// Values: "upgrade", "sync", "upgrade-sync".
	OperationMode string

	// HasSyncRun is true once a sync or upgrade-sync operation has completed.
	// It distinguishes "sync hasn't run yet" (false) from "sync ran with 0 changes" (true, filesChanged=0).
	HasSyncRun bool

	// UpgradeErr holds the error from the last upgrade run (nil on success).
	UpgradeErr error

	// Profile management state
	ProfileList          []model.Profile // profiles detected from opencode.json
	ProfileCreateStep    int             // 0=name, 1=assign-models, 2=confirm
	ProfileDraft         model.Profile   // profile being created/edited
	ProfileEditMode      bool            // true when editing, false when creating
	ProfileDeleteTarget  string          // name of profile to delete
	ProfileNameInput     string          // text input buffer for name step
	ProfileNamePos       int             // cursor position in name input
	ProfileNameErr       string          // validation error message
	ProfileNameCollision bool            // true when name collides with existing profile (awaiting second enter to overwrite)
	ProfileDeleteErr     error           // error from the last RemoveProfileAgents call, displayed on ScreenProfiles

	// AgentBuilder holds the transient state for the agent-builder TUI flow.
	AgentBuilder AgentBuilderState

	// CommitDate is the date of the last git commit embedded in the binary.
	// Nil for dev builds or when build info is unavailable.
	CommitDate *time.Time
}

func NewModel(detection system.DetectionResult, version string) Model {
	selection := model.Selection{
		Agents:     preselectedAgents(detection),
		Persona:    model.PersonaCustom,
		Preset:     model.PresetFull,
		Components: componentsForPreset(model.PresetFull),
		SDDMode:    model.SDDModeMulti,
	}

	return Model{
		Screen:        ScreenWelcome,
		Version:       version,
		Selection:     selection,
		Detection:     detection,
		CommitDate: resolveCommitDate(),
		Progress: NewProgressState([]string{
			"Install dependencies",
			"Configure selected agents",
			"Inject ecosystem components",
		}),
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height
		return m, nil
	case TickMsg:
		if m.Screen == ScreenInstalling && !m.Progress.Done() {
			m.SpinnerFrame = (m.SpinnerFrame + 1) % 10
			return m, tickCmd()
		}
		// Keep spinner running for operation screens.
		if m.OperationRunning || (m.Screen == ScreenUpgrade && !m.UpdateCheckDone) {
			m.SpinnerFrame = (m.SpinnerFrame + 1) % 10
			return m, tickCmd()
		}
		// Keep spinner running for agent builder generating/installing screens.
		if m.AgentBuilder.Generating || m.AgentBuilder.Installing {
			m.SpinnerFrame = (m.SpinnerFrame + 1) % 10
			return m, tickCmd()
		}
		return m, nil
	case AgentBuilderGeneratedMsg:
		// If generation was cancelled (Esc while generating), ignore the result.
		if !m.AgentBuilder.Generating {
			return m, nil
		}
		m.AgentBuilder.Generating = false
		if msg.Err != nil {
			m.AgentBuilder.GenerationErr = msg.Err
			// Stay on generating screen to show error.
		} else {
			m.AgentBuilder.Generated = msg.Agent
			m.AgentBuilder.GenerationErr = nil
			// Check for builtin conflict and set warning before showing preview.
			if msg.Agent != nil && agentbuilder.HasConflictWithBuiltin(msg.Agent.Name) {
				m.AgentBuilder.ConflictWarning = fmt.Sprintf(
					"Warning: '%s' conflicts with a built-in skill. It will be installed as '%s-custom'.",
					msg.Agent.Name, msg.Agent.Name,
				)
			} else {
				m.AgentBuilder.ConflictWarning = ""
			}
			m.setScreen(ScreenAgentBuilderPreview)
		}
		return m, nil
	case AgentBuilderInstallDoneMsg:
		m.AgentBuilder.Installing = false
		if msg.Err != nil {
			m.AgentBuilder.InstallErr = msg.Err
			m.setScreen(ScreenAgentBuilderPreview)
		} else {
			m.AgentBuilder.InstallResults = msg.Results
			m.AgentBuilder.InstallErr = nil
			m.setScreen(ScreenAgentBuilderComplete)
		}
		return m, nil
	case StepProgressMsg:
		return m.handleStepProgress(msg)
	case PipelineDoneMsg:
		return m.handlePipelineDone(msg)
	case BackupRestoreMsg:
		return m.handleBackupRestore(msg)
	case UpdateCheckResultMsg:
		m.UpdateResults = msg.Results
		m.UpdateCheckDone = true
		return m, nil
	case UpgradeDoneMsg:
		m.OperationRunning = false
		m.UpgradeErr = msg.Err
		if msg.Err == nil {
			report := msg.Report
			m.UpgradeReport = &report
		}
		m.UpdateResults = nil
		m.UpdateCheckDone = false
		return m, nil
	case SyncDoneMsg:
		m.OperationRunning = false
		m.SyncFilesChanged = msg.FilesChanged
		m.SyncErr = msg.Err
		m.HasSyncRun = true
		m.PendingSyncOverrides = nil
		// Refresh profile list after sync (profile create/delete/edit flows use sync).
		// On failure, keep the existing list — this is a non-critical background refresh.
		// Do NOT set m.Err: ScreenSync never renders it and it would leak to other screens.
		if profiles, err := readProfilesFn(opencode.DefaultSettingsPath()); err == nil {
			m.ProfileList = profiles
			// Clamp cursor to avoid out-of-bounds access when list shrinks after a delete.
			if m.Cursor >= len(m.ProfileList) {
				if len(m.ProfileList) > 0 {
					m.Cursor = len(m.ProfileList) - 1
				} else {
					m.Cursor = 0
				}
			}
		} // else keep existing list
		return m, nil
	case UpgradePhaseCompletedMsg:
		// Pull phase done; sync phase is about to start (OperationRunning stays true).
		m.UpgradeErr = msg.Err
		if msg.Err == nil && msg.Report.Results != nil {
			report := msg.Report
			m.UpgradeReport = &report
		}
		m.UpdateResults = nil
		m.UpdateCheckDone = false
		return m, nil
	case tea.KeyMsg:
		if m.Screen == ScreenRenameBackup {
			return m.handleRenameInput(msg)
		}
		if m.Screen == ScreenMonday {
			return m.handleMondayInput(msg)
		}
		if m.Screen == ScreenProfileCreate && m.ProfileCreateStep == 0 && !m.ProfileEditMode {
			return m.handleProfileNameInput(msg)
		}
		// Delegate to textarea when on the agent builder prompt screen,
		// unless the user pressed Esc (to go back) or Tab (to continue).
		if m.Screen == ScreenAgentBuilderPrompt {
			if msg.String() == "esc" {
				return m.handleKeyPress(msg)
			}
			if msg.String() == "tab" || msg.String() == "ctrl+enter" {
				// "Continue" — proceed to SDD selection if textarea is not empty.
				if m.AgentBuilder.Textarea.Value() != "" {
					m.setScreen(ScreenAgentBuilderSDD)
				}
				return m, nil
			}
			// All other keys go to the textarea.
			var taCmd tea.Cmd
			m.AgentBuilder.Textarea, taCmd = m.AgentBuilder.Textarea.Update(msg)
			return m, taCmd
		}
		return m.handleKeyPress(msg)
	}

	return m, nil
}

func (m Model) handleStepProgress(msg StepProgressMsg) (tea.Model, tea.Cmd) {
	if m.Screen != ScreenInstalling {
		return m, nil
	}

	idx := m.findProgressItem(msg.StepID)
	if idx < 0 {
		return m, nil
	}

	switch msg.Status {
	case pipeline.StepStatusRunning:
		m.Progress.Start(idx)
		m.Progress.AppendLog("running: %s", msg.StepID)
	case pipeline.StepStatusSucceeded:
		m.Progress.Mark(idx, string(pipeline.StepStatusSucceeded))
		m.Progress.AppendLog("done: %s", msg.StepID)
	case pipeline.StepStatusFailed:
		m.Progress.Mark(idx, string(pipeline.StepStatusFailed))
		errMsg := "unknown error"
		if msg.Err != nil {
			errMsg = msg.Err.Error()
		}
		m.Progress.AppendLog("FAILED: %s — %s", msg.StepID, errMsg)
	}

	return m, nil
}

func (m Model) handlePipelineDone(msg PipelineDoneMsg) (tea.Model, tea.Cmd) {
	m.Execution = msg.Result
	m.pipelineRunning = false

	// Rebuild progress from real step results so failed steps show ✗ instead
	// of being blindly marked as succeeded.
	m.Progress = ProgressFromExecution(msg.Result)

	// Surface individual error messages so the user knows WHAT failed.
	appendStepErrors := func(steps []pipeline.StepResult) {
		for _, step := range steps {
			if step.Status == pipeline.StepStatusFailed && step.Err != nil {
				m.Progress.AppendLog("FAILED: %s — %s", step.StepID, step.Err.Error())
			}
		}
	}
	appendStepErrors(msg.Result.Prepare.Steps)
	appendStepErrors(msg.Result.Apply.Steps)

	if msg.Result.Err != nil {
		m.Progress.AppendLog("pipeline completed with errors")
	} else {
		m.Progress.AppendLog("pipeline completed successfully")
	}

	return m, nil
}

func (m Model) handleBackupRestore(msg BackupRestoreMsg) (tea.Model, tea.Cmd) {
	m.RestoreErr = msg.Err
	// Navigate to the result screen regardless of success or failure.
	// The result screen shows success or the error message.
	m.setScreen(ScreenRestoreResult)
	return m, nil
}

func (m Model) findProgressItem(stepID string) int {
	for i, item := range m.Progress.Items {
		if item.Label == stepID {
			return i
		}
	}
	return -1
}

func (m Model) View() string {
	switch m.Screen {
	case ScreenWelcome:
		return screens.RenderWelcome(m.Cursor, m.Version, "", m.hasAgentBuilderEngines(), m.CommitDate)
	case ScreenUpgrade:
		return screens.RenderUpgrade(m.UpdateResults, m.UpgradeReport, m.UpgradeErr, m.OperationRunning, m.UpdateCheckDone, m.Cursor, m.SpinnerFrame)
	case ScreenSync:
		return screens.RenderSync(m.SyncFilesChanged, m.SyncErr, m.OperationRunning, m.HasSyncRun, m.SpinnerFrame)
	case ScreenModelConfig:
		return screens.RenderModelConfig(m.Cursor)
	case ScreenProfiles:
		return screens.RenderProfiles(m.ProfileList, m.Cursor, m.ProfileDeleteErr)
	case ScreenProfileCreate:
		return screens.RenderProfileCreate(
			m.ProfileCreateStep,
			m.ProfileDraft,
			m.ProfileNameInput,
			m.ProfileNamePos,
			m.ProfileNameErr,
			m.ProfileEditMode,
			m.Selection.ModelAssignments,
			m.ModelPicker,
			m.Cursor,
		)
	case ScreenProfileDelete:
		return screens.RenderProfileDelete(m.ProfileDeleteTarget, m.Cursor)
	case ScreenUpgradeSync:
		return screens.RenderUpgradeSync(m.UpdateResults, m.UpgradeReport, m.SyncFilesChanged, m.UpgradeErr, m.SyncErr, m.OperationRunning, m.UpdateCheckDone, m.Cursor, m.SpinnerFrame)
	case ScreenDetection:
		return screens.RenderDetection(m.Detection, m.Cursor)
	case ScreenAgents:
		return screens.RenderAgents(m.Selection.Agents, m.Cursor)
	case ScreenPreset:
		return screens.RenderPreset(m.Selection.Preset, m.Cursor)
	case ScreenClaudeModelPicker:
		return screens.RenderClaudeModelPicker(m.ClaudeModelPicker, m.Cursor)
	case ScreenSDDMode:
		return screens.RenderSDDMode(m.Selection.SDDMode, m.Cursor)
	case ScreenStrictTDD:
		return screens.RenderStrictTDD(m.Selection.StrictTDD, m.Cursor)
	case ScreenModelPicker:
		return screens.RenderModelPicker(m.Selection.ModelAssignments, m.ModelPicker, m.Cursor)
	case ScreenDependencyTree:
		return screens.RenderDependencyTree(m.DependencyPlan, m.Selection, m.Cursor)
	case ScreenSkillPicker:
		return screens.RenderSkillPicker(m.SkillPicker, m.Cursor)
	case ScreenDevSkillPicker:
		return screens.RenderDevSkillPicker(m.DevSkills, m.DevSkillChecked, m.Cursor)
	case ScreenDevAgentPicker:
		return screens.RenderDevAgentPicker(m.DevAgents, m.DevAgentChecked, m.Cursor)
	case ScreenMonday:
		return screens.RenderMonday(m.MondayTokenInput, m.MondayBoardInput, m.MondayActiveField, mondayCursorPos(m))
	case ScreenReview:
		return screens.RenderReview(m.Review, m.Cursor)
	case ScreenInstalling:
		return screens.RenderInstalling(m.Progress.ViewModel(), screens.SpinnerChar(m.SpinnerFrame))
	case ScreenComplete:
		return screens.RenderComplete(screens.CompletePayload{
			ConfiguredAgents:    len(m.Selection.Agents),
			InstalledComponents: len(m.Selection.Components),
			FailedSteps:         extractFailedSteps(m.Execution),
			RollbackPerformed:   len(m.Execution.Rollback.Steps) > 0,
			MissingDeps:         extractMissingDeps(m.Detection),
			AvailableUpdates:    extractAvailableUpdates(m.UpdateResults),
		})
	case ScreenBackups:
		return screens.RenderBackups(m.Backups, m.Cursor, m.BackupScroll, m.PinErr)
	case ScreenRestoreConfirm:
		return screens.RenderRestoreConfirm(m.SelectedBackup, m.Cursor)
	case ScreenRestoreResult:
		return screens.RenderRestoreResult(m.SelectedBackup, m.RestoreErr)
	case ScreenDeleteConfirm:
		return screens.RenderDeleteConfirm(m.SelectedBackup, m.Cursor)
	case ScreenDeleteResult:
		return screens.RenderDeleteResult(m.SelectedBackup, m.DeleteErr)
	case ScreenRenameBackup:
		return screens.RenderRenameBackup(m.SelectedBackup, m.BackupRenameText, m.BackupRenamePos)
	case ScreenAgentBuilderEngine:
		return screens.RenderABEngine(m.AgentBuilder.AvailableEngines, m.Cursor)
	case ScreenAgentBuilderPrompt:
		return screens.RenderABPrompt(m.AgentBuilder.Textarea)
	case ScreenAgentBuilderSDD:
		return screens.RenderABSDD(string(m.AgentBuilder.SDDMode), m.Cursor)
	case ScreenAgentBuilderSDDPhase:
		return screens.RenderABSDDPhase(screens.ABSDDPhases(), m.Cursor, m.AgentBuilder.SDDMode == agentbuilder.SDDNewPhase)
	case ScreenAgentBuilderGenerating:
		engineName := string(m.AgentBuilder.SelectedEngine)
		return screens.RenderABGenerating(engineName, m.SpinnerFrame, m.AgentBuilder.GenerationErr)
	case ScreenAgentBuilderPreview:
		targets := m.agentBuilderInstallTargets()
		return screens.RenderABPreview(m.AgentBuilder.Generated, targets, m.AgentBuilder.PreviewScroll, m.Height, m.Cursor, m.AgentBuilder.InstallErr, m.AgentBuilder.ConflictWarning)
	case ScreenAgentBuilderInstalling:
		engineName := string(m.AgentBuilder.SelectedEngine)
		return screens.RenderABInstalling(engineName, m.SpinnerFrame, m.AgentBuilder.InstallErr)
	case ScreenAgentBuilderComplete:
		return screens.RenderABComplete(m.AgentBuilder.Generated, m.AgentBuilder.InstallResults)
	default:
		return ""
	}
}

func (m Model) handleKeyPress(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	keyStr := key.String()

	// When the model picker is in a sub-mode, delegate navigation there first.
	if m.Screen == ScreenModelPicker && m.ModelPicker.Mode != screens.ModePhaseList {
		handled, updated := screens.HandleModelPickerNav(keyStr, &m.ModelPicker, m.Selection.ModelAssignments)
		if handled {
			m.Selection.ModelAssignments = updated
			return m, nil
		}
	}

	// Profile create step 1 reuses the ModelPicker sub-modes (provider/model drill-down).
	if (m.Screen == ScreenProfileCreate && m.ProfileCreateStep == 1) &&
		m.ModelPicker.Mode != screens.ModePhaseList {
		handled, updated := screens.HandleModelPickerNav(keyStr, &m.ModelPicker, m.Selection.ModelAssignments)
		if handled {
			m.Selection.ModelAssignments = updated
			return m, nil
		}
	}

	if m.Screen == ScreenClaudeModelPicker {
		wasInCustomMode := m.ClaudeModelPicker.InCustomMode
		handled, updated := screens.HandleClaudeModelPickerNav(keyStr, &m.ClaudeModelPicker, m.Cursor)
		if handled {
			// Issue #147: reset cursor when exiting custom mode (Esc or Back row).
			if wasInCustomMode && !m.ClaudeModelPicker.InCustomMode {
				m.Cursor = 0
			}
			if updated != nil {
				m.Selection.ClaudeModelAssignments = updated
				// In ModelConfigMode, persist model assignments via sync.
				if m.ModelConfigMode {
					m.ModelConfigMode = false
					m.PendingSyncOverrides = &model.SyncOverrides{
						ClaudeModelAssignments: updated,
					}
					m = m.withResetSyncState()
					m.setScreen(ScreenSync)
				} else if m.shouldShowSDDModeScreen() {
					m.setScreen(ScreenSDDMode)
				} else if m.Selection.Preset == model.PresetCustom {
					// Custom preset: dependency plan was already built before model picker.
					// Check StrictTDD, then skill picker before going to review.
					if m.shouldShowStrictTDDScreen() {
						m.setScreen(ScreenStrictTDD)
					} else if m.shouldShowSkillPickerScreen() {
						if len(m.SkillPicker) == 0 {
							m.initSkillPicker()
						}
						m.setScreen(ScreenSkillPicker)
					} else {
						m.goToReviewOrMonday()
					}
				} else if m.shouldShowStrictTDDScreen() {
					m.setScreen(ScreenStrictTDD)
				} else {
					m.buildDependencyPlan()
					m.setScreen(ScreenDependencyTree)
				}
			}
			return m, nil
		}
	}

	switch keyStr {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "up":
		// On the preview screen, up arrow scrolls content up.
		if m.Screen == ScreenAgentBuilderPreview {
			if m.AgentBuilder.PreviewScroll > 0 {
				m.AgentBuilder.PreviewScroll--
			}
			return m, nil
		}
		count := m.optionCount()
		if count > 0 {
			if m.Cursor > 0 {
				m.Cursor--
			} else if !m.isScrollableScreen() {
				// Issue #150: wrap-around — Up at 0 goes to last option.
				m.Cursor = count - 1
			}
		}
		// Adjust scroll for the backup list.
		if m.Screen == ScreenBackups {
			if m.Cursor < m.BackupScroll {
				m.BackupScroll = m.Cursor
			}
		}
		return m, nil
	case "down":
		// On the preview screen, down arrow scrolls content down.
		if m.Screen == ScreenAgentBuilderPreview {
			m.AgentBuilder.PreviewScroll++
			return m, nil
		}
		count := m.optionCount()
		if m.Cursor+1 < count {
			m.Cursor++
		} else if count > 0 && !m.isScrollableScreen() {
			// Issue #150: wrap-around — Down at last goes to 0.
			m.Cursor = 0
		}
		// Adjust scroll for the backup list.
		if m.Screen == ScreenBackups {
			if m.Cursor >= m.BackupScroll+screens.BackupMaxVisible {
				m.BackupScroll = m.Cursor - screens.BackupMaxVisible + 1
			}
		}
		return m, nil
	case "k":
		count := m.optionCount()
		if count > 0 {
			if m.Cursor > 0 {
				m.Cursor--
			} else if !m.isScrollableScreen() {
				// Issue #150: wrap-around — Up at 0 goes to last option.
				m.Cursor = count - 1
			}
		}
		// Adjust scroll for the backup list.
		if m.Screen == ScreenBackups {
			if m.Cursor < m.BackupScroll {
				m.BackupScroll = m.Cursor
			}
		}
		return m, nil
	case "j":
		count := m.optionCount()
		if m.Cursor+1 < count {
			m.Cursor++
		} else if count > 0 && !m.isScrollableScreen() {
			// Issue #150: wrap-around — Down at last goes to 0.
			m.Cursor = 0
		}
		// Adjust scroll for the backup list.
		if m.Screen == ScreenBackups {
			if m.Cursor >= m.BackupScroll+screens.BackupMaxVisible {
				m.BackupScroll = m.Cursor - screens.BackupMaxVisible + 1
			}
		}
		return m, nil
	case "esc":
		// Don't allow going back while pipeline is running.
		if m.Screen == ScreenInstalling && m.pipelineRunning {
			return m, nil
		}
		return m.goBack(), nil
	case " ":
		switch m.Screen {
		case ScreenAgents:
			m.toggleCurrentAgent()
		case ScreenDependencyTree:
			if m.Selection.Preset == model.PresetCustom {
				m.toggleCurrentComponent()
			}
		case ScreenSkillPicker:
			m.toggleCurrentSkill()
		case ScreenDevSkillPicker:
			m.toggleCurrentDevSkill()
		case ScreenDevAgentPicker:
			m.toggleCurrentDevAgent()
		}
		return m, nil
	case "r":
		// Rename: only when on ScreenBackups and cursor is on a backup item (not "Back").
		if m.Screen == ScreenBackups && m.Cursor < len(m.Backups) {
			m.SelectedBackup = m.Backups[m.Cursor]
			m.BackupRenameText = m.SelectedBackup.Description
			m.BackupRenamePos = len([]rune(m.SelectedBackup.Description))
			m.setScreen(ScreenRenameBackup)
			return m, nil
		}
	case "n":
		// "n" on ScreenProfiles: shortcut for "Create new profile".
		if m.Screen == ScreenProfiles {
			m.ProfileEditMode = false
			m.ProfileDraft = model.Profile{}
			m.ProfileCreateStep = 0
			m.ProfileNameInput = ""
			m.ProfileNamePos = 0
			m.ProfileNameErr = ""
			m.Selection.ModelAssignments = nil
			m.setScreen(ScreenProfileCreate)
			return m, nil
		}
	case "d":
		// Delete: only when on ScreenBackups and cursor is on a backup item (not "Back").
		if m.Screen == ScreenBackups && m.Cursor < len(m.Backups) {
			m.SelectedBackup = m.Backups[m.Cursor]
			m.setScreen(ScreenDeleteConfirm)
			return m, nil
		}
		// Delete on ScreenProfiles: only non-default profiles (those in ProfileList).
		if m.Screen == ScreenProfiles && m.Cursor < len(m.ProfileList) {
			m.ProfileDeleteTarget = m.ProfileList[m.Cursor].Name
			m.setScreen(ScreenProfileDelete)
			return m, nil
		}
	case "p":
		// Pin/unpin: only when on ScreenBackups and cursor is on a backup item (not "Back").
		if m.Screen == ScreenBackups && m.Cursor < len(m.Backups) {
			// Clear any stale error from a previous attempt before trying again.
			m.PinErr = nil
			if m.TogglePinFn != nil {
				if err := m.TogglePinFn(m.Backups[m.Cursor]); err != nil {
					// Pin failed — surface the error inline; leave list unchanged.
					m.PinErr = err
					return m, nil
				}
			}
			if m.ListBackupsFn != nil {
				m.Backups = m.ListBackupsFn()
			}
			return m, nil
		}
	case "enter":
		return m.confirmSelection()
	}

	return m, nil
}

func (m Model) confirmSelection() (tea.Model, tea.Cmd) {
	switch m.Screen {
	case ScreenWelcome:
		switch m.Cursor {
		case 0:
			m.setScreen(ScreenDetection)
		case 1:
			m = m.withResetOperationState()
			m.setScreen(ScreenUpgradeSync)
		case 2:
			// "Configure Monday"
			m.setScreen(ScreenMonday)
		case 3:
			m.setScreen(ScreenModelConfig)
		case 4:
			// "Create your own Agent" — blocked when no engines are available.
			if !m.hasAgentBuilderEngines() {
				return m, nil
			}
			m.AgentBuilder = AgentBuilderState{}
			m.AgentBuilder.AvailableEngines = m.detectAgentBuilderEngines()
			ta := textarea.New()
			ta.Placeholder = "Describe what you want your agent to do..."
			ta.Focus()
			ta.SetWidth(60)
			ta.SetHeight(5)
			m.AgentBuilder.Textarea = ta
			m.setScreen(ScreenAgentBuilderEngine)
		case 5:
			// "Manage backups"
			m.setScreen(ScreenBackups)
		case 6:
			// "Quit"
			return m, tea.Quit
		}
	case ScreenUpgrade:
		// Guard: don't re-launch while running.
		if m.OperationRunning {
			return m, nil
		}
		// If showing results (UpgradeReport != nil or UpgradeErr != nil), return to welcome.
		if m.UpgradeReport != nil || m.UpgradeErr != nil {
			m = m.withResetOperationState()
			m.setScreen(ScreenWelcome)
			return m, nil
		}
		// If update check is not done yet, no-op.
		if !m.UpdateCheckDone {
			return m, nil
		}
		// If no updates available, just return to welcome.
		if !update.HasUpdates(m.UpdateResults) {
			m.setScreen(ScreenWelcome)
			return m, nil
		}
		// Start upgrade.
		m.OperationRunning = true
		m.OperationMode = "upgrade"
		return m, tea.Batch(tickCmd(), m.startUpgrade())
	case ScreenSync:
		// Guard: don't re-launch while running.
		if m.OperationRunning {
			return m, nil
		}
		// If sync already ran, return to welcome.
		if m.HasSyncRun {
			m = m.withResetOperationState()
			m.setScreen(ScreenWelcome)
			return m, nil
		}
		// Start sync.
		m.OperationRunning = true
		m.OperationMode = "sync"
		return m, tea.Batch(tickCmd(), m.startSync(m.PendingSyncOverrides))
	case ScreenUpgradeSync:
		// Guard: don't re-launch while running.
		if m.OperationRunning {
			return m, nil
		}
		// If operations are done, return to welcome.
		if m.HasSyncRun || m.UpgradeReport != nil || m.UpgradeErr != nil {
			m = m.withResetOperationState()
			m.setScreen(ScreenWelcome)
			return m, nil
		}
		// Start upgrade+sync.
		m.OperationRunning = true
		m.OperationMode = "upgrade-sync"
		return m, tea.Batch(tickCmd(), m.startUpgradeSync())
	case ScreenProfiles:
		// Profiles are: 0..len(ProfileList)-1, then Create, then Back.
		profileCount := len(m.ProfileList)
		switch {
		case m.Cursor < profileCount:
			// Edit an existing profile.
			profile := m.ProfileList[m.Cursor]
			m.ProfileEditMode = true
			m.ProfileDraft = profile
			m.ProfileCreateStep = 0
			m.ProfileNameInput = profile.Name
			m.ProfileNamePos = len([]rune(profile.Name))
			m.ProfileNameErr = ""
			// Build ModelAssignments from the profile's phase assignments + orchestrator.
			// The ModelPicker shows sdd-orchestrator as the first row, so we need
			// to include it in the map for it to display the current model.
			assignments := make(map[string]model.ModelAssignment)
			for k, v := range profile.PhaseAssignments {
				assignments[k] = v
			}
			if profile.OrchestratorModel.ProviderID != "" {
				assignments[screens.SDDOrchestratorPhase] = profile.OrchestratorModel
			}
			m.Selection.ModelAssignments = assignments
			m.setScreen(ScreenProfileCreate)
		case m.Cursor == profileCount:
			// "Create new profile"
			m.ProfileEditMode = false
			m.ProfileDraft = model.Profile{}
			m.ProfileCreateStep = 0
			m.ProfileNameInput = ""
			m.ProfileNamePos = 0
			m.ProfileNameErr = ""
			m.Selection.ModelAssignments = nil
			m.setScreen(ScreenProfileCreate)
		default:
			// "Back"
			m.setScreen(ScreenWelcome)
		}
		return m, nil
	case ScreenProfileCreate:
		return m.confirmProfileCreate()
	case ScreenProfileDelete:
		switch m.Cursor {
		case 0: // "Delete & Sync"
			if err := sdd.RemoveProfileAgents(opencode.DefaultSettingsPath(), m.ProfileDeleteTarget); err != nil {
				// Store the error so it can be displayed on ScreenProfiles.
				m.ProfileDeleteErr = err
				m.setScreen(ScreenProfiles)
			} else {
				m.ProfileDeleteErr = nil
				m.PendingSyncOverrides = nil
				m = m.withResetSyncState()
				m.setScreen(ScreenSync)
				return m, tea.Batch(tickCmd(), m.startSync(nil))
			}
		default: // "Cancel"
			m.setScreen(ScreenProfiles)
		}
		return m, nil
	case ScreenModelConfig:
		switch m.Cursor {
		case 0: // Configure Claude models
			m.ModelConfigMode = true
			m.ClaudeModelPicker = screens.NewClaudeModelPickerState()
			m.setScreen(ScreenClaudeModelPicker)
		case 1: // Configure OpenCode models
			m.ModelConfigMode = true
			cachePath := opencode.DefaultCachePath()
			if _, err := osStatModelCache(cachePath); err == nil {
				m.ModelPicker = screens.NewModelPickerState(cachePath)
			} else {
				m.ModelPicker = screens.ModelPickerState{}
			}
			// Pre-populate with existing assignments from opencode.json.
			// Only when there are no in-session assignments yet — the nil guard
			// ensures we don't overwrite changes the user already made this session.
			if m.Selection.ModelAssignments == nil {
				settingsPath := opencode.DefaultSettingsPath()
				if current, err := readCurrentAssignmentsFn(settingsPath); err == nil && len(current) > 0 {
					m.Selection.ModelAssignments = current
				}
			}
			m.setScreen(ScreenModelPicker)
		default: // Back
			m.setScreen(ScreenWelcome)
		}
		return m, nil
	case ScreenDetection:
		if m.Cursor == 0 {
			m.setScreen(ScreenAgents)
			return m, nil
		}
		m.setScreen(ScreenWelcome)
	case ScreenAgents:
		agentCount := len(screens.AgentOptions())
		switch {
		case m.Cursor < agentCount:
			m.toggleCurrentAgent()
		case m.Cursor == agentCount && len(m.Selection.Agents) > 0:
			m.setScreen(ScreenPreset)
		case m.Cursor == agentCount+1:
			m.setScreen(ScreenDetection)
		}
	case ScreenPreset:
		options := screens.PresetOptions()
		if m.Cursor < len(options) {
			m.Selection.Preset = options[m.Cursor]
			m.Selection.Components = componentsForPreset(options[m.Cursor])
			if m.shouldShowClaudeModelPickerScreen() {
				m.ClaudeModelPicker = screens.NewClaudeModelPickerState()
				m.setScreen(ScreenClaudeModelPicker)
				return m, nil
			}
			if m.shouldShowSDDModeScreen() {
				m.setScreen(ScreenSDDMode)
				return m, nil
			}
			if m.shouldShowStrictTDDScreen() {
				m.setScreen(ScreenStrictTDD)
				return m, nil
			}
			m.buildDependencyPlan()
			m.setScreen(ScreenDependencyTree)
			return m, nil
		}
		m.setScreen(ScreenAgents)
	case ScreenClaudeModelPicker:
		if !m.ClaudeModelPicker.InCustomMode && m.Cursor == screens.ClaudeModelPickerOptionCount(m.ClaudeModelPicker)-1 {
			// "Back" option: in ModelConfigMode return to the config menu,
			// otherwise navigate to the previous install-flow screen.
			if m.ModelConfigMode {
				m.ModelConfigMode = false
				m.setScreen(ScreenModelConfig)
				return m, nil
			}
			if m.Selection.Preset == model.PresetCustom {
				m.setScreen(ScreenDependencyTree)
			} else {
				m.setScreen(ScreenPreset)
			}
			return m, nil
		}
	case ScreenSDDMode:
		options := screens.SDDModeOptions()
		if m.Cursor < len(options) {
			m.Selection.SDDMode = options[m.Cursor]
			if m.Selection.SDDMode == model.SDDModeMulti {
				cachePath := opencode.DefaultCachePath()
				if _, err := osStatModelCache(cachePath); err == nil {
					// Cache exists — OpenCode has been run at least once.
					// Show the model picker so the user can assign models.
					m.ModelPicker = screens.NewModelPickerState(cachePath)
					m.Selection.ModelAssignments = nil
					m.setScreen(ScreenModelPicker)
					return m, nil
				}
				// Cache missing — OpenCode hasn't been run yet on this machine.
				// Skip the model picker; models will use OpenCode defaults.
				// The picker empty-state message explains what to do after install.
				m.ModelPicker = screens.ModelPickerState{}
			}
			// Clear assignments for both single mode and multi-no-cache paths.
			m.Selection.ModelAssignments = nil
			// Show StrictTDD screen when OpenCode + SDD are selected.
			// This is the next step before the dependency tree.
			if m.shouldShowSDDModeScreen() {
				m.setScreen(ScreenStrictTDD)
				return m, nil
			}
			if m.Selection.Preset == model.PresetCustom {
				// Custom preset: dependency plan was already built before SDD mode.
				// Check skill picker before going to review.
				if m.shouldShowSkillPickerScreen() {
					if len(m.SkillPicker) == 0 {
						m.initSkillPicker()
					}
					m.setScreen(ScreenSkillPicker)
				} else {
					m.goToReviewOrMonday()
				}
			} else {
				m.buildDependencyPlan()
				m.setScreen(ScreenDependencyTree)
			}
			return m, nil
		}
		// Back — in custom preset, return to ClaudeModelPicker if applicable,
		// otherwise DependencyTree (component selector).
		// NOTE: SDDMode back logic is also in goBack() — keep in sync.
		if m.Selection.Preset == model.PresetCustom {
			if m.shouldShowClaudeModelPickerScreen() {
				m.setScreen(ScreenClaudeModelPicker)
			} else {
				m.setScreen(ScreenDependencyTree)
			}
		} else {
			// NOTE: Back logic also in goBack() — keep in sync.
			if m.shouldShowClaudeModelPickerScreen() {
				m.setScreen(ScreenClaudeModelPicker)
			} else {
				m.setScreen(ScreenPreset)
			}
		}
	case ScreenModelPicker:
		// When no providers are detected the screen only shows a "Back" option
		// at cursor 0.  Handle that before the normal row logic.
		if len(m.ModelPicker.AvailableIDs) == 0 {
			if m.ModelConfigMode {
				m.ModelConfigMode = false
				m.setScreen(ScreenModelConfig)
				return m, nil
			}
			// Go back to SDD mode so the user can switch to single mode.
			m.setScreen(ScreenSDDMode)
			return m, nil
		}
		rows := screens.ModelPickerRows()
		if m.Cursor < len(rows) {
			// Enter sub-selection: pick provider then model.
			m.ModelPicker.SelectedPhaseIdx = m.Cursor
			m.ModelPicker.Mode = screens.ModeProviderSelect
			m.ModelPicker.ProviderCursor = 0
			m.ModelPicker.ProviderScroll = 0
			return m, nil
		}
		// After the rows: Continue (cursor == len(rows)), Back (cursor == len(rows)+1).
		if m.Cursor == len(rows) {
			// In ModelConfigMode, persist model assignments via sync.
			if m.ModelConfigMode {
				m.ModelConfigMode = false
				m.PendingSyncOverrides = &model.SyncOverrides{
					ModelAssignments: m.Selection.ModelAssignments,
					SDDMode:          model.SDDModeMulti,
				}
				m = m.withResetSyncState()
				m.setScreen(ScreenSync)
				return m, nil
			}
			if m.Selection.Preset == model.PresetCustom {
				// Custom preset: dependency plan was already built before SDD mode.
				// Check StrictTDD, then skill picker before going to review.
				if m.shouldShowStrictTDDScreen() {
					m.setScreen(ScreenStrictTDD)
				} else if m.shouldShowSkillPickerScreen() {
					if len(m.SkillPicker) == 0 {
						m.initSkillPicker()
					}
					m.setScreen(ScreenSkillPicker)
				} else {
					m.goToReviewOrMonday()
				}
			} else {
				// Continue -> check StrictTDD before dependency tree.
				if m.shouldShowStrictTDDScreen() {
					m.setScreen(ScreenStrictTDD)
				} else {
					m.buildDependencyPlan()
					m.setScreen(ScreenDependencyTree)
				}
			}
			return m, nil
		}
		// Back -> return to SDDMode (or ModelConfig in shortcut mode).
		// ModelPicker sits BETWEEN SDDMode and StrictTDD in the forward flow:
		//   SDDMode → ModelPicker → StrictTDD → DependencyTree
		// So Back from ModelPicker must go to SDDMode, NOT StrictTDD
		// (going to StrictTDD would create a loop: ModelPicker ↔ StrictTDD).
		if m.ModelConfigMode {
			m.ModelConfigMode = false
			m.setScreen(ScreenModelConfig)
			return m, nil
		}
		m.setScreen(ScreenSDDMode)
	case ScreenStrictTDD:
		options := screens.StrictTDDOptions()
		if m.Cursor < len(options) {
			// Enable is index 0, Disable is index 1.
			m.Selection.StrictTDD = (m.Cursor == screens.StrictTDDOptionEnable)
			if m.Selection.Preset == model.PresetCustom {
				// Custom preset: dependency plan was already built before SDD mode.
				// Check skill picker before going to review.
				if m.shouldShowSkillPickerScreen() {
					if len(m.SkillPicker) == 0 {
						m.initSkillPicker()
					}
					m.setScreen(ScreenSkillPicker)
				} else {
					m.goToReviewOrMonday()
				}
			} else {
				m.buildDependencyPlan()
				m.setScreen(ScreenDependencyTree)
			}
			return m, nil
		}
		// Back — depends on which flow brought us here.
		if m.shouldShowSDDModeScreen() {
			// OpenCode path: ModelPicker (if multi + cache) or SDDMode.
			if m.Selection.SDDMode == model.SDDModeMulti {
				cachePath := opencode.DefaultCachePath()
				if _, err := osStatModelCache(cachePath); err == nil {
					m.setScreen(ScreenModelPicker)
					return m, nil
				}
			}
			m.setScreen(ScreenSDDMode)
		} else if m.shouldShowClaudeModelPickerScreen() {
			m.setScreen(ScreenClaudeModelPicker)
		} else if m.Selection.Preset == model.PresetCustom {
			// Custom preset: DependencyTree is the component selector that precedes StrictTDD.
			m.setScreen(ScreenDependencyTree)
		} else {
			m.setScreen(ScreenPreset)
		}
	case ScreenDependencyTree:
		if m.Selection.Preset == model.PresetCustom {
			allComps := screens.AllComponents()
			switch {
			case m.Cursor < len(allComps):
				m.toggleCurrentComponent()
			case m.Cursor == len(allComps):
				m.buildDependencyPlan()
				// Show model picker screens if needed (components are now set).
				if m.shouldShowClaudeModelPickerScreen() {
					m.ClaudeModelPicker = screens.NewClaudeModelPickerState()
					m.setScreen(ScreenClaudeModelPicker)
					return m, nil
				}
				if m.shouldShowSDDModeScreen() {
					m.setScreen(ScreenSDDMode)
					return m, nil
				}
				if m.shouldShowStrictTDDScreen() {
					m.setScreen(ScreenStrictTDD)
					return m, nil
				}
				// Show skill picker if Skills component is selected.
				if m.shouldShowSkillPickerScreen() {
					if len(m.SkillPicker) == 0 {
						m.initSkillPicker()
					}
					m.setScreen(ScreenSkillPicker)
					return m, nil
				}
				m.goToReviewOrMonday()
			default:
				m.setScreen(ScreenPreset)
			}
			return m, nil
		}
		if m.Cursor == 0 {
			m.goToReviewOrMonday()
			return m, nil
		}
		// NOTE: Back logic also in goBack() — keep in sync.
		if m.shouldShowStrictTDDScreen() {
			// StrictTDD screen is between ModelPicker/SDDMode and DependencyTree.
			m.setScreen(ScreenStrictTDD)
		} else if m.shouldShowSDDModeScreen() {
			if m.Selection.SDDMode == model.SDDModeMulti {
				cachePath := opencode.DefaultCachePath()
				if _, err := osStatModelCache(cachePath); err == nil {
					m.setScreen(ScreenModelPicker)
				} else {
					m.setScreen(ScreenSDDMode)
				}
			} else {
				m.setScreen(ScreenSDDMode)
			}
		} else if m.shouldShowClaudeModelPickerScreen() {
			m.setScreen(ScreenClaudeModelPicker)
		} else {
			m.setScreen(ScreenPreset)
		}
	case ScreenSkillPicker:
		allSkills := screens.AllSkillsOrdered()
		switch {
		case m.Cursor < len(allSkills):
			m.toggleCurrentSkill()
		case m.Cursor == len(allSkills):
			// "Continue" — store selected skills into Selection and proceed to review.
			m.Selection.Skills = make([]model.SkillID, len(m.SkillPicker))
			copy(m.Selection.Skills, m.SkillPicker)
			m.goToReviewOrMonday()
		default:
			// "Back" — in custom preset, return to the screen that preceded SkillPicker.
			if m.Selection.Preset == model.PresetCustom {
				if m.shouldShowStrictTDDScreen() {
					m.setScreen(ScreenStrictTDD)
				} else if m.shouldShowSDDModeScreen() {
					if m.Selection.SDDMode == model.SDDModeMulti {
						cachePath := opencode.DefaultCachePath()
						if _, err := osStatModelCache(cachePath); err == nil {
							m.setScreen(ScreenModelPicker)
						} else {
							m.setScreen(ScreenSDDMode)
						}
					} else {
						m.setScreen(ScreenSDDMode)
					}
				} else if m.shouldShowClaudeModelPickerScreen() {
					m.setScreen(ScreenClaudeModelPicker)
				} else {
					m.setScreen(ScreenDependencyTree)
				}
			} else {
				m.setScreen(ScreenDependencyTree)
			}
		}
	case ScreenDevSkillPicker:
		// Enter on DevSkillPicker: collect checked skill IDs and advance.
		selections := make([]string, 0, len(m.DevSkillChecked))
		for idx, checked := range m.DevSkillChecked {
			if checked && idx < len(m.DevSkills) {
				selections = append(selections, m.DevSkills[idx].ID)
			}
		}
		m.Selection.DevSkillSelections = selections
		m.goToReviewOrMonday()
		return m, nil
	case ScreenDevAgentPicker:
		// Enter on DevAgentPicker: collect checked agent IDs and advance.
		selections := make([]string, 0, len(m.DevAgentChecked))
		for idx, checked := range m.DevAgentChecked {
			if checked && idx < len(m.DevAgents) {
				selections = append(selections, m.DevAgents[idx].ID)
			}
		}
		m.Selection.DevAgentSelections = selections
		m.goToMondayOrReview()
		return m, nil
	case ScreenMonday:
		// Enter on Monday screen: save inputs and return to welcome menu.
		// Monday is configured from the welcome menu, not the install flow.
		m.Selection.Monday.Token = m.MondayTokenInput
		m.Selection.Monday.BoardID = m.MondayBoardInput
		m.setScreen(ScreenWelcome)
		return m, nil
	case ScreenReview:
		if m.Cursor == 0 {
			return m.startInstalling()
		}
		// Back — in custom preset, walk back through the screens that were shown.
		if m.Selection.Preset == model.PresetCustom {
			if m.shouldShowDevAgentPickerScreen() {
				m.setScreen(ScreenDevAgentPicker)
			} else if m.shouldShowDevSkillPickerScreen() {
				m.setScreen(ScreenDevSkillPicker)
			} else if m.shouldShowSkillPickerScreen() {
				if len(m.SkillPicker) == 0 {
					m.initSkillPicker()
				}
				m.setScreen(ScreenSkillPicker)
			} else if m.shouldShowStrictTDDScreen() {
				m.setScreen(ScreenStrictTDD)
			} else if m.shouldShowSDDModeScreen() {
				if m.Selection.SDDMode == model.SDDModeMulti {
					cachePath := opencode.DefaultCachePath()
					if _, err := osStatModelCache(cachePath); err == nil {
						m.setScreen(ScreenModelPicker)
					} else {
						m.setScreen(ScreenSDDMode)
					}
				} else {
					m.setScreen(ScreenSDDMode)
				}
			} else if m.shouldShowClaudeModelPickerScreen() {
				m.setScreen(ScreenClaudeModelPicker)
			} else {
				m.setScreen(ScreenDependencyTree)
			}
		} else {
			m.setScreen(ScreenDependencyTree)
		}
	case ScreenInstalling:
		if m.Progress.Done() {
			m.setScreen(ScreenComplete)
			return m, nil
		}
		// If no ExecuteFn, fall back to manual step-through for dev/tests.
		if m.ExecuteFn == nil && !m.pipelineRunning {
			m.Progress.Mark(m.Progress.Current, "succeeded")
			if m.Progress.Done() {
				m.setScreen(ScreenComplete)
			}
		}
	case ScreenComplete:
		return m, tea.Quit
	case ScreenBackups:
		if m.Cursor < len(m.Backups) {
			// Navigate to confirmation screen instead of immediately restoring.
			m.SelectedBackup = m.Backups[m.Cursor]
			m.setScreen(ScreenRestoreConfirm)
			return m, nil
		}
		m.setScreen(ScreenWelcome)
	case ScreenRestoreConfirm:
		// Cursor 0 = "Restore", Cursor 1 = "Cancel".
		if m.Cursor == 0 {
			return m.restoreBackup(m.SelectedBackup)
		}
		m.setScreen(ScreenBackups)
	case ScreenRestoreResult:
		// Enter on the result screen returns to backup selection.
		// Refresh the backup list to reflect any changes from the restore.
		if m.ListBackupsFn != nil {
			m.Backups = m.ListBackupsFn()
		}
		m.setScreen(ScreenBackups)
	case ScreenDeleteConfirm:
		// Cursor 0 = "Delete", Cursor 1 = "Cancel".
		if m.Cursor == 0 {
			if m.DeleteBackupFn != nil {
				m.DeleteErr = m.DeleteBackupFn(m.SelectedBackup)
			}
			m.setScreen(ScreenDeleteResult)
		} else {
			m.setScreen(ScreenBackups)
		}
	case ScreenDeleteResult:
		// Enter on the result screen returns to backup selection.
		// Refresh the backup list to reflect any changes from the delete.
		if m.ListBackupsFn != nil {
			m.Backups = m.ListBackupsFn()
		}
		m.DeleteErr = nil
		m.setScreen(ScreenBackups)
	case ScreenAgentBuilderEngine:
		engines := m.AgentBuilder.AvailableEngines
		if m.Cursor < len(engines) {
			m.AgentBuilder.SelectedEngine = engines[m.Cursor]
			m.setScreen(ScreenAgentBuilderPrompt)
		} else {
			// "Back" option.
			m.setScreen(ScreenWelcome)
		}
	case ScreenAgentBuilderPrompt:
		// "Continue" only if textarea is not empty.
		if m.AgentBuilder.Textarea.Value() != "" {
			m.setScreen(ScreenAgentBuilderSDD)
		}
	case ScreenAgentBuilderSDD:
		opts := screens.ABSDDOptions()
		switch m.Cursor {
		case 0:
			m.AgentBuilder.SDDMode = agentbuilder.SDDStandalone
			return m.startGeneration()
		case 1:
			m.AgentBuilder.SDDMode = agentbuilder.SDDNewPhase
			m.setScreen(ScreenAgentBuilderSDDPhase)
		case 2:
			m.AgentBuilder.SDDMode = agentbuilder.SDDPhaseSupport
			m.setScreen(ScreenAgentBuilderSDDPhase)
		case len(opts) - 1:
			m.setScreen(ScreenAgentBuilderPrompt)
		}
	case ScreenAgentBuilderSDDPhase:
		phases := screens.ABSDDPhases()
		if m.Cursor < len(phases) {
			m.AgentBuilder.SDDTargetPhase = phases[m.Cursor]
			return m.startGeneration()
		}
		// "Back" option.
		m.setScreen(ScreenAgentBuilderSDD)
	case ScreenAgentBuilderGenerating:
		// Only interactive when an error is shown (retry/back).
		if m.AgentBuilder.GenerationErr != nil {
			if m.Cursor == 0 {
				// Retry.
				return m.startGeneration()
			}
			// Back.
			m.AgentBuilder.GenerationErr = nil
			m.setScreen(ScreenAgentBuilderPrompt)
		}
	case ScreenAgentBuilderPreview:
		switch m.Cursor {
		case 0:
			// Install — guard against nil generated agent.
			if m.AgentBuilder.Generated == nil {
				return m, nil
			}
			return m.startInstallation()
		case 1:
			// Regenerate — go back to generating.
			return m.startGeneration()
		default:
			// Back.
			m.setScreen(ScreenAgentBuilderPrompt)
		}
	case ScreenAgentBuilderInstalling:
		if !m.AgentBuilder.Installing {
			m.setScreen(ScreenAgentBuilderComplete)
		}
	case ScreenAgentBuilderComplete:
		m.setScreen(ScreenWelcome)
	}

	return m, nil
}

// startInstalling initializes the progress state from the resolved plan and
// starts the pipeline execution in a goroutine if ExecuteFn is provided.
func (m Model) startInstalling() (tea.Model, tea.Cmd) {
	m.setScreen(ScreenInstalling)
	m.SpinnerFrame = 0

	// Build progress labels from the resolved plan.
	labels := buildProgressLabels(m.DependencyPlan)
	if len(labels) == 0 {
		// Fallback labels when the plan is empty (dev/test).
		labels = []string{
			"Install dependencies",
			"Configure selected agents",
			"Inject ecosystem components",
		}
	}

	m.Progress = NewProgressState(labels)
	m.Progress.Start(0)
	m.Progress.AppendLog("starting installation")

	if m.ExecuteFn == nil {
		// No real executor; fall back to manual step-through.
		return m, tickCmd()
	}

	m.pipelineRunning = true

	// Capture values for the goroutine closure.
	executeFn := m.ExecuteFn
	selection := m.Selection
	resolved := m.DependencyPlan
	detection := m.Detection

	return m, tea.Batch(tickCmd(), func() tea.Msg {
		onProgress := func(event pipeline.ProgressEvent) {
			// NOTE: ProgressFunc is called synchronously from the pipeline goroutine.
			// We cannot use p.Send() here because we don't have a reference to the
			// tea.Program. Instead, these events are collected in the ExecutionResult
			// and the PipelineDoneMsg handles the final state. For real-time updates,
			// we rely on the pipeline calling this synchronously from each step.
		}

		result := executeFn(selection, resolved, detection, onProgress)
		return PipelineDoneMsg{Result: result}
	})
}

// withResetSyncState clears sync-result state so ScreenSync shows the confirmation
// screen (State 3) instead of stale results from a previous run.
// Unlike withResetOperationState, this preserves PendingSyncOverrides.
func (m Model) withResetSyncState() Model {
	m.SyncFilesChanged = 0
	m.SyncErr = nil
	m.HasSyncRun = false
	m.OperationRunning = false
	m.OperationMode = ""
	m.Cursor = 0
	return m
}

// withResetOperationState clears all operation-related state and resets the cursor,
// returning a new Model with these fields cleared (value-receiver pattern for MVU).
// This includes clearing PendingSyncOverrides, unlike withResetSyncState.
func (m Model) withResetOperationState() Model {
	m.UpgradeReport = nil
	m.UpgradeErr = nil
	m.SyncFilesChanged = 0
	m.SyncErr = nil
	m.HasSyncRun = false
	m.OperationRunning = false
	m.OperationMode = ""
	m.PendingSyncOverrides = nil
	m.Cursor = 0
	return m
}

// startUpgrade launches the upgrade goroutine and returns a tea.Cmd.
func (m Model) startUpgrade() tea.Cmd {
	upgradeFn := m.UpgradeFn
	updateResults := m.UpdateResults
	return func() tea.Msg {
		if upgradeFn == nil {
			return UpgradeDoneMsg{Err: fmt.Errorf("upgrade function not configured")}
		}
		ctx := context.Background()
		report := upgradeFn(ctx, updateResults)
		return UpgradeDoneMsg{Report: report}
	}
}

// startSync launches the sync goroutine and returns a tea.Cmd.
// When overrides is non-nil, model assignments are merged into the sync selection.
func (m Model) startSync(overrides *model.SyncOverrides) tea.Cmd {
	syncFn := m.SyncFn
	return func() tea.Msg {
		if syncFn == nil {
			return SyncDoneMsg{Err: fmt.Errorf("sync function not configured")}
		}
		filesChanged, err := syncFn(overrides)
		return SyncDoneMsg{FilesChanged: filesChanged, Err: err}
	}
}

// startUpgradeSync runs git pulls then sync sequentially via tea.Sequence.
// It pulls informa-wizard (if running from a git clone), dev-skills, and
// dev-agents repos, then runs sync. Missing repos are skipped silently.
//
// The first command runs the git pulls and sends UpgradePhaseCompletedMsg
// (so the UI can show State 2: sync running). The second command runs sync
// and sends SyncDoneMsg.
func (m Model) startUpgradeSync() tea.Cmd {
	syncFn := m.SyncFn

	pullCmd := func() tea.Msg {
		// Pull informa-wizard itself if running from a git clone.
		if exe, err := os.Executable(); err == nil {
			dir := filepath.Dir(exe)
			// Walk upward to find a .git directory (at most 5 levels).
			candidate := dir
			for i := 0; i < 5; i++ {
				if _, err := os.Stat(filepath.Join(candidate, ".git")); err == nil {
					// Found the git root — pull then rebuild.
					_ = devskills.Pull(candidate)
					goInstall := exec.Command("go", "install", "./cmd/informa-wizard")
					goInstall.Dir = candidate
					_ = goInstall.Run()
					break
				}
				parent := filepath.Dir(candidate)
				if parent == candidate {
					break
				}
				candidate = parent
			}
		}

		// Pull dev-skills and dev-agents repos (skip silently if absent).
		home := homeDir()
		if home != "" {
			devSkillsDir := filepath.Join(home, ".informa-wizard", "dev-skills")
			if _, err := os.Stat(devSkillsDir); err == nil {
				_ = devskills.Pull(devSkillsDir)
			}
			devAgentsDir := filepath.Join(home, ".informa-wizard", "dev-agents")
			if _, err := os.Stat(devAgentsDir); err == nil {
				_ = devagents.Pull(devAgentsDir)
			}
		}

		return UpgradePhaseCompletedMsg{}
	}

	syncCmd := func() tea.Msg {
		if syncFn == nil {
			return SyncDoneMsg{Err: fmt.Errorf("sync function not configured")}
		}
		// Overrides are intentionally nil: update-sync is triggered from
		// Welcome menu, not ModelConfig. PendingSyncOverrides is cleared
		// by withResetOperationState before entering this flow.
		filesChanged, err := syncFn(nil)
		return SyncDoneMsg{FilesChanged: filesChanged, Err: err}
	}

	return tea.Sequence(pullCmd, syncCmd)
}

// restoreBackup triggers a backup restore in a goroutine.
func (m Model) restoreBackup(manifest backup.Manifest) (tea.Model, tea.Cmd) {
	if m.RestoreFn == nil {
		m.Err = fmt.Errorf("restore not available")
		return m, nil
	}

	restoreFn := m.RestoreFn
	return m, func() tea.Msg {
		err := restoreFn(manifest)
		return BackupRestoreMsg{Err: err}
	}
}

// buildProgressLabels creates step labels from the resolved plan that match
// the step IDs the pipeline will produce.
func buildProgressLabels(resolved planner.ResolvedPlan) []string {
	labels := make([]string, 0, 2+len(resolved.Agents)+len(resolved.OrderedComponents)+1)

	labels = append(labels, "prepare:check-dependencies")
	labels = append(labels, "prepare:backup-snapshot")
	labels = append(labels, "apply:rollback-restore")

	for _, agent := range resolved.Agents {
		labels = append(labels, "agent:"+string(agent))
	}

	for _, component := range resolved.OrderedComponents {
		labels = append(labels, "component:"+string(component))
	}

	return labels
}

func (m Model) goBack() Model {
	// Block navigation while an operation (upgrade/sync) is running.
	if m.OperationRunning {
		return m
	}

	// Block going back while agent installation is in progress.
	if m.AgentBuilder.Installing {
		return m
	}

	// Agent builder back navigation.
	switch m.Screen {
	case ScreenAgentBuilderComplete:
		m.setScreen(ScreenWelcome)
		return m
	case ScreenAgentBuilderInstalling:
		// Can't go back while installing — guard above handles this.
		return m
	case ScreenAgentBuilderGenerating:
		if m.AgentBuilder.GenerationErr != nil {
			// Error state: allow going back.
			m.AgentBuilder.GenerationErr = nil
			m.setScreen(ScreenAgentBuilderPrompt)
			return m
		}
		if m.AgentBuilder.Generating {
			// Cancel in-progress generation and navigate back to prompt.
			if m.AgentBuilder.GenerationCancel != nil {
				m.AgentBuilder.GenerationCancel()
			}
			m.AgentBuilder.Generating = false
			m.setScreen(ScreenAgentBuilderPrompt)
			return m
		}
		return m
	}

	// ModelConfigMode: pickers reached via Model Config shortcut return to ScreenModelConfig.
	if m.ModelConfigMode && (m.Screen == ScreenClaudeModelPicker || m.Screen == ScreenModelPicker) {
		m.ModelConfigMode = false
		m.setScreen(ScreenModelConfig)
		return m
	}

	// From SkillPicker, go back to the preceding screen.
	// In custom preset: StrictTDD precedes SkillPicker; SDDMode/ModelPicker/ClaudeModelPicker precede StrictTDD.
	if m.Screen == ScreenSkillPicker {
		if m.Selection.Preset == model.PresetCustom {
			if m.shouldShowStrictTDDScreen() {
				m.setScreen(ScreenStrictTDD)
			} else if m.shouldShowSDDModeScreen() {
				if m.Selection.SDDMode == model.SDDModeMulti {
					cachePath := opencode.DefaultCachePath()
					if _, err := osStatModelCache(cachePath); err == nil {
						m.setScreen(ScreenModelPicker)
					} else {
						m.setScreen(ScreenSDDMode)
					}
				} else {
					m.setScreen(ScreenSDDMode)
				}
			} else if m.shouldShowClaudeModelPickerScreen() {
				m.setScreen(ScreenClaudeModelPicker)
			} else {
				m.setScreen(ScreenDependencyTree)
			}
		} else {
			m.setScreen(ScreenDependencyTree)
		}
		return m
	}

	// If going back from DependencyTree and the SDDMode/ClaudeModelPicker/StrictTDD
	// screens were shown BEFORE it (non-custom presets only), navigate to them.
	// In custom mode these screens appear AFTER the dependency tree, so
	// going back should return to the preset screen (handled by linearRoutes).
	// NOTE: DependencyTree back logic also in confirmSelection() — keep in sync.
	if m.Screen == ScreenDependencyTree && m.Selection.Preset != model.PresetCustom {
		if m.shouldShowStrictTDDScreen() {
			// StrictTDD screen is between (SDDMode/ClaudeModelPicker/Preset) and DependencyTree.
			m.setScreen(ScreenStrictTDD)
			return m
		}
		if m.shouldShowClaudeModelPickerScreen() {
			m.setScreen(ScreenClaudeModelPicker)
			return m
		}
	}

	// Going back from ScreenStrictTDD depends on which flow brought us here:
	//   - OpenCode flow: ModelPicker (multi + cache) → SDDMode
	//   - ClaudeCode flow: ClaudeModelPicker
	//   - Custom preset (other agents): DependencyTree (the component selector)
	//   - Non-custom other agents: Preset
	if m.Screen == ScreenStrictTDD {
		if m.shouldShowSDDModeScreen() {
			// OpenCode path: ModelPicker (if multi + cache) or SDDMode.
			if m.Selection.SDDMode == model.SDDModeMulti {
				cachePath := opencode.DefaultCachePath()
				if _, err := osStatModelCache(cachePath); err == nil {
					m.setScreen(ScreenModelPicker)
					return m
				}
			}
			m.setScreen(ScreenSDDMode)
			return m
		}
		if m.shouldShowClaudeModelPickerScreen() {
			m.setScreen(ScreenClaudeModelPicker)
			return m
		}
		// Custom preset: DependencyTree is the component selector that precedes StrictTDD.
		if m.Selection.Preset == model.PresetCustom {
			m.setScreen(ScreenDependencyTree)
			return m
		}
		// All other non-custom agents: go back to Preset selection.
		m.setScreen(ScreenPreset)
		return m
	}

	// In custom preset, going back from SDDMode should return to ClaudeModelPicker
	// if applicable, otherwise DependencyTree (the component selector).
	// For non-custom, check if ClaudeModelPicker was shown first.
	// NOTE: SDDMode back logic is also in confirmSelection — keep in sync.
	if m.Screen == ScreenSDDMode {
		if m.Selection.Preset == model.PresetCustom {
			if m.shouldShowClaudeModelPickerScreen() {
				m.setScreen(ScreenClaudeModelPicker)
			} else {
				m.setScreen(ScreenDependencyTree)
			}
			return m
		}
		if m.shouldShowClaudeModelPickerScreen() {
			m.setScreen(ScreenClaudeModelPicker)
			return m
		}
	}

	// In custom preset, going back from ClaudeModelPicker should return to DependencyTree.
	if m.Screen == ScreenClaudeModelPicker && m.Selection.Preset == model.PresetCustom {
		m.setScreen(ScreenDependencyTree)
		return m
	}

	// Going back from DevSkillPicker: return to SkillPicker if shown, else DependencyTree.
	if m.Screen == ScreenDevSkillPicker {
		if m.shouldShowSkillPickerScreen() {
			m.setScreen(ScreenSkillPicker)
		} else {
			m.setScreen(ScreenDependencyTree)
		}
		return m
	}

	// Going back from DevAgentPicker: return to DevSkillPicker if shown, else DependencyTree.
	if m.Screen == ScreenDevAgentPicker {
		if m.shouldShowDevSkillPickerScreen() {
			m.setScreen(ScreenDevSkillPicker)
		} else {
			m.setScreen(ScreenDependencyTree)
		}
		return m
	}

	// Going back from Review: if DevAgentPicker was shown, go there.
	if m.Screen == ScreenReview && m.shouldShowDevAgentPickerScreen() {
		m.setScreen(ScreenDevAgentPicker)
		return m
	}

	// Going back from Review: if DevSkillPicker was shown (and DevAgentPicker was not), go there.
	if m.Screen == ScreenReview && m.shouldShowDevSkillPickerScreen() {
		m.setScreen(ScreenDevSkillPicker)
		return m
	}

	// Going back from Monday (accessed from welcome menu): return to welcome.
	if m.Screen == ScreenMonday {
		m.setScreen(ScreenWelcome)
		return m
	}

	// In custom preset, going back from Review walks through intermediate screens.
	// Order (reverse of forward): DevAgentPicker → DevSkillPicker → SkillPicker → StrictTDD → SDDMode/ModelPicker → ClaudeModelPicker → DependencyTree.
	if m.Screen == ScreenReview && m.Selection.Preset == model.PresetCustom {
		if m.shouldShowDevAgentPickerScreen() {
			m.setScreen(ScreenDevAgentPicker)
			return m
		}
		if m.shouldShowDevSkillPickerScreen() {
			m.setScreen(ScreenDevSkillPicker)
			return m
		}
		if m.shouldShowSkillPickerScreen() {
			if len(m.SkillPicker) == 0 {
				m.initSkillPicker()
			}
			m.setScreen(ScreenSkillPicker)
			return m
		}
		if m.shouldShowStrictTDDScreen() {
			m.setScreen(ScreenStrictTDD)
			return m
		}
		if m.shouldShowSDDModeScreen() {
			if m.Selection.SDDMode == model.SDDModeMulti {
				cachePath := opencode.DefaultCachePath()
				if _, err := osStatModelCache(cachePath); err == nil {
					m.setScreen(ScreenModelPicker)
				} else {
					m.setScreen(ScreenSDDMode)
				}
			} else {
				m.setScreen(ScreenSDDMode)
			}
			return m
		}
		if m.shouldShowClaudeModelPickerScreen() {
			m.setScreen(ScreenClaudeModelPicker)
			return m
		}
		m.setScreen(ScreenDependencyTree)
		return m
	}

	// Leaving ScreenSync via Esc: clear stale overrides so they don't leak
	// into a future sync triggered from a different flow (e.g. Welcome menu).
	if m.Screen == ScreenSync && m.PendingSyncOverrides != nil {
		m.PendingSyncOverrides = nil
	}

	previous, ok := PreviousScreen(m.Screen)
	if !ok {
		return m
	}

	m.setScreen(previous)
	return m
}

func (m *Model) setScreen(next Screen) {
	m.PreviousScreen = m.Screen
	m.Screen = next
	m.Cursor = 0
	if next == ScreenBackups {
		m.BackupScroll = 0
		m.PinErr = nil
	}
	if next == ScreenProfiles {
		// Clear stale delete error so it is not shown after Cancel/Esc from ScreenProfileDelete.
		m.ProfileDeleteErr = nil
		// Refresh profile list on entry. Surface errors via m.Err so callers can react.
		profiles, err := readProfilesFn(opencode.DefaultSettingsPath())
		if err != nil {
			m.Err = err
			m.ProfileList = nil
		} else {
			m.ProfileList = profiles
		}
		// Clamp cursor so it never points past the end of a refreshed list.
		// m.Cursor was just reset to 0 above, so this only triggers if ProfileList is empty.
		if m.Cursor >= len(m.ProfileList) {
			m.Cursor = 0
		}
	}
}

// handleRenameInput processes key events when the rename backup screen is active.
// It manages text input for the new backup description.
func (m Model) handleRenameInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEnter:
		// Execute rename and return to backups.
		if m.RenameBackupFn != nil {
			_ = m.RenameBackupFn(m.SelectedBackup, m.BackupRenameText)
		}
		if m.ListBackupsFn != nil {
			m.Backups = m.ListBackupsFn()
		}
		m.setScreen(ScreenBackups)
		return m, nil
	case tea.KeyEsc:
		m.setScreen(ScreenBackups)
		return m, nil
	case tea.KeyBackspace:
		if m.BackupRenamePos > 0 {
			runes := []rune(m.BackupRenameText)
			m.BackupRenameText = string(append(runes[:m.BackupRenamePos-1], runes[m.BackupRenamePos:]...))
			m.BackupRenamePos--
		}
		return m, nil
	case tea.KeyLeft:
		if m.BackupRenamePos > 0 {
			m.BackupRenamePos--
		}
		return m, nil
	case tea.KeyRight:
		if m.BackupRenamePos < len([]rune(m.BackupRenameText)) {
			m.BackupRenamePos++
		}
		return m, nil
	case tea.KeyRunes:
		runes := []rune(m.BackupRenameText)
		newRunes := make([]rune, 0, len(runes)+len(msg.Runes))
		newRunes = append(newRunes, runes[:m.BackupRenamePos]...)
		newRunes = append(newRunes, msg.Runes...)
		newRunes = append(newRunes, runes[m.BackupRenamePos:]...)
		m.BackupRenameText = string(newRunes)
		m.BackupRenamePos += len(msg.Runes)
		return m, nil
	}
	return m, nil
}

func (m Model) handleMondayInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEnter:
		// Save inputs and return to welcome menu.
		// Monday is configured from the welcome menu, not the install flow.
		m.Selection.Monday.Token = m.MondayTokenInput
		m.Selection.Monday.BoardID = m.MondayBoardInput
		m.setScreen(ScreenWelcome)
		return m, nil
	case tea.KeyEsc:
		m.setScreen(ScreenWelcome)
		return m, nil
	case tea.KeyTab:
		// Switch between token and board ID fields.
		if m.MondayActiveField == screens.MondayFieldToken {
			m.MondayActiveField = screens.MondayFieldBoardID
		} else {
			m.MondayActiveField = screens.MondayFieldToken
		}
		return m, nil
	case tea.KeyBackspace:
		if m.MondayActiveField == screens.MondayFieldToken {
			if m.MondayTokenPos > 0 {
				runes := []rune(m.MondayTokenInput)
				m.MondayTokenInput = string(append(runes[:m.MondayTokenPos-1], runes[m.MondayTokenPos:]...))
				m.MondayTokenPos--
			}
		} else {
			if m.MondayBoardPos > 0 {
				runes := []rune(m.MondayBoardInput)
				m.MondayBoardInput = string(append(runes[:m.MondayBoardPos-1], runes[m.MondayBoardPos:]...))
				m.MondayBoardPos--
			}
		}
		return m, nil
	case tea.KeyLeft:
		if m.MondayActiveField == screens.MondayFieldToken {
			if m.MondayTokenPos > 0 {
				m.MondayTokenPos--
			}
		} else {
			if m.MondayBoardPos > 0 {
				m.MondayBoardPos--
			}
		}
		return m, nil
	case tea.KeyRight:
		if m.MondayActiveField == screens.MondayFieldToken {
			if m.MondayTokenPos < len([]rune(m.MondayTokenInput)) {
				m.MondayTokenPos++
			}
		} else {
			if m.MondayBoardPos < len([]rune(m.MondayBoardInput)) {
				m.MondayBoardPos++
			}
		}
		return m, nil
	case tea.KeyRunes:
		if m.MondayActiveField == screens.MondayFieldToken {
			runes := []rune(m.MondayTokenInput)
			newRunes := make([]rune, 0, len(runes)+len(msg.Runes))
			newRunes = append(newRunes, runes[:m.MondayTokenPos]...)
			newRunes = append(newRunes, msg.Runes...)
			newRunes = append(newRunes, runes[m.MondayTokenPos:]...)
			m.MondayTokenInput = string(newRunes)
			m.MondayTokenPos += len(msg.Runes)
		} else {
			runes := []rune(m.MondayBoardInput)
			newRunes := make([]rune, 0, len(runes)+len(msg.Runes))
			newRunes = append(newRunes, runes[:m.MondayBoardPos]...)
			newRunes = append(newRunes, msg.Runes...)
			newRunes = append(newRunes, runes[m.MondayBoardPos:]...)
			m.MondayBoardInput = string(newRunes)
			m.MondayBoardPos += len(msg.Runes)
		}
		return m, nil
	}
	return m, nil
}

func (m Model) optionCount() int {
	switch m.Screen {
	case ScreenWelcome:
		return len(screens.WelcomeOptions(m.hasAgentBuilderEngines()))
	case ScreenUpgrade:
		if m.UpgradeReport != nil || m.UpgradeErr != nil {
			return 1 // "return" option in results/error state
		}
		if !m.UpdateCheckDone {
			return 0 // no options while checking
		}
		return 1 // "upgrade all" or "return" when up to date
	case ScreenSync:
		return 1
	case ScreenUpgradeSync:
		return 1
	case ScreenModelConfig:
		return len(screens.ModelConfigOptions())
	case ScreenDetection:
		return len(screens.DetectionOptions())
	case ScreenAgents:
		return len(screens.AgentOptions()) + 2
	case ScreenPreset:
		return len(screens.PresetOptions()) + 1
	case ScreenClaudeModelPicker:
		return screens.ClaudeModelPickerOptionCount(m.ClaudeModelPicker)
	case ScreenSDDMode:
		return len(screens.SDDModeOptions()) + 1
	case ScreenStrictTDD:
		return len(screens.StrictTDDOptions()) + 1 // Enable + Disable + Back
	case ScreenModelPicker:
		if len(m.ModelPicker.AvailableIDs) == 0 {
			return 1 // only "Back to SDD mode"
		}
		return len(screens.ModelPickerRows()) + 2 // rows + Continue + Back
	case ScreenDependencyTree:
		if m.Selection.Preset == model.PresetCustom {
			return len(screens.AllComponents()) + len(screens.DependencyTreeOptions())
		}
		return len(screens.DependencyTreeOptions())
	case ScreenSkillPicker:
		return screens.SkillPickerOptionCount()
	case ScreenDevSkillPicker:
		return len(m.DevSkills)
	case ScreenDevAgentPicker:
		return len(m.DevAgents)
	case ScreenMonday:
		return 0 // text input mode — no cursor navigation
	case ScreenReview:
		return len(screens.ReviewOptions())
	case ScreenInstalling:
		return 1
	case ScreenComplete:
		return 1
	case ScreenBackups:
		return len(m.Backups) + 1
	case ScreenRestoreConfirm:
		return 2 // "Restore" + "Cancel"
	case ScreenRestoreResult:
		return 1 // "Done" / continue
	case ScreenDeleteConfirm:
		return 2 // "Delete" + "Cancel"
	case ScreenDeleteResult:
		return 1 // "Done" / continue
	case ScreenRenameBackup:
		return 0 // text input mode — no cursor navigation
	case ScreenProfiles:
		return screens.ProfileListOptionCount(m.ProfileList)
	case ScreenProfileCreate:
		return screens.ProfileCreateOptionCount(m.ProfileCreateStep, m.ModelPicker)
	case ScreenProfileDelete:
		return screens.ProfileDeleteOptionCount()
	case ScreenAgentBuilderEngine:
		return len(m.AgentBuilder.AvailableEngines) + 1 // engines + Back
	case ScreenAgentBuilderPrompt:
		return 0 // textarea mode — cursor navigation via textarea
	case ScreenAgentBuilderSDD:
		return len(screens.ABSDDOptions()) // 3 modes + Back
	case ScreenAgentBuilderSDDPhase:
		return len(screens.ABSDDPhases()) + 1 // phases + Back
	case ScreenAgentBuilderGenerating:
		if m.AgentBuilder.GenerationErr != nil {
			return 2 // Retry + Back
		}
		return 0 // generating — no cursor navigation
	case ScreenAgentBuilderPreview:
		return len(screens.ABPreviewActions()) // Install + Regenerate + Back
	case ScreenAgentBuilderInstalling:
		return 0 // no cursor navigation while installing
	case ScreenAgentBuilderComplete:
		return 1 // Done
	default:
		return 0
	}
}

func (m *Model) toggleCurrentAgent() {
	options := screens.AgentOptions()
	if m.Cursor >= len(options) {
		return
	}

	agent := options[m.Cursor]
	for idx, selected := range m.Selection.Agents {
		if selected == agent {
			m.Selection.Agents = append(m.Selection.Agents[:idx], m.Selection.Agents[idx+1:]...)
			return
		}
	}

	m.Selection.Agents = append(m.Selection.Agents, agent)
}

func (m *Model) toggleCurrentComponent() {
	allComps := screens.AllComponents()
	if m.Cursor >= len(allComps) {
		return
	}

	compID := allComps[m.Cursor].ID
	for idx, selected := range m.Selection.Components {
		if selected == compID {
			m.Selection.Components = append(m.Selection.Components[:idx], m.Selection.Components[idx+1:]...)
			return
		}
	}

	m.Selection.Components = append(m.Selection.Components, compID)
}

func (m *Model) toggleCurrentSkill() {
	allSkills := screens.AllSkillsOrdered()
	if m.Cursor >= len(allSkills) {
		return
	}

	skillID := allSkills[m.Cursor]
	for idx, selected := range m.SkillPicker {
		if selected == skillID {
			m.SkillPicker = append(m.SkillPicker[:idx], m.SkillPicker[idx+1:]...)
			return
		}
	}

	m.SkillPicker = append(m.SkillPicker, skillID)
}

// toggleCurrentDevSkill toggles the checked state of the dev skill at the current cursor position.
func (m *Model) toggleCurrentDevSkill() {
	if m.Cursor < 0 || m.Cursor >= len(m.DevSkills) {
		return
	}
	if m.Cursor < len(m.DevSkillChecked) {
		m.DevSkillChecked[m.Cursor] = !m.DevSkillChecked[m.Cursor]
	}
}

// toggleCurrentDevAgent toggles the checked state of the dev agent at the current cursor position.
func (m *Model) toggleCurrentDevAgent() {
	if m.Cursor < 0 || m.Cursor >= len(m.DevAgents) {
		return
	}
	if m.Cursor < len(m.DevAgentChecked) {
		m.DevAgentChecked[m.Cursor] = !m.DevAgentChecked[m.Cursor]
	}
}

// initSkillPicker pre-selects ALL available skills (custom mode default).
func (m *Model) initSkillPicker() {
	all := screens.AllSkillsOrdered()
	m.SkillPicker = make([]model.SkillID, len(all))
	copy(m.SkillPicker, all)
}

// initDevSkillPicker attempts to discover dev-skills from the cloned repo.
// If the repo is not yet cloned, DevSkills is left as an empty slice —
// the user can still proceed; skills will be injected after clone during install.
func (m *Model) initDevSkillPicker() {
	if len(m.DevSkills) > 0 {
		return // already initialized, preserve selections
	}
	homeDir, _ := os.UserHomeDir()
	if homeDir == "" {
		return
	}
	repoDir := filepath.Join(homeDir, ".informa-wizard", "dev-skills")

	// Clone the repo if not already present so skills can be discovered.
	if _, err := os.Stat(repoDir); os.IsNotExist(err) {
		_ = devskills.Clone(devskills.DefaultRepoURL, repoDir)
	}

	discovered, err := devskills.DiscoverSkills(repoDir)
	if err != nil {
		m.DevSkills = nil
		m.DevSkillChecked = nil
		return
	}
	m.DevSkills = discovered
	m.DevSkillChecked = make([]bool, len(discovered))
	m.DevSkillCursor = 0
}

// initDevAgentPicker attempts to discover dev-agents from the cloned repo.
// If the repo is not yet cloned, DevAgents is left as an empty slice —
// the user can still proceed; agents will be injected after clone during install.
func (m *Model) initDevAgentPicker() {
	if len(m.DevAgents) > 0 {
		return // already initialized, preserve selections
	}
	homeDir, _ := os.UserHomeDir()
	if homeDir == "" {
		return
	}
	repoDir := filepath.Join(homeDir, ".informa-wizard", "dev-agents")

	// Clone the repo if not already present so agents can be discovered.
	if _, err := os.Stat(repoDir); os.IsNotExist(err) {
		_ = devagents.Clone(devagents.DefaultRepoURL, repoDir)
	}

	discovered, err := devagents.DiscoverAgents(repoDir)
	if err != nil {
		m.DevAgents = nil
		m.DevAgentChecked = nil
		return
	}
	m.DevAgents = discovered
	m.DevAgentChecked = make([]bool, len(discovered))
	m.DevAgentCursor = 0
}

// shouldShowSkillPickerScreen returns true when the custom preset is active
// and the Skills component has been selected.
func (m Model) shouldShowSkillPickerScreen() bool {
	return m.Selection.Preset == model.PresetCustom &&
		hasSelectedComponent(m.Selection.Components, model.ComponentSkills)
}

func (m *Model) buildDependencyPlan() {
	resolved, err := planner.NewResolver(planner.MVPGraph()).Resolve(m.Selection)
	if err != nil {
		m.Err = err
		m.DependencyPlan = planner.ResolvedPlan{}
		return
	}

	m.DependencyPlan = resolved
}

func preselectedAgents(detection system.DetectionResult) []model.AgentID {
	// Only detect agents that are in the active catalog.
	catalogAgentSet := make(map[model.AgentID]bool)
	for _, a := range catalog.AllAgents() {
		catalogAgentSet[a.ID] = true
	}

	selected := []model.AgentID{}
	for _, state := range detection.Configs {
		if !state.Exists {
			continue
		}
		agentID := model.AgentID(strings.TrimSpace(state.Agent))
		if catalogAgentSet[agentID] {
			selected = append(selected, agentID)
		}
	}

	if len(selected) > 0 {
		return selected
	}

	agents := catalog.AllAgents()
	selected = make([]model.AgentID, 0, len(agents))
	for _, agent := range agents {
		selected = append(selected, agent.ID)
	}

	return selected
}

func extractMissingDeps(detection system.DetectionResult) []screens.MissingDep {
	if detection.Dependencies.AllPresent {
		return nil
	}

	var deps []screens.MissingDep
	for _, dep := range detection.Dependencies.Dependencies {
		if !dep.Installed && dep.Required {
			deps = append(deps, screens.MissingDep{Name: dep.Name, InstallHint: dep.InstallHint})
		}
	}
	return deps
}

func extractFailedSteps(result pipeline.ExecutionResult) []screens.FailedStep {
	var failed []screens.FailedStep
	collect := func(steps []pipeline.StepResult) {
		for _, step := range steps {
			if step.Status == pipeline.StepStatusFailed {
				errMsg := "unknown error"
				if step.Err != nil {
					errMsg = step.Err.Error()
				}
				failed = append(failed, screens.FailedStep{ID: step.StepID, Error: errMsg})
			}
		}
	}
	collect(result.Prepare.Steps)
	collect(result.Apply.Steps)
	return failed
}

func extractAvailableUpdates(results []update.UpdateResult) []screens.UpdateInfo {
	var updates []screens.UpdateInfo
	for _, r := range results {
		if r.Status == update.UpdateAvailable {
			updates = append(updates, screens.UpdateInfo{
				Name:             r.Tool.Name,
				InstalledVersion: r.InstalledVersion,
				LatestVersion:    r.LatestVersion,
				UpdateHint:       r.UpdateHint,
			})
		}
	}
	return updates
}

// hasDetectedOpenCode returns true if OpenCode config directory was detected.
func (m Model) hasDetectedOpenCode() bool {
	for _, cfg := range m.Detection.Configs {
		if cfg.Agent == string(model.AgentOpenCode) && cfg.Exists {
			return true
		}
	}
	return false
}

// shouldShowSDDModeScreen always returns false — the SDD Mode selection screen
// has been removed from the install flow. Multi-agent mode is the default.
func (m Model) shouldShowSDDModeScreen() bool {
	return false
}

// shouldShowStrictTDDScreen reports whether the Strict TDD Mode screen should
// be shown in the navigation flow. It requires only that the SDD component is
// selected — the screen is agent-agnostic.
func (m Model) shouldShowStrictTDDScreen() bool {
	return hasSelectedComponent(m.Selection.Components, model.ComponentSDD)
}

func (m Model) shouldShowMondayScreen() bool {
	return hasSelectedComponent(m.Selection.Components, model.ComponentMonday)
}

// shouldShowDevSkillPickerScreen returns true when the DevSkills component is selected.
func (m Model) shouldShowDevSkillPickerScreen() bool {
	return hasSelectedComponent(m.Selection.Components, model.ComponentDevSkills)
}

// shouldShowDevAgentPickerScreen returns true when the DevAgents component is selected.
func (m Model) shouldShowDevAgentPickerScreen() bool {
	return hasSelectedComponent(m.Selection.Components, model.ComponentDevAgents)
}

// goToReviewOrMonday navigates to the DevSkillPicker screen (when the DevSkills
// component is selected and we're not already on that screen), then to the
// DevAgentPicker screen (when the DevAgents component is selected and we're not
// already on that screen), then directly to Review.
// Note: Monday configuration has been moved to the welcome menu and is no longer
// part of the install flow.
func (m *Model) goToReviewOrMonday() {
	if m.shouldShowDevSkillPickerScreen() && m.Screen != ScreenDevSkillPicker {
		m.initDevSkillPicker()
		m.setScreen(ScreenDevSkillPicker)
		return
	}
	if m.shouldShowDevAgentPickerScreen() && m.Screen != ScreenDevAgentPicker {
		m.initDevAgentPicker()
		m.setScreen(ScreenDevAgentPicker)
		return
	}
	m.Review = planner.BuildReviewPayload(m.Selection, m.DependencyPlan)
	m.setScreen(ScreenReview)
}

// goToMondayOrReview navigates directly to Review.
// Called after the DevAgentPicker is confirmed.
// Note: Monday configuration has been moved to the welcome menu and is no longer
// part of the install flow.
func (m *Model) goToMondayOrReview() {
	m.Review = planner.BuildReviewPayload(m.Selection, m.DependencyPlan)
	m.setScreen(ScreenReview)
}

func mondayCursorPos(m Model) int {
	if m.MondayActiveField == screens.MondayFieldToken {
		return m.MondayTokenPos
	}
	return m.MondayBoardPos
}

func (m Model) shouldShowClaudeModelPickerScreen() bool {
	return m.Selection.HasAgent(model.AgentClaudeCode) &&
		hasSelectedComponent(m.Selection.Components, model.ComponentSDD)
}

func componentsForPreset(preset model.PresetID) []model.ComponentID {
	switch preset {
	case model.PresetMinimal:
		return []model.ComponentID{model.ComponentSDD, model.ComponentDevSkills, model.ComponentDevAgents}
	case model.PresetEcosystemOnly:
		return []model.ComponentID{model.ComponentSDD, model.ComponentSkills, model.ComponentContext7, model.ComponentMonday, model.ComponentDevSkills, model.ComponentDevAgents}
	case model.PresetCustom:
		return nil
	default:
		return []model.ComponentID{
			model.ComponentSDD,
			model.ComponentSkills,
			model.ComponentContext7,
			model.ComponentPermission,
			model.ComponentMonday,
			model.ComponentDevSkills,
			model.ComponentDevAgents,
		}
	}
}

func hasSelectedComponent(components []model.ComponentID, target model.ComponentID) bool {
	for _, c := range components {
		if c == target {
			return true
		}
	}
	return false
}

// isScrollableScreen returns true for screens that use scroll-based navigation
// instead of a fixed option list. Wrap-around navigation (Issue #150) must be
// disabled for these screens to avoid confusing the scroll offset logic.
func (m Model) isScrollableScreen() bool {
	return m.Screen == ScreenBackups
}

// resolveCommitDate returns the date of the last commit embedded by the Go
// toolchain (vcs.time build setting). Returns nil for dev builds or when
// build info is unavailable.
func resolveCommitDate() *time.Time {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return nil
	}
	for _, s := range info.Settings {
		if s.Key == "vcs.time" {
			t, err := time.Parse(time.RFC3339, s.Value)
			if err == nil {
				return &t
			}
		}
	}
	return nil
}

// handleProfileNameInput processes key events when the profile create screen
// is at step 0 (name input). In edit mode, step 0 is skipped to step 1 — this
// handler is only called when NOT in edit mode.
func (m Model) handleProfileNameInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEnter:
		// Validate and advance to step 1.
		name := strings.ToLower(m.ProfileNameInput)
		if err := sdd.ValidateProfileName(name); err != nil {
			m.ProfileNameErr = err.Error()
			m.ProfileNameCollision = false
			return m, nil
		}

		// Check for collision with an existing profile.
		if !m.ProfileNameCollision {
			for _, p := range m.ProfileList {
				if p.Name == name {
					m.ProfileNameErr = fmt.Sprintf("Profile '%s' already exists. Press enter to overwrite.", name)
					m.ProfileNameCollision = true
					return m, nil
				}
			}
		}

		// Clear collision flag and proceed.
		m.ProfileNameErr = ""
		m.ProfileNameCollision = false
		m.ProfileDraft.Name = name
		m.ProfileCreateStep = 1
		// Initialize model picker for orchestrator step.
		cachePath := opencode.DefaultCachePath()
		if _, err := osStatModelCache(cachePath); err == nil {
			m.ModelPicker = screens.NewModelPickerState(cachePath)
		} else {
			m.ModelPicker = screens.ModelPickerState{}
		}
		m.Cursor = 0
		return m, nil
	case tea.KeyEsc:
		m.ProfileNameCollision = false
		m.setScreen(ScreenProfiles)
		return m, nil
	case tea.KeyBackspace:
		if m.ProfileNamePos > 0 {
			runes := []rune(m.ProfileNameInput)
			m.ProfileNameInput = string(append(runes[:m.ProfileNamePos-1], runes[m.ProfileNamePos:]...))
			m.ProfileNamePos--
			// Typing clears the collision warning so the user can modify the name.
			m.ProfileNameCollision = false
			m.ProfileNameErr = ""
		}
		return m, nil
	case tea.KeyLeft:
		if m.ProfileNamePos > 0 {
			m.ProfileNamePos--
		}
		return m, nil
	case tea.KeyRight:
		if m.ProfileNamePos < len([]rune(m.ProfileNameInput)) {
			m.ProfileNamePos++
		}
		return m, nil
	case tea.KeyRunes:
		runes := []rune(m.ProfileNameInput)
		newRunes := make([]rune, 0, len(runes)+len(msg.Runes))
		newRunes = append(newRunes, runes[:m.ProfileNamePos]...)
		newRunes = append(newRunes, msg.Runes...)
		newRunes = append(newRunes, runes[m.ProfileNamePos:]...)
		m.ProfileNameInput = string(newRunes)
		m.ProfileNamePos += len(msg.Runes)
		// Typing clears the collision warning so the user can modify the name.
		m.ProfileNameCollision = false
		m.ProfileNameErr = ""
		return m, nil
	}
	return m, nil
}

// confirmProfileCreate handles enter key presses on ScreenProfileCreate.
// Step 0 (name input) is handled by handleProfileNameInput for create mode.
// Steps: 0=name, 1=assign models (orchestrator + sub-agents), 2=confirm.
func (m Model) confirmProfileCreate() (tea.Model, tea.Cmd) {
	switch m.ProfileCreateStep {
	case 0:
		// Edit mode: step 0 shows read-only name, enter advances to step 1.
		if m.ProfileEditMode {
			m.ProfileCreateStep = 1
			cachePath := opencode.DefaultCachePath()
			if _, err := osStatModelCache(cachePath); err == nil {
				m.ModelPicker = screens.NewModelPickerState(cachePath)
			} else {
				m.ModelPicker = screens.ModelPickerState{}
			}
			m.Cursor = 0
		}
		return m, nil
	case 1:
		// Model assignment picker: orchestrator + all sub-agent phases in one screen.
		// Reuse the same enter-on-row logic as ScreenModelPicker.
		rows := screens.ModelPickerRows()
		if m.Cursor < len(rows) {
			// Enter sub-selection: pick provider then model.
			m.ModelPicker.SelectedPhaseIdx = m.Cursor
			m.ModelPicker.Mode = screens.ModeProviderSelect
			m.ModelPicker.ProviderCursor = 0
			m.ModelPicker.ProviderScroll = 0
			return m, nil
		}
		if m.Cursor == len(rows) {
			// "Continue": extract orchestrator + phase assignments, advance to confirm.
			if m.Selection.ModelAssignments != nil {
				// Extract orchestrator model.
				if orch, ok := m.Selection.ModelAssignments[screens.SDDOrchestratorPhase]; ok {
					m.ProfileDraft.OrchestratorModel = orch
				}
				// Copy all phase assignments (excluding orchestrator).
				if m.ProfileDraft.PhaseAssignments == nil {
					m.ProfileDraft.PhaseAssignments = make(map[string]model.ModelAssignment)
				}
				for k, v := range m.Selection.ModelAssignments {
					if k != screens.SDDOrchestratorPhase {
						m.ProfileDraft.PhaseAssignments[k] = v
					}
				}
			}
			m.ProfileCreateStep = 2
			m.Cursor = 0
		}
		if m.Cursor == len(rows)+1 {
			// "Back": return to step 0 (name) or profiles list.
			if m.ProfileEditMode {
				m.setScreen(ScreenProfiles)
			} else {
				m.ProfileCreateStep = 0
				m.Cursor = 0
			}
		}
		return m, nil
	default:
		// Step 2: confirm.
		switch m.Cursor {
		case 0: // "Create & Sync" / "Save & Sync"
			draft := m.ProfileDraft
			m.PendingSyncOverrides = &model.SyncOverrides{
				Profiles: []model.Profile{draft},
			}
			m = m.withResetSyncState()
			m.setScreen(ScreenSync)
			return m, tea.Batch(tickCmd(), m.startSync(m.PendingSyncOverrides))
		default: // "Cancel"
			m.setScreen(ScreenProfiles)
		}
		return m, nil
	}
}

// detectAgentBuilderEngines scans for supported AI agent binaries on PATH and
// returns the list of available AgentIDs.
func (m Model) detectAgentBuilderEngines() []model.AgentID {
	candidateIDs := []model.AgentID{
		model.AgentClaudeCode,
		model.AgentOpenCode,
		model.AgentGeminiCLI,
		model.AgentCodex,
	}
	var available []model.AgentID
	for _, id := range candidateIDs {
		engine := agentbuilder.NewEngine(id)
		if engine != nil && engine.Available() {
			available = append(available, id)
		}
	}
	return available
}

// hasAgentBuilderEngines reports whether any supported AI agent binary is installed.
func (m Model) hasAgentBuilderEngines() bool {
	return len(m.detectAgentBuilderEngines()) > 0
}

// agentBuilderInstallTargets returns the list of install target paths for the preview screen.
// Each path is the full destination: {SkillsDir}/{agent.Name}/SKILL.md
func (m Model) agentBuilderInstallTargets() []string {
	adapters := m.buildAgentBuilderAdapters()
	agent := m.AgentBuilder.Generated
	targets := make([]string, 0, len(adapters))
	for _, a := range adapters {
		if agent != nil {
			targets = append(targets, filepath.Join(a.SkillsDir, agent.Name, "SKILL.md"))
		} else {
			targets = append(targets, a.SkillsDir)
		}
	}
	return targets
}

// buildAgentBuilderAdapters returns the AdapterInfo list for all detected agents.
func (m Model) buildAgentBuilderAdapters() []agentbuilder.AdapterInfo {
	var adapters []agentbuilder.AdapterInfo
	for _, cfg := range m.Detection.Configs {
		if !cfg.Exists {
			continue
		}
		agentID := model.AgentID(strings.TrimSpace(cfg.Agent))
		if skillsDir, ok := agentBuilderSkillsDir(agentID); ok {
			adapters = append(adapters, agentbuilder.AdapterInfo{
				AgentID:   agentID,
				SkillsDir: skillsDir,
			})
		}
	}
	// Fallback: if no agents detected via config, use all engines that are available.
	if len(adapters) == 0 {
		for _, id := range m.AgentBuilder.AvailableEngines {
			if skillsDir, ok := agentBuilderSkillsDir(id); ok {
				adapters = append(adapters, agentbuilder.AdapterInfo{
					AgentID:   id,
					SkillsDir: skillsDir,
				})
			}
		}
	}
	return adapters
}

// homeDir returns the current user's home directory path.
func homeDir() string {
	if h, err := os.UserHomeDir(); err == nil && h != "" {
		return h
	}
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	return "/tmp"
}

// buildInstalledAgentIDs returns the list of AgentIDs from the adapter list.
func buildInstalledAgentIDs(adapters []agentbuilder.AdapterInfo) []model.AgentID {
	ids := make([]model.AgentID, 0, len(adapters))
	for _, a := range adapters {
		ids = append(ids, a.AgentID)
	}
	return ids
}

// agentBuilderSkillsDir returns the skills directory for the given agent and a
// flag indicating whether the path was found among the well-known agents.
func agentBuilderSkillsDir(agentID model.AgentID) (string, bool) {
	home := homeDir()
	switch agentID {
	case model.AgentClaudeCode:
		return filepath.Join(home, ".claude", "skills"), true
	case model.AgentOpenCode:
		return filepath.Join(home, ".config", "opencode", "skills"), true
	case model.AgentGeminiCLI:
		return filepath.Join(home, ".gemini", "skills"), true
	case model.AgentCodex:
		return filepath.Join(home, ".codex", "skills"), true
	default:
		return "", false
	}
}

// startGeneration launches the AI generation goroutine and transitions to the
// generating screen.
func (m Model) startGeneration() (tea.Model, tea.Cmd) {
	m.AgentBuilder.Generating = true
	m.AgentBuilder.GenerationErr = nil
	m.AgentBuilder.Generated = nil
	m.setScreen(ScreenAgentBuilderGenerating)

	engineID := m.AgentBuilder.SelectedEngine
	userInput := m.AgentBuilder.Textarea.Value()

	var sddConfig *agentbuilder.SDDIntegration
	if m.AgentBuilder.SDDMode != agentbuilder.SDDStandalone {
		sddConfig = &agentbuilder.SDDIntegration{
			Mode:        m.AgentBuilder.SDDMode,
			TargetPhase: m.AgentBuilder.SDDTargetPhase,
		}
		// For SDDNewPhase, set a placeholder PhaseName before prompt composition.
		// The actual PhaseName is updated after generation from agent.Name.
		if m.AgentBuilder.SDDMode == agentbuilder.SDDNewPhase {
			sddConfig.PhaseName = "to-be-determined-from-title"
		}
		// PhaseName will be set after generation from the agent's Name field.
		// SDDTargetPhase is the "insert after" position, not the new phase name.
	}

	// Capture for goroutine.
	capturedSDD := sddConfig
	adapters := m.buildAgentBuilderAdapters()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	m.AgentBuilder.GenerationCancel = cancel

	return m, tea.Batch(tickCmd(), func() tea.Msg {
		defer cancel()

		engine := agentbuilder.NewEngine(engineID)
		if engine == nil {
			return AgentBuilderGeneratedMsg{
				Err: fmt.Errorf("no engine available for %s", engineID),
			}
		}

		installedAgents := buildInstalledAgentIDs(adapters)
		prompt := agentbuilder.ComposePrompt(userInput, capturedSDD, installedAgents)

		raw, err := engine.Generate(ctx, prompt)
		if err != nil {
			return AgentBuilderGeneratedMsg{Err: err}
		}

		agent, err := agentbuilder.Parse(raw)
		if err != nil {
			return AgentBuilderGeneratedMsg{Err: err}
		}

		if capturedSDD != nil {
			// For SDDNewPhase, derive the new phase name from the agent's Name,
			// not from SDDTargetPhase (which is the "insert after" position).
			if capturedSDD.Mode == agentbuilder.SDDNewPhase {
				capturedSDD.PhaseName = agent.Name
			}
			agent.SDDConfig = capturedSDD
		}

		return AgentBuilderGeneratedMsg{Agent: agent}
	})
}

// startInstallation launches the agent installation goroutine.
func (m Model) startInstallation() (tea.Model, tea.Cmd) {
	m.AgentBuilder.Installing = true
	m.AgentBuilder.InstallErr = nil
	m.setScreen(ScreenAgentBuilderInstalling)

	agent := m.AgentBuilder.Generated
	adapters := m.buildAgentBuilderAdapters()
	engineID := m.AgentBuilder.SelectedEngine

	return m, tea.Batch(tickCmd(), func() (msg tea.Msg) {
		// Recover from panics so the spinner never runs forever.
		defer func() {
			if r := recover(); r != nil {
				msg = AgentBuilderInstallDoneMsg{
					Err: fmt.Errorf("install panicked: %v", r),
				}
			}
		}()

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		_ = ctx // timeout enforced; Install itself is synchronous

		// Resolve agent name, applying conflict suffix if needed.
		installAgent := agent
		if agentbuilder.HasConflictWithBuiltin(agent.Name) {
			// Shallow copy so we don't mutate the generated agent in state.
			copy := *agent
			copy.Name = agent.Name + "-custom"
			installAgent = &copy
		}

		results, err := agentbuilder.Install(installAgent, adapters, "")
		if err != nil {
			return AgentBuilderInstallDoneMsg{Results: results, Err: err}
		}

		// Persist entry to registry.
		registryPath := filepath.Join(homeDir(), ".config", "informa-wizard", "custom-agents.json")
		_ = os.MkdirAll(filepath.Dir(registryPath), 0755)
		if reg, loadErr := agentbuilder.LoadRegistry(registryPath); loadErr == nil {
			// Collect IDs of agents that were successfully installed.
			var installedIDs []model.AgentID
			for _, r := range results {
				if r.Success {
					installedIDs = append(installedIDs, r.AgentID)
				}
			}
			entry := agentbuilder.RegistryEntry{
				Name:             installAgent.Name,
				Title:            installAgent.Title,
				Description:      installAgent.Description,
				CreatedAt:        time.Now(),
				GenerationEngine: engineID,
				SDDIntegration:   installAgent.SDDConfig,
				InstalledAgents:  installedIDs,
			}
			// Update existing entry if present; otherwise append.
			if existing := reg.FindByName(installAgent.Name); existing != nil {
				existing.Title = entry.Title
				existing.Description = entry.Description
				existing.CreatedAt = entry.CreatedAt
				existing.GenerationEngine = entry.GenerationEngine
				existing.SDDIntegration = entry.SDDIntegration
				existing.InstalledAgents = entry.InstalledAgents
			} else {
				reg.Add(entry)
			}
			// Best-effort save — ignore save errors.
			_ = agentbuilder.SaveRegistry(registryPath, reg)
		}

		// Wire SDD injection: append custom-agent reference blocks to system prompts.
		// Best-effort — don't fail the whole install if SDD injection fails.
		if installAgent.SDDConfig != nil && installAgent.SDDConfig.Mode != agentbuilder.SDDStandalone {
			for _, adapter := range adapters {
				if systemPromptPath, ok := agentBuilderSystemPromptPath(adapter.AgentID); ok {
					_ = agentbuilder.InjectSDDReference(installAgent, systemPromptPath)
				}
			}
		}

		return AgentBuilderInstallDoneMsg{Results: results, Err: nil}
	})
}

// agentBuilderSystemPromptPath returns the system prompt file path for the given agent.
func agentBuilderSystemPromptPath(agentID model.AgentID) (string, bool) {
	home := homeDir()
	switch agentID {
	case model.AgentClaudeCode:
		return filepath.Join(home, ".claude", "CLAUDE.md"), true
	case model.AgentOpenCode:
		return filepath.Join(home, ".config", "opencode", "AGENTS.md"), true
	case model.AgentGeminiCLI:
		return filepath.Join(home, ".gemini", "GEMINI.md"), true
	case model.AgentCodex:
		return filepath.Join(home, ".codex", "AGENTS.md"), true
	default:
		return "", false
	}
}
