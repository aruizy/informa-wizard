package devagents

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

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
func InjectAgents(homeDir string, adapter agents.Adapter, agentIDs []string) (InjectionResult, error) {
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

// agentFileSuffix returns the file suffix to use for the given agent type.
// VS Code uses ".agent.md"; all others (Cursor, etc.) use ".md".
func agentFileSuffix(agentID model.AgentID) string {
	if agentID == model.AgentVSCodeCopilot {
		return ".agent.md"
	}
	return ".md"
}

