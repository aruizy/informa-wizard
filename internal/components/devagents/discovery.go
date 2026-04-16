package devagents

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// DiscoveredAgent represents an agent found in the cloned dev-orchestrators repository.
type DiscoveredAgent struct {
	ID          string
	Name        string
	Description string
	// MainFile is the filename of the primary .md file (relative to the agent dir).
	MainFile string
}

// DefaultRepoURL is the default HTTPS git URL for the dev-orchestrators repository.
const DefaultRepoURL = "https://gitlab.informa.tools/ai/agents/dev-orchestrators.git"

// DiscoverAgents scans repoDir for agent subdirectories and returns the
// discovered agents sorted alphabetically by ID.
//
// Each agent must be a top-level subdirectory of repoDir containing at least
// one .md file that is not README.md. The first such file (alphabetically) is
// the main file; its first non-empty line is used as the description.
// Subdirectories without a qualifying .md file are silently skipped.
func DiscoverAgents(repoDir string) ([]DiscoveredAgent, error) {
	entries, err := os.ReadDir(repoDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("devagents: repo directory not found at %q", repoDir)
		}
		return nil, fmt.Errorf("devagents: reading repo directory: %w", err)
	}

	var results []DiscoveredAgent
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		id := entry.Name()
		agentDir := filepath.Join(repoDir, id)

		mainFile, description, ok := findMainMD(agentDir)
		if !ok {
			continue
		}

		results = append(results, DiscoveredAgent{
			ID:          id,
			Name:        toTitleCase(id),
			Description: description,
			MainFile:    mainFile,
		})
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].ID < results[j].ID
	})

	return results, nil
}

// findMainMD finds the first non-README .md file in agentDir (alphabetically).
// Returns (filename, description, true) when found, or ("", "", false) when none.
// Description is the first non-empty non-heading line of the file.
func findMainMD(agentDir string) (filename, description string, ok bool) {
	entries, err := os.ReadDir(agentDir)
	if err != nil {
		return "", "", false
	}

	// Collect candidate .md files sorted alphabetically.
	var candidates []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(strings.ToLower(name), ".md") {
			continue
		}
		if strings.EqualFold(name, "README.md") {
			continue
		}
		candidates = append(candidates, name)
	}

	sort.Strings(candidates)

	if len(candidates) == 0 {
		return "", "", false
	}

	mainFile := candidates[0]
	data, err := os.ReadFile(filepath.Join(agentDir, mainFile))
	if err != nil {
		return mainFile, "", true
	}

	description = firstContentLine(string(data))
	return mainFile, description, true
}

// firstContentLine returns the first non-empty, non-heading line of text.
// It skips YAML frontmatter (delimited by ---), blank lines, and Markdown
// heading lines (starting with #).
func firstContentLine(content string) string {
	lines := strings.Split(content, "\n")

	// Skip YAML frontmatter if present.
	start := 0
	if len(lines) > 0 && strings.TrimSpace(lines[0]) == "---" {
		for i := 1; i < len(lines); i++ {
			if strings.TrimSpace(lines[i]) == "---" {
				start = i + 1
				break
			}
		}
	}

	for _, line := range lines[start:] {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		// Skip Markdown headings.
		if strings.HasPrefix(trimmed, "#") {
			continue
		}
		return trimmed
	}

	return ""
}

// toTitleCase converts a hyphenated or underscore-separated directory name to a
// title-cased display name.
// e.g. "complex-problem-solving" -> "Complex Problem Solving"
func toTitleCase(s string) string {
	// Replace underscores with hyphens for uniform splitting.
	s = strings.ReplaceAll(s, "_", "-")
	parts := strings.Split(s, "-")
	for i, p := range parts {
		if len(p) > 0 {
			parts[i] = strings.ToUpper(p[:1]) + p[1:]
		}
	}
	return strings.Join(parts, " ")
}
