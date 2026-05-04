package devagents

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"gitlab.informa.tools/ai/wizard/informa-wizard/internal/agents"
	"gitlab.informa.tools/ai/wizard/informa-wizard/internal/components/filemerge"
	"gitlab.informa.tools/ai/wizard/informa-wizard/internal/model"
)

// vscodeVersionFn is the production function used to detect VS Code's version.
// It's a var (not const) so tests can swap in a counter/mock and call
// resetVSCodeVersionCache() to re-arm sync.OnceValue. Production code never
// reassigns this.
var vscodeVersionFn = vscodeVersion

// cachedVSCodeVersion memoises vscodeVersionFn for the lifetime of the process
// so we don't shell out to `code --version` on every ExpectedAgentFiles call
// (sync preview, post-apply verify, post-sync verify, uninstall).
var cachedVSCodeVersion = sync.OnceValue(func() string { return vscodeVersionFn() })

// resetVSCodeVersionCache re-arms the OnceValue so a test can observe the
// underlying call count after swapping vscodeVersionFn. Test-only helper.
func resetVSCodeVersionCache() {
	cachedVSCodeVersion = sync.OnceValue(func() string { return vscodeVersionFn() })
}

// InjectionResult reports the outcome of an InjectAgents call.
type InjectionResult struct {
	Changed bool
	Files   []string
}

// subAgentInjector is the minimal interface needed to inject agents into an
// adapter's sub-agents directory. Both the VS Code and Cursor adapters satisfy
// this interface. Adapters that do not implement it are silently skipped.
type subAgentInjector interface {
	SupportsSubAgents() bool
	SubAgentsDir(homeDir string) string
}

// InjectAgents copies the main .md file for each agentID from the cloned
// dev-orchestrators repository to the adapter's sub-agents directory.
//
// The source file for each agent is the first non-README .md file found in:
//
//	~/.informa-wizard/dev-agents/<agentID>/
//
// Destination path and filename suffix vary by agent type:
//   - VS Code: <SubAgentsDir>/<agentID>.agent.md
//   - Cursor:  <SubAgentsDir>/<agentID>.md
//
// Adapters that do not implement the subAgentInjector interface are silently
// skipped. If an agent's main .md is not found in the repo, a warning is
// logged and the agent is skipped (no error).
// InjectAgents copies agent files to the adapter's sub-agents directory.
// defaultModel controls the "model:" field in Claude Code frontmatter
// (e.g., "opus", "sonnet"). Pass "" to default to "sonnet".
// installedAgents is the list of agent IDs being installed in this run.
// When VS Code >= 1.116.0 and Claude Code is also installed, VS Code reads
// agents directly from ~/.claude/agents/ — no need to copy them separately.
func InjectAgents(homeDir string, adapter agents.Adapter, agentIDs []string, defaultModel string, installedAgentIDs ...model.AgentID) (InjectionResult, error) {
	// OpenCode uses JSON config, not agent files.
	if adapter.Agent() == model.AgentOpenCode {
		return injectOpenCodeAgents(homeDir, adapter, agentIDs)
	}

	// VS Code >= 1.116.0 reads agents from ~/.claude/agents/ when Claude Code
	// is also installed — skip duplicate injection.
	if adapter.Agent() == model.AgentVSCodeCopilot && hasAgent(installedAgentIDs, model.AgentClaudeCode) {
		if vsVer := cachedVSCodeVersion(); vsVer != "" && versionAtLeast(vsVer, "1.116.0") {
			log.Printf("devagents: skipping VS Code injection — v%s reads from ~/.claude/agents/", vsVer)
			return InjectionResult{}, nil
		}
	}

	sai, ok := adapter.(subAgentInjector)
	if !ok || !sai.SupportsSubAgents() {
		return InjectionResult{}, nil
	}

	repoDir := filepath.Join(homeDir, ".informa-wizard", "dev-agents")
	destDir := sai.SubAgentsDir(homeDir)

	// VS Code orchestrator agents go to the prompts/ subdirectory,
	// not the root User dir (which is for SDD sub-agents).
	if adapter.Agent() == model.AgentVSCodeCopilot {
		destDir = filepath.Join(destDir, "prompts")
	}

	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return InjectionResult{}, fmt.Errorf("dev-agents: create dest dir %q: %w", destDir, err)
	}

	suffix := agentFileSuffix(adapter.Agent())

	var files []string
	changed := false

	for _, agentID := range agentIDs {
		agentDir := filepath.Join(repoDir, agentID)
		mainFile, _, ok := findMainMD(agentDir)
		if !ok {
			log.Printf("devagents: skipping %q — no main .md file found in repo", agentID)
			continue
		}

		sourcePath := filepath.Join(agentDir, mainFile)
		content, err := os.ReadFile(sourcePath)
		if err != nil {
			if os.IsNotExist(err) {
				log.Printf("devagents: skipping %q — source file %q not found", agentID, sourcePath)
				continue
			}
			return InjectionResult{}, fmt.Errorf("dev-agent %s: read failed: %w", agentID, err)
		}

		// Claude Code agents need YAML frontmatter for the agent system.
		if adapter.Agent() == model.AgentClaudeCode {
			content = ensureClaudeFrontmatter(content, agentID, defaultModel)
		}

		destFilename := agentID + suffix
		destPath := filepath.Join(destDir, destFilename)
		result, writeErr := filemerge.WriteFileAtomic(destPath, content, 0o644)
		if writeErr != nil {
			return InjectionResult{}, fmt.Errorf("dev-agent %s: write failed: %w", agentID, writeErr)
		}

		changed = changed || result.Changed
		files = append(files, destPath)

		// Copy any sub-skills (skills/<name>/SKILL.md + reference files) from
		// the agent's skills/ directory to the adapter's SkillsDir.
		subFiles, subChanged, subErr := copyAgentSubSkills(filepath.Join(agentDir, "skills"), adapter, homeDir)
		if subErr != nil {
			return InjectionResult{}, fmt.Errorf("dev-agent %s: sub-skills failed: %w", agentID, subErr)
		}
		if subChanged {
			changed = true
		}
		files = append(files, subFiles...)
	}

	return InjectionResult{Changed: changed, Files: files}, nil
}

// copyAgentSubSkills copies all sub-skills from sourceSkillsDir (e.g.,
// ~/.informa-wizard/dev-agents/dia-del-juicio/skills/) to the adapter's
// SkillsDir. Each sub-skill's directory is copied recursively.
// Returns silently (no error) if sourceSkillsDir doesn't exist.
func copyAgentSubSkills(sourceSkillsDir string, adapter agents.Adapter, homeDir string) ([]string, bool, error) {
	if !adapter.SupportsSkills() {
		return nil, false, nil
	}
	entries, err := os.ReadDir(sourceSkillsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, err
	}

	skillsDestRoot := adapter.SkillsDir(homeDir)
	var files []string
	changed := false

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		subID := entry.Name()
		sourceSubDir := filepath.Join(sourceSkillsDir, subID)
		destSubDir := filepath.Join(skillsDestRoot, subID)

		if err := os.MkdirAll(destSubDir, 0o755); err != nil {
			return nil, false, fmt.Errorf("create sub-skill dir %q: %w", destSubDir, err)
		}

		subEntries, err := os.ReadDir(sourceSubDir)
		if err != nil {
			return nil, false, fmt.Errorf("read sub-skill dir %q: %w", sourceSubDir, err)
		}
		for _, subEntry := range subEntries {
			if subEntry.IsDir() {
				continue
			}
			content, readErr := os.ReadFile(filepath.Join(sourceSubDir, subEntry.Name()))
			if readErr != nil {
				return nil, false, fmt.Errorf("read sub-skill file %q: %w", subEntry.Name(), readErr)
			}
			destPath := filepath.Join(destSubDir, subEntry.Name())
			result, writeErr := filemerge.WriteFileAtomic(destPath, content, 0o644)
			if writeErr != nil {
				return nil, false, fmt.Errorf("write sub-skill file %q: %w", destPath, writeErr)
			}
			if result.Changed {
				changed = true
			}
			files = append(files, destPath)
		}
	}
	return files, changed, nil
}

// expectedAgentSubSkillFiles enumerates the destination paths that
// copyAgentSubSkills will (or did) write for the given source skills dir.
// Mirrors copyAgentSubSkills' walk: only subdirectories at depth 1 are
// considered, and only regular files inside each are emitted (no recursion
// beyond depth 1, matching the copy logic).
//
// Read errors are logged (matching the rest of the package's "best-effort but
// observable" pattern) and the affected entry is skipped. Returns nil silently
// when the parent skills dir doesn't exist (the agent has no sub-skills, which
// is normal — copyAgentSubSkills also returns silently in this case).
func expectedAgentSubSkillFiles(sourceSkillsDir string, skillsDestRoot string) []string {
	entries, err := os.ReadDir(sourceSkillsDir)
	if err != nil {
		// IsNotExist is the "no skills/" case — silent, like copyAgentSubSkills.
		// Other read errors (perm, I/O) are surfaced as a warning so the drift
		// is observable; we still skip and let the verifier proceed.
		if !os.IsNotExist(err) {
			log.Printf("devagents: ExpectedAgentFiles read %q failed: %v", sourceSkillsDir, err)
		}
		return nil
	}

	var files []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		subID := entry.Name()
		sourceSubDir := filepath.Join(sourceSkillsDir, subID)
		destSubDir := filepath.Join(skillsDestRoot, subID)

		subEntries, err := os.ReadDir(sourceSubDir)
		if err != nil {
			if !os.IsNotExist(err) {
				log.Printf("devagents: ExpectedAgentFiles read %q failed: %v", sourceSubDir, err)
			}
			continue
		}
		for _, subEntry := range subEntries {
			if subEntry.IsDir() {
				continue
			}
			files = append(files, filepath.Join(destSubDir, subEntry.Name()))
		}
	}
	return files
}

// injectOpenCodeAgents merges agent definitions into opencode.json's "agent" key.
func injectOpenCodeAgents(homeDir string, adapter agents.Adapter, agentIDs []string) (InjectionResult, error) {
	repoDir := filepath.Join(homeDir, ".informa-wizard", "dev-agents")
	settingsPath := adapter.SettingsPath(homeDir)

	agentOverlay := make(map[string]any)
	for _, agentID := range agentIDs {
		agentDir := filepath.Join(repoDir, agentID)
		mainFile, _, ok := findMainMD(agentDir)
		if !ok {
			log.Printf("devagents: skipping %q for OpenCode — no main .md file found", agentID)
			continue
		}

		sourcePath := filepath.Join(agentDir, mainFile)
		content, err := os.ReadFile(sourcePath)
		if err != nil {
			if os.IsNotExist(err) {
				log.Printf("devagents: skipping %q for OpenCode — source not found", agentID)
				continue
			}
			return InjectionResult{}, fmt.Errorf("dev-agent %s: read failed: %w", agentID, err)
		}

		// Extract description from first non-empty content line.
		desc := agentID
		for _, line := range strings.Split(string(content), "\n") {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" || trimmed == "---" {
				continue
			}
			trimmed = strings.TrimLeft(trimmed, "# ")
			trimmed = strings.TrimRight(trimmed, "=")
			trimmed = strings.TrimSpace(trimmed)
			if trimmed != "" {
				desc = trimmed
				break
			}
		}

		agentOverlay[agentID] = map[string]any{
			"description": desc,
			"mode":        "all",
			"prompt":      string(content),
			"tools": map[string]any{
				"bash":            true,
				"edit":            true,
				"read":            true,
				"write":           true,
				"delegate":        true,
				"delegation_list": true,
				"delegation_read": true,
			},
		}

		// Also copy any sub-skills referenced by this agent.
		if _, _, subErr := copyAgentSubSkills(filepath.Join(agentDir, "skills"), adapter, homeDir); subErr != nil {
			return InjectionResult{}, fmt.Errorf("dev-agent %s: sub-skills failed: %w", agentID, subErr)
		}
	}

	if len(agentOverlay) == 0 {
		return InjectionResult{}, nil
	}

	overlay := map[string]any{"agent": agentOverlay}
	overlayJSON, err := json.Marshal(overlay)
	if err != nil {
		return InjectionResult{}, fmt.Errorf("dev-agents: marshal overlay: %w", err)
	}

	result, err := mergeJSONFile(settingsPath, overlayJSON)
	if err != nil {
		return InjectionResult{}, fmt.Errorf("dev-agents: merge into opencode.json: %w", err)
	}

	return InjectionResult{Changed: result.Changed, Files: []string{settingsPath}}, nil
}

// mergeJSONFile reads a JSON file, deep-merges the overlay, and writes back atomically.
func mergeJSONFile(path string, overlay []byte) (filemerge.WriteResult, error) {
	baseJSON, _ := os.ReadFile(path)
	if baseJSON == nil {
		baseJSON = []byte("{}")
	}
	merged, err := filemerge.MergeJSONObjects(baseJSON, overlay)
	if err != nil {
		return filemerge.WriteResult{}, err
	}
	return filemerge.WriteFileAtomic(path, merged, 0o644)
}

// agentFileSuffix returns the file suffix to use for the given agent type.
// VS Code uses ".agent.md"; all others (Claude Code, Cursor, etc.) use ".md".
func agentFileSuffix(agentID model.AgentID) string {
	return AgentFileSuffix(agentID)
}

// ExpectedAgentFiles returns the absolute paths InjectAgents will (or did)
// write for the given adapter and agent IDs. Verifier and uninstall callers
// use this so they stay in lock-step with the inject logic — same skip rules,
// same destination directory, same suffix, same source-file gating.
//
// Returns nil when the adapter is excluded from per-file injection:
//   - OpenCode (uses JSON config, not per-agent files)
//   - VS Code Copilot when Claude Code is also installed and VS Code is >=
//     1.116.0 (it reads agents from ~/.claude/agents/ directly)
//   - Adapters that do not implement subAgentInjector or report
//     SupportsSubAgents() == false
//
// Agents whose source dir lacks a usable top-level .md file are dropped — this
// matches inject's findMainMD-skip behaviour and prevents the verifier from
// expecting files that the inject step never wrote.
func ExpectedAgentFiles(homeDir string, adapter agents.Adapter, agentIDs []string, installedAgentIDs ...model.AgentID) []string {
	if adapter.Agent() == model.AgentOpenCode {
		return nil
	}
	if adapter.Agent() == model.AgentVSCodeCopilot && hasAgent(installedAgentIDs, model.AgentClaudeCode) {
		if vsVer := cachedVSCodeVersion(); vsVer != "" && versionAtLeast(vsVer, "1.116.0") {
			return nil
		}
	}
	sai, ok := adapter.(subAgentInjector)
	if !ok || !sai.SupportsSubAgents() {
		return nil
	}

	repoDir := filepath.Join(homeDir, ".informa-wizard", "dev-agents")
	destDir := sai.SubAgentsDir(homeDir)
	if adapter.Agent() == model.AgentVSCodeCopilot {
		destDir = filepath.Join(destDir, "prompts")
	}
	suffix := AgentFileSuffix(adapter.Agent())

	// Adapters that support skills also receive sub-skill files from each
	// agent's skills/ subtree (see copyAgentSubSkills). The verifier and
	// uninstall logic must include those, otherwise drift creeps back in.
	includeSubSkills := adapter.SupportsSkills()
	skillsDestRoot := ""
	if includeSubSkills {
		skillsDestRoot = adapter.SkillsDir(homeDir)
	}

	paths := make([]string, 0, len(agentIDs))
	for _, agentID := range agentIDs {
		agentDir := filepath.Join(repoDir, agentID)
		if _, _, ok := findMainMD(agentDir); !ok {
			continue
		}
		paths = append(paths, filepath.Join(destDir, agentID+suffix))
		if includeSubSkills {
			paths = append(paths, expectedAgentSubSkillFiles(filepath.Join(agentDir, "skills"), skillsDestRoot)...)
		}
	}
	return paths
}

// AgentFileSuffix is the exported equivalent of agentFileSuffix so callers
// outside this package (e.g. cli.componentPaths) can resolve the same suffix
// without duplicating the per-agent rule.
func AgentFileSuffix(agentID model.AgentID) string {
	if agentID == model.AgentVSCodeCopilot {
		return ".agent.md"
	}
	return ".md"
}

// ensureClaudeFrontmatter prepends Claude Code YAML frontmatter to the agent
// content if it doesn't already have one. The frontmatter is required for
// Claude Code's agent system (~/.claude/agents/).
func ensureClaudeFrontmatter(content []byte, agentID string, defaultModel string) []byte {
	text := string(content)

	// Already has frontmatter — don't add another.
	if strings.HasPrefix(strings.TrimSpace(text), "---") {
		return content
	}

	// Extract the first non-empty line as description.
	desc := agentID
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		// Strip markdown heading markers and decoration.
		trimmed = strings.TrimLeft(trimmed, "# ")
		trimmed = strings.TrimRight(trimmed, "=")
		trimmed = strings.TrimSpace(trimmed)
		if trimmed != "" {
			desc = trimmed
		}
		break
	}

	if defaultModel == "" {
		defaultModel = "sonnet"
	}

	frontmatter := "---\n" +
		"name: " + agentID + "\n" +
		"description: \"" + desc + "\"\n" +
		"model: " + defaultModel + "\n" +
		"memory: user\n" +
		"---\n\n"

	return []byte(frontmatter + text)
}

// vscodeVersion returns the VS Code version string (e.g., "1.116.0") or "" if unavailable.
func vscodeVersion() string {
	out, err := exec.Command("code", "--version").Output()
	if err != nil {
		return ""
	}
	// First line is the version number.
	line := strings.TrimSpace(strings.Split(string(out), "\n")[0])
	return line
}

// versionAtLeast returns true if version >= minVersion (semver comparison).
func versionAtLeast(version, minVersion string) bool {
	v := parseVersion(version)
	m := parseVersion(minVersion)
	for i := 0; i < 3; i++ {
		if v[i] > m[i] {
			return true
		}
		if v[i] < m[i] {
			return false
		}
	}
	return true // equal
}

func parseVersion(s string) [3]int {
	var parts [3]int
	for i, p := range strings.SplitN(s, ".", 3) {
		n, _ := strconv.Atoi(p)
		parts[i] = n
	}
	return parts
}

func hasAgent(ids []model.AgentID, target model.AgentID) bool {
	for _, id := range ids {
		if id == target {
			return true
		}
	}
	return false
}

