package cli

import (
	"testing"

	"gitlab.informa.tools/ai/wizard/informa-wizard/internal/model"
	"gitlab.informa.tools/ai/wizard/informa-wizard/internal/planner"
	"gitlab.informa.tools/ai/wizard/informa-wizard/internal/verify"
)

func TestWithPostInstallNotesDoesNotChangeNonGGA(t *testing.T) {
	// Set GOBIN to a directory already in PATH so that withGoInstallPathNote
	// does not append a PATH guidance note for the Engram component.
	t.Setenv("GOBIN", "/usr/local/bin")

	report := verify.Report{Ready: true, FinalNote: "You're ready."}
	resolved := planner.ResolvedPlan{OrderedComponents: []model.ComponentID{model.ComponentEngram}}

	updated := withPostInstallNotes(report, resolved)
	if updated.FinalNote != report.FinalNote {
		t.Fatalf("FinalNote changed unexpectedly: %q", updated.FinalNote)
	}
}
