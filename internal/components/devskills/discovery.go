package devskills

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// DiscoveredSkill represents a skill found in the cloned dev-skills repository.
type DiscoveredSkill struct {
	ID          string
	Name        string
	Description string
}

// KnownSkills returns the hardcoded catalog of skills available in the
// dev-skills repository. Used by the TUI before the repo is cloned.
func KnownSkills() []DiscoveredSkill {
	return []DiscoveredSkill{
		{ID: "context0-instructions", Name: "Context0 Instructions", Description: "AI-optimized documentation in docs/instructions/"},
		{ID: "human-documentation", Name: "Human Documentation", Description: "Human-readable docs generation and maintenance"},
		{ID: "informads-development", Name: "InformaDS Development", Description: "InformaDS React component library patterns"},
		{ID: "java-development", Name: "Java Development", Description: "Java 21+ idiomatic code, Spring Boot, clean architecture"},
		{ID: "java-testing", Name: "Java Testing", Description: "JUnit 6, Spring Boot testing, JaCoCo coverage"},
		{ID: "skill-creator", Name: "Skill Creator", Description: "Create, evaluate and iterate new AI agent skills"},
	}
}

// DiscoverSkills scans the skills/ subdirectory of repoDir and returns the
// discovered skills sorted alphabetically by ID.
//
// Each skill must be a directory inside skills/ containing a SKILL.md file.
// Subdirectories without SKILL.md are silently skipped.
// If the skills/ directory does not exist, a non-nil error is returned.
func DiscoverSkills(repoDir string) ([]DiscoveredSkill, error) {
	skillsDir := filepath.Join(repoDir, "skills")
	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("devskills: skills directory not found in %q", repoDir)
		}
		return nil, fmt.Errorf("devskills: reading skills directory: %w", err)
	}

	var results []DiscoveredSkill
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		id := entry.Name()
		skillMD := filepath.Join(skillsDir, id, "SKILL.md")
		data, readErr := os.ReadFile(skillMD)
		if readErr != nil {
			// No SKILL.md — silently skip.
			continue
		}
		name, description := parseFrontmatter(data, id)
		results = append(results, DiscoveredSkill{
			ID:          id,
			Name:        name,
			Description: description,
		})
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].ID < results[j].ID
	})

	return results, nil
}

// parseFrontmatter extracts name and description from YAML frontmatter in SKILL.md.
// The frontmatter is delimited by lines containing only "---".
// If frontmatter is absent or malformed, name defaults to the title-cased dirName
// and description defaults to "".
func parseFrontmatter(data []byte, dirName string) (name, description string) {
	content := string(data)
	lines := strings.Split(content, "\n")

	// Find the opening and closing --- delimiters.
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return toTitleCase(dirName), ""
	}

	closeIdx := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			closeIdx = i
			break
		}
	}
	if closeIdx < 0 {
		return toTitleCase(dirName), ""
	}

	// Parse the YAML lines between the two --- delimiters.
	frontmatter := lines[1:closeIdx]
	for _, line := range frontmatter {
		key, val, ok := parseYAMLLine(line)
		if !ok {
			continue
		}
		switch key {
		case "name":
			name = val
		case "description":
			description = val
		}
	}

	if name == "" {
		name = toTitleCase(dirName)
	}
	return name, description
}

// parseYAMLLine parses a simple "key: value" YAML line.
func parseYAMLLine(line string) (key, value string, ok bool) {
	idx := strings.Index(line, ":")
	if idx < 0 {
		return "", "", false
	}
	key = strings.TrimSpace(line[:idx])
	value = strings.TrimSpace(line[idx+1:])
	// Strip surrounding quotes if present.
	if len(value) >= 2 && ((value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'')) {
		value = value[1 : len(value)-1]
	}
	return key, value, true
}

// toTitleCase converts a hyphenated directory name to a title-cased display name.
// e.g. "java-development" -> "Java Development"
func toTitleCase(s string) string {
	parts := strings.Split(s, "-")
	for i, p := range parts {
		if len(p) > 0 {
			parts[i] = strings.ToUpper(p[:1]) + p[1:]
		}
	}
	return strings.Join(parts, " ")
}
