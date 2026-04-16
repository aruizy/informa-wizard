# Exploration: dev-skills-profiles

**Change ID**: dev-skills-profiles
**Date**: 2026-04-15
**Status**: exploration

---

## 1. TUI Flow — Where the New Screen Fits

### Current install flow (linear route chain):

```
Welcome → Detection → Agents → Preset → [ClaudeModelPicker?] → [SDDMode?]
→ [StrictTDD?] → DependencyTree → [SkillPicker?] → [Monday?] → Review → Installing → Complete
```

The `ScreenSkillPicker` is the existing analog: it appears **after** `ScreenDependencyTree` and **before** `Review`/`Monday`, gated by `shouldShowSkillPickerScreen()` (which returns true when `PresetCustom` AND `ComponentSkills` is selected). The same gate pattern applies to the new dev-skills screen.

### Where the new screen fits:

The new screen (`ScreenDevSkillProfiles`) should slot in **after** `ScreenSkillPicker` (or after `ScreenDependencyTree` when SkillPicker is not shown) and **before** `goToReviewOrMonday()`. Proposed chain:

```
... → [SkillPicker?] → [DevSkillProfiles?] → [Monday?] → Review
```

Gate condition: shown when:
- `ComponentSkills` (or a new dedicated component) is selected, AND
- the dev-skills repo is configured (a repo URL is available in config).

A simpler alternative: always show it as its own optional step independent of `ComponentSkills` (since dev-skills profiles are a separate concern from the embedded skills catalog).

### Router changes required:

In `internal/tui/router.go`, add:
```go
ScreenDevSkillProfiles: {Forward: ScreenReview, Backward: ScreenDependencyTree},
```
(Backward logic may need to account for SkillPicker placement.)

In `internal/tui/model.go`, add `ScreenDevSkillProfiles` to the `Screen` iota block, add a `shouldShowDevSkillProfilesScreen()` method, wire the new screen into:
- `confirmSelection()` → `ScreenDependencyTree` / `ScreenSkillPicker` transitions
- `goToReviewOrMonday()` caller sites
- `View()` method
- `goBack()` / `handleKeyPress()` esc handling

---

## 2. Skills Injection Pattern (Follow This)

`internal/components/skills/inject.go` is the reference:

```go
func Inject(homeDir string, adapter agents.Adapter, skillIDs []model.SkillID) (InjectionResult, error)
```

Key observations:
1. `adapter.SupportsSkills()` — guard before any work.
2. `adapter.SkillsDir(homeDir)` — returns the target directory per agent.
3. Content comes from `assets.Read("skills/{id}/SKILL.md")` — the **embedded FS**.
4. Written via `filemerge.WriteFileAtomic(path, content, 0o644)` — atomic write with change detection.
5. Path structure: `{skillsDir}/{skillID}/SKILL.md`.

For dev-skills, the difference is **content source**: instead of the embedded FS, skills are read from a **local git-cloned directory** on disk (e.g., `~/.informa-wizard/dev-skills/skills/{skillID}/SKILL.md`).

The target path structure is **identical**: `{adapter.SkillsDir(homeDir)}/{skillID}/SKILL.md`.

The new component should expose a function with a parallel signature:
```go
func InjectDevSkills(homeDir string, adapter agents.Adapter, profileIDs []DevSkillProfileID) (InjectionResult, error)
```

Where each `DevSkillProfileID` maps to a set of skill folder names. The function reads `SKILL.md` from the cloned repo on disk rather than the embedded FS.

---

## 3. Dev-Skills Repository Structure

The repo at `C:/GIT/dev-skills` has this structure:

```
skills/
  java-development/
    SKILL.md
    (+ reference .md files: concurrency.md, spring-boot.md, etc.)
  java-testing/
    SKILL.md
    (+ reference .md files)
  informads-development/
    SKILL.md
    (+ references/)
  human-documentation/
    SKILL.md
    (+ content guidelines)
  context0-instructions/
    SKILL.md
    (+ general.md, generation.md, maintenance.md)
  skill-creator/
    SKILL.md
    (+ agents/, scripts/, eval-viewer/, references/)
prompts/
  full-project-review.prompt.md
  generate-commit-from-staged-changes.prompt.md
  improve-java-coverage.prompt.md
  review-task-changes.prompt.md
  squash-branch-into-single-commit.prompt.md
  update-docs-from-staged.prompt.md
template/
```

### Key structural finding:
Each skill directory may contain **additional reference files** (not just `SKILL.md`). The `skills.Inject` pattern only copies `SKILL.md`. For dev-skills, we need to decide: copy only `SKILL.md` or copy the full skill directory?

Given that agents (Claude Code, OpenCode, etc.) only load `SKILL.md`, the **safest first implementation** is to copy only `SKILL.md`. Reference files can be added in a follow-up.

### Defined profiles (proposed):

| Profile Name | Skills Included |
|---|---|
| `java` | `java-development` + `java-testing` |
| `frontend` | `informads-development` |
| `docs` | `human-documentation` + `context0-instructions` |
| `skill-creator` | `skill-creator` |

These should be defined in a config file or hardcoded in a `profiles.go` file in the new component package. Config-driven is more flexible; hardcoded is simpler.

---

## 4. Git Operations — Existing Patterns

The codebase uses `os/exec` + `git` commands in several places:

| File | Pattern |
|---|---|
| `internal/update/upgrade/strategy.go` | `git clone <url> <tmpDir>` for GGA upgrades |
| `internal/cli/run.go` | `os/exec` via `executeCommand` wrapper |
| `internal/components/sdd/inject.go` | `os/exec` with git-like usage |
| `internal/app/selfupdate.go` | `os/exec` for self-update |

**`upgrade/strategy.go` is the best reference** — it demonstrates git clone with:
- Injected `execCommand` variable for testability (swappable in tests)
- Error message includes command output for diagnostics
- `CombinedOutput()` for capturing stdout+stderr

For dev-skills, we need two operations:
1. **Clone**: `git clone <url> ~/.informa-wizard/dev-skills` (first install)
2. **Pull**: `git -C ~/.informa-wizard/dev-skills pull` (sync)

Both should use the same injectable `execCommand` pattern for testability. The target directory should be `~/.informa-wizard/dev-skills` (parallel to the existing `~/.informa-wizard/` state directory).

---

## 5. Configuration — Where the Repo URL Lives

### Existing config patterns:
- `.informa-wizard/state.json` — persists installed agents + components (from `internal/state/state.go`)
- `.informa-wizard/monday.json` — Monday.com `board_id` (already referenced in orchestrator instructions)
- `openspec/config.yaml` — project-level SDD config (not the right place for runtime config)

### Recommended approach for repo URL:
Extend `state.json` by adding a `DevSkillsConfig` sub-object, OR create a new `.informa-wizard/dev-skills.json` config file. A dedicated file is cleaner because it separates install-time state from user configuration.

**Proposed `.informa-wizard/dev-skills.json`**:
```json
{
  "repo_url": "git@gitlab.informa.tools:ai/skills/dev-skills.git",
  "installed_profiles": ["java", "docs"]
}
```

This file:
- Is written on first install (TUI flow captures profile selection and repo URL)
- Is read on sync to know which profiles to re-apply
- Could be pre-seeded with the default URL, letting the user override it

The repo URL should default to `git@gitlab.informa.tools:ai/skills/dev-skills.git` (or the HTTPS equivalent) but be configurable, since different teams may fork the repo.

### Alternative: hardcode the URL
Since there's only one dev-skills repo in Informa, the URL could be hardcoded as a constant in the new component package, similar to how GGA's GitHub URL is hardcoded in `internal/update/upgrade/strategy.go`. This eliminates user-facing configuration complexity at the cost of flexibility.

---

## 6. State Persistence — Persisting Selected Profiles

`internal/state/state.go` currently has:
```go
type InstallState struct {
    InstalledAgents     []string `json:"installed_agents"`
    InstalledComponents []string `json:"installed_components"`
}
```

The `Write` function signature is `func Write(homeDir string, agents []string, components []string)`.

### Options:

**Option A — Extend `state.json`** (low-friction):
Add `InstalledDevSkillProfiles []string` to `InstallState` and update `Write()` to accept it. Risk: breaks existing callers of `Write()` (there are many in `internal/cli/`).

**Option B — Dedicated dev-skills config file** (preferred):
Store profiles in `.informa-wizard/dev-skills.json` alongside state.json. Avoids changing the `Write()` signature. Reads are isolated to the new component.

**Option C — Extend Selection and flow through**:
Add `DevSkillProfiles []string` to `model.Selection`. The sync path reads it from the dedicated config file and injects it into the selection. This is analogous to how `Skills []SkillID` flows through Selection.

Option B + C combined is the cleanest: the TUI populates `Selection.DevSkillProfiles`, which is persisted to `.informa-wizard/dev-skills.json` during install, and read back during sync.

---

## 7. Sync Behavior

During sync (`internal/cli/sync.go`), a new component `ComponentDevSkills` would be added as a case in `componentSyncStep.Run()`. Its sync step would:

1. Read `~/.informa-wizard/dev-skills.json` to get `repo_url` and `installed_profiles`.
2. If the dev-skills repo dir exists: run `git -C <dir> pull`.
3. If not: run `git clone <url> <dir>`.
4. For each installed profile, resolve the skill IDs and copy `SKILL.md` files to each agent's skills directory.

This mirrors the `ComponentGGA` sync step which calls `gga.EnsureRuntimeAssets()` before injecting config.

---

## 8. Risks and Open Questions

### Risks:

1. **Git availability**: `git` must be on PATH at runtime. The existing `system/deps.go` already checks for git as a dependency for some operations — the wizard should emit a clear error if git is missing. This is LOW risk since git is universally available in dev environments.

2. **SSH key / credentials for private GitLab**: The default dev-skills repo is on GitLab with SSH. On CI machines or fresh installs, `git clone git@...` will fail silently if no SSH key is configured. **HTTPS URL** (`https://gitlab.informa.tools/ai/skills/dev-skills.git`) + credential prompting is safer, or we support both and let the user configure.

3. **Slow operation**: `git clone` can be slow on first install (network dependency). The TUI's installing screen already shows a spinner — the git clone step should appear as a named pipeline step with progress feedback, not a silent hang.

4. **Reference files not copied**: Skills like `java-development` have 12+ reference `.md` files. Copying only `SKILL.md` means agents won't have inline access to `spring-boot.md` etc. This is the same limitation as the existing embedded skills component (which also only embeds `SKILL.md`). Not a blocker but worth documenting.

5. **Profile definition ownership**: Who adds new profiles? If hardcoded in the wizard, a code change is required for each new profile. If defined in the repo itself (e.g., a `profiles.yaml` in dev-skills), the wizard reads it dynamically — more flexible but requires a different parsing step.

### Open Questions:

1. **New component or extension of ComponentSkills?** A dedicated `ComponentDevSkills` keeps concerns separate. Recommended.

2. **Who defines profiles?** Two options:
   - **Wizard-side**: `profiles.go` in the component package defines `"java" → [java-development, java-testing]`. Simple, requires wizard release for new profiles.
   - **Repo-side**: A `profiles.yaml` in the dev-skills repo defines the mapping. More flexible, supports adding profiles without touching the wizard.

3. **HTTPS vs SSH?** For ease of use, HTTPS is safer as default. Add a `--ssh` flag or config option for teams that prefer SSH.

4. **Should sync always pull?** Or only when explicitly requested? The current `sync` command is designed to be idempotent and re-run freely. A `git pull` on every sync is consistent with this philosophy.

5. **Prompts directory**: The dev-skills repo also has a `prompts/` directory with `.prompt.md` files. Should these be installed somewhere? Claude Code supports custom slash commands via `.claude/commands/`. This is out of scope for the first iteration.

---

## 9. Recommended Approach (High-Level)

### New packages:
- `internal/components/devskills/` — clone, pull, inject, profile definitions
- `internal/tui/screens/dev_skill_profiles.go` — new TUI screen renderer

### New model additions:
- `model.ComponentDevSkills ComponentID = "dev-skills"` in `internal/model/types.go`
- `DevSkillProfiles []string` in `model.Selection`

### New state:
- `.informa-wizard/dev-skills.json` for `{ repo_url, installed_profiles }`
- Read/Write functions in the new `devskills` package (not extending `state.go`)

### TUI screen:
- Style: checkbox list of profiles, each showing included skills (e.g., `java  [java-development, java-testing]`)
- Gated by `shouldShowDevSkillProfilesScreen()` → true when `ComponentDevSkills` is in the plan
- Sits after `ScreenSkillPicker` (or `ScreenDependencyTree` if SkillPicker not shown), before `goToReviewOrMonday()`
- Screen constant `ScreenDevSkillProfiles` added to the `Screen` iota

### Install pipeline:
- New `componentApplyStep` case for `ComponentDevSkills`:
  1. `git clone <url> ~/.informa-wizard/dev-skills` (first time) or skip if already cloned
  2. Copy selected profile skills to all selected agent skill directories

### Sync pipeline:
- New `componentSyncStep` case for `ComponentDevSkills`:
  1. `git pull` in `~/.informa-wizard/dev-skills`
  2. Re-copy selected profile skills (idempotent)

### Git layer:
- `devskills.Clone(repoURL, targetDir string) error`
- `devskills.Pull(targetDir string) error`
- Both use injectable `execCommand` for testability (same pattern as `upgrade/strategy.go`)

### Profile definition (Phase 1 recommendation):
Hardcode profiles in `devskills/profiles.go`. Each profile is a named struct with an ID and a slice of skill directory names. This avoids repo parsing complexity for the first version.

---

## Summary

| Area | Finding |
|---|---|
| TUI screen position | After SkillPicker, before Review/Monday; gated by ComponentDevSkills |
| Skills injection pattern | Copy SKILL.md from local disk to `adapter.SkillsDir(homeDir)/{id}/SKILL.md` |
| Git ops | No existing clone/pull utility — build new `devskills` package with injectable exec |
| Config | Dedicated `.informa-wizard/dev-skills.json`; default URL hardcoded, overridable |
| State persistence | `DevSkillProfiles []string` in `model.Selection` + dev-skills.json on disk |
| Sync | git pull + re-copy SKILL.md files, as new ComponentDevSkills sync step |
| Main risks | SSH/credentials on first clone; slow git clone in TUI; reference files not copied |
