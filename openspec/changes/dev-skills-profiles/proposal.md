# Proposal: dev-skills-profiles

**Change ID**: dev-skills-profiles
**Date**: 2026-04-15
**Status**: proposal

---

## Intent

### What

Add a new optional step to the informa-wizard install flow that lets users select and install _dev-skills profiles_ — curated sets of agent skill files sourced from an external git repository (`dev-skills`). A matching sync step keeps installed profiles up-to-date by pulling the latest version of the repo on each `wizard sync` run.

### Why

The embedded `skills` component ships a fixed catalog that is versioned with the wizard binary. The dev-skills repository evolves independently and contains team- and domain-specific skills (Java development, InformaDS frontend, documentation generation, etc.) that are not part of the core ecosystem. Teams need a first-class way to opt in to these skills at install time without manual copying, and to keep them current as the repo grows.

---

## Scope

### In scope (Phase 1)

- New `ComponentDevSkills` component constant.
- New package `internal/components/devskills/` with:
  - `Clone(repoURL, targetDir string) error` — first-time repo setup.
  - `Pull(targetDir string) error` — subsequent updates.
  - Profile registry hardcoded in `profiles.go` (four profiles: `java`, `frontend`, `docs`, `skill-creator`).
  - `InjectProfiles(homeDir string, adapter agents.Adapter, profileIDs []ProfileID) (InjectionResult, error)` — reads `SKILL.md` files from the cloned repo on disk and writes them to `adapter.SkillsDir(homeDir)/{skillID}/SKILL.md`.
  - `ReadConfig(homeDir string) (Config, error)` / `WriteConfig(homeDir string, cfg Config) error` — manages `.informa-wizard/dev-skills.json`.
- New TUI screen `ScreenDevSkillProfiles` — checkbox list of profiles shown during install, gated by `ComponentDevSkills` being in the plan.
- `DevSkillProfiles []string` field added to `model.Selection`.
- Install pipeline step for `ComponentDevSkills`: clone (or skip if already present) then inject selected profiles.
- Sync pipeline step for `ComponentDevSkills`: pull then re-inject all installed profiles.
- Default repo URL: `https://gitlab.informa.tools/ai/skills/dev-skills.git` (HTTPS for credential safety).
- Configuration file `.informa-wizard/dev-skills.json` stores `repo_url` and `installed_profiles`.

### Out of scope (Phase 1)

- Copying reference `.md` files alongside `SKILL.md` (e.g., `spring-boot.md`, `concurrency.md`).
- Installing the `prompts/` directory from dev-skills as Claude Code slash commands.
- Dynamic profile definitions read from a `profiles.yaml` in the repo itself.
- SSH URL support (planned for a follow-up flag `--ssh`).
- A TUI option to customise the repo URL (advanced users can edit `dev-skills.json` directly).
- Progress streaming for the `git clone` operation beyond what the existing spinner provides.

---

## Approach

### TUI Flow

The new screen slots in after `ScreenSkillPicker` (or after `ScreenDependencyTree` when SkillPicker is skipped) and before the `goToReviewOrMonday()` transition:

```
... → [SkillPicker?] → [DevSkillProfiles?] → [Monday?] → Review
```

Gate condition (`shouldShowDevSkillProfilesScreen()`): `ComponentDevSkills` is present in the current component plan. This component is selectable in the dependency tree step (added alongside `ComponentSkills`, `ComponentEngram`, etc.).

The screen renders a multi-select checkbox list. Each row shows the profile name and its constituent skill IDs in brackets, e.g.:

```
  [x] java          [java-development, java-testing]
  [ ] frontend      [informads-development]
  [ ] docs          [human-documentation, context0-instructions]
  [ ] skill-creator [skill-creator]
```

Navigation and selection follow the same keyboard pattern as `ScreenSkillPicker`.

### New Package: `internal/components/devskills/`

| File | Responsibility |
|---|---|
| `profiles.go` | Hardcoded `Profile` structs mapping IDs to skill directory names |
| `git.go` | `Clone()` and `Pull()` — thin wrappers over `os/exec` with injectable `execCommand` for tests |
| `inject.go` | `InjectProfiles()` — reads from `~/.informa-wizard/dev-skills/skills/{id}/SKILL.md`, writes via `filemerge.WriteFileAtomic` |
| `config.go` | `ReadConfig()` / `WriteConfig()` — JSON marshal/unmarshal of `.informa-wizard/dev-skills.json` |

The `execCommand` injection pattern mirrors `internal/update/upgrade/strategy.go`, enabling unit tests to swap the git binary for a mock.

### Config File

`.informa-wizard/dev-skills.json`:
```json
{
  "repo_url": "https://gitlab.informa.tools/ai/skills/dev-skills.git",
  "installed_profiles": ["java", "docs"]
}
```

The file is written at the end of the install pipeline step and read by both the install (idempotency check) and sync steps. If the file is absent during sync, the sync step is skipped with a no-op result.

### Install Pipeline Step

Added as a new `componentApplyStep` case for `ComponentDevSkills` in `internal/cli/install.go` (or wherever component apply steps are dispatched):

1. Read `.informa-wizard/dev-skills.json`; if absent, create it with defaults.
2. If `~/.informa-wizard/dev-skills/` does not exist: run `Clone(repoURL, targetDir)`.
3. For each selected profile, resolve skill IDs and call `InjectProfiles()` for each selected agent adapter.
4. Write updated `.informa-wizard/dev-skills.json` with `installed_profiles` set to the user's selections.

### Sync Pipeline Step

Added as a new `componentSyncStep` case for `ComponentDevSkills`:

1. Read `.informa-wizard/dev-skills.json`; if absent or `installed_profiles` is empty, return no-op.
2. Run `Pull(targetDir)`.
3. Call `InjectProfiles()` for all `installed_profiles` and all detected agent adapters.

### Model Changes

- `internal/model/types.go`: add `ComponentDevSkills ComponentID = "dev-skills"`.
- `internal/model/selection.go`: add `DevSkillProfiles []string` to `Selection`.

### Router Changes

- `internal/tui/router.go`: add `ScreenDevSkillProfiles` to `linearRoutes`.
- `internal/tui/model.go`: add `ScreenDevSkillProfiles` to the `Screen` iota, implement `shouldShowDevSkillProfilesScreen()`, wire into `confirmSelection()`, `goToReviewOrMonday()`, `View()`, and `goBack()`.

---

## Rollback Plan

`ComponentDevSkills` is fully opt-in — existing installs are unaffected. If a user wants to remove it:

1. Delete `~/.informa-wizard/dev-skills/` (the cloned repo).
2. Delete `~/.informa-wizard/dev-skills.json`.
3. Manually remove any written `SKILL.md` files from agent skill directories (the same files that `wizard sync` would re-write if the component remained).

No existing wizard configuration, state.json, or agent config files are modified by this component. A future `wizard remove --component dev-skills` command could automate cleanup, but it is out of scope for Phase 1.

---

## Affected Packages

| Package | Change Type |
|---|---|
| `internal/model/types.go` | Add `ComponentDevSkills` constant |
| `internal/model/selection.go` | Add `DevSkillProfiles []string` field |
| `internal/components/devskills/` | **New package** — git ops, injection, config, profile registry |
| `internal/tui/model.go` | Add screen constant, gate method, routing wires |
| `internal/tui/router.go` | Add `ScreenDevSkillProfiles` route entry |
| `internal/tui/screens/dev_skill_profiles.go` | **New screen** — checkbox profile picker |
| `internal/cli/install.go` | Add `ComponentDevSkills` apply step case |
| `internal/cli/sync.go` | Add `ComponentDevSkills` sync step case |

---

## Risks

| Risk | Likelihood | Mitigation |
|---|---|---|
| SSH credentials missing on first clone | Medium | Default to HTTPS URL; document SSH override in `dev-skills.json` |
| `git` not on PATH at runtime | Low | Emit actionable error in pipeline step; existing dependency checks cover common cases |
| Slow `git clone` blocking TUI installer | Medium | The install step runs inside the existing spinner pipeline; no special handling needed for Phase 1, but a timeout guard is advisable |
| Reference files not copied (agents lose context) | Low | Documented as out-of-scope; SKILL.md is the primary agent-loaded file for all supported agents |
| Profile definitions stale if repo structure changes | Low | Profiles are hardcoded; a mismatch logs a warning and skips the missing skill directory gracefully |
