package screens

import (
	"strings"

	"gitlab.informa.tools/ai/wizard/informa-wizard/internal/cli"
	"gitlab.informa.tools/ai/wizard/informa-wizard/internal/tui/styles"
)

// RenderHealth renders the health check results screen.
func RenderHealth(report cli.Report) string {
	var b strings.Builder

	b.WriteString(styles.TitleStyle.Render("Health Check"))
	b.WriteString("\n\n")

	if len(report.Checks) == 0 {
		b.WriteString(styles.SubtextStyle.Render("No checks available."))
		b.WriteString("\n")
	} else {
		for _, c := range report.Checks {
			icon := healthIcon(c.Status)
			b.WriteString(icon + "  ")
			b.WriteString(styles.HeadingStyle.Render(c.Name))
			if c.Message != "" {
				b.WriteString("\n     ")
				b.WriteString(styles.SubtextStyle.Render(c.Message))
			}
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(styles.HelpStyle.Render("enter / esc: back"))

	return b.String()
}

func healthIcon(s cli.CheckStatus) string {
	switch s {
	case cli.CheckPass:
		return "✅"
	case cli.CheckWarn:
		return "⚠ "
	case cli.CheckFail:
		return "❌"
	default:
		return "? "
	}
}
