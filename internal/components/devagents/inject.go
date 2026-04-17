package devagents

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"gitlab.informa.tools/ai/wizard/informa-wizard/internal/agents"
	"gitlab.informa.tools/ai/wizard/informa-wizard/internal/components/filemerge"
	"gitlab.informa.tools/ai/wizard/informa-wizard/internal/model"
)

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
func InjectAgents(homeDir string, adapter agents.Adapter, agentIDs []string, defaultModel string) (InjectionResult, error) {
	// OpenCode uses JSON config, not agent files.
	if adapter.Agent() == model.AgentOpenCode {
		return injectOpenCodeAgents(homeDir, adapter, agentIDs)
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
	}

	return InjectionResult{Changed: changed, Files: files}, nil
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

