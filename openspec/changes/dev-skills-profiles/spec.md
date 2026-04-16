# Spec: dev-skills-profiles

**Change ID**: dev-skills-profiles
**Date**: 2026-04-15
**Status**: spec

---

## Purpose

Define the precise behavior of the dev-skills feature: skill discovery from the cloned repository, TUI individual skill picker screen, install pipeline step, sync pipeline step, config file format, CLI flag, and all error cases.

---

## 1. skill-discovery

### Requirement: Dynamic Skill Discovery from Cloned Repository

The system MUST NOT use a hardcoded profile registry. Instead, it MUST dynamically discover available skills by scanning the cloned repository's `skills/` directory at `~/.informa-wizard/dev-skills/skills/`.

Discovery MUST be implemented in `internal/components/devskills/discovery.go`. The function `DiscoverSkills(repoDir string) ([]DiscoveredSkill, error)` MUST:

1. List all immediate subdirectories of `<repoDir>/skills/`.
2. For each subdirectory, check if a `SKILL.md` file exists within it.
3. If `SKILL.md` exists, parse its YAML frontmatter to extract `name` (display name) and `description` (one-line summary).
4. Return a `[]DiscoveredSkill` sorted alphabetically by skill ID (directory name).
5. Subdirectories without a `SKILL.md` file MUST be silently skipped (not an error).

Each `DiscoveredSkill` MUST have:

| Field         | Type   | Description                                                   |
|---------------|--------|---------------------------------------------------------------|
| `ID`          | string | Directory name under `skills/` (e.g., `"java-development"`)  |
| `Name`        | string | Human-readable name from SKILL.md frontmatter                 |
| `Description` | string | One-line description from SKILL.md frontmatter                |

#### Scenario: All skills with SKILL.md are discovered

- GIVEN the cloned repo has `skills/java-development/SKILL.md`, `skills/java-testing/SKILL.md`, and `skills/informads-development/SKILL.md`
- WHEN `DiscoverSkills(repoDir)` is called
- THEN exactly 3 `DiscoveredSkill` entries are returned, sorted alphabetically by ID

#### Scenario: Subdirectory without SKILL.md is skipped

- GIVEN the cloned repo has `skills/incomplete/` with no `SKILL.md` inside
- WHEN `DiscoverSkills(repoDir)` is called
- THEN `"incomplete"` does not appear in the result

#### Scenario: SKILL.md frontmatter is parsed for name and description

- GIVEN `skills/java-development/SKILL.md` has frontmatter `name: Java Development` and `description: Java coding patterns and best practices`
- WHEN `DiscoverSkills(repoDir)` is called
- THEN the entry for `"java-development"` has `Name == "Java Development"` and `Description == "Java coding patterns and best practices"`

#### Scenario: Empty skills directory returns empty slice

- GIVEN the `skills/` directory exists but contains no subdirectories
- WHEN `DiscoverSkills(repoDir)` is called
- THEN an empty `[]DiscoveredSkill` is returned with no error

#### Scenario: Missing skills directory returns error

- GIVEN `<repoDir>/skills/` does not exist
- WHEN `DiscoverSkills(repoDir)` is called
- THEN a non-nil error is returned describing that the skills directory was not found

---

## 2. tui-screen

### Requirement: Screen Placement

A new TUI screen `ScreenDevSkillPicker` MUST be inserted into the install flow after `ScreenSkillPicker` (or after `ScreenDependencyTree` when SkillPicker is skipped) and before the `goToReviewOrMonday()` transition.

The screen MUST only be shown when `ComponentDevSkills` is present in the current component plan (`Selection.Components`) AND the cloned repository exists at `~/.informa-wizard/dev-skills/`. The gate condition MUST be implemented as `shouldShowDevSkillPickerScreen()` on the TUI `Model`.

The `linearRoutes` map in `internal/tui/router.go` MUST be updated so:
- `ScreenSkillPicker.Forward` = `ScreenDevSkillPicker`
- `ScreenDevSkillPicker.Forward` = `ScreenReview` (or `ScreenMonday` when applicable)
- `ScreenDevSkillPicker.Backward` = `ScreenSkillPicker` (or `ScreenDependencyTree` when SkillPicker is absent)

#### Scenario: Screen shown when ComponentDevSkills selected

- GIVEN `ComponentDevSkills` is in `Selection.Components`
- AND the cloned repo exists at `~/.informa-wizard/dev-skills/`
- WHEN the user advances past ScreenSkillPicker (or ScreenDependencyTree)
- THEN `ScreenDevSkillPicker` is displayed showing all discovered skills

#### Scenario: Screen skipped when ComponentDevSkills not selected

- GIVEN `ComponentDevSkills` is NOT in `Selection.Components`
- WHEN the user advances past ScreenSkillPicker
- THEN navigation jumps directly to Review (or Monday screen if applicable), skipping `ScreenDevSkillPicker`

---

### Requirement: Checkbox Skill List

The screen MUST render a multi-select checkbox list. Each row MUST display one individual skill discovered from the cloned repo's `skills/` directory. Each row shows the skill's name left-aligned and its one-line description to the right, e.g.:

```
  [x] Java Development       Java coding patterns and best practices
  [x] Java Testing           JUnit and testing patterns for Java
  [ ] InformaDS Development  React component development with InformaDS
  [ ] Human Documentation    Documentation writing guidelines
  [ ] Context0 Instructions  AI-optimized docs management
  [ ] Skill Creator          Create new AI agent skills
```

The list is populated dynamically by calling `DiscoverSkills(repoDir)` when the screen is initialized. The skills shown depend entirely on what exists in the cloned repo — there is no hardcoded list.

No skill is pre-selected by default.

#### Scenario: All discovered skills rendered

- GIVEN the cloned repo contains 6 skill subdirectories with valid SKILL.md files
- WHEN `View()` is called
- THEN all 6 skill rows are shown with checkbox indicators

#### Scenario: No skills pre-selected

- GIVEN the screen is first shown
- WHEN `View()` renders the list
- THEN all checkboxes show the unchecked indicator `[ ]`

---

### Requirement: Keyboard Navigation

The screen MUST follow the same keyboard conventions as `ScreenSkillPicker`:

| Key       | Action                                           |
|-----------|--------------------------------------------------|
| Up / k    | Move cursor to previous row                      |
| Down / j  | Move cursor to next row                          |
| Space     | Toggle the checkbox of the row under the cursor  |
| Enter     | Confirm selection and advance to next screen     |
| Esc       | Navigate back to the previous screen             |

Pressing Enter with zero skills selected MUST be allowed (it results in no skills installed).

#### Scenario: Space toggles selection

- GIVEN the cursor is on `java-development`
- WHEN Space is pressed
- THEN the `java-development` row switches from unchecked to checked (or vice versa)

#### Scenario: Enter with empty selection advances

- GIVEN no skills are checked
- WHEN Enter is pressed
- THEN the TUI navigates to the next screen and `Selection.DevSkillSelections` is set to `[]`

#### Scenario: Enter with selections stores them

- GIVEN `java-development` and `java-testing` are checked
- WHEN Enter is pressed
- THEN `Selection.DevSkillSelections` is set to `["java-development", "java-testing"]` and the TUI navigates forward

#### Scenario: Enter with all skills selected

- GIVEN all 6 discovered skills are checked
- WHEN Enter is pressed
- THEN `Selection.DevSkillSelections` contains all 6 skill IDs

#### Scenario: Esc navigates back

- GIVEN `ScreenDevSkillPicker` is active
- WHEN Esc is pressed
- THEN the TUI navigates to the previous screen and `Selection.DevSkillSelections` is not modified

---

### Requirement: Selection Persistence in Model

`Selection.DevSkillSelections []string` MUST be added to `internal/model/selection.go`. The field stores the individual skill IDs (directory names) chosen on `ScreenDevSkillPicker`. An empty slice means "no skills selected."

#### Scenario: Field is present in Selection

- GIVEN a `Selection` struct
- WHEN `DevSkillSelections` is accessed
- THEN it is of type `[]string` and defaults to `nil` / empty

---

## 3. install-step

### Requirement: ComponentDevSkills Apply Step

A new `componentApplyStep` case for `ComponentDevSkills` MUST be added in the install pipeline. It MUST execute in this exact order:

1. Read `~/.informa-wizard/dev-skills.json`; if absent, create an in-memory `Config` with the default repo URL and empty `installed_skills`.
2. Determine the target directory: `~/.informa-wizard/dev-skills/`.
3. If the target directory does NOT exist: call `Clone(repoURL, targetDir)`.
4. If the target directory DOES exist: skip clone (no pull during install — only sync pulls).
5. Call `InjectSkills(homeDir, adapter, selectedSkillIDs)` for each detected agent adapter.
6. Write `~/.informa-wizard/dev-skills.json` with `repo_url` from the resolved config and `installed_skills` set to the selected skill IDs.

"Selected skill IDs" are read from `Selection.DevSkillSelections`.

#### Scenario: First install clones repo

- GIVEN `~/.informa-wizard/dev-skills/` does not exist
- AND `Selection.DevSkillSelections` is `["java-development", "java-testing"]`
- WHEN the `ComponentDevSkills` apply step runs
- THEN `Clone(defaultRepoURL, targetDir)` is called exactly once
- AND selected skills are injected for all detected adapters
- AND `dev-skills.json` is written with `"installed_skills": ["java-development", "java-testing"]`

#### Scenario: Re-install skips clone

- GIVEN `~/.informa-wizard/dev-skills/` already exists
- AND `Selection.DevSkillSelections` is `["informads-development"]`
- WHEN the `ComponentDevSkills` apply step runs
- THEN `Clone` is NOT called
- AND selected skills are injected
- AND `dev-skills.json` is written with `"installed_skills": ["informads-development"]`

#### Scenario: Empty skill selection runs no injection

- GIVEN `Selection.DevSkillSelections` is `[]`
- WHEN the `ComponentDevSkills` apply step runs
- THEN clone (if needed) still runs
- AND `InjectSkills` is called with an empty skill list
- AND `dev-skills.json` is written with `"installed_skills": []`

#### Scenario: Clone fails — step aborts

- GIVEN `~/.informa-wizard/dev-skills/` does not exist
- AND `git clone` exits with a non-zero code
- WHEN the apply step runs
- THEN the step returns an error containing the git stderr output
- AND no injection is attempted
- AND `dev-skills.json` is NOT written

#### Scenario: git not on PATH — step aborts

- GIVEN `git` binary is not found on PATH
- WHEN `Clone` is called
- THEN an error is returned with a message indicating git is required
- AND the pipeline step surfaces this as a user-facing actionable error

---

## 4. sync-step

### Requirement: ComponentDevSkills Sync Step

A new `componentSyncStep` case for `ComponentDevSkills` MUST be added in the sync pipeline. It MUST execute in this order:

1. Read `~/.informa-wizard/dev-skills.json`.
2. If the file does NOT exist, return a no-op result (zero files changed, no error).
3. If `installed_skills` is empty, return a no-op result.
4. Call `Pull(targetDir)` to update the cloned repo.
5. Call `InjectSkills(homeDir, adapter, installedSkillIDs)` for each detected agent adapter.

The sync step MUST NOT prompt the user. It reads `installed_skills` entirely from `dev-skills.json`.

#### Scenario: Config absent — sync no-ops

- GIVEN `~/.informa-wizard/dev-skills.json` does not exist
- WHEN the `ComponentDevSkills` sync step runs
- THEN no git or file operations are performed
- AND the step returns success with zero files changed

#### Scenario: Empty installed_skills — sync no-ops

- GIVEN `dev-skills.json` exists with `"installed_skills": []`
- WHEN the sync step runs
- THEN no git or file operations are performed
- AND the step returns success with zero files changed

#### Scenario: Happy path sync

- GIVEN `dev-skills.json` has `"installed_skills": ["java-development", "java-testing"]`
- AND `~/.informa-wizard/dev-skills/` exists
- WHEN the sync step runs
- THEN `Pull(targetDir)` is called
- AND `InjectSkills` is called with `["java-development", "java-testing"]` for all detected adapters
- AND the step returns the count of files changed

#### Scenario: Pull fails — sync returns error

- GIVEN `~/.informa-wizard/dev-skills/` exists
- AND `git pull` exits with non-zero code
- WHEN the sync step runs
- THEN the step returns an error containing the git stderr output
- AND no injection is attempted

#### Scenario: Sync updates skills when repo changes

- GIVEN a prior `git pull` brings new content to `skills/java-development/SKILL.md`
- WHEN `InjectSkills` runs for skill `java-development`
- THEN the updated `SKILL.md` content is written to all agent skill directories
- AND `InjectionResult.Changed` is `true`

---

## 5. git-operations

### Requirement: Clone

`Clone(repoURL, targetDir string) error` MUST:
- Execute `git clone <repoURL> <targetDir>` via `os/exec`.
- Capture combined stdout+stderr and include it in the returned error when the exit code is non-zero.
- Use an injectable `execCommand` variable (defaulting to `exec.Command`) to allow test substitution.
- NOT set a hardcoded timeout (the spinner pipeline handles UI responsiveness).

#### Scenario: Successful clone

- GIVEN `git clone` exits with code 0
- WHEN `Clone` returns
- THEN the error is nil

#### Scenario: Failed clone surfaces stderr

- GIVEN `git clone` exits with code 128 and stderr `"fatal: repository not found"`
- WHEN `Clone` returns
- THEN the error message contains `"fatal: repository not found"`

---

### Requirement: Pull

`Pull(targetDir string) error` MUST:
- Execute `git -C <targetDir> pull` via the injectable `execCommand`.
- Capture combined stdout+stderr and include it in the returned error on non-zero exit.

#### Scenario: Successful pull

- GIVEN `git pull` exits with code 0
- WHEN `Pull` returns
- THEN the error is nil

#### Scenario: Failed pull surfaces stderr

- GIVEN `git pull` exits with non-zero and stderr `"error: cannot lock ref"`
- WHEN `Pull` returns
- THEN the error message contains `"error: cannot lock ref"`

---

## 6. inject-skills

### Requirement: InjectSkills

`InjectSkills(homeDir string, adapter agents.Adapter, skillIDs []string) (InjectionResult, error)` MUST:

1. Skip injection entirely if `adapter.SupportsSkills()` is false, returning an empty `InjectionResult`.
2. For each skill ID in `skillIDs`, read `<targetDir>/skills/<skillID>/SKILL.md` from disk.
3. If the `SKILL.md` file does not exist on disk (e.g., skill was removed from repo after selection), log a warning and skip that skill — do NOT return an error.
4. Write the content to `adapter.SkillsDir(homeDir)/<skillID>/SKILL.md` using `filemerge.WriteFileAtomic`.
5. Return `InjectionResult{Changed: <any file changed>, Files: <written paths>}`.

The function MUST NOT copy any other files from the skill directory (e.g., reference `.md` files or `prompts/`). Only `SKILL.md` is copied. This is an explicit Phase 1 constraint.

The target directory for reading MUST be `~/.informa-wizard/dev-skills/`.

There is NO profile lookup step. Skill IDs are passed directly — they correspond 1:1 to subdirectory names under `skills/`.

#### Scenario: Single skill injected successfully

- GIVEN `"java-development"` is in the skill list and `java-development/SKILL.md` exists in the repo
- AND adapter supports skills
- WHEN `InjectSkills` is called
- THEN `SKILL.md` is written to `adapter.SkillsDir(homeDir)/java-development/SKILL.md`
- AND `InjectionResult.Files` contains that path

#### Scenario: Adapter does not support skills — no-op

- GIVEN an adapter where `SupportsSkills()` returns false
- WHEN `InjectSkills` is called
- THEN no files are written and an empty `InjectionResult` is returned with no error

#### Scenario: Missing skill directory — warn and skip

- GIVEN `"human-documentation"` is in the skill list but `human-documentation/SKILL.md` is absent in the cloned repo
- WHEN `InjectSkills` is called
- THEN a warning is logged for `human-documentation`
- AND injection continues for remaining skills
- AND no error is returned

#### Scenario: Multiple skills injected

- GIVEN `["java-development", "java-testing", "human-documentation"]` are selected
- AND all three skill directories exist in the repo
- WHEN `InjectSkills` is called
- THEN three `SKILL.md` files are written
- AND `InjectionResult.Files` has length 3

#### Scenario: Write failure returns error

- GIVEN `filemerge.WriteFileAtomic` fails for one skill (e.g., permission denied)
- WHEN `InjectSkills` is called
- THEN an error is returned immediately
- AND partial writes are NOT rolled back (caller is responsible for surfacing the error)

---

## 7. config-file

### Requirement: Config File Format

The configuration file MUST be stored at `~/.informa-wizard/dev-skills.json` and MUST conform to this JSON schema:

```json
{
  "repo_url": "https://gitlab.informa.tools/ai/skills/dev-skills.git",
  "installed_skills": ["java-development", "java-testing"]
}
```

| Field              | Type            | Required | Description                                              |
|--------------------|-----------------|----------|----------------------------------------------------------|
| `repo_url`         | string          | yes      | HTTPS git URL of the dev-skills repository               |
| `installed_skills` | array of string | yes      | Skill IDs (directory names) installed by the last successful apply step |

The default `repo_url` MUST be `"https://gitlab.informa.tools/ai/skills/dev-skills.git"`.

#### Scenario: Config written after install

- GIVEN the apply step completes successfully with skills `["java-development", "java-testing"]`
- WHEN `WriteConfig` is called
- THEN `dev-skills.json` is written with `"installed_skills": ["java-development", "java-testing"]` and the resolved `"repo_url"`

#### Scenario: Config read during sync

- GIVEN `dev-skills.json` contains `{"repo_url": "...", "installed_skills": ["informads-development"]}`
- WHEN `ReadConfig` is called
- THEN a `Config` struct with `RepoURL` and `InstalledSkills: ["informads-development"]` is returned

#### Scenario: Missing file returns sentinel — not error

- GIVEN `dev-skills.json` does not exist
- WHEN `ReadConfig` is called
- THEN it returns `(Config{}, nil)` with a zero-value `Config` (empty `InstalledSkills`)
- AND the caller MUST detect absence via empty `RepoURL` or a dedicated `ErrConfigNotFound` sentinel, not via a non-nil error

#### Scenario: Malformed JSON returns error

- GIVEN `dev-skills.json` contains invalid JSON
- WHEN `ReadConfig` is called
- THEN a non-nil error is returned describing the parse failure

---

### Requirement: WriteConfig Atomicity

`WriteConfig` MUST use `filemerge.WriteFileAtomic` (or equivalent write-to-temp-then-rename) to prevent partial writes from corrupting the config file.

#### Scenario: Atomic write on successful install

- GIVEN a valid `Config` to persist
- WHEN `WriteConfig` is called
- THEN the file is written atomically (temp file + rename), leaving no partial state on disk if the process is interrupted

---

## 8. cli-flag

### Requirement: --dev-skills-repo Flag

A `--dev-skills-repo <url>` flag MUST be added to the `wizard install` command via `InstallFlags` in `internal/cli/install.go`. When provided, this value overrides the default repo URL and MUST be stored in `InstallFlags.DevSkillsRepo`.

The install pipeline step MUST use `InstallFlags.DevSkillsRepo` as the `repo_url` when non-empty, falling back to the default URL otherwise.

The flag MUST NOT be required. Omitting it results in the default URL being used.

#### Scenario: Flag overrides default URL

- GIVEN `wizard install --dev-skills-repo https://github.com/myorg/dev-skills.git`
- WHEN the `ComponentDevSkills` apply step resolves the repo URL
- THEN `Clone` is called with `"https://github.com/myorg/dev-skills.git"` as `repoURL`

#### Scenario: Flag absent uses default

- GIVEN `wizard install` is called with no `--dev-skills-repo` flag
- WHEN the `ComponentDevSkills` apply step resolves the repo URL
- THEN `Clone` is called with `"https://gitlab.informa.tools/ai/skills/dev-skills.git"`

#### Scenario: Custom URL is persisted in config

- GIVEN `--dev-skills-repo https://github.com/myorg/dev-skills.git` was used during install
- WHEN `WriteConfig` is called
- THEN `dev-skills.json` stores `"repo_url": "https://github.com/myorg/dev-skills.git"`
- AND subsequent sync runs use that stored URL

---

## 9. model-changes

### Requirement: ComponentDevSkills Constant

`ComponentDevSkills ComponentID = "dev-skills"` MUST be added to the `ComponentID` constants block in `internal/model/types.go`.

#### Scenario: Constant value matches expected string

- GIVEN `model.ComponentDevSkills` is referenced
- WHEN its string value is read
- THEN it equals `"dev-skills"`

---

### Requirement: DevSkillSelections Field in Selection

`DevSkillSelections []string` MUST be added to `internal/model/selection.go`'s `Selection` struct. The field stores the individual skill IDs (directory names) chosen on the TUI screen.

It MUST NOT be included in `SyncOverrides` — sync always reads installed skills from `dev-skills.json`, not from user-provided overrides.

---

## 10. error-handling

### Requirement: Actionable Error Messages

All errors produced by the `devskills` package MUST include the operation context so users can diagnose the issue without reading source code.

| Error condition           | Required context in message                                        |
|---------------------------|--------------------------------------------------------------------|
| git binary not on PATH    | `"git is required for dev-skills; install git and try again"`      |
| Clone exits non-zero      | `"git clone failed: <stderr>"`                                     |
| Pull exits non-zero       | `"git pull failed: <stderr>"`                                      |
| Malformed dev-skills.json | `"dev-skills.json: <parse error>"`                                 |
| WriteFileAtomic fails     | `"skill <id>: write failed: <underlying error>"`                   |

#### Scenario: User sees actionable message on clone failure

- GIVEN the git repository URL is unreachable
- AND `git clone` exits with stderr `"Could not resolve host"`
- WHEN the pipeline step fails
- THEN the error surfaced to the user contains `"git clone failed: Could not resolve host"`

---

## Interface Contracts

### DiscoveredSkill Struct

| Field         | Type   | Description                                                        |
|---------------|--------|--------------------------------------------------------------------|
| `ID`          | string | Directory name under `skills/` (e.g., `"java-development"`)       |
| `Name`        | string | Human-readable name from SKILL.md frontmatter                     |
| `Description` | string | One-line description from SKILL.md frontmatter                    |

### Config Struct

| Field            | Type     | Description                             |
|------------------|----------|-----------------------------------------|
| `RepoURL`        | string   | HTTPS git URL of the dev-skills repo    |
| `InstalledSkills` | []string | Skill IDs (directory names) from last successful apply |

### InjectionResult Struct

| Field     | Type     | Description                                        |
|-----------|----------|----------------------------------------------------|
| `Changed` | bool     | True if any file content changed on disk           |
| `Files`   | []string | Absolute paths of all files written                |

### Package Layout

| File                                             | Exports                                                                         |
|--------------------------------------------------|---------------------------------------------------------------------------------|
| `internal/components/devskills/discovery.go`     | `DiscoveredSkill`, `DiscoverSkills(repoDir string) ([]DiscoveredSkill, error)`  |
| `internal/components/devskills/git.go`           | `Clone(repoURL, targetDir string) error`, `Pull(targetDir string) error`        |
| `internal/components/devskills/inject.go`        | `InjectSkills(homeDir string, adapter agents.Adapter, skillIDs []string) (InjectionResult, error)` |
| `internal/components/devskills/config.go`        | `Config`, `ReadConfig(homeDir string) (Config, error)`, `WriteConfig(homeDir string, cfg Config) error` |

---

## Screen-Constant Mapping

| Constant               | Position in Install Flow                      | Forward                     | Backward                   |
|------------------------|-----------------------------------------------|-----------------------------|----------------------------|
| `ScreenDevSkillPicker` | After SkillPicker (or DependencyTree)         | ScreenMonday or ScreenReview | ScreenSkillPicker or ScreenDependencyTree |
