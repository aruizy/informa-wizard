package cli

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"gitlab.informa.tools/ai/wizard/informa-wizard/internal/agents"
	"gitlab.informa.tools/ai/wizard/informa-wizard/internal/backup"
	"gitlab.informa.tools/ai/wizard/informa-wizard/internal/components/devagents"
	"gitlab.informa.tools/ai/wizard/informa-wizard/internal/components/devskills"
	"gitlab.informa.tools/ai/wizard/informa-wizard/internal/components/engram"
	"gitlab.informa.tools/ai/wizard/informa-wizard/internal/components/mcp"
	"gitlab.informa.tools/ai/wizard/informa-wizard/internal/components/monday"
	"gitlab.informa.tools/ai/wizard/informa-wizard/internal/components/permissions"
	"gitlab.informa.tools/ai/wizard/informa-wizard/internal/components/sdd"
	"gitlab.informa.tools/ai/wizard/informa-wizard/internal/components/skills"
	"gitlab.informa.tools/ai/wizard/informa-wizard/internal/components/theme"
	"gitlab.informa.tools/ai/wizard/informa-wizard/internal/model"
	"gitlab.informa.tools/ai/wizard/informa-wizard/internal/pipeline"
	"gitlab.informa.tools/ai/wizard/informa-wizard/internal/planner"
	"gitlab.informa.tools/ai/wizard/informa-wizard/internal/state"
	"gitlab.informa.tools/ai/wizard/informa-wizard/internal/system"
	"gitlab.informa.tools/ai/wizard/informa-wizard/internal/verify"
)

type InstallResult struct {
	Selection    model.Selection
	Resolved     planner.ResolvedPlan
	Review       planner.ReviewPayload
	Plan         pipeline.StagePlan
	Execution    pipeline.ExecutionResult
	Verify       verify.Report
	Dependencies system.DependencyReport
	DryRun       bool
}

var (
	osUserHomeDir       = os.UserHomeDir
	osSetenv            = os.Setenv
	osStat              = os.Stat
	runCommand          = executeCommand
	cmdLookPath         = exec.LookPath
	streamCommandOutput = true

	// ggaAvailableCheck is an optional override for ggaAvailable behavior.
	// When set, it is called instead of the default filesystem check.
	ggaAvailableCheck func(system.PlatformProfile) bool

	// engramDownloadFn is the function used to download the engram binary on non-brew platforms.
	// Package-level var for testability — tests can replace this to avoid real HTTP calls.
	engramDownloadFn = engram.DownloadLatestBinary

	// AppVersion is the informa-wizard version that will be written into backup manifests.
	// It is set by app.go before any CLI operation so that every backup created during
	// an install or sync records which version of informa-wizard made it.
	// Default "dev" matches the ldflags default in app.Version.
	AppVersion = "dev"
)

// SetCommandOutputStreaming toggles whether command stdout/stderr is streamed
// directly to the terminal. It returns a restore function.
func SetCommandOutputStreaming(enabled bool) func() {
	previous := streamCommandOutput
	streamCommandOutput = enabled
	return func() {
		streamCommandOutput = previous
	}
}

func RunInstall(args []string, detection system.DetectionResult) (InstallResult, error) {
	flags, err := ParseInstallFlags(args)
	if err != nil {
		return InstallResult{}, err
	}

	input, err := NormalizeInstallFlags(flags, detection)
	if err != nil {
		return InstallResult{}, err
	}

	resolved, err := planner.NewResolver(planner.MVPGraph()).Resolve(input.Selection)
	if err != nil {
		return InstallResult{}, err
	}
	profile := ResolveInstallProfile(detection)
	resolved.PlatformDecision = planner.PlatformDecisionFromProfile(profile)

	review := planner.BuildReviewPayload(input.Selection, resolved)
	stagePlan := buildStagePlan(input.Selection, resolved)

	result := InstallResult{
		Selection:    input.Selection,
		Resolved:     resolved,
		Review:       review,
		Plan:         stagePlan,
		Dependencies: detection.Dependencies,
		DryRun:       input.DryRun,
	}

	if input.DryRun {
		return result, nil
	}

	homeDir, err := osUserHomeDir()
	if err != nil {
		return result, fmt.Errorf("resolve user home directory: %w", err)
	}

	runtime, err := newInstallRuntime(homeDir, input.Selection, resolved, profile, flags.DevSkillsRepo)
	if err != nil {
		return result, err
	}

	// Print dependency warnings before the pipeline starts (CLI only).
	// The TUI surfaces these on the complete screen instead.
	if !detection.Dependencies.AllPresent {
		fmt.Fprintf(os.Stderr, "WARNING: missing dependencies: %s\n\n%s\n",
			strings.Join(detection.Dependencies.MissingRequired, ", "),
			system.FormatMissingDepsMessage(detection.Dependencies))
	}

	stagePlan = runtime.stagePlan()
	result.Plan = stagePlan

	orchestrator := pipeline.NewOrchestrator(pipeline.DefaultRollbackPolicy())
	result.Execution = orchestrator.Execute(stagePlan)
	if result.Execution.Err != nil {
		return result, fmt.Errorf("execute install pipeline: %w", result.Execution.Err)
	}

	result.Verify = runPostApplyVerification(homeDir, input.Selection, resolved)
	result.Verify = withPostInstallNotes(result.Verify, resolved)
	if !result.Verify.Ready {
		return result, fmt.Errorf("post-apply verification failed:\n%s", verify.RenderReport(result.Verify))
	}

	// Persist the user's agent and component selection so that future `sync`
	// runs target only what the user actually installed.
	agentIDs := make([]string, 0, len(input.Selection.Agents))
	for _, a := range input.Selection.Agents {
		agentIDs = append(agentIDs, string(a))
	}
	componentIDs := make([]string, 0, len(input.Selection.Components))
	for _, c := range input.Selection.Components {
		componentIDs = append(componentIDs, string(c))
	}
	// Non-fatal: a state write failure must not break an otherwise successful install.
	_ = state.Write(homeDir, agentIDs, componentIDs)

	// Persist the source repo directory so Update+Sync can find it for git pull + go install.
	if wd, wdErr := os.Getwd(); wdErr == nil {
		if _, gitErr := os.Stat(filepath.Join(wd, ".git")); gitErr == nil {
			sourceDirFile := filepath.Join(homeDir, ".informa-wizard", "source-dir")
			_ = os.MkdirAll(filepath.Dir(sourceDirFile), 0o755)
			_ = os.WriteFile(sourceDirFile, []byte(wd+"\n"), 0o644)
		}
	}

	return result, nil
}

func withPostInstallNotes(report verify.Report, resolved planner.ResolvedPlan) verify.Report {
	report = withGoInstallPathNote(report, resolved)
	return report
}

// withGoInstallPathNote appends a PATH guidance note when engram was installed
// on a non-brew platform (Linux/Windows). Since engram is now installed via
// direct binary download to /usr/local/bin or ~/.local/bin, this note helps
// users who may need to add the install directory to their PATH.
func withGoInstallPathNote(report verify.Report, resolved planner.ResolvedPlan) verify.Report {
	if !hasComponent(resolved.OrderedComponents, model.ComponentEngram) {
		return report
	}
	if resolved.PlatformDecision.PackageManager == "brew" {
		return report
	}
	binDir := goInstallBinDir()
	if isInPATH(binDir) {
		return report
	}
	report.FinalNote = report.FinalNote + fmt.Sprintf(
		"\n\nThe engram binary was installed to %s via `go install`.\nAdd it to your PATH: %s",
		binDir,
		engramPathGuidance(os.Getenv("SHELL")),
	)
	return report
}

// goInstallBinDir returns the directory where `go install` places binaries.
// Resolution order: $GOBIN > $GOPATH/bin > $HOME/go/bin.
func goInstallBinDir() string {
	if gobin := os.Getenv("GOBIN"); gobin != "" {
		return gobin
	}
	if gopath := os.Getenv("GOPATH"); gopath != "" {
		return filepath.Join(gopath, "bin")
	}
	if home, err := osUserHomeDir(); err == nil {
		return filepath.Join(home, "go", "bin")
	}
	return filepath.Join("~", "go", "bin")
}

// isInPATH reports whether dir is present in the current PATH.
func isInPATH(dir string) bool {
	for _, entry := range filepath.SplitList(os.Getenv("PATH")) {
		if entry == dir {
			return true
		}
	}
	return false
}

func buildStagePlan(selection model.Selection, resolved planner.ResolvedPlan) pipeline.StagePlan {
	prepare := []pipeline.Step{
		noopStep{id: "prepare:system-check"},
		noopStep{id: "prepare:check-dependencies"},
	}
	apply := make([]pipeline.Step, 0, len(resolved.Agents)+len(resolved.OrderedComponents))

	for _, agent := range resolved.Agents {
		apply = append(apply, noopStep{id: "agent:" + string(agent)})
	}

	for _, component := range resolved.OrderedComponents {
		apply = append(apply, noopStep{id: "component:" + string(component)})
	}

	if len(selection.Agents) == 0 && len(resolved.OrderedComponents) == 0 {
		prepare = nil
	}

	return pipeline.StagePlan{Prepare: prepare, Apply: apply}
}

type installRuntime struct {
	homeDir       string
	workspaceDir  string
	selection     model.Selection
	resolved      planner.ResolvedPlan
	profile       system.PlatformProfile
	backupRoot    string
	state         *runtimeState
	devSkillsRepo string
}

type runtimeState struct {
	manifest backup.Manifest
}

func newInstallRuntime(homeDir string, selection model.Selection, resolved planner.ResolvedPlan, profile system.PlatformProfile, devSkillsRepo string) (*installRuntime, error) {
	backupRoot := filepath.Join(homeDir, ".informa-wizard", "backups")
	if err := os.MkdirAll(backupRoot, 0o755); err != nil {
		return nil, fmt.Errorf("create backup root directory %q: %w", backupRoot, err)
	}

	workspaceDir, _ := os.Getwd()

	return &installRuntime{
		homeDir:       homeDir,
		workspaceDir:  workspaceDir,
		selection:     selection,
		resolved:      resolved,
		profile:       profile,
		backupRoot:    backupRoot,
		state:         &runtimeState{},
		devSkillsRepo: devSkillsRepo,
	}, nil
}

func (r *installRuntime) stagePlan() pipeline.StagePlan {
	targets := backupTargets(r.homeDir, r.selection, r.resolved)
	prepare := []pipeline.Step{
		checkDependenciesStep{id: "prepare:check-dependencies", profile: r.profile},
		prepareBackupStep{
			id:          "prepare:backup-snapshot",
			snapshotter: backup.NewSnapshotter(),
			snapshotDir: filepath.Join(r.backupRoot, time.Now().UTC().Format("20060102150405.000000000")),
			targets:     targets,
			state:       r.state,
			backupRoot:  r.backupRoot,
			source:      backup.BackupSourceInstall,
			description: "pre-install snapshot",
			appVersion:  AppVersion,
		},
	}

	apply := make([]pipeline.Step, 0, len(r.resolved.Agents)+len(r.resolved.OrderedComponents)+1)
	apply = append(apply, rollbackRestoreStep{id: "apply:rollback-restore", state: r.state})

	for _, agent := range r.resolved.Agents {
		apply = append(apply, agentInstallStep{id: "agent:" + string(agent), agent: agent, homeDir: r.homeDir, profile: r.profile})
	}

	for _, component := range r.resolved.OrderedComponents {
		apply = append(apply, componentApplyStep{
			id:            "component:" + string(component),
			component:     component,
			homeDir:       r.homeDir,
			workspaceDir:  r.workspaceDir,
			agents:        r.resolved.Agents,
			selection:     r.selection,
			profile:       r.profile,
			devSkillsRepo: r.devSkillsRepo,
		})
	}

	return pipeline.StagePlan{Prepare: prepare, Apply: apply}
}

type prepareBackupStep struct {
	id          string
	snapshotter backup.Snapshotter
	snapshotDir string
	targets     []string
	state       *runtimeState

	// backupRoot is the parent directory of all backup snapshots.
	// When set, deduplication (IsDuplicate) and retention pruning (Prune) are
	// enabled. When empty, both are skipped (backward-compatible default).
	backupRoot string

	// source and description are optional metadata written into the manifest.
	// When set, they help users identify what created the backup.
	source      backup.BackupSource
	description string

	// appVersion is the informa-wizard version that created this backup.
	// When set, it is written into the manifest as CreatedByVersion.
	appVersion string
}

func (s prepareBackupStep) ID() string {
	return s.id
}

func (s prepareBackupStep) Run() error {
	// Deduplication: skip snapshot creation when content is identical to the
	// most recent backup. Only active when backupRoot is set.
	if s.backupRoot != "" {
		checksum, err := backup.ComputeChecksum(s.targets)
		if err == nil && checksum != "" {
			if dup, dupErr := backup.IsDuplicate(s.backupRoot, checksum); dupErr != nil {
				log.Printf("backup: check duplicate: %v", dupErr)
			} else if dup {
				// Content is identical to the most recent backup — skip creation.
				// state.manifest is left at its zero value; rollback is a no-op.
				return nil
			}
		}
	}

	manifest, err := s.snapshotter.Create(s.snapshotDir, s.targets)
	if err != nil {
		return fmt.Errorf("create backup snapshot: %w", err)
	}

	// Annotate with source metadata and version when provided, then re-write.
	// FileCount is already populated by Snapshotter.Create.
	if s.source != "" || s.appVersion != "" {
		manifest.Source = s.source
		manifest.Description = s.description
		manifest.CreatedByVersion = s.appVersion
		manifestPath := filepath.Join(s.snapshotDir, backup.ManifestFilename)
		if err := backup.WriteManifest(manifestPath, manifest); err != nil {
			// Non-fatal: metadata annotation failed but the snapshot is intact.
			// The backup is still usable — restore will work. We just lose the label.
			log.Printf("backup: annotate manifest: %v", err)
		}
	}

	s.state.manifest = manifest

	// Retention pruning: remove oldest unpinned backups beyond the limit.
	// Non-fatal: a prune failure must not prevent the install/sync from succeeding.
	if s.backupRoot != "" {
		if _, pruneErr := backup.Prune(s.backupRoot, backup.DefaultRetentionCount); pruneErr != nil {
			log.Printf("backup: prune: %v", pruneErr)
		}
	}

	return nil
}

type rollbackRestoreStep struct {
	id    string
	state *runtimeState
}

func (s rollbackRestoreStep) ID() string {
	return s.id
}

func (s rollbackRestoreStep) Run() error {
	return nil
}

func (s rollbackRestoreStep) Rollback() error {
	if len(s.state.manifest.Entries) == 0 {
		return nil
	}

	return backup.RestoreService{}.Restore(s.state.manifest)
}

type agentInstallStep struct {
	id      string
	agent   model.AgentID
	homeDir string
	profile system.PlatformProfile
}

func (s agentInstallStep) ID() string {
	return s.id
}

func (s agentInstallStep) Run() error {
	adapter, err := agents.NewAdapter(s.agent)
	if err != nil {
		return fmt.Errorf("create adapter for %q: %w", s.agent, err)
	}

	if !adapter.SupportsAutoInstall() {
		return nil
	}

	installed, _, _, _, err := adapter.Detect(context.Background(), s.homeDir)
	if err != nil {
		return fmt.Errorf("detect agent %q: %w", s.agent, err)
	}
	if installed {
		return nil
	}

	commands, err := adapter.InstallCommand(s.profile)
	if err != nil {
		return fmt.Errorf("resolve install command for %q: %w", s.agent, err)
	}

	return runCommandSequence(commands)
}

type componentApplyStep struct {
	id             string
	component      model.ComponentID
	homeDir        string
	workspaceDir   string
	agents         []model.AgentID
	selection      model.Selection
	profile        system.PlatformProfile
	devSkillsRepo  string
}

func (s componentApplyStep) ID() string {
	return s.id
}

// resolveAdapters creates adapters for each agent ID, skipping unsupported ones.
func resolveAdapters(agentIDs []model.AgentID) []agents.Adapter {
	adapters := make([]agents.Adapter, 0, len(agentIDs))
	for _, id := range agentIDs {
		adapter, err := agents.NewAdapter(id)
		if err != nil {
			continue
		}
		adapters = append(adapters, adapter)
	}
	return adapters
}

func (s componentApplyStep) Run() error {
	adapters := resolveAdapters(s.agents)

	switch s.component {
	case model.ComponentEngram:
		if _, err := cmdLookPath("engram"); err != nil {
			// Engram not on PATH — install it.
			if s.profile.PackageManager == "brew" {
				// macOS (or Linux with Homebrew): use brew tap + brew install.
				commands, err := engram.InstallCommand(s.profile)
				if err != nil {
					return fmt.Errorf("resolve install command for component %q: %w", s.component, err)
				}
				if err := runCommandSequence(commands); err != nil {
					return err
				}
			} else {
				// Linux / Windows: download the pre-built binary from GitHub Releases.
				// No Go required — engram ships pre-built binaries.
				binaryPath, err := engramDownloadFn(s.profile)
				if err != nil {
					return fmt.Errorf("download engram binary: %w", err)
				}
				// Add the install directory to PATH so subsequent commands
				// (engram setup, engram.Inject → resolveEngramCommand) can find it.
				// On Windows this also persists the change to the user registry via PowerShell.
				binDir := filepath.Dir(binaryPath)
				if err := system.AddToUserPath(binDir); err != nil {
					// Non-fatal: warn but continue — the binary was downloaded successfully.
					fmt.Fprintf(os.Stderr, "WARNING: could not add %s to PATH: %v\n", binDir, err)
				}
			}
		}
		setupMode := engram.ParseSetupMode(os.Getenv(engram.SetupModeEnvVar))
		setupStrict := engram.ParseSetupStrict(os.Getenv(engram.SetupStrictEnvVar))
		for _, adapter := range adapters {
			if engram.ShouldAttemptSetup(setupMode, adapter.Agent()) {
				slug, _ := engram.SetupAgentSlug(adapter.Agent())
				if err := runCommand("engram", "setup", slug); err != nil {
					if setupStrict {
						return fmt.Errorf("engram setup for %q: %w", adapter.Agent(), err)
					}
				}
			}
			if _, err := engram.Inject(s.homeDir, adapter); err != nil {
				return fmt.Errorf("inject engram for %q: %w", adapter.Agent(), err)
			}
		}
		return nil
	case model.ComponentContext7:
		for _, adapter := range adapters {
			if _, err := mcp.Inject(s.homeDir, adapter); err != nil {
				return fmt.Errorf("inject context7 for %q: %w", adapter.Agent(), err)
			}
		}
		return nil
	case model.ComponentPermission:
		for _, adapter := range adapters {
			if _, err := permissions.Inject(s.homeDir, adapter); err != nil {
				return fmt.Errorf("inject permissions for %q: %w", adapter.Agent(), err)
			}
		}
		return nil
	case model.ComponentSDD:
		for _, adapter := range adapters {
			opts := sdd.InjectOptions{
				OpenCodeModelAssignments: s.selection.ModelAssignments,
				ClaudeModelAssignments:   s.selection.ClaudeModelAssignments,
				WorkspaceDir:             s.workspaceDir,
				StrictTDD:                s.selection.StrictTDD,
			}
			if _, err := sdd.Inject(s.homeDir, adapter, s.selection.SDDMode, opts); err != nil {
				return fmt.Errorf("inject sdd for %q: %w", adapter.Agent(), err)
			}
		}
		return nil
	case model.ComponentSkills:
		skillIDs := selectedSkillIDs(s.selection)
		if len(skillIDs) == 0 {
			return nil
		}
		for _, adapter := range adapters {
			if _, err := skills.Inject(s.homeDir, adapter, skillIDs); err != nil {
				return fmt.Errorf("inject skills for %q: %w", adapter.Agent(), err)
			}
		}
		return nil
	case model.ComponentTheme:
		for _, adapter := range adapters {
			if _, err := theme.Inject(s.homeDir, adapter); err != nil {
				return fmt.Errorf("inject theme for %q: %w", adapter.Agent(), err)
			}
		}
		return nil
	case model.ComponentMonday:
		if s.selection.Monday.Token == "" {
			fmt.Fprintf(os.Stderr, "WARNING: monday component selected but no --monday-token provided — skipping MCP injection\n")
			return nil
		}
		for _, adapter := range adapters {
			if _, err := monday.Inject(s.homeDir, adapter, s.selection.Monday); err != nil {
				return fmt.Errorf("inject monday for %q: %w", adapter.Agent(), err)
			}
		}
		return nil
	case model.ComponentDevSkills:
		cfg, err := devskills.ReadConfig(s.homeDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "WARNING: dev-skills config read failed: %v\n", err)
		}
		repoURL := s.devSkillsRepo
		if repoURL == "" {
			repoURL = cfg.RepoURL
		}
		if repoURL == "" {
			repoURL = devskills.DefaultRepoURL
		}
		targetDir := filepath.Join(s.homeDir, ".informa-wizard", "dev-skills")
		if _, statErr := os.Stat(targetDir); statErr != nil {
			if !os.IsNotExist(statErr) {
				return fmt.Errorf("check dev-skills dir: %w", statErr)
			}
			// Dir doesn't exist — clone it
			if err := devskills.Clone(repoURL, targetDir); err != nil {
				return fmt.Errorf("clone dev-skills repo: %w", err)
			}
		}
		skillSelections := s.selection.DevSkillSelections
		if len(skillSelections) == 0 {
			if discovered, discErr := devskills.DiscoverSkills(targetDir); discErr == nil {
				for _, ds := range discovered {
					skillSelections = append(skillSelections, ds.ID)
				}
			}
		}
		for _, adapter := range adapters {
			if _, err := devskills.InjectSkills(s.homeDir, adapter, skillSelections); err != nil {
				return fmt.Errorf("inject dev-skills for %q: %w", adapter.Agent(), err)
			}
		}
		return devskills.WriteConfig(s.homeDir, devskills.Config{RepoURL: repoURL, InstalledSkills: skillSelections})
	case model.ComponentDevAgents:
		agentCfg, err := devagents.ReadConfig(s.homeDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "WARNING: dev-agents config read failed: %v\n", err)
		}
		repoURL := agentCfg.RepoURL
		if repoURL == "" {
			repoURL = devagents.DefaultRepoURL
		}
		targetDir := filepath.Join(s.homeDir, ".informa-wizard", "dev-agents")
		if _, statErr := os.Stat(targetDir); statErr != nil {
			if !os.IsNotExist(statErr) {
				return fmt.Errorf("check dev-agents dir: %w", statErr)
			}
			// Dir doesn't exist — clone it.
			if err := devagents.Clone(repoURL, targetDir); err != nil {
				return fmt.Errorf("clone dev-agents repo: %w", err)
			}
		}
		agentSelections := s.selection.DevAgentSelections
		if len(agentSelections) == 0 {
			if discovered, discErr := devagents.DiscoverAgents(targetDir); discErr == nil {
				for _, da := range discovered {
					agentSelections = append(agentSelections, da.ID)
				}
			}
		}
		// Resolve the default model for Claude Code agents from the model assignments.
		agentModel := "sonnet"
		if m, ok := s.selection.ClaudeModelAssignments["orchestrator"]; ok {
			agentModel = string(m)
		}
		for _, adapter := range adapters {
			if _, err := devagents.InjectAgents(s.homeDir, adapter, agentSelections, agentModel, s.agents...); err != nil {
				return fmt.Errorf("inject dev-agents for %q: %w", adapter.Agent(), err)
			}
		}
		return devagents.WriteConfig(s.homeDir, devagents.Config{RepoURL: repoURL, InstalledAgents: agentSelections})
	default:
		return fmt.Errorf("component %q is not supported in install runtime", s.component)
	}
}

func ensureGoAvailableAfterInstall(profile system.PlatformProfile) error {
	if _, err := cmdLookPath("go"); err == nil {
		return nil
	}

	if profile.OS != "windows" {
		return fmt.Errorf("go was installed but is still not available in PATH")
	}

	for _, candidate := range windowsGoCandidates() {
		if candidate == "" {
			continue
		}
		if _, err := osStat(candidate); err == nil {
			binDir := filepath.Dir(candidate)
			currentPath := os.Getenv("PATH")
			if currentPath == "" {
				return osSetenv("PATH", binDir)
			}
			return osSetenv("PATH", binDir+string(os.PathListSeparator)+currentPath)
		}
	}

	return fmt.Errorf("go was installed but is still not available in PATH; restart the terminal and retry")
}

func windowsGoCandidates() []string {
	programFiles := os.Getenv("ProgramFiles")
	programFilesX86 := os.Getenv("ProgramFiles(x86)")

	return []string{
		filepath.Join(programFiles, "Go", "bin", "go.exe"),
		filepath.Join(programFilesX86, "Go", "bin", "go.exe"),
		`C:\Program Files\Go\bin\go.exe`,
	}
}

// BuildRealStagePlan creates a StagePlan with real backup, agent install, and component apply steps.
// It is used by both the CLI and TUI paths.
func BuildRealStagePlan(homeDir string, selection model.Selection, resolved planner.ResolvedPlan, profile system.PlatformProfile) (pipeline.StagePlan, error) {
	backupRoot := filepath.Join(homeDir, ".informa-wizard", "backups")
	if err := os.MkdirAll(backupRoot, 0o755); err != nil {
		return pipeline.StagePlan{}, fmt.Errorf("create backup root directory %q: %w", backupRoot, err)
	}

	runtime, err := newInstallRuntime(homeDir, selection, resolved, profile, "")
	if err != nil {
		return pipeline.StagePlan{}, err
	}

	return runtime.stagePlan(), nil
}

// ResolveInstallProfile returns the platform profile from detection, defaulting to darwin/brew.
func ResolveInstallProfile(detection system.DetectionResult) system.PlatformProfile {
	if detection.System.Profile.OS != "" {
		return detection.System.Profile
	}

	return system.PlatformProfile{
		OS:             "darwin",
		PackageManager: "brew",
		Supported:      true,
	}
}

// ggaAvailable reports whether the gga binary is reachable. gga is often
// installed to ~/.local/bin (the default for install.sh on Linux and macOS)
// or ~/bin (the default for install.sh on Windows), which may not be on PATH.
// On macOS with Homebrew, gga may be in /opt/homebrew/bin or /usr/local/bin.
// We check the filesystem directly to avoid spawning a subprocess and to work
// regardless of whether the install directory has been added to PATH.
func ggaAvailable(profile system.PlatformProfile) bool {
	// Allow test override.
	if ggaAvailableCheck != nil {
		return ggaAvailableCheck(profile)
	}
	if _, err := cmdLookPath("gga"); err == nil {
		return true
	}
	homeDir, err := osUserHomeDir()
	if err != nil {
		return false
	}
	if _, err := osStat(filepath.Join(homeDir, ".local", "bin", "gga")); err == nil {
		return true
	}
	// Check well-known Homebrew prefixes for macOS (arm64 and x86).
	// gga may be installed via brew but not yet in the shell PATH
	// (e.g. new terminal session, Rosetta environment mismatch).
	if profile.OS == "darwin" || profile.PackageManager == "brew" {
		for _, brewBin := range []string{
			"/opt/homebrew/bin/gga",
			"/usr/local/bin/gga",
		} {
			if _, err := osStat(brewBin); err == nil {
				return true
			}
		}
	}
	if profile.OS == "windows" {
		if _, err := osStat(filepath.Join(homeDir, "bin", "gga")); err == nil {
			return true
		}
	}
	return false
}

// runCommandSequence runs each command in the sequence one at a time, stopping on first error.
func runCommandSequence(commands [][]string) error {
	if len(commands) == 0 {
		return fmt.Errorf("empty command sequence")
	}

	for _, command := range commands {
		if len(command) == 0 {
			return fmt.Errorf("empty command in sequence")
		}

		if err := runCommand(command[0], command[1:]...); err != nil {
			return fmt.Errorf("run command %q: %w", strings.Join(command, " "), err)
		}
	}

	return nil
}

func executeCommand(name string, args ...string) error {
	cmd := exec.Command(name, args...)

	if streamCommandOutput {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		if len(output) > 0 {
			return fmt.Errorf("%w\noutput:\n%s", err, strings.TrimSpace(string(output)))
		}
		return err
	}

	return nil
}

// selectedSkillIDs returns the skill IDs to install. If the selection
// has explicit skills, those are used; otherwise skills are derived from the preset.
func selectedSkillIDs(selection model.Selection) []model.SkillID {
	if len(selection.Skills) > 0 {
		return selection.Skills
	}

	return skills.SkillsForPreset(selection.Preset)
}

func backupTargets(homeDir string, selection model.Selection, resolved planner.ResolvedPlan) []string {
	paths := map[string]struct{}{}
	adapters := resolveAdapters(resolved.Agents)

	// Back up entire agent config directories — any file inside these dirs
	// could be overwritten by the install pipeline. Walking the full tree
	// ensures nothing is lost regardless of which components are selected.
	for _, adapter := range adapters {
		configDir := adapter.GlobalConfigDir(homeDir)
		if configDir == "" {
			continue
		}
		for _, p := range walkDirFiles(configDir) {
			paths[p] = struct{}{}
		}
	}

	// Component-specific paths catch files outside agent config dirs
	// (e.g., VS Code settings in platform-specific locations).
	for _, component := range resolved.OrderedComponents {
		for _, path := range componentPaths(homeDir, selection, adapters, component) {
			paths[path] = struct{}{}
		}
	}

	targets := make([]string, 0, len(paths))
	for path := range paths {
		targets = append(targets, path)
	}

	return targets
}

// walkDirFiles returns all regular file paths under dir, recursively.
// If the directory does not exist or is empty, it returns nil.
func walkDirFiles(dir string) []string {
	var files []string
	_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // skip inaccessible entries
		}
		if !d.IsDir() {
			files = append(files, path)
		}
		return nil
	})
	return files
}

func componentPaths(homeDir string, selection model.Selection, adapters []agents.Adapter, component model.ComponentID) []string {
	paths := []string{}
	for _, adapter := range adapters {
		switch component {
		case model.ComponentEngram:
			switch adapter.MCPStrategy() {
			case model.StrategySeparateMCPFiles:
				paths = append(paths, adapter.MCPConfigPath(homeDir, "engram"))
			case model.StrategyMergeIntoSettings:
				if p := adapter.SettingsPath(homeDir); p != "" {
					paths = append(paths, p)
				}
			case model.StrategyMCPConfigFile:
				if p := adapter.MCPConfigPath(homeDir, "engram"); p != "" {
					paths = append(paths, p)
				}
			case model.StrategyTOMLFile:
				if p := adapter.MCPConfigPath(homeDir, "engram"); p != "" {
					paths = append(paths, p)
				}
			}
			if adapter.SystemPromptStrategy() == model.StrategyMarkdownSections {
				paths = append(paths, adapter.SystemPromptFile(homeDir))
			}
		case model.ComponentSDD:
			if adapter.SupportsSystemPrompt() {
				paths = append(paths, adapter.SystemPromptFile(homeDir))
			}
			if adapter.SupportsSlashCommands() {
				for _, command := range sdd.OpenCodeCommands() {
					paths = append(paths, filepath.Join(adapter.CommandsDir(homeDir), command.Name+".md"))
				}
			}
			if adapter.Agent() == model.AgentOpenCode {
				if p := adapter.SettingsPath(homeDir); p != "" {
					paths = append(paths, p)
				}
				paths = append(paths, filepath.Join(homeDir, ".config", "opencode", "plugins", "background-agents.ts"))
				// Shared prompt files in ~/.config/opencode/prompts/sdd/ — back these up
				// so a sync does not silently overwrite user-customized prompt content.
				// These files are only written for multi-mode (SDDModeMulti), so we only
				// include them in the path list when that mode is active. This prevents
				// false-negative verification failures in single/empty mode syncs.
				if selection.SDDMode == model.SDDModeMulti {
					promptDir := sdd.SharedPromptDir(homeDir)
					for _, phase := range sdd.SharedPromptPhases() {
						paths = append(paths, filepath.Join(promptDir, phase+".md"))
					}
				}
			}
			if adapter.SupportsSkills() {
				skillDir := adapter.SkillsDir(homeDir)
				if skillDir != "" {
					paths = append(paths,
						filepath.Join(skillDir, "_shared", "persistence-contract.md"),
						filepath.Join(skillDir, "_shared", "engram-convention.md"),
						filepath.Join(skillDir, "_shared", "openspec-convention.md"),
						filepath.Join(skillDir, "_shared", "sdd-phase-common.md"),
						filepath.Join(skillDir, "_shared", "skill-resolver.md"),
						filepath.Join(skillDir, "sdd-init", "SKILL.md"),
						filepath.Join(skillDir, "sdd-explore", "SKILL.md"),
						filepath.Join(skillDir, "sdd-propose", "SKILL.md"),
						filepath.Join(skillDir, "sdd-spec", "SKILL.md"),
						filepath.Join(skillDir, "sdd-design", "SKILL.md"),
						filepath.Join(skillDir, "sdd-tasks", "SKILL.md"),
						filepath.Join(skillDir, "sdd-apply", "SKILL.md"),
						filepath.Join(skillDir, "sdd-verify", "SKILL.md"),
						filepath.Join(skillDir, "sdd-archive", "SKILL.md"),
					)
				}
			}
		case model.ComponentSkills:
			for _, skillID := range selectedSkillIDs(selection) {
				path := skills.SkillPathForAgent(homeDir, adapter, skillID)
				if path != "" {
					paths = append(paths, path)
				}
			}
		case model.ComponentContext7:
			switch adapter.MCPStrategy() {
			case model.StrategySeparateMCPFiles:
				paths = append(paths, adapter.MCPConfigPath(homeDir, "context7"))
			case model.StrategyMergeIntoSettings:
				if p := adapter.SettingsPath(homeDir); p != "" {
					paths = append(paths, p)
				}
			case model.StrategyMCPConfigFile:
				if p := adapter.MCPConfigPath(homeDir, "context7"); p != "" {
					paths = append(paths, p)
				}
			case model.StrategyTOMLFile:
				// Codex uses TOML for Engram but Context7 is not injected via TOML.
				// No path to report — Context7 injection is skipped for TOML agents.
			}
		case model.ComponentPermission:
			if p := adapter.SettingsPath(homeDir); p != "" {
				paths = append(paths, p)
			}
		case model.ComponentTheme:
			if p := adapter.SettingsPath(homeDir); p != "" {
				paths = append(paths, p)
			}
		case model.ComponentMonday:
			switch adapter.MCPStrategy() {
			case model.StrategySeparateMCPFiles:
				paths = append(paths, adapter.MCPConfigPath(homeDir, "monday"))
			case model.StrategyMergeIntoSettings:
				if p := adapter.SettingsPath(homeDir); p != "" {
					paths = append(paths, p)
				}
			case model.StrategyMCPConfigFile:
				if p := adapter.MCPConfigPath(homeDir, "monday"); p != "" {
					paths = append(paths, p)
				}
			}
		case model.ComponentDevSkills:
			if adapter.SupportsSkills() {
				skillsDir := adapter.SkillsDir(homeDir)
				if skillsDir != "" {
					cfg, cfgErr := devskills.ReadConfig(homeDir)
					if cfgErr == nil {
						for _, skillID := range cfg.InstalledSkills {
							paths = append(paths, filepath.Join(skillsDir, skillID, "SKILL.md"))
						}
					}
				}
			}
		case model.ComponentDevAgents:
			if sai, ok := adapter.(interface {
				SupportsSubAgents() bool
				SubAgentsDir(homeDir string) string
			}); ok && sai.SupportsSubAgents() {
				agentsDir := sai.SubAgentsDir(homeDir)
				if agentsDir != "" {
					agentCfg, cfgErr := devagents.ReadConfig(homeDir)
					if cfgErr == nil {
						suffix := ".md"
						if adapter.Agent() == model.AgentVSCodeCopilot {
							suffix = ".agent.md"
						}
						for _, agentID := range agentCfg.InstalledAgents {
							paths = append(paths, filepath.Join(agentsDir, agentID+suffix))
						}
					}
				}
			}
		}
	}

	return paths
}

func runPostApplyVerification(homeDir string, selection model.Selection, resolved planner.ResolvedPlan) verify.Report {
	checks := make([]verify.Check, 0)
	adapters := resolveAdapters(resolved.Agents)

	for _, component := range resolved.OrderedComponents {
		for _, path := range componentPaths(homeDir, selection, adapters, component) {
			currentPath := path
			checks = append(checks, verify.Check{
				ID:          "verify:file:" + currentPath,
				Description: "required file exists",
				Run: func(context.Context) error {
					if _, err := os.Stat(currentPath); err != nil {
						return err
					}
					return nil
				},
			})
		}
	}

	if hasComponent(resolved.OrderedComponents, model.ComponentEngram) {
		checks = append(checks, engramHealthChecks()...)
	}
	checks = append(checks, antigravityCollisionCheck(resolved.Agents)...)

	return verify.BuildReport(verify.RunChecks(context.Background(), checks))
}

func hasComponent(components []model.ComponentID, target model.ComponentID) bool {
	for _, c := range components {
		if c == target {
			return true
		}
	}
	return false
}

func engramHealthChecks() []verify.Check {
	return []verify.Check{
		{
			ID:          "verify:engram:binary",
			Description: "engram binary on PATH (restart shell if missing)",
			Soft:        true,
			Run: func(context.Context) error {
				if err := engram.VerifyInstalled(); err != nil {
					return fmt.Errorf("%w\nIf engram was installed via `go install`, add it to PATH:\n  %s", err, engramPathGuidance(os.Getenv("SHELL")))
				}
				return nil
			},
		},
		{
			ID:          "verify:engram:version",
			Description: "engram version returns valid output",
			Soft:        true,
			Run: func(context.Context) error {
				if err := engram.VerifyInstalled(); err != nil {
					// Binary not on PATH — skip version check gracefully.
					return nil
				}
				_, err := engram.VerifyVersion()
				return err
			},
		},
	}
}

// antigravityCollisionCheck returns a soft verify check that warns the user
// when both Antigravity and Gemini CLI are selected. Both agents write to
// ~/.gemini/GEMINI.md — content is merged (not overwritten) but the user
// should be aware.
func antigravityCollisionCheck(agents []model.AgentID) []verify.Check {
	hasAntigravity := false
	hasGemini := false
	for _, id := range agents {
		if id == model.AgentAntigravity {
			hasAntigravity = true
		}
		if id == model.AgentGeminiCLI {
			hasGemini = true
		}
	}
	if !hasAntigravity || !hasGemini {
		return nil
	}
	return []verify.Check{
		{
			ID:          "verify:antigravity:rules-collision",
			Description: "Antigravity and Gemini CLI share ~/.gemini/GEMINI.md",
			Soft:        true,
			Run: func(context.Context) error {
				return fmt.Errorf(
					"both Antigravity and Gemini CLI write rules to ~/.gemini/GEMINI.md\n" +
						"Content is merged, not overwritten — rules from both agents coexist in the same file.\n" +
						"This is expected behavior. No action required unless you want to separate them manually.",
				)
			},
		},
	}
}

func engramPathGuidance(shellPath string) string {
	binDir := goInstallBinDir()
	if strings.Contains(shellPath, "fish") {
		return fmt.Sprintf("set -Ux fish_user_paths %s $fish_user_paths", binDir)
	}
	if strings.Contains(shellPath, "zsh") {
		return fmt.Sprintf("echo 'export PATH=\"%s:$PATH\"' >> ~/.zshrc && source ~/.zshrc", binDir)
	}
	if strings.Contains(shellPath, "bash") {
		return fmt.Sprintf("echo 'export PATH=\"%s:$PATH\"' >> ~/.bashrc && source ~/.bashrc", binDir)
	}
	return fmt.Sprintf("Add %s to your shell PATH and restart the terminal.", binDir)
}

// checkDependenciesStep verifies that required system dependencies are present.
// It logs warnings for missing optional deps but only fails if required deps are missing.
type checkDependenciesStep struct {
	id      string
	profile system.PlatformProfile
}

func (s checkDependenciesStep) ID() string {
	return s.id
}

func (s checkDependenciesStep) Run() error {
	// Run detection but do NOT write to stdout/stderr — this step runs
	// inside the Bubble Tea alternate screen in TUI mode, so any raw
	// output corrupts the display (see issue #2). Missing deps are
	// surfaced on the TUI complete screen and by the actual install steps
	// failing with real error messages.
	_ = system.DetectDependencies(context.Background(), s.profile)
	return nil
}

type noopStep struct {
	id string
}

func (s noopStep) ID() string {
	return s.id
}

func (s noopStep) Run() error {
	return nil
}
