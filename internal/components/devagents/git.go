package devagents

import (
	"errors"
	"fmt"
	"os/exec"

	"gitlab.informa.tools/ai/wizard/informa-wizard/internal/components/gitops"
)

// execCommand is the package-level command factory, injectable for tests.
var execCommand = exec.Command

// retryAttempts and retryBaseDelay control retry behaviour and are package-level
// variables so tests can override them to avoid multi-second delays.
var (
	retryAttempts  = gitops.RetryAttempts
	retryBaseDelay = gitops.RetryBaseDelay
)

// Clone clones repoURL into targetDir using git.
// Retries up to 3 times with exponential backoff for transient network failures.
// Returns an error if git is not found or all clone attempts fail.
func Clone(repoURL, targetDir string) error {
	return gitops.RunWithRetry(func() error {
		cmd := execCommand("git", "clone", repoURL, targetDir)
		out, err := cmd.CombinedOutput()
		if err != nil {
			if isGitNotFound(err) {
				return fmt.Errorf("git is required for dev-agents; install git and try again")
			}
			return fmt.Errorf("git clone failed: %s", string(out))
		}
		return nil
	}, isGitNotFound, retryAttempts, retryBaseDelay)
}

// Pull updates the repository in targetDir by running git pull.
// Retries up to 3 times with exponential backoff for transient network failures.
// Returns an error if git is not found or all pull attempts fail.
func Pull(targetDir string) error {
	return gitops.RunWithRetry(func() error {
		cmd := execCommand("git", "-C", targetDir, "pull")
		out, err := cmd.CombinedOutput()
		if err != nil {
			if isGitNotFound(err) {
				return fmt.Errorf("git is required for dev-agents; install git and try again")
			}
			return fmt.Errorf("git pull failed: %s", string(out))
		}
		return nil
	}, isGitNotFound, retryAttempts, retryBaseDelay)
}

// isGitNotFound reports whether err indicates the git binary was not found.
func isGitNotFound(err error) bool {
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return false
	}
	return errors.Is(err, exec.ErrNotFound) || isPathError(err)
}

// isPathError reports whether err is an *exec.Error indicating the binary was
// not found in PATH.
func isPathError(err error) bool {
	var execErr *exec.Error
	return errors.As(err, &execErr)
}
