package devskills

import (
	"errors"
	"fmt"
	"os/exec"
)

// execCommand is the package-level command factory, injectable for tests.
// Tests replace this with a fake that returns controlled exit codes / output.
var execCommand = exec.Command

// Clone clones repoURL into targetDir using git.
// Returns an error if git is not found or the clone fails.
func Clone(repoURL, targetDir string) error {
	cmd := execCommand("git", "clone", repoURL, targetDir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		if isGitNotFound(err) {
			return fmt.Errorf("git is required for dev-skills; install git and try again")
		}
		return fmt.Errorf("git clone failed: %s", string(out))
	}
	return nil
}

// Pull updates the repository in targetDir by running git pull.
// Returns an error if git is not found or the pull fails.
func Pull(targetDir string) error {
	cmd := execCommand("git", "-C", targetDir, "pull")
	out, err := cmd.CombinedOutput()
	if err != nil {
		if isGitNotFound(err) {
			return fmt.Errorf("git is required for dev-skills; install git and try again")
		}
		return fmt.Errorf("git pull failed: %s", string(out))
	}
	return nil
}

// isGitNotFound reports whether err indicates the git binary was not found.
func isGitNotFound(err error) bool {
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return false
	}
	// exec.LookPath error or exec.ErrNotFound wraps indicate binary not found.
	return errors.Is(err, exec.ErrNotFound) || isPathError(err)
}

// isPathError reports whether err is an *exec.Error or *os.PathError indicating
// the binary was not found in PATH.
func isPathError(err error) bool {
	var execErr *exec.Error
	return errors.As(err, &execErr)
}
