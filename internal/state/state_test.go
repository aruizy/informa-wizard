package state

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// TestWriteAndRead writes state and reads it back, verifying agents match.
func TestWriteAndRead(t *testing.T) {
	home := t.TempDir()
	agents := []string{"claude-code", "opencode"}

	if err := Write(home, agents, nil, nil, "", ""); err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	s, err := Read(home)
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}

	if !reflect.DeepEqual(s.InstalledAgents, agents) {
		t.Errorf("InstalledAgents = %v, want %v", s.InstalledAgents, agents)
	}
}

// TestWriteCreatesStateDir verifies that Write creates the .informa-wizard directory
// when it does not exist yet.
func TestWriteCreatesStateDir(t *testing.T) {
	home := t.TempDir()

	if err := Write(home, []string{"opencode"}, nil, nil, "", ""); err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	if _, err := os.Stat(filepath.Join(home, stateDir)); err != nil {
		t.Errorf("Write() did not create %q: %v", stateDir, err)
	}
}

// TestWriteStateFilePath verifies Path() returns the expected location.
func TestWriteStateFilePath(t *testing.T) {
	home := t.TempDir()
	got := Path(home)
	want := filepath.Join(home, ".informa-wizard", "state.json")
	if got != want {
		t.Errorf("Path() = %q, want %q", got, want)
	}
}

// TestReadMissing verifies that reading a non-existent file returns an error (not a panic).
func TestReadMissing(t *testing.T) {
	home := t.TempDir()
	// No Write — state.json does not exist.

	_, err := Read(home)
	if err == nil {
		t.Fatalf("Read() expected error for missing file, got nil")
	}

	if !os.IsNotExist(err) {
		t.Logf("Read() error = %v (non-nil, as expected — OS-level may differ)", err)
	}
}

// TestReadCorrupt verifies that writing garbage produces an error on read.
func TestReadCorrupt(t *testing.T) {
	home := t.TempDir()

	// Create the directory and write garbage JSON.
	if err := os.MkdirAll(filepath.Join(home, stateDir), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(Path(home), []byte("not valid json {{{{"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	_, err := Read(home)
	if err == nil {
		t.Fatalf("Read() expected error for corrupt JSON, got nil")
	}
}

// TestWriteOverwrite verifies that a second Write call replaces the previous state.
func TestWriteOverwrite(t *testing.T) {
	home := t.TempDir()

	if err := Write(home, []string{"claude-code"}, nil, nil, "", ""); err != nil {
		t.Fatalf("Write() first error = %v", err)
	}

	if err := Write(home, []string{"opencode", "gemini-cli"}, nil, nil, "", ""); err != nil {
		t.Fatalf("Write() second error = %v", err)
	}

	s, err := Read(home)
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}

	want := []string{"opencode", "gemini-cli"}
	if !reflect.DeepEqual(s.InstalledAgents, want) {
		t.Errorf("InstalledAgents after overwrite = %v, want %v", s.InstalledAgents, want)
	}
}

// TestWriteAndReadWithComponents verifies that components round-trip correctly.
func TestWriteAndReadWithComponents(t *testing.T) {
	home := t.TempDir()
	agents := []string{"claude-code"}
	components := []string{"sdd", "engram", "skills"}

	if err := Write(home, agents, components, nil, "", ""); err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	s, err := Read(home)
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}

	if !reflect.DeepEqual(s.InstalledComponents, components) {
		t.Errorf("InstalledComponents = %v, want %v", s.InstalledComponents, components)
	}
}

// TestWriteEmptyComponentsRoundTrips verifies that an empty component list
// serializes as [] (not omitted) and reads back as an empty slice.
func TestWriteEmptyComponentsRoundTrips(t *testing.T) {
	home := t.TempDir()

	if err := Write(home, []string{"claude-code"}, []string{}, nil, "", ""); err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	s, err := Read(home)
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}

	if s.InstalledComponents == nil {
		t.Errorf("InstalledComponents = nil, want empty slice (not nil)")
	}
	if len(s.InstalledComponents) != 0 {
		t.Errorf("InstalledComponents = %v, want empty", s.InstalledComponents)
	}
}

// TestWriteEmptyAgents verifies that an empty agent list round-trips correctly.
func TestWriteEmptyAgents(t *testing.T) {
	home := t.TempDir()

	if err := Write(home, []string{}, nil, nil, "", ""); err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	s, err := Read(home)
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}

	// An empty slice round-trips as an empty slice (not nil).
	if len(s.InstalledAgents) != 0 {
		t.Errorf("InstalledAgents = %v, want empty", s.InstalledAgents)
	}
}
