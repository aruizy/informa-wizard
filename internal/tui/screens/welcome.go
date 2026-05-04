package screens

import (
	"fmt"
	"strings"
	"time"

	"gitlab.informa.tools/ai/wizard/informa-wizard/internal/tui/styles"
)

// WelcomeOptions returns the welcome menu options.
// When showProfiles is true, an "OpenCode SDD Profiles" option is inserted
// between "Create your own Agent" and "Manage backups".
// profileCount is used to show a badge with the current profile count.
// When hasEngines is false, "Create your own Agent" is shown as disabled
// (labelled "(no agents)") to signal that no supported AI engine is installed.
func WelcomeOptions(showProfiles bool, profileCount int, hasEngines bool) []string {
	agentLabel := "Create your own Agent"
	if !hasEngines {
		agentLabel = "Create your own Agent (no agents)"
	}

	opts := []string{
		"Start installation",
		"Update + Sync",
		"View installation",
		"Uninstall component",
		"Health check",
		"Configure Monday",
		"Configure models",
		"Switch Claude preset",
		agentLabel,
	}

	if showProfiles {
		profileLabel := "OpenCode SDD Profiles"
		if profileCount > 0 {
			profileLabel = fmt.Sprintf("OpenCode SDD Profiles (%d)", profileCount)
		}
		opts = append(opts, profileLabel)
	}

	opts = append(opts, "Manage backups", "Quit")
	return opts
}

func RenderWelcome(cursor int, version string, updateBanner string, showProfiles bool, profileCount int, hasEngines bool, commitDate *time.Time) string {
	var b strings.Builder

	b.WriteString(styles.RenderLogo())
	b.WriteString("\n\n")

	if commitDate != nil {
		b.WriteString(styles.SubtextStyle.Render(fmt.Sprintf("Last update: %s", commitDate.Local().Format("2006-01-02 15:04"))))
		b.WriteString("\n")
	}

	if updateBanner != "" {
		b.WriteString(styles.WarningStyle.Render(updateBanner))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(styles.HeadingStyle.Render("Menu"))
	b.WriteString("\n\n")
	b.WriteString(renderOptions(WelcomeOptions(showProfiles, profileCount, hasEngines), cursor))
	b.WriteString("\n")
	b.WriteString(styles.HelpStyle.Render("j/k: navigate • enter: select • q: quit"))

	return styles.FrameStyle.Render(b.String())
}
