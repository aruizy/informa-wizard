package screens

import (
	"fmt"
	"strings"

	"gitlab.informa.tools/ai/wizard/informa-wizard/internal/cli"
	"gitlab.informa.tools/ai/wizard/informa-wizard/internal/tui/styles"
	"gitlab.informa.tools/ai/wizard/informa-wizard/internal/update"
	"gitlab.informa.tools/ai/wizard/informa-wizard/internal/update/upgrade"
)

// previewMaxFilesPerComponent is the maximum number of file paths shown per
// component in the sync preview before truncation.
const previewMaxFilesPerComponent = 5

// RenderUpgradeSync handles all states of the combined update+sync screen.
//
// State logic:
//  1. operationRunning && upgradeReport == nil && upgradeErr == nil && phase == 0 → "Updating repositories..." with spinner
//  2. phase == 1 → show sync preview, await user confirmation
//  3. operationRunning && (upgradeReport != nil || upgradeErr != nil) → "Syncing configurations..." with spinner
//  4. !operationRunning && (upgradeReport != nil || upgradeErr != nil) → show combined results
//  5. Otherwise → show confirmation screen
func RenderUpgradeSync(results []update.UpdateResult, upgradeReport *upgrade.UpgradeReport, syncFilesChanged int, upgradeErr error, syncErr error, operationRunning bool, updateCheckDone bool, cursor int, spinnerFrame int, wizardNeedsRestart bool, phase int, preview cli.SyncPreview) string {
	var b strings.Builder

	b.WriteString(styles.TitleStyle.Render("Update + Sync"))
	b.WriteString("\n\n")

	// State 1: update is running (report not yet available)
	if operationRunning && upgradeReport == nil && upgradeErr == nil && phase == 0 {
		b.WriteString(styles.WarningStyle.Render(SpinnerChar(spinnerFrame) + "  Updating repositories..."))
		b.WriteString("\n\n")
		b.WriteString(styles.HelpStyle.Render("Please wait..."))
		return b.String()
	}

	// State 2: preview ready — show diff preview and await user confirmation.
	// Pass upgradeErr so any pull failures are visible above the preview;
	// otherwise the user couldn't tell that pulls failed before deciding to apply.
	if phase == 1 {
		b.WriteString(renderUpgradeSyncPreview(preview, upgradeErr))
		return b.String()
	}

	// State 3: update done, sync now running
	if operationRunning && (upgradeReport != nil || upgradeErr != nil) {
		if upgradeErr != nil {
			b.WriteString(styles.ErrorStyle.Render("✗ Update failed"))
		} else {
			b.WriteString(styles.SuccessStyle.Render("✓ Update complete"))
		}
		b.WriteString("\n\n")
		b.WriteString(styles.WarningStyle.Render(SpinnerChar(spinnerFrame) + "  Syncing configurations..."))
		b.WriteString("\n\n")
		b.WriteString(styles.HelpStyle.Render("Please wait..."))
		return b.String()
	}

	// State 4: both operations done — show combined results
	// Triggered when not running and either upgrade report or upgrade error is present.
	if !operationRunning && (upgradeReport != nil || upgradeErr != nil) {
		b.WriteString(renderUpgradeSyncResult(upgradeReport, syncFilesChanged, upgradeErr, syncErr))
		if wizardNeedsRestart {
			b.WriteString("\n")
			b.WriteString(styles.WarningStyle.Render("⚠ A new version of Informa Wizard was downloaded."))
			b.WriteString("\n")
			b.WriteString(styles.HeadingStyle.Render("Close the wizard and rebuild now?"))
			b.WriteString("\n\n")
			b.WriteString(styles.SubtextStyle.Render("  y / enter   yes — close, run go install, exit"))
			b.WriteString("\n")
			b.WriteString(styles.SubtextStyle.Render("  n / esc     no — go back to menu (rebuild later)"))
			b.WriteString("\n")
		}
		return b.String()
	}

	// State 5: confirmation screen
	b.WriteString(renderUpgradeSyncConfirm())
	return b.String()
}

func renderUpgradeSyncConfirm() string {
	var b strings.Builder

	b.WriteString(styles.UnselectedStyle.Render("This will perform two operations in sequence:"))
	b.WriteString("\n\n")

	b.WriteString("  " + styles.WarningStyle.Render("1.") + " " + styles.HeadingStyle.Render("Update repositories"))
	b.WriteString("\n")
	b.WriteString("     " + styles.SubtextStyle.Render("Pulls latest changes for informa-wizard, dev-skills, and dev-agents"))
	b.WriteString("\n\n")

	b.WriteString("  " + styles.WarningStyle.Render("2.") + " " + styles.HeadingStyle.Render("Sync configurations"))
	b.WriteString("\n")
	b.WriteString("     " + styles.SubtextStyle.Render("Re-applies dotfile configs to all detected agents"))
	b.WriteString("\n\n")

	b.WriteString(styles.HeadingStyle.Render("Press enter to begin"))
	b.WriteString("\n\n")
	b.WriteString(styles.HelpStyle.Render("enter: confirm • esc: back • q: quit"))

	return b.String()
}

func renderUpgradeSyncResult(report *upgrade.UpgradeReport, syncFilesChanged int, upgradeErr error, syncErr error) string {
	var b strings.Builder

	// Show the "Update Results" section only when a real upgrade ran (report has
	// per-tool results) or when the update phase produced an error worth showing.
	// The Update+Sync flow's pull phase does not populate Results, so we skip
	// the section in that case to avoid printing an empty "Update Results" box.
	hasUpgradeResults := report != nil && len(report.Results) > 0
	if hasUpgradeResults || upgradeErr != nil {
		if hasUpgradeResults {
			b.WriteString(styles.HeadingStyle.Render("Update Results"))
		} else {
			b.WriteString(styles.HeadingStyle.Render("Pull Results"))
		}
		b.WriteString("\n\n")
	}

	if upgradeErr != nil {
		label := "✗ Update failed: "
		if !hasUpgradeResults {
			label = "✗ Pull failed: "
		}
		b.WriteString(styles.ErrorStyle.Render(label + upgradeErr.Error()))
		b.WriteString("\n")
	} else if hasUpgradeResults {
		upgradeSucceeded, upgradeFailed, upgradeSkipped := 0, 0, 0

		for _, r := range report.Results {
			switch r.Status {
			case upgrade.UpgradeSucceeded:
				upgradeSucceeded++
				line := fmt.Sprintf("%s  %s → %s",
					r.ToolName,
					styles.SubtextStyle.Render(r.OldVersion),
					styles.SuccessStyle.Render(r.NewVersion),
				)
				b.WriteString("  " + styles.SuccessStyle.Render("✓") + "  " + line)
			case upgrade.UpgradeFailed:
				upgradeFailed++
				b.WriteString("  " + styles.ErrorStyle.Render("✗") + "  " + styles.ErrorStyle.Render(r.ToolName))
				if r.Err != nil {
					b.WriteString("\n     " + styles.SubtextStyle.Render(r.Err.Error()))
				}
			case upgrade.UpgradeSkipped:
				upgradeSkipped++
				hint := ""
				if r.ManualHint != "" {
					hint = "  " + styles.SubtextStyle.Render(r.ManualHint)
				}
				b.WriteString("  " + styles.SubtextStyle.Render("-") + "  " + styles.SubtextStyle.Render(r.ToolName+" (skipped)") + hint)
			}
			b.WriteString("\n")
		}

		// Upgrade summary
		parts := []string{}
		if upgradeSucceeded > 0 {
			parts = append(parts, styles.SuccessStyle.Render(fmt.Sprintf("%d upgraded", upgradeSucceeded)))
		}
		if upgradeFailed > 0 {
			parts = append(parts, styles.ErrorStyle.Render(fmt.Sprintf("%d failed", upgradeFailed)))
		}
		if upgradeSkipped > 0 {
			parts = append(parts, styles.SubtextStyle.Render(fmt.Sprintf("%d skipped", upgradeSkipped)))
		}
		if len(parts) > 0 {
			b.WriteString("  " + strings.Join(parts, "  "))
			b.WriteString("\n")
		}

		if report.BackupWarning != "" {
			b.WriteString("  " + styles.WarningStyle.Render("⚠ "+report.BackupWarning))
			b.WriteString("\n")
		}
	}

	if hasUpgradeResults || upgradeErr != nil {
		b.WriteString("\n")
	}

	// --- Sync section ---
	b.WriteString(styles.HeadingStyle.Render("Sync Results"))
	b.WriteString("\n\n")

	if syncErr != nil {
		b.WriteString("  " + styles.ErrorStyle.Render("✗ Sync failed: "+syncErr.Error()))
	} else if syncFilesChanged == 0 {
		b.WriteString("  " + styles.SubtextStyle.Render("No files needed updating"))
	} else {
		b.WriteString("  " + styles.SuccessStyle.Render("✓") + "  " + fmt.Sprintf("%s synchronized", styles.HeadingStyle.Render(fmt.Sprintf("%d file(s)", syncFilesChanged))))
	}

	b.WriteString("\n\n")
	b.WriteString(styles.HelpStyle.Render("enter: return • esc: back • q: quit"))

	return b.String()
}

// renderUpgradeSyncPreview renders the diff preview between pull and sync.
// It shows the components that will run and the files they would touch,
// truncating to previewMaxFilesPerComponent per component.
//
// If upgradeErr is non-nil, a warning banner is rendered at the top so the
// user can decide whether to proceed even though one or more pulls failed.
func renderUpgradeSyncPreview(preview cli.SyncPreview, upgradeErr error) string {
	var b strings.Builder

	if upgradeErr != nil {
		// errors.Join renders multi-line; keep continuation lines indented so
		// the warning banner stays visually coherent.
		b.WriteString(styles.WarningStyle.Render("⚠ Some pulls failed:"))
		b.WriteString("\n")
		for _, line := range strings.Split(upgradeErr.Error(), "\n") {
			b.WriteString(styles.WarningStyle.Render("    " + line))
			b.WriteString("\n")
		}
		b.WriteString(styles.SubtextStyle.Render("  Sync will run against the local copy as-is."))
		b.WriteString("\n\n")
	}

	b.WriteString(styles.HeadingStyle.Render("Sync Preview"))
	b.WriteString("\n\n")

	if len(preview.Components) == 0 {
		b.WriteString(styles.SubtextStyle.Render("  No files would be changed by sync."))
		b.WriteString("\n\n")
		b.WriteString(styles.HelpStyle.Render("enter: apply anyway • esc: cancel"))
		return b.String()
	}

	b.WriteString(styles.UnselectedStyle.Render("The following files will be modified or created:"))
	b.WriteString("\n\n")

	for _, comp := range preview.Components {
		total := len(comp.Files)

		// Component header line
		newLabel := ""
		modLabel := ""
		if comp.NewFiles > 0 {
			newLabel = styles.SuccessStyle.Render(fmt.Sprintf("%d new", comp.NewFiles))
		}
		if comp.ModifiedFiles > 0 {
			modLabel = styles.WarningStyle.Render(fmt.Sprintf("%d modified", comp.ModifiedFiles))
		}

		countParts := []string{}
		if newLabel != "" {
			countParts = append(countParts, newLabel)
		}
		if modLabel != "" {
			countParts = append(countParts, modLabel)
		}
		countStr := strings.Join(countParts, ", ")
		if countStr == "" {
			countStr = fmt.Sprintf("%d files", total)
		}

		b.WriteString("  ")
		b.WriteString(styles.HeadingStyle.Render(comp.ID))
		b.WriteString("  ")
		b.WriteString(countStr)
		b.WriteString("\n")

		// Show up to previewMaxFilesPerComponent file paths.
		shown := total
		if shown > previewMaxFilesPerComponent {
			shown = previewMaxFilesPerComponent
		}
		for i := 0; i < shown; i++ {
			file := comp.Files[i]
			var marker string
			if file.New {
				marker = styles.SuccessStyle.Render("+")
			} else {
				marker = styles.WarningStyle.Render("~")
			}
			b.WriteString("    ")
			b.WriteString(marker)
			b.WriteString(" ")
			b.WriteString(styles.SubtextStyle.Render(file.Path))
			b.WriteString("\n")
		}
		if total > previewMaxFilesPerComponent {
			b.WriteString("    ")
			b.WriteString(styles.SubtextStyle.Render(fmt.Sprintf("... and %d more", total-previewMaxFilesPerComponent)))
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	total := preview.TotalFiles()
	compCount := len(preview.Components)
	b.WriteString(styles.SubtextStyle.Render(fmt.Sprintf("Total: %d file(s) across %d component(s)", total, compCount)))
	b.WriteString("\n\n")
	b.WriteString(styles.HelpStyle.Render("enter: apply • esc: cancel"))

	return b.String()
}
