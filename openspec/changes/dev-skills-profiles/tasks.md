# Tasks: dev-skills-profiles

**Change ID**: dev-skills-profiles
**Date**: 2026-04-15
**Status**: tasks

---

## Overview

Implementation covers five phases: infrastructure (model constants, new package skeleton), core package implementation (skill discovery, git ops, inject, config), TUI screen, CLI integration (pipeline steps, flag), and testing.

Dependency order between phases:

```
Phase 1 (infrastructure) → Phase 2 (core package) → Phase 3 (TUI) → Phase 4 (CLI)
                                                                     ↓
                                                              Phase 5 (tests)
```

Phase 5 tasks can be written alongside each phase but depend on the completed implementation.

---

## Phase 1: Infrastructure

### Task 1.1 — Add `ComponentDevSkills` constant

**Description**: Add the new component constant to the existing constants block.

**Files to modify**:
- `internal/model/types.go` — add `ComponentDevSkills ComponentID = "dev-skills"` to the `ComponentID` const block, after `ComponentMonday`.

**Acceptance criteria**:
- `model.ComponentDevSkills` compiles.
- Its string value is exactly `"dev-skills"`.
- Existing constants are unchanged.

**Dependencies**: none

---

### Task 1.2 — Add `DevSkillSelections` field to `Selection`

**Description**: Add the individual skill IDs field to the install-time selection struct.

**Files to modify**:
- `internal/model/selection.go` — add `DevSkillSelections []string` to the `Selection` struct, after the `Monday` field.

**Acceptance criteria**:
- `Selection.DevSkillSelections` is of type `[]string`.
- Zero value is `nil` / empty — no default skills.
- Field is NOT added to `SyncOverrides` (sync reads from `dev-skills.json`, not from overrides).

**Dependencies**: Task 1.1

---

### Task 1.3 — Register component in catalog and dependency graph

**Description**: Make `ComponentDevSkills` visible to the dependency tree screen and the install planner.

**Files to modify**:
- `internal/catalog/components.go` — append `{ID: model.ComponentDevSkills, Name: "Dev Skills", Description: "External dev skills from a shared repository"}` to `mvpComponents`.
- `internal/planner/graph.go` — add `model.ComponentDevSkills: nil` to `MVPGraph()` (no hard dependencies).

**Acceptance criteria**:
- `catalog.MVPComponents()` includes `ComponentDevSkills`.
- `planner.MVPGraph().Has(model.ComponentDevSkills)` returns `true`.
- `planner.MVPGraph().DependenciesOf(model.ComponentDevSkills)` returns `nil`.

**Dependencies**: Task 1.1

---

### Task 1.4 — Create `internal/components/devskills/` package skeleton

**Description**: Create the directory with Go file stubs that compile but contain no logic yet. This unblocks Tasks 2.x and prevents import-cycle issues during parallel development.

**Files to create**:
- `internal/components/devskills/discovery.go` — package declaration + stub `DiscoveredSkill` struct, `DiscoverSkills()`.
- `internal/components/devskills/git.go` — package declaration + stubs for `Clone()`, `Pull()`, `execCommand` var.
- `internal/components/devskills/inject.go` — package declaration + stubs for `InjectionResult`, `InjectSkills()`.
- `internal/components/devskills/config.go` — package declaration + stubs for `Config`, `ReadConfig()`, `WriteConfig()`.

**Acceptance criteria**:
- `go build ./internal/components/devskills/...` succeeds.
- All exported names match the Interface Contracts section in the spec exactly.

**Dependencies**: Task 1.1

---

## Phase 2: Core Package Implementation

### Task 2.1 — Implement skill discovery (`discovery.go`)

**Description**: Implement dynamic skill discovery by scanning the cloned repo's `skills/` directory and parsing SKILL.md frontmatter.

**Files to modify**:
- `internal/components/devskills/discovery.go`

**Implementation details**:
- `DiscoveredSkill` struct fields: `ID string`, `Name string`, `Description string`.
- `DiscoverSkills(repoDir string) ([]DiscoveredSkill, error)`:
  1. Reads entries from `filepath.Join(repoDir, "skills")` via `os.ReadDir`.
  2. If the `skills/` directory does not exist, returns an error.
  3. For each directory entry (skipping files), checks for `SKILL.md` inside the subdirectory.
  4. If `SKILL.md` exists, reads the file and parses the YAML frontmatter (between `---` delimiters) to extract `name` and `description`.
  5. If frontmatter is missing or malformed, `Name` defaults to the directory name (title-cased) and `Description` defaults to `""`.
  6. Subdirectories without `SKILL.md` are silently skipped.
  7. Returns results sorted alphabetically by `ID` (directory name).
- Frontmatter parsing: read bytes until second `---`, unmarshal as YAML into a struct with `Name string \`yaml:"name"\`` and `Description string \`yaml:"description"\``.

**Acceptance criteria**:
- Given a repo dir with 3 skill subdirectories containing SKILL.md, returns 3 `DiscoveredSkill` entries sorted alphabetically.
- Subdirectory without SKILL.md is silently skipped.
- SKILL.md with valid frontmatter populates `Name` and `Description` correctly.
- SKILL.md without frontmatter defaults `Name` to title-cased directory name.
- Missing `skills/` directory returns a non-nil error.

**Dependencies**: Task 1.4

---

### Task 2.2 — Implement git operations (`git.go`)

**Description**: Implement `Clone` and `Pull` with injectable `execCommand` following the pattern in `internal/update/upgrade/executor.go`.

**Files to modify**:
- `internal/components/devskills/git.go`

**Implementation details**:
- `var execCommand = exec.Command` at package level (same pattern as `internal/update/upgrade/executor.go`).
- `Clone(repoURL, targetDir string) error`:
  - Runs `execCommand("git", "clone", repoURL, targetDir)`.
  - Calls `cmd.CombinedOutput()`.
  - On non-zero exit: returns `fmt.Errorf("git clone failed: %s", string(out))`.
  - When the `git` binary is not found (exec.ErrNotFound or os/exec path error): returns `fmt.Errorf("git is required for dev-skills; install git and try again")`.
- `Pull(targetDir string) error`:
  - Runs `execCommand("git", "-C", targetDir, "pull")`.
  - On non-zero exit: returns `fmt.Errorf("git pull failed: %s", string(out))`.
  - Same git-not-found guard as `Clone`.

**Acceptance criteria**:
- Successful command (exit 0) → returns `nil`.
- Non-zero exit → error message contains the git stderr text per spec error table.
- `execCommand` can be replaced in tests without modifying production code.

**Dependencies**: Task 1.4

---

### Task 2.3 — Implement config read/write (`config.go`)

**Description**: Implement JSON config file management for `~/.informa-wizard/dev-skills.json`.

**Files to modify**:
- `internal/components/devskills/config.go`

**Implementation details**:
- `Config` struct: `RepoURL string \`json:"repo_url"\`` and `InstalledSkills []string \`json:"installed_skills"\``.
- `DefaultRepoURL = "https://gitlab.informa.tools/ai/skills/dev-skills.git"` exported constant.
- `ReadConfig(homeDir string) (Config, error)`:
  - Path: `filepath.Join(homeDir, ".informa-wizard", "dev-skills.json")`.
  - If file does not exist (`os.IsNotExist`): returns `(Config{}, nil)` — callers detect absence via empty `RepoURL`.
  - If file exists but JSON is malformed: returns `(Config{}, fmt.Errorf("dev-skills.json: %w", err))`.
  - On success: returns the parsed `Config`.
- `WriteConfig(homeDir string, cfg Config) error`:
  - Serializes `cfg` to JSON with `json.MarshalIndent` (2-space indent for readability).
  - Writes via `filemerge.WriteFileAtomic` (write-to-temp-then-rename) to prevent partial writes.
  - Creates parent directory with `os.MkdirAll` before writing.

**Acceptance criteria**:
- Missing file → `(Config{}, nil)`, zero-value `Config`.
- Malformed JSON → non-nil error containing `"dev-skills.json:"`.
- Round-trip: `WriteConfig` then `ReadConfig` returns the original `Config`.
- Write is atomic (uses `filemerge.WriteFileAtomic`).

**Dependencies**: Task 1.4

---

### Task 2.4 — Implement skill injection (`inject.go`)

**Description**: Implement `InjectSkills` — reads `SKILL.md` files from the cloned repo on disk and writes them to agent skill dirs via `filemerge.WriteFileAtomic`.

**Files to modify**:
- `internal/components/devskills/inject.go`

**Implementation details**:
- `InjectionResult` struct: `Changed bool`, `Files []string`.
- `InjectSkills(homeDir string, adapter agents.Adapter, skillIDs []string) (InjectionResult, error)`:
  1. If `!adapter.SupportsSkills()`: return `(InjectionResult{}, nil)`.
  2. `repoDir := filepath.Join(homeDir, ".informa-wizard", "dev-skills")`.
  3. For each `skillID` in `skillIDs`:
     - sourcePath: `filepath.Join(repoDir, "skills", skillID, "SKILL.md")`.
     - Read `sourcePath` with `os.ReadFile`.
     - If file not found: `log.Printf("devskills: skipping %q — SKILL.md not found in repo", skillID)`, continue.
     - destPath: `filepath.Join(adapter.SkillsDir(homeDir), skillID, "SKILL.md")`.
     - `filemerge.WriteFileAtomic(destPath, content, 0o644)`.
     - On write error: return `(InjectionResult{}, fmt.Errorf("skill %s: write failed: %w", skillID, err))`.
     - Accumulate `changed` flag and append `destPath` to `Files`.
  4. Return `InjectionResult{Changed: changed, Files: files}, nil`.

Note: There is no profile lookup step. Each `skillID` maps directly to a subdirectory name under `skills/`.

**Acceptance criteria**:
- Adapter with `SupportsSkills() == false` → returns empty result, no files touched.
- Missing `SKILL.md` in repo → logs warning, skips that skill, continues injection, no error returned.
- Successful injection → correct destination paths; `InjectionResult.Files` has the right length.
- Write failure → immediate error return with message matching `"skill <id>: write failed: ..."`.

**Dependencies**: Task 1.4

---

## Phase 3: TUI

### Task 3.1 — Add `ScreenDevSkillPicker` to the screen iota

**Description**: Register the new screen constant in the `Model`'s `Screen` iota.

**Files to modify**:
- `internal/tui/model.go` — add `ScreenDevSkillPicker` to the `Screen` iota, after `ScreenSkillPicker` and before `ScreenReview`.

**Acceptance criteria**:
- `ScreenDevSkillPicker` compiles.
- The iota order does not shift existing `ScreenReview` or later constants (insert in the right position).

**Dependencies**: Tasks 1.1, 1.2

---

### Task 3.2 — Add dev skill picker state to `Model`

**Description**: Add the state fields needed to drive the new screen.

**Files to modify**:
- `internal/tui/model.go` — add `DevSkills []devskills.DiscoveredSkill` (discovered skills from repo), `DevSkillChecked []bool` (checked state per skill), and `DevSkillCursor int` (cursor row) fields to the `Model` struct, grouped with `SkillPicker`.

**Acceptance criteria**:
- `Model.DevSkills`, `Model.DevSkillChecked`, and `Model.DevSkillCursor` are accessible.
- No existing fields renamed or removed.

**Dependencies**: Task 3.1

---

### Task 3.3 — Implement `ScreenDevSkillPicker` render function

**Description**: Create the new screen file with its `RenderDevSkillPicker` function.

**Files to create**:
- `internal/tui/screens/dev_skill_picker.go`

**Implementation details**:
- `RenderDevSkillPicker(skills []devskills.DiscoveredSkill, checked []bool, cursor int) string`:
  - Title: "Select Dev Skills" (uses `styles.TitleStyle`).
  - Subtitle: "Toggle skills with space. Press enter to confirm." (uses `styles.SubtextStyle`).
  - Iterates the `skills` slice and renders each as a checkbox row.
  - Each row format: `[x] <Name>  <Description>` — checkbox indicator, skill name left-aligned, description to the right.
  - Uses `renderCheckbox(label, checked, focused)` from `screens/common.go` (same function used by `RenderSkillPicker`).
  - The `label` passed to `renderCheckbox` includes both the name and the description.
  - Help line: `"j/k: navigate • space: toggle • enter: confirm • esc: back"` (uses `styles.HelpStyle`).
  - No skill is pre-selected when `checked` is all false.
- The function receives discovered skills as a parameter — it does NOT call `DiscoverSkills` itself (pure render function).

**Acceptance criteria**:
- Given 6 skills with all `checked` false, renders all 6 rows with `[ ]` indicators.
- Given `checked[0] == true`, the first skill row shows `[x]`.
- Skill names and descriptions appear on each row.

**Dependencies**: Tasks 2.1, 3.1

---

### Task 3.4 — Wire `ScreenDevSkillPicker` into router and gate

**Description**: Add routing entries and the gate method that controls when the screen appears.

**Files to modify**:
- `internal/tui/router.go`:
  - Change `ScreenSkillPicker.Forward` from `ScreenReview` to `ScreenDevSkillPicker`.
  - Add `ScreenDevSkillPicker: {Forward: ScreenReview, Backward: ScreenSkillPicker}`.
  - Note: the actual forward/backward for the DevSkillPicker screen is dynamic (depends on Monday and SkillPicker presence), so the router entries serve as fallback references. The live navigation uses `goToReviewOrMonday` and `goBack` logic in model.go.
- `internal/tui/model.go`:
  - Add `shouldShowDevSkillPickerScreen() bool` method: returns `hasSelectedComponent(m.Selection.Components, model.ComponentDevSkills)`.

**Acceptance criteria**:
- `shouldShowDevSkillPickerScreen()` returns `true` iff `ComponentDevSkills` is in `Selection.Components`.
- `linearRoutes[ScreenSkillPicker].Forward == ScreenDevSkillPicker`.

**Dependencies**: Tasks 3.1, 3.2, Task 1.1

---

### Task 3.5 — Wire navigation into `Update`, `View`, and `goBack`

**Description**: Integrate `ScreenDevSkillPicker` into the full TUI event loop.

**Files to modify**:
- `internal/tui/model.go`:

  **In `View()`**:
  - Add a `case ScreenDevSkillPicker:` branch that calls `screens.RenderDevSkillPicker(m.DevSkills, m.DevSkillChecked, m.DevSkillCursor)`.

  **In `Update()` keyboard handler**:
  - Add key handling for `ScreenDevSkillPicker`: Up/k → decrement cursor, Down/j → increment cursor, Space → toggle `m.DevSkillChecked[cursor]`, Enter → collect IDs of checked skills into `m.Selection.DevSkillSelections`, then `m.goToReviewOrMonday()`, Esc → `m.goBack()`.

  **In `goToReviewOrMonday()`**:
  - Before checking `shouldShowMondayScreen`, insert: if `m.shouldShowDevSkillPickerScreen() && m.Screen != ScreenDevSkillPicker` → `m.initDevSkillPicker(); m.setScreen(ScreenDevSkillPicker); return`.
  - This ensures the DevSkillPicker screen intercepts the flow when ComponentDevSkills is selected.

  **In `goBack()`**:
  - When current screen is `ScreenDevSkillPicker`: navigate back to `ScreenSkillPicker` if `shouldShowSkillPickerScreen()`, else to `ScreenDependencyTree`.
  - When current screen is `ScreenMonday` or `ScreenReview` and `shouldShowDevSkillPickerScreen()`: the backward route should resolve to `ScreenDevSkillPicker` if navigating back past it; handle this by inserting logic before existing Monday/Review back cases.

  **In forward navigation from `ScreenDependencyTree` and `ScreenSkillPicker`**:
  - Before calling `m.goToReviewOrMonday()`, the existing code already calls `m.goToReviewOrMonday()`. The modification to `goToReviewOrMonday()` above will automatically intercept when `ComponentDevSkills` is selected — no further changes needed for forward navigation from those screens.

**Acceptance criteria**:
- When `ComponentDevSkills` is selected: advancing from SkillPicker (or DependencyTree when SkillPicker absent) shows `ScreenDevSkillPicker`.
- When `ComponentDevSkills` is NOT selected: advancing skips `ScreenDevSkillPicker` entirely.
- Enter on `ScreenDevSkillPicker` stores checked skill IDs to `Selection.DevSkillSelections` and advances.
- Enter with empty selection is allowed and sets `Selection.DevSkillSelections = []`.
- Esc on `ScreenDevSkillPicker` returns to the correct previous screen.

**Dependencies**: Tasks 3.2, 3.3, 3.4

---

## Phase 4: CLI Integration

### Task 4.1 — Add `--dev-skills-repo` flag to `InstallFlags`

**Description**: Add the optional CLI flag to the install command.

**Files to modify**:
- `internal/cli/install.go`:
  - Add `DevSkillsRepo string` field to `InstallFlags`.
  - Register `fs.StringVar(&opts.DevSkillsRepo, "dev-skills-repo", "", "HTTPS git URL for dev-skills repository (overrides default)")` in `ParseInstallFlags`.

**Acceptance criteria**:
- `wizard install --dev-skills-repo https://github.com/myorg/dev-skills.git` parses without error.
- Omitting the flag leaves `DevSkillsRepo` as empty string.
- `NormalizeInstallFlags` propagates the value to the `Selection` or makes it available to the apply step (see Task 4.2).

**Dependencies**: Task 1.1

---

### Task 4.2 — Add `ComponentDevSkills` apply step in `run.go`

**Description**: Add the install-pipeline step that clones the repo (when needed) and injects selected skills.

**Files to modify**:
- `internal/cli/run.go`:
  - Import `devskills` package.
  - Add `case model.ComponentDevSkills:` to `componentApplyStep.Run()` switch, after the `ComponentMonday` case and before `default`.
  - The flag value `InstallFlags.DevSkillsRepo` must be threaded from `NormalizeInstallFlags` → `Selection` or stored alongside the selection. Preferred approach: add `DevSkillsRepo string` to `model.Selection` as a transient field (not serialized, not in SyncOverrides), or pass it via a dedicated apply step field. Use the dedicated step field approach to avoid polluting `model.Selection` — add `devSkillsRepo string` to `componentApplyStep` and populate it in `installRuntime.stagePlan()`.

**Apply step logic** (per spec §3):
  1. `cfg, _ := devskills.ReadConfig(s.homeDir)` — if absent, use zero-value `Config` with empty `RepoURL`.
  2. Resolve effective repo URL: `repoURL := s.devSkillsRepo; if repoURL == "" { repoURL = cfg.RepoURL }; if repoURL == "" { repoURL = devskills.DefaultRepoURL }`.
  3. `targetDir := filepath.Join(s.homeDir, ".informa-wizard", "dev-skills")`.
  4. If `targetDir` does not exist → call `devskills.Clone(repoURL, targetDir)`. Return error on failure.
  5. Call `devskills.InjectSkills(s.homeDir, adapter, s.selection.DevSkillSelections)` for each adapter. Return error on failure.
  6. `devskills.WriteConfig(s.homeDir, devskills.Config{RepoURL: repoURL, InstalledSkills: s.selection.DevSkillSelections})`. Return error on failure.

**Files to modify**:
- `internal/cli/run.go` — add the case as described.
- `internal/cli/install.go` — thread `DevSkillsRepo` from `InstallFlags` into a custom field on `componentApplyStep` (requires updating `newInstallRuntime` / `stagePlan()` to pass the flag value through).

**Acceptance criteria**:
- First install (no target dir): `Clone` is called once with the correct URL.
- Re-install (target dir exists): `Clone` is NOT called.
- Empty `DevSkillSelections`: clone still runs if needed; `InjectSkills` called with empty slice; config written with `"installed_skills": []`.
- Clone failure → step returns error, no injection or config write occurs.

**Dependencies**: Tasks 1.1, 1.2, 2.2, 2.3, 2.4, 4.1

---

### Task 4.3 — Add `ComponentDevSkills` sync step in `sync.go`

**Description**: Add the sync-pipeline step that pulls the repo and re-injects all installed skills.

**Files to modify**:
- `internal/cli/sync.go`:
  - Import `devskills` package.
  - Add `case model.ComponentDevSkills:` to `componentSyncStep.Run()` switch, after `ComponentMonday` and before `default`.

**Sync step logic** (per spec §4):
  1. `cfg, err := devskills.ReadConfig(s.homeDir)` — if file absent (`cfg.RepoURL == ""`): return `nil` (no-op).
  2. If `len(cfg.InstalledSkills) == 0`: return `nil` (no-op).
  3. `targetDir := filepath.Join(s.homeDir, ".informa-wizard", "dev-skills")`.
  4. `devskills.Pull(targetDir)` — return error on failure.
  5. For each detected adapter: `devskills.InjectSkills(s.homeDir, adapter, cfg.InstalledSkills)`. Return error on failure. Accumulate `filesChanged` via `s.countChanged(boolToInt(res.Changed))`.

**Acceptance criteria**:
- Missing `dev-skills.json` → no-op, step returns success.
- `installed_skills: []` → no-op, step returns success.
- Happy path: `Pull` called, then `InjectSkills` for all adapters; `filesChanged` incremented.
- Pull failure → step returns error, no injection attempted.

**Dependencies**: Tasks 2.2, 2.3, 2.4, Task 1.1

---

## Phase 5: Testing

### Task 5.1 — Unit tests for skill discovery

**File to create**: `internal/components/devskills/discovery_test.go`

**Tests**:
- `TestDiscoverSkills_HappyPath` — create temp repo dir with 3 skill subdirectories each containing SKILL.md with frontmatter; verify 3 results sorted alphabetically by ID.
- `TestDiscoverSkills_SkipsSubdirWithoutSkillMD` — one subdirectory has no SKILL.md; verify it is excluded from results.
- `TestDiscoverSkills_ParsesFrontmatter` — SKILL.md with `name: Java Development` and `description: ...`; verify `Name` and `Description` are populated.
- `TestDiscoverSkills_MissingFrontmatterDefaultsToDirectoryName` — SKILL.md with no frontmatter; verify `Name` defaults to title-cased directory name.
- `TestDiscoverSkills_EmptySkillsDir` — empty `skills/` directory returns empty slice, no error.
- `TestDiscoverSkills_MissingSkillsDir` — `skills/` directory does not exist; returns non-nil error.
- `TestDiscoverSkills_SortedAlphabetically` — verify results are sorted by ID regardless of filesystem order.

**Dependencies**: Task 2.1

---

### Task 5.2 — Unit tests for config read/write

**File to create**: `internal/components/devskills/config_test.go`

**Tests**:
- `TestReadConfig_MissingFile` — returns `(Config{}, nil)` with empty `RepoURL`.
- `TestReadConfig_MalformedJSON` — returns non-nil error containing `"dev-skills.json:"`.
- `TestReadConfig_ValidFile` — round-trip read of a known config with `installed_skills`.
- `TestWriteConfig_AtomicWrite` — writes config, verifies file content is valid JSON with `installed_skills` field.
- `TestWriteConfig_RoundTrip` — `WriteConfig` then `ReadConfig` returns original struct with `InstalledSkills` intact.
- `TestReadConfig_DefaultRepoURL` — `DefaultRepoURL` equals the expected string literal.

**Dependencies**: Task 2.3

---

### Task 5.3 — Unit tests for git operations

**File to create**: `internal/components/devskills/git_test.go`

**Tests** (use `execCommand` injection — set to a fake command function):
- `TestClone_Success` — mock `git clone` exits 0 → `Clone` returns nil.
- `TestClone_Failure` — mock exits non-zero with stderr text → error contains that text and the prefix `"git clone failed:"`.
- `TestPull_Success` — mock `git pull` exits 0 → `Pull` returns nil.
- `TestPull_Failure` — mock exits non-zero → error contains the prefix `"git pull failed:"`.

**Test helper**: Create a helper that writes a shell script (or Go test binary) to a temp dir and sets `execCommand` to invoke it, restoring after the test. Follow the pattern used in `internal/update/upgrade/` tests.

**Dependencies**: Task 2.2

---

### Task 5.4 — Unit tests for `InjectSkills`

**File to create**: `internal/components/devskills/inject_test.go`

**Tests**:
- `TestInjectSkills_AdapterNoSkills` — adapter with `SupportsSkills() == false` → returns empty result, no files written.
- `TestInjectSkills_SingleSkill` — creates repo structure in `t.TempDir()`, injects `"java-development"`, verifies `SKILL.md` is written to correct adapter path.
- `TestInjectSkills_MissingSkillDirectoryWarnsAndContinues` — partial repo (skill dir absent for one ID), verifies present skills are written and no error is returned.
- `TestInjectSkills_MultipleSkills` — inject `["java-development", "java-testing", "human-documentation"]`, verify three files written.
- `TestInjectSkills_Idempotent` — inject twice, second result has `Changed == false`.
- `TestInjectSkills_EmptySkillList` — returns empty `InjectionResult{}`, no files written.

**Test setup**: Use `claude.NewAdapter()` or `opencode.NewAdapter()` (same adapters used in `skills/inject_test.go`). Build a fake repo directory under `t.TempDir()` with the expected `skills/<id>/SKILL.md` structure.

**Dependencies**: Task 2.4

---

### Task 5.5 — TUI screen render tests

**File to create**: `internal/tui/screens/dev_skill_picker_test.go`

**Tests**:
- `TestRenderDevSkillPicker_AllSkillsShown` — output contains all skill names passed to the function.
- `TestRenderDevSkillPicker_NonePreSelected` — with all `checked` false, output does not contain `[x]`.
- `TestRenderDevSkillPicker_SelectedSkillMarked` — with `checked[0] == true`, the first skill row contains `[x]`.
- `TestRenderDevSkillPicker_DescriptionsShown` — output contains the description text for each skill.

**Dependencies**: Task 3.3

---

### Task 5.6 — TUI gate method tests

**File to create or append**: `internal/tui/model_test.go` (add to existing file)

**Tests**:
- `TestShouldShowDevSkillPickerScreen_WhenSelected` — build a `Model` with `ComponentDevSkills` in components, assert `shouldShowDevSkillPickerScreen() == true`.
- `TestShouldShowDevSkillPickerScreen_WhenNotSelected` — assert `false` when component absent.
- `TestDevSkillPickerScreenSkippedInFlow` — simulate pressing Enter through the flow when `ComponentDevSkills` is NOT selected; assert `ScreenDevSkillPicker` is never reached.
- `TestDevSkillPickerScreenShownInFlow` — simulate pressing Enter through the flow when `ComponentDevSkills` IS selected; assert `ScreenDevSkillPicker` is shown after SkillPicker (or DependencyTree).

**Dependencies**: Tasks 3.4, 3.5

---

### Task 5.7 — Integration tests for apply and sync steps

**File to create**: `internal/cli/run_devskills_test.go`

**Tests** (use `t.TempDir()` for homeDir; mock `devskills.execCommand` via package-level var):
- `TestComponentApplyStep_DevSkills_FirstInstall` — target dir absent → Clone called, skills injected, config written with correct `installed_skills`.
- `TestComponentApplyStep_DevSkills_ReInstall` — target dir present → Clone NOT called.
- `TestComponentApplyStep_DevSkills_EmptySelections` — empty `DevSkillSelections` → clone runs, config written with `"installed_skills": []`.
- `TestComponentApplyStep_DevSkills_CloneFailure` — Clone returns error → step returns error, config not written.

**File to create**: `internal/cli/sync_devskills_test.go`

**Tests**:
- `TestComponentSyncStep_DevSkills_NoConfig` — missing `dev-skills.json` → no-op, success.
- `TestComponentSyncStep_DevSkills_EmptySkills` — config exists but `installed_skills` empty → no-op, success.
- `TestComponentSyncStep_DevSkills_HappyPath` — config with `installed_skills: ["java-development"]` → Pull called, InjectSkills called, filesChanged incremented.
- `TestComponentSyncStep_DevSkills_PullFailure` — Pull returns error → step returns error, no injection.

**Dependencies**: Tasks 4.2, 4.3

---

## Completion Checklist

- [ ] 1.1 Add `ComponentDevSkills` constant
- [ ] 1.2 Add `DevSkillSelections` field to `Selection`
- [ ] 1.3 Register component in catalog and dependency graph
- [ ] 1.4 Create `devskills/` package skeleton
- [ ] 2.1 Implement skill discovery
- [ ] 2.2 Implement git operations
- [ ] 2.3 Implement config read/write
- [ ] 2.4 Implement skill injection
- [ ] 3.1 Add `ScreenDevSkillPicker` to screen iota
- [ ] 3.2 Add dev skill picker state fields to `Model`
- [ ] 3.3 Implement `RenderDevSkillPicker`
- [ ] 3.4 Wire routing and gate method
- [ ] 3.5 Wire navigation into `Update`, `View`, and `goBack`
- [ ] 4.1 Add `--dev-skills-repo` flag
- [ ] 4.2 Add apply step in `run.go`
- [ ] 4.3 Add sync step in `sync.go`
- [ ] 5.1 Unit tests — skill discovery
- [ ] 5.2 Unit tests — config read/write
- [ ] 5.3 Unit tests — git operations
- [ ] 5.4 Unit tests — InjectSkills
- [ ] 5.5 TUI screen render tests
- [ ] 5.6 TUI gate method tests
- [ ] 5.7 Integration tests — apply and sync steps
