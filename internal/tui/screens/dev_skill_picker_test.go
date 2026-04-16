package screens_test

import (
	"strings"
	"testing"

	"gitlab.informa.tools/ai/wizard/informa-wizard/internal/components/devskills"
	"gitlab.informa.tools/ai/wizard/informa-wizard/internal/tui/screens"
)

// makeSkills is a test helper that builds a slice of DiscoveredSkill from
// parallel slices of ids, names, and descriptions.
func makeSkills(ids, names, descs []string) []devskills.DiscoveredSkill {
	skills := make([]devskills.DiscoveredSkill, len(ids))
	for i := range ids {
		skills[i] = devskills.DiscoveredSkill{
			ID:          ids[i],
			Name:        names[i],
			Description: descs[i],
		}
	}
	return skills
}

// TestRenderDevSkillPicker_AllSkillsShown verifies that all skill names appear
// in the rendered output.
func TestRenderDevSkillPicker_AllSkillsShown(t *testing.T) {
	skills := makeSkills(
		[]string{"alpha", "beta", "gamma"},
		[]string{"Alpha Skill", "Beta Skill", "Gamma Skill"},
		[]string{"desc a", "desc b", "desc c"},
	)
	checked := []bool{false, false, false}

	output := screens.RenderDevSkillPicker(skills, checked, 0)

	for _, name := range []string{"Alpha Skill", "Beta Skill", "Gamma Skill"} {
		if !strings.Contains(output, name) {
			t.Errorf("output missing skill name %q", name)
		}
	}
}

// TestRenderDevSkillPicker_NonePreSelected verifies that when all checked values
// are false, the output contains no "[x]" markers.
func TestRenderDevSkillPicker_NonePreSelected(t *testing.T) {
	skills := makeSkills(
		[]string{"alpha", "beta"},
		[]string{"Alpha Skill", "Beta Skill"},
		[]string{"", ""},
	)
	checked := []bool{false, false}

	output := screens.RenderDevSkillPicker(skills, checked, 0)

	if strings.Contains(output, "[x]") {
		t.Errorf("output contains [x] but all skills are unchecked; output: %q", output)
	}
}

// TestRenderDevSkillPicker_SelectedSkillMarked verifies that when checked[0] is
// true, the first skill row contains "[x]".
func TestRenderDevSkillPicker_SelectedSkillMarked(t *testing.T) {
	skills := makeSkills(
		[]string{"alpha", "beta"},
		[]string{"Alpha Skill", "Beta Skill"},
		[]string{"", ""},
	)
	checked := []bool{true, false}

	output := screens.RenderDevSkillPicker(skills, checked, 0)

	if !strings.Contains(output, "[x]") {
		t.Errorf("output does not contain [x] but checked[0]=true; output: %q", output)
	}
}

// TestRenderDevSkillPicker_DescriptionsShown verifies that skill descriptions
// appear in the rendered output.
func TestRenderDevSkillPicker_DescriptionsShown(t *testing.T) {
	skills := makeSkills(
		[]string{"alpha", "beta"},
		[]string{"Alpha Skill", "Beta Skill"},
		[]string{"Handles alpha patterns", "Handles beta patterns"},
	)
	checked := []bool{false, false}

	output := screens.RenderDevSkillPicker(skills, checked, 0)

	for _, desc := range []string{"Handles alpha patterns", "Handles beta patterns"} {
		if !strings.Contains(output, desc) {
			t.Errorf("output missing description %q", desc)
		}
	}
}
