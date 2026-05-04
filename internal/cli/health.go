package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"gitlab.informa.tools/ai/wizard/informa-wizard/internal/agents"
	"gitlab.informa.tools/ai/wizard/informa-wizard/internal/components/monday"
	"gitlab.informa.tools/ai/wizard/informa-wizard/internal/model"
	"gitlab.informa.tools/ai/wizard/informa-wizard/internal/state"
)

// CheckStatus is the result of a single health check.
type CheckStatus string

const (
	CheckPass CheckStatus = "pass"
	CheckWarn CheckStatus = "warn"
	CheckFail CheckStatus = "fail"
)

// Check holds the result of a single diagnostic check.
type Check struct {
	Name    string
	Status  CheckStatus
	Message string
}

// Report is a collection of Check results from RunHealth.
type Report struct {
	Checks []Check
}

// RunHealth performs all diagnostic checks and returns a Report.
func RunHealth(homeDir string) (Report, error) {
	var checks []Check

	// 1. git on PATH
	checks = append(checks, checkBinaryOnPath("git", "git --version"))

	// 2. go on PATH
	checks = append(checks, checkBinaryOnPath("go", "go version"))

	// 3. state.json present and valid
	stateCheck, st := checkStateFile(homeDir)
	checks = append(checks, stateCheck)

	// 4. wizard source repo present
	checks = append(checks, checkSourceRepo(homeDir))

	// 5. dev-skills repo cloned (if dev-skills installed)
	if st != nil && hasComponentInState(st.InstalledComponents, string(model.ComponentDevSkills)) {
		checks = append(checks, checkDir(
			"dev-skills repo",
			filepath.Join(homeDir, ".informa-wizard", "dev-skills", ".git"),
			"~/.informa-wizard/dev-skills/.git not found — run Update+Sync to reclone",
		))
	}

	// 6. dev-agents repo cloned (if dev-agents installed)
	if st != nil && hasComponentInState(st.InstalledComponents, string(model.ComponentDevAgents)) {
		checks = append(checks, checkDir(
			"dev-agents repo",
			filepath.Join(homeDir, ".informa-wizard", "dev-agents", ".git"),
			"~/.informa-wizard/dev-agents/.git not found — run Update+Sync to reclone",
		))
	}

	// 7. Each installed agent has config dir
	if st != nil {
		for _, agentStr := range st.InstalledAgents {
			agentID := model.AgentID(agentStr)
			adapter, err := agents.NewAdapter(agentID)
			if err != nil {
				checks = append(checks, Check{
					Name:    "agent config: " + agentStr,
					Status:  CheckWarn,
					Message: "unknown agent type",
				})
				continue
			}
			configDir := adapter.GlobalConfigDir(homeDir)
			if configDir == "" {
				continue
			}
			if _, err := os.Stat(configDir); err != nil {
				checks = append(checks, Check{
					Name:    "agent config: " + agentStr,
					Status:  CheckFail,
					Message: fmt.Sprintf("config dir missing: %s", configDir),
				})
			} else {
				checks = append(checks, Check{
					Name:    "agent config: " + agentStr,
					Status:  CheckPass,
					Message: configDir,
				})
			}
		}
	}

	// 8. Monday token valid (if monday.json exists)
	checks = append(checks, checkMondayToken(homeDir))

	// 9. VS Code on PATH (if vscode-copilot installed)
	if st != nil && hasComponentInState(st.InstalledAgents, string(model.AgentVSCodeCopilot)) {
		checks = append(checks, checkBinaryOnPath("code (VS Code)", "code --version"))
	}

	return Report{Checks: checks}, nil
}

// RunHealthCLI runs health checks and prints results to stdout.
func RunHealthCLI(homeDir string) {
	report, _ := RunHealth(homeDir)

	fmt.Println("Informa Wizard — Health Check")
	fmt.Println()

	for _, c := range report.Checks {
		icon := statusIcon(c.Status)
		fmt.Printf("%s  %s", icon, c.Name)
		if c.Message != "" {
			fmt.Printf(": %s", c.Message)
		}
		fmt.Println()
	}

	fmt.Println()

	pass, warn, fail := countResults(report)
	fmt.Printf("Results: %d pass, %d warn, %d fail\n", pass, warn, fail)
}

func statusIcon(s CheckStatus) string {
	switch s {
	case CheckPass:
		return "✅"
	case CheckWarn:
		return "⚠"
	case CheckFail:
		return "❌"
	default:
		return "?"
	}
}

func countResults(r Report) (pass, warn, fail int) {
	for _, c := range r.Checks {
		switch c.Status {
		case CheckPass:
			pass++
		case CheckWarn:
			warn++
		case CheckFail:
			fail++
		}
	}
	return
}

func checkBinaryOnPath(name, command string) Check {
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return Check{Name: name, Status: CheckFail, Message: "empty command"}
	}
	path, err := exec.LookPath(parts[0])
	if err != nil {
		return Check{Name: name, Status: CheckFail, Message: parts[0] + " not found on PATH"}
	}
	// Run command to get version info
	cmd := exec.Command(parts[0], parts[1:]...)
	out, err := cmd.Output()
	if err != nil {
		return Check{Name: name, Status: CheckWarn, Message: fmt.Sprintf("found at %s but version check failed: %v", path, err)}
	}
	version := strings.TrimSpace(strings.SplitN(string(out), "\n", 2)[0])
	return Check{Name: name, Status: CheckPass, Message: version}
}

func checkStateFile(homeDir string) (Check, *state.InstallState) {
	st, err := state.Read(homeDir)
	if err != nil {
		if os.IsNotExist(err) {
			return Check{
				Name:    "state.json",
				Status:  CheckWarn,
				Message: "not found — run installation first",
			}, nil
		}
		if errors.Is(err, state.ErrInvalidState) {
			return Check{
				Name:    "state.json",
				Status:  CheckFail,
				Message: "malformed: " + err.Error(),
			}, nil
		}
		return Check{
			Name:    "state.json",
			Status:  CheckFail,
			Message: "read failed: " + err.Error(),
		}, nil
	}
	return Check{
		Name:    "state.json",
		Status:  CheckPass,
		Message: fmt.Sprintf("%d agent(s), %d component(s)", len(st.InstalledAgents), len(st.InstalledComponents)),
	}, &st
}

func checkSourceRepo(homeDir string) Check {
	sourceDirFile := filepath.Join(homeDir, ".informa-wizard", "source-dir")
	data, err := os.ReadFile(sourceDirFile)
	if err != nil {
		return Check{
			Name:    "wizard source repo",
			Status:  CheckWarn,
			Message: "source-dir file not found — Update+Sync may not work",
		}
	}
	dir := strings.TrimSpace(string(data))
	if _, err := os.Stat(filepath.Join(dir, ".git")); err != nil {
		return Check{
			Name:    "wizard source repo",
			Status:  CheckFail,
			Message: fmt.Sprintf("%s: .git not found", dir),
		}
	}
	return Check{
		Name:    "wizard source repo",
		Status:  CheckPass,
		Message: dir,
	}
}

func checkDir(name, path, failMsg string) Check {
	if _, err := os.Stat(path); err != nil {
		return Check{Name: name, Status: CheckFail, Message: failMsg}
	}
	return Check{Name: name, Status: CheckPass, Message: filepath.Dir(path)}
}

func checkMondayToken(homeDir string) Check {
	mondayPath := filepath.Join(homeDir, ".informa-wizard", "monday.json")
	data, err := os.ReadFile(mondayPath)
	if err != nil {
		// Not configured — not a failure
		return Check{Name: "monday token", Status: CheckWarn, Message: "monday.json not found (component not configured)"}
	}

	var mc struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(data, &mc); err != nil {
		return Check{Name: "monday token", Status: CheckFail, Message: "monday.json is malformed JSON"}
	}

	if mc.Token == "" {
		return Check{Name: "monday token", Status: CheckWarn, Message: "token is empty"}
	}

	if err := monday.ValidateToken(mc.Token); err != nil {
		// Network failures during validation should not block the doctor — keep
		// it fast and offline-friendly. Surface as a Warn instead of Fail.
		if monday.IsNetworkError(err) {
			return Check{
				Name:    "monday token",
				Status:  CheckWarn,
				Message: "Token validation skipped — network unavailable",
			}
		}
		return Check{Name: "monday token", Status: CheckFail, Message: err.Error()}
	}

	return Check{Name: "monday token", Status: CheckPass, Message: "token is valid"}
}

func hasComponentInState(list []string, target string) bool {
	for _, s := range list {
		if s == target {
			return true
		}
	}
	return false
}
