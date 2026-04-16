package devskills

import (
	"os"
	"os/exec"
	"testing"
)

// TestMain enables the "test helper subprocess" pattern:
// when the env var GO_TEST_HELPER_PROCESS is set, this process acts as a fake
// command and exits with the requested code, printing the fake output.
func TestMain(m *testing.M) {
	// If running as a helper subprocess, handle the fake command and exit.
	if os.Getenv("GO_TEST_HELPER_PROCESS") == "1" {
		output := os.Getenv("GO_TEST_HELPER_OUTPUT")
		exitCode := 0
		if os.Getenv("GO_TEST_HELPER_EXIT") == "1" {
			exitCode = 1
		}
		if output != "" {
			os.Stdout.WriteString(output) //nolint:errcheck
		}
		os.Exit(exitCode)
	}
	os.Exit(m.Run())
}

// fakeCommand returns an *exec.Cmd that re-invokes this test binary as a helper
// subprocess that exits with exitCode and writes output to combined output.
func fakeCommand(t *testing.T, output string, exitCode int) func(name string, args ...string) *exec.Cmd {
	t.Helper()
	testBin, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable(): %v", err)
	}
	exitEnv := "0"
	if exitCode != 0 {
		exitEnv = "1"
	}
	return func(name string, args ...string) *exec.Cmd {
		cmd := exec.Command(testBin, "-test.run=^$")
		cmd.Env = append(os.Environ(),
			"GO_TEST_HELPER_PROCESS=1",
			"GO_TEST_HELPER_OUTPUT="+output,
			"GO_TEST_HELPER_EXIT="+exitEnv,
		)
		return cmd
	}
}

// TestClone_Success verifies that Clone returns nil when git exits with code 0.
func TestClone_Success(t *testing.T) {
	orig := execCommand
	t.Cleanup(func() { execCommand = orig })
	execCommand = fakeCommand(t, "", 0)

	if err := Clone("https://example.com/repo.git", t.TempDir()); err != nil {
		t.Errorf("Clone() error = %v, want nil", err)
	}
}

// TestClone_Failure verifies that Clone returns an error containing the git
// output and the prefix "git clone failed:" when git exits with a non-zero code.
func TestClone_Failure(t *testing.T) {
	orig := execCommand
	t.Cleanup(func() { execCommand = orig })
	execCommand = fakeCommand(t, "repository not found", 1)

	err := Clone("https://example.com/repo.git", t.TempDir())
	if err == nil {
		t.Fatal("Clone() error = nil, want non-nil")
	}
	msg := err.Error()
	if len(msg) < len("git clone failed:") || msg[:len("git clone failed:")] != "git clone failed:" {
		t.Errorf("Clone() error = %q, want prefix %q", msg, "git clone failed:")
	}
}

// TestPull_Success verifies that Pull returns nil when git exits with code 0.
func TestPull_Success(t *testing.T) {
	orig := execCommand
	t.Cleanup(func() { execCommand = orig })
	execCommand = fakeCommand(t, "", 0)

	if err := Pull(t.TempDir()); err != nil {
		t.Errorf("Pull() error = %v, want nil", err)
	}
}

// TestPull_Failure verifies that Pull returns an error with prefix "git pull failed:"
// when git exits with a non-zero code.
func TestPull_Failure(t *testing.T) {
	orig := execCommand
	t.Cleanup(func() { execCommand = orig })
	execCommand = fakeCommand(t, "not a git repository", 1)

	err := Pull(t.TempDir())
	if err == nil {
		t.Fatal("Pull() error = nil, want non-nil")
	}
	msg := err.Error()
	if len(msg) < len("git pull failed:") || msg[:len("git pull failed:")] != "git pull failed:" {
		t.Errorf("Pull() error = %q, want prefix %q", msg, "git pull failed:")
	}
}
