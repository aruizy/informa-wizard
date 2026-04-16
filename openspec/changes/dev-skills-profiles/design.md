# Design: dev-skills-profiles

**Change ID**: dev-skills-profiles
**Date**: 2026-04-15
**Status**: design

---

## 1. Package Structure

### `internal/components/devskills/`

| File | Exports | Responsibility |
|---|---|---|
| `discovery.go` | `DiscoveredSkill`, `DiscoverSkills(repoDir string) ([]DiscoveredSkill, error)` | Scans cloned repo `skills/` dir; parses SKILL.md frontmatter for name + description |
| `git.go` | `Clone(repoURL, targetDir string) error`, `Pull(targetDir string) error`, `var execCommand` | Thin os/exec wrappers; injectable for tests |
| `inject.go` | `InjectionResult`, `InjectSkills(homeDir string, adapter agents.Adapter, skillIDs []string) (InjectionResult, error)` | Reads SKILL.md from cloned repo; writes via filemerge |
| `config.go` | `Config`, `ErrConfigNotFound`, `ReadConfig(homeDir string) (Config, error)`, `WriteConfig(homeDir string, cfg Config) error` | JSON marshal/unmarshal; atomic writes |

The package has NO sub-packages. It imports `agents`, `filemerge`, and standard library only (plus a YAML frontmatter parser for `discovery.go`).

---

## 2. Data Types

### `DiscoveredSkill` (discovery.go)

```
type DiscoveredSkill struct {
    ID          string // directory name under skills/ (e.g., "java-development")
    Name        string // human-readable name from SKILL.md frontmatter
    Description string // one-line description from SKILL.md frontmatter
}
```

There is no hardcoded registry. Skills are discovered dynamically by scanning the cloned repo.

`DiscoverSkills(repoDir string) ([]DiscoveredSkill, error)`:
1. Reads `<repoDir>/skills/` directory entries.
2. For each subdirectory, checks for `SKILL.md` and parses its YAML frontmatter.
3. Returns results sorted alphabetically by `ID`.
4. Subdirectories without `SKILL.md` are silently skipped.
5. If the `skills/` directory does not exist, returns an error.

Frontmatter parsing: The function reads the leading `---` delimited YAML block from `SKILL.md` and extracts `name` and `description` fields. If frontmatter is missing or malformed, `Name` defaults to the directory name (title-cased) and `Description` defaults to an empty string.

### `Config` (config.go)

```
type Config struct {
    RepoURL        string   `json:"repo_url"`
    InstalledSkills []string `json:"installed_skills"`
}

const DefaultRepoURL = "https://gitlab.informa.tools/ai/skills/dev-skills.git"
var ErrConfigNotFound = errors.New("dev-skills.json not found")
```

`ReadConfig` returns `(Config{}, nil)` — not `ErrConfigNotFound` as an error — when the file is absent. The caller detects absence via `cfg.RepoURL == ""`. `ErrConfigNotFound` is available as a sentinel for future use but is not returned by `ReadConfig` in Phase 1 (zero-value detection is simpler and consistent with the spec).

### `InjectionResult` (inject.go)

```
type InjectionResult struct {
    Changed bool
    Files   []string // absolute paths written
}
```

Mirrors `skills.InjectionResult` shape but without the `Skipped` field (missing skills are warned and skipped silently, not tracked).

### Model additions

- `internal/model/types.go`: append `ComponentDevSkills ComponentID = "dev-skills"` to the `ComponentID` const block.
- `internal/model/selection.go`: append `DevSkillSelections []string` to `Selection`. Field stores individual skill IDs (directory names), NOT profile IDs. Field is NOT added to `SyncOverrides` — sync always reads from `dev-skills.json`.

---

## 3. TUI Integration

### Screen constant placement

In `internal/tui/model.go` Screen iota, insert `ScreenDevSkillPicker` immediately after `ScreenSkillPicker`:

```
ScreenSkillPicker
ScreenDevSkillPicker   // NEW
ScreenReview
```

### Gate method

```go
func (m Model) shouldShowDevSkillPickerScreen() bool {
    return hasSelectedComponent(m.Selection.Components, model.ComponentDevSkills)
}
```

This mirrors the pattern of `shouldShowSkillPickerScreen()`, `shouldShowMondayScreen()`, etc.

### State on Model

Add these fields to `Model`:

```
DevSkills       []devskills.DiscoveredSkill // populated by DiscoverSkills() when screen initializes
DevSkillChecked []bool                      // checked state per skill index (len == len(DevSkills))
DevSkillCursor  int                         // cursor position on the skill list
```

`DevSkills` is populated by calling `devskills.DiscoverSkills(repoDir)` during screen initialization (when the TUI transitions to `ScreenDevSkillPicker`). `DevSkillChecked` is a `[]bool` slice matching the length of `DevSkills`. On Enter, the IDs of checked skills are written to `Selection.DevSkillSelections`.

### Navigation wiring

All call sites that currently call `m.goToReviewOrMonday()` at the end of `ScreenSkillPicker` confirm must first check `shouldShowDevSkillPickerScreen()`:

```go
// After SkillPicker confirm (and anywhere goToReviewOrMonday was called from SkillPicker):
if m.shouldShowDevSkillPickerScreen() {
    m.initDevSkillPicker()
    m.setScreen(ScreenDevSkillPicker)
} else {
    m.goToReviewOrMonday()
}
```

`initDevSkillPicker()`:
1. Calls `devskills.DiscoverSkills(repoDir)` where `repoDir = filepath.Join(homeDir, ".informa-wizard", "dev-skills")`.
2. Stores the result in `m.DevSkills`.
3. Initializes `m.DevSkillChecked = make([]bool, len(m.DevSkills))` (all false).
4. If `DiscoverSkills` returns an error (e.g., repo not cloned yet), sets `m.DevSkills` to an empty slice.

Call sites affected in `model.go` (all `case ScreenSkillPicker` confirm paths):
1. The "Continue" button handler in the main `ScreenSkillPicker` Enter case.
2. The `goBack()` function does NOT need changes — back from `ScreenDevSkillPicker` returns to `ScreenSkillPicker` (or `ScreenDependencyTree` if SkillPicker was absent).

**`goBack()` extension** — add a new block after the existing `ScreenSkillPicker` block:

```go
if m.Screen == ScreenDevSkillPicker {
    if m.shouldShowSkillPickerScreen() {
        m.setScreen(ScreenSkillPicker)
    } else {
        m.setScreen(ScreenDependencyTree)
    }
    return m
}
```

**`View()` extension** — add `case ScreenDevSkillPicker:` that calls `screens.RenderDevSkillPicker(...)`.

**`maxCursor()` extension** — add `case ScreenDevSkillPicker:` returning `len(m.DevSkills) + 1` (skills + one "Continue" button; Esc handled directly, no "Back" button row needed since existing goBack handles Esc).

**Cursor reset** — `setScreen()` already resets `m.Cursor = 0`, so no special initialization is needed beyond `initDevSkillPicker()` which zeroes the `[]bool` slice.

### New screen file

`internal/tui/screens/dev_skill_picker.go` — package `screens`.

Exported functions:

```go
func RenderDevSkillPicker(skills []devskills.DiscoveredSkill, checked []bool, cursor int) string
```

Render format per row (mirroring `renderCheckbox` used in `skill_picker.go`):

```
  [x] Java Development       Java coding patterns and best practices
  [ ] Java Testing            JUnit and testing patterns for Java
  [ ] InformaDS Development   React component development with InformaDS
```

Each row shows the skill name (from SKILL.md frontmatter) left-aligned, and the description to the right. Uses the same `renderCheckbox` helper (already in the `screens` package). Title: "Select Dev Skills". Help line: `j/k: navigate • space: toggle • enter: confirm • esc: back`.

---

## 4. Pipeline Integration

### Install pipeline (`internal/cli/run.go`)

Add a new case to `componentApplyStep.Run()`:

```go
case model.ComponentDevSkills:
    return applyDevSkills(s.homeDir, s.selection, adapters, s.flags.DevSkillsRepo)
```

`applyDevSkills` is a package-level function in `run.go` (following the existing pattern for `ComponentMonday`). It orchestrates:

1. `devskills.ReadConfig(homeDir)` → resolve `repoURL` (flag overrides config, config overrides default).
2. `os.Stat(targetDir)` → if absent, call `devskills.Clone(repoURL, targetDir)`.
3. For each adapter: `devskills.InjectSkills(homeDir, adapter, selection.DevSkillSelections)`.
4. `devskills.WriteConfig(homeDir, cfg)` with `InstalledSkills` set to `selection.DevSkillSelections`.

`targetDir` = `filepath.Join(homeDir, ".informa-wizard", "dev-skills")`.

`componentPaths()` gets a new `case model.ComponentDevSkills:` that returns the paths of all SKILL.md files that would be written (for dry-run/review display). It iterates `selection.DevSkillSelections` directly (each entry is a skill ID) and constructs `adapter.SkillsDir(homeDir)/<skillID>/SKILL.md` for each one. No profile lookup is needed.

### Dependency graph (`internal/planner/graph.go`)

Add to `MVPGraph()`:

```go
model.ComponentDevSkills: nil,  // no hard dependencies
```

No soft-ordering pair needed — DevSkills writes to its own skill subdirectories and does not conflict with `ComponentSkills` (different skill IDs).

### `InstallFlags` (flag definition file in `internal/cli/`)

Add `DevSkillsRepo string` to the `InstallFlags` struct. Wire `--dev-skills-repo` flag in the `wizard install` cobra command setup.

### Sync pipeline (`internal/cli/sync.go`)

Add a new case to `componentSyncStep.Run()`:

```go
case model.ComponentDevSkills:
    return syncDevSkills(s.homeDir, s.selection, adapters, s.filesChanged)
```

`syncDevSkills`:

1. `devskills.ReadConfig(homeDir)` → if `cfg.RepoURL == ""` or `len(cfg.InstalledSkills) == 0`, return nil (no-op).
2. `devskills.Pull(targetDir)`.
3. For each adapter: `devskills.InjectSkills(homeDir, adapter, cfg.InstalledSkills)` → `s.countChanged(boolToInt(result.Changed))`.

---

## 5. Git Operations

### `git.go` — injectable exec pattern

```go
// execCommand is the exec.Command constructor. Overridden in tests.
var execCommand = exec.Command

func Clone(repoURL, targetDir string) error {
    cmd := execCommand("git", "clone", repoURL, targetDir)
    out, err := cmd.CombinedOutput()
    if err != nil {
        return fmt.Errorf("git clone failed: %s", strings.TrimSpace(string(out)))
    }
    return nil
}

func Pull(targetDir string) error {
    cmd := execCommand("git", "-C", targetDir, "pull")
    out, err := cmd.CombinedOutput()
    if err != nil {
        return fmt.Errorf("git pull failed: %s", strings.TrimSpace(string(out)))
    }
    return nil
}
```

`exec.LookPath("git")` is NOT called inside these functions. The "git not on PATH" case surfaces naturally as a `exec.ErrNotFound`-wrapped error from `cmd.CombinedOutput()`. The pipeline step wraps it with the actionable message: `"git is required for dev-skills; install git and try again"`.

This mirrors `internal/update/upgrade/strategy.go`'s `execCommand` pattern exactly.

---

## 6. File Layout on Disk

```
~/.informa-wizard/
├── dev-skills.json              ← Config: repo_url + installed_skills
└── dev-skills/                  ← Cloned git repository (targetDir)
    └── skills/
        ├── java-development/
        │   └── SKILL.md         ← has YAML frontmatter with name + description
        ├── java-testing/
        │   └── SKILL.md
        ├── informads-development/
        │   └── SKILL.md
        ├── human-documentation/
        │   └── SKILL.md
        ├── context0-instructions/
        │   └── SKILL.md
        └── skill-creator/
            └── SKILL.md
```

`DiscoverSkills` reads: all subdirectories of `<repoDir>/skills/`, parsing `SKILL.md` frontmatter in each.

`InjectSkills` reads: `filepath.Join(homeDir, ".informa-wizard", "dev-skills", "skills", skillID, "SKILL.md")`

`InjectSkills` writes to each adapter: `adapter.SkillsDir(homeDir)/<skillID>/SKILL.md`

Example for Claude Code adapter:
```
~/.claude/skills/java-development/SKILL.md
~/.claude/skills/java-testing/SKILL.md
```

No other files from the cloned repo are read or written in Phase 1.

---

## 7. Data Flow Summary

```
wizard install
  └─ TUI: DependencyTree selects ComponentDevSkills
       └─ TUI: ScreenSkillPicker (if custom) → confirms Skills
            └─ TUI: ScreenDevSkillPicker
                 DiscoverSkills(repoDir) → populates checkbox list
                 User selects individual skills
                 Selection.DevSkillSelections = ["java-development", "java-testing"]
            └─ TUI: ScreenMonday / ScreenReview
  └─ Pipeline: componentApplyStep{component: ComponentDevSkills}
       └─ ReadConfig → resolve repoURL (flag > config > default)
       └─ Clone(repoURL, targetDir)  [if targetDir absent]
       └─ for each adapter:
            InjectSkills(homeDir, adapter, ["java-development","java-testing"])
              → reads ~/.informa-wizard/dev-skills/skills/{id}/SKILL.md
              → writes adapter.SkillsDir(homeDir)/{id}/SKILL.md (atomic)
       └─ WriteConfig (stores installed_skills)

wizard sync
  └─ componentSyncStep{component: ComponentDevSkills}
       └─ ReadConfig → absent/empty → no-op
       └─ Pull(targetDir)
       └─ for each adapter:
            InjectSkills(homeDir, adapter, cfg.InstalledSkills)
```

---

## 8. Testing Seams

| Concern | Seam |
|---|---|
| git clone/pull | `var execCommand` override in `devskills` package |
| Config read/write | `homeDir` parameter — tests use `t.TempDir()` |
| DiscoverSkills | `repoDir` parameter; test creates fake repo structure with SKILL.md frontmatter in temp dir |
| InjectSkills disk reads | `homeDir` parameter; test creates fake repo structure in temp dir |
| Adapter injection | Pass a test adapter with known `SkillsDir()` return |
| TUI screen render | `RenderDevSkillPicker(skills, checked, cursor)` is a pure function — no model needed |
| Pipeline step | Use the same `resolveAdapters` + fake home pattern as existing component tests |
