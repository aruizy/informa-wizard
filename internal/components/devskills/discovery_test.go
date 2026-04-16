package devskills

import (
	"os"
	"path/filepath"
	"testing"
)

// writeSkillMD creates skills/<id>/SKILL.md in repoDir with the given content.
func writeSkillMD(t *testing.T, repoDir, id, content string) {
	t.Helper()
	dir := filepath.Join(repoDir, "skills", id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll(%q): %v", dir, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile SKILL.md: %v", err)
	}
}

// makeSkillsDir creates the skills/ directory in repoDir (but no subdirectories).
func makeSkillsDir(t *testing.T, repoDir string) {
	t.Helper()
	dir := filepath.Join(repoDir, "skills")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll skills/: %v", err)
	}
}

const frontmatterTpl = "---\nname: %s\ndescription: %s\n---\n"

// TestDiscoverSkills_HappyPath verifies that 3 skill dirs with SKILL.md each
// produce 3 results sorted alphabetically by ID.
func TestDiscoverSkills_HappyPath(t *testing.T) {
	repoDir := t.TempDir()
	writeSkillMD(t, repoDir, "zebra-skill", "---\nname: Zebra Skill\ndescription: Z skill\n---\n")
	writeSkillMD(t, repoDir, "alpha-skill", "---\nname: Alpha Skill\ndescription: A skill\n---\n")
	writeSkillMD(t, repoDir, "middle-skill", "---\nname: Middle Skill\ndescription: M skill\n---\n")

	skills, err := DiscoverSkills(repoDir)
	if err != nil {
		t.Fatalf("DiscoverSkills() error = %v", err)
	}
	if len(skills) != 3 {
		t.Fatalf("len(skills) = %d, want 3", len(skills))
	}

	// Must be sorted alphabetically by ID.
	if skills[0].ID != "alpha-skill" {
		t.Errorf("skills[0].ID = %q, want alpha-skill", skills[0].ID)
	}
	if skills[1].ID != "middle-skill" {
		t.Errorf("skills[1].ID = %q, want middle-skill", skills[1].ID)
	}
	if skills[2].ID != "zebra-skill" {
		t.Errorf("skills[2].ID = %q, want zebra-skill", skills[2].ID)
	}
}

// TestDiscoverSkills_SkipsSubdirWithoutSkillMD verifies that a subdirectory
// lacking SKILL.md is silently omitted from results.
func TestDiscoverSkills_SkipsSubdirWithoutSkillMD(t *testing.T) {
	repoDir := t.TempDir()
	writeSkillMD(t, repoDir, "has-skill", "---\nname: Has Skill\n---\n")

	// Create a subdir WITHOUT SKILL.md.
	noSkillDir := filepath.Join(repoDir, "skills", "no-skill")
	if err := os.MkdirAll(noSkillDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	skills, err := DiscoverSkills(repoDir)
	if err != nil {
		t.Fatalf("DiscoverSkills() error = %v", err)
	}
	if len(skills) != 1 {
		t.Fatalf("len(skills) = %d, want 1 (no-skill dir should be skipped)", len(skills))
	}
	if skills[0].ID != "has-skill" {
		t.Errorf("skills[0].ID = %q, want has-skill", skills[0].ID)
	}
}

// TestDiscoverSkills_ParsesFrontmatter verifies that name and description are
// correctly extracted from well-formed YAML frontmatter.
func TestDiscoverSkills_ParsesFrontmatter(t *testing.T) {
	repoDir := t.TempDir()
	content := "---\nname: Java Development\ndescription: Expert Java 21+ patterns\n---\nsome body text\n"
	writeSkillMD(t, repoDir, "java-development", content)

	skills, err := DiscoverSkills(repoDir)
	if err != nil {
		t.Fatalf("DiscoverSkills() error = %v", err)
	}
	if len(skills) != 1 {
		t.Fatalf("len(skills) = %d, want 1", len(skills))
	}

	s := skills[0]
	if s.Name != "Java Development" {
		t.Errorf("Name = %q, want %q", s.Name, "Java Development")
	}
	if s.Description != "Expert Java 21+ patterns" {
		t.Errorf("Description = %q, want %q", s.Description, "Expert Java 21+ patterns")
	}
}

// TestDiscoverSkills_MissingFrontmatterDefaultsToDirectoryName verifies that
// when SKILL.md has no frontmatter the Name defaults to a title-cased
// version of the directory name and Description defaults to "".
func TestDiscoverSkills_MissingFrontmatterDefaultsToDirectoryName(t *testing.T) {
	repoDir := t.TempDir()
	writeSkillMD(t, repoDir, "java-development", "# Java Development\nno frontmatter here\n")

	skills, err := DiscoverSkills(repoDir)
	if err != nil {
		t.Fatalf("DiscoverSkills() error = %v", err)
	}
	if len(skills) != 1 {
		t.Fatalf("len(skills) = %d, want 1", len(skills))
	}

	s := skills[0]
	if s.Name != "Java Development" {
		t.Errorf("Name = %q, want %q", s.Name, "Java Development")
	}
	if s.Description != "" {
		t.Errorf("Description = %q, want empty string", s.Description)
	}
}

// TestDiscoverSkills_EmptySkillsDir verifies that an empty skills/ directory
// returns an empty slice with no error.
func TestDiscoverSkills_EmptySkillsDir(t *testing.T) {
	repoDir := t.TempDir()
	makeSkillsDir(t, repoDir)

	skills, err := DiscoverSkills(repoDir)
	if err != nil {
		t.Fatalf("DiscoverSkills() error = %v, want nil", err)
	}
	if len(skills) != 0 {
		t.Errorf("len(skills) = %d, want 0", len(skills))
	}
}

// TestDiscoverSkills_MissingSkillsDir verifies that a missing skills/ directory
// returns a non-nil error.
func TestDiscoverSkills_MissingSkillsDir(t *testing.T) {
	repoDir := t.TempDir()
	// Do NOT create skills/ dir.

	_, err := DiscoverSkills(repoDir)
	if err == nil {
		t.Fatal("DiscoverSkills() error = nil, want non-nil when skills/ does not exist")
	}
}

// TestDiscoverSkills_SortedAlphabetically verifies that results are sorted by
// ID regardless of the order returned by the filesystem.
func TestDiscoverSkills_SortedAlphabetically(t *testing.T) {
	repoDir := t.TempDir()
	ids := []string{"zz-skill", "aa-skill", "mm-skill", "bb-skill"}
	for _, id := range ids {
		writeSkillMD(t, repoDir, id, "---\nname: "+id+"\n---\n")
	}

	skills, err := DiscoverSkills(repoDir)
	if err != nil {
		t.Fatalf("DiscoverSkills() error = %v", err)
	}
	if len(skills) != 4 {
		t.Fatalf("len(skills) = %d, want 4", len(skills))
	}

	want := []string{"aa-skill", "bb-skill", "mm-skill", "zz-skill"}
	for i, s := range skills {
		if s.ID != want[i] {
			t.Errorf("skills[%d].ID = %q, want %q", i, s.ID, want[i])
		}
	}
}
