package screens

import (
	"strings"
	"testing"
)

func TestRenderCompleteSuccessShowsNextSteps(t *testing.T) {
	out := RenderComplete(CompletePayload{
		ConfiguredAgents:    1,
		InstalledComponents: 1,
	})

	if !strings.Contains(out, "Next steps") {
		t.Fatalf("missing Next steps section: %q", out)
	}
	if !strings.Contains(out, "sdd-new") {
		t.Fatalf("missing sdd-new hint: %q", out)
	}
}

func TestRenderCompleteSuccessDoesNotShowGGASection(t *testing.T) {
	out := RenderComplete(CompletePayload{
		ConfiguredAgents:    1,
		InstalledComponents: 1,
	})

	if strings.Contains(out, "GGA (per project)") {
		t.Fatalf("unexpected GGA section: %q", out)
	}
}
