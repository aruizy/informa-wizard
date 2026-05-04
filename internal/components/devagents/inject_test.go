package devagents

import (
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync/atomic"
	"testing"

	"gitlab.informa.tools/ai/wizard/informa-wizard/internal/agents/cursor"
	"gitlab.informa.tools/ai/wizard/informa-wizard/internal/agents/opencode"
)

// TestCachedVSCodeVersion_OnlyCalledOnce verifies that sync.OnceValue actually
// memoises the underlying lookup so we don't shell out to `code --version` on
// every ExpectedAgentFiles call.
func TestCachedVSCodeVersion_OnlyCalledOnce(t *testing.T) {
	// Save & restore production wiring so we don't leak state to other tests.
	origFn := vscodeVersionFn
	t.Cleanup(func() {
		vscodeVersionFn = origFn
		resetVSCodeVersionCache()
	})

	var calls atomic.Int64
	vscodeVersionFn = func() string {
		calls.Add(1)
		return "1.116.0"
	}
	resetVSCodeVersionCache()

	for i := 0; i < 5; i++ {
		if got := cachedVSCodeVersion(); got != "1.116.0" {
			t.Fatalf("cachedVSCodeVersion() = %q, want %q", got, "1.116.0")
		}
	}

	if n := calls.Load(); n != 1 {
		t.Errorf("underlying vscodeVersion should be called once; got %d invocations", n)
	}
}

// TestExpectedAgentFiles_MatchesInjectAgentsOutput is the drift lock-down test.
// ExpectedAgentFiles MUST emit the exact same paths as InjectAgents writes —
// otherwise the verifier and uninstall logic drift and either flag false
// positives or leave orphan files. This test exercises Cursor (an adapter that
// supports both sub-agents AND skills, so it covers the sub-skills path too).
func TestExpectedAgentFiles_MatchesInjectAgentsOutput(t *testing.T) {
	home := t.TempDir()

	// Build a fake dev-agents repo. "alpha" has sub-skills; "beta" doesn't.
	repoDir := filepath.Join(home, ".informa-wizard", "dev-agents")

	// alpha: main file + skills/skill-x/{SKILL.md,reference.md}
	alphaDir := filepath.Join(repoDir, "alpha")
	if err := os.MkdirAll(filepath.Join(alphaDir, "skills", "skill-x"), 0o755); err != nil {
		t.Fatalf("mkdir alpha: %v", err)
	}
	if err := os.WriteFile(filepath.Join(alphaDir, "alpha.md"), []byte("# alpha\n"), 0o644); err != nil {
		t.Fatalf("write alpha main: %v", err)
	}
	if err := os.WriteFile(filepath.Join(alphaDir, "skills", "skill-x", "SKILL.md"), []byte("skill\n"), 0o644); err != nil {
		t.Fatalf("write SKILL.md: %v", err)
	}
	if err := os.WriteFile(filepath.Join(alphaDir, "skills", "skill-x", "reference.md"), []byte("ref\n"), 0o644); err != nil {
		t.Fatalf("write reference.md: %v", err)
	}

	// beta: main file only, no skills dir.
	betaDir := filepath.Join(repoDir, "beta")
	if err := os.MkdirAll(betaDir, 0o755); err != nil {
		t.Fatalf("mkdir beta: %v", err)
	}
	if err := os.WriteFile(filepath.Join(betaDir, "beta.md"), []byte("# beta\n"), 0o644); err != nil {
		t.Fatalf("write beta main: %v", err)
	}

	adapter := cursor.NewAdapter()

	// Run inject; capture the actual files written.
	result, err := InjectAgents(home, adapter, []string{"alpha", "beta"}, "sonnet")
	if err != nil {
		t.Fatalf("InjectAgents: %v", err)
	}

	// Run the verifier-side enumerator on the same inputs.
	expected := ExpectedAgentFiles(home, adapter, []string{"alpha", "beta"})

	got := append([]string(nil), result.Files...)
	want := append([]string(nil), expected...)
	sort.Strings(got)
	sort.Strings(want)

	if !slices.Equal(got, want) {
		t.Errorf("ExpectedAgentFiles drifted from InjectAgents:\n  inject:   %v\n  expected: %v", got, want)
	}
}

// TestExpectedAgentFiles_SkipPathsMatchInject locks down the historically
// drift-prone skip paths: agents whose source dir lacks a usable .md file
// (findMainMD-skip), and the OpenCode adapter (which uses JSON config rather
// than per-agent files). Both are paths where a previous bug had inject
// silently skip while ExpectedAgentFiles still registered files — leaving
// verify with phantom expectations and uninstall with orphan deletes.
func TestExpectedAgentFiles_SkipPathsMatchInject(t *testing.T) {
	t.Run("agent-without-main-md-is-skipped-by-both", func(t *testing.T) {
		home := t.TempDir()
		repoDir := filepath.Join(home, ".informa-wizard", "dev-agents")

		// "alpha" has a usable main file. "broken" has only a subdir, no .md
		// at the top level — findMainMD returns ok=false, inject skips.
		alphaDir := filepath.Join(repoDir, "alpha")
		if err := os.MkdirAll(alphaDir, 0o755); err != nil {
			t.Fatalf("mkdir alpha: %v", err)
		}
		if err := os.WriteFile(filepath.Join(alphaDir, "alpha.md"), []byte("# alpha\n"), 0o644); err != nil {
			t.Fatalf("write alpha main: %v", err)
		}
		brokenDir := filepath.Join(repoDir, "broken", "nested-only-subdir")
		if err := os.MkdirAll(brokenDir, 0o755); err != nil {
			t.Fatalf("mkdir broken: %v", err)
		}

		adapter := cursor.NewAdapter()
		result, err := InjectAgents(home, adapter, []string{"alpha", "broken"}, "sonnet")
		if err != nil {
			t.Fatalf("InjectAgents: %v", err)
		}
		expected := ExpectedAgentFiles(home, adapter, []string{"alpha", "broken"})

		got := append([]string(nil), result.Files...)
		want := append([]string(nil), expected...)
		sort.Strings(got)
		sort.Strings(want)
		if !slices.Equal(got, want) {
			t.Errorf("findMainMD-skip drift:\n  inject:   %v\n  expected: %v", got, want)
		}
		// Belt and suspenders: assert "broken" is in neither list.
		joined := strings.Join(want, "\n")
		if strings.Contains(joined, "broken") {
			t.Errorf("'broken' agent should be skipped (no .md), but expected paths contain it:\n%s", joined)
		}
	})

	t.Run("opencode-adapter-returns-no-files", func(t *testing.T) {
		home := t.TempDir()
		repoDir := filepath.Join(home, ".informa-wizard", "dev-agents")
		alphaDir := filepath.Join(repoDir, "alpha")
		if err := os.MkdirAll(alphaDir, 0o755); err != nil {
			t.Fatalf("mkdir alpha: %v", err)
		}
		if err := os.WriteFile(filepath.Join(alphaDir, "alpha.md"), []byte("# alpha\n"), 0o644); err != nil {
			t.Fatalf("write alpha main: %v", err)
		}

		adapter := opencode.NewAdapter()
		expected := ExpectedAgentFiles(home, adapter, []string{"alpha"})
		if len(expected) != 0 {
			t.Errorf("ExpectedAgentFiles for OpenCode should be empty (uses JSON config); got %v", expected)
		}
	})
}
