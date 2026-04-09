package monday

import (
	"fmt"
	"os"

	"gitlab.informa.tools/ai/wizard/informa-wizard/internal/agents"
	"gitlab.informa.tools/ai/wizard/informa-wizard/internal/components/filemerge"
	"gitlab.informa.tools/ai/wizard/informa-wizard/internal/model"
)

type InjectionResult struct {
	Changed bool
	Files   []string
}

// Inject configures the Monday.com MCP server for the given agent.
// The token is required — if empty, injection is skipped with a warning.
func Inject(homeDir string, adapter agents.Adapter, cfg model.MondayConfig) (InjectionResult, error) {
	if cfg.Token == "" {
		return InjectionResult{}, nil
	}

	if !adapter.SupportsMCP() {
		return InjectionResult{}, nil
	}

	switch adapter.MCPStrategy() {
	case model.StrategySeparateMCPFiles:
		return injectSeparateFile(homeDir, adapter, cfg.Token)
	case model.StrategyMergeIntoSettings:
		return injectMergeIntoSettings(homeDir, adapter, cfg.Token)
	case model.StrategyMCPConfigFile:
		return injectMCPConfigFile(homeDir, adapter, cfg.Token)
	case model.StrategyTOMLFile:
		// Monday MCP is not supported via TOML (Codex).
		return InjectionResult{}, nil
	default:
		return InjectionResult{}, fmt.Errorf("monday injector does not support MCP strategy %d for agent %q", adapter.MCPStrategy(), adapter.Agent())
	}
}

func injectSeparateFile(homeDir string, adapter agents.Adapter, token string) (InjectionResult, error) {
	path := adapter.MCPConfigPath(homeDir, "monday")
	writeResult, err := filemerge.WriteFileAtomic(path, mondayServerJSON(token), 0o644)
	if err != nil {
		return InjectionResult{}, err
	}
	return InjectionResult{Changed: writeResult.Changed, Files: []string{path}}, nil
}

func injectMergeIntoSettings(homeDir string, adapter agents.Adapter, token string) (InjectionResult, error) {
	settingsPath := adapter.SettingsPath(homeDir)
	if settingsPath == "" {
		return InjectionResult{}, nil
	}

	overlay := mondayOverlayJSON(adapter.Agent(), token)
	settingsWrite, err := mergeJSONFile(settingsPath, overlay)
	if err != nil {
		return InjectionResult{}, err
	}
	return InjectionResult{Changed: settingsWrite.Changed, Files: []string{settingsPath}}, nil
}

func injectMCPConfigFile(homeDir string, adapter agents.Adapter, token string) (InjectionResult, error) {
	path := adapter.MCPConfigPath(homeDir, "monday")
	if path == "" {
		return InjectionResult{}, nil
	}

	overlay := mondayOverlayJSON(adapter.Agent(), token)
	settingsWrite, err := mergeJSONFile(path, overlay)
	if err != nil {
		return InjectionResult{}, err
	}
	return InjectionResult{Changed: settingsWrite.Changed, Files: []string{path}}, nil
}

func mergeJSONFile(path string, overlay []byte) (filemerge.WriteResult, error) {
	baseJSON, err := osReadFile(path)
	if err != nil {
		return filemerge.WriteResult{}, err
	}

	merged, err := filemerge.MergeJSONObjects(baseJSON, overlay)
	if err != nil {
		return filemerge.WriteResult{}, err
	}

	return filemerge.WriteFileAtomic(path, merged, 0o644)
}

var osReadFile = func(path string) ([]byte, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read json file %q: %w", path, err)
	}
	return content, nil
}
