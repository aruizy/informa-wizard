package screens

import (
	"reflect"
	"testing"

	"gitlab.informa.tools/ai/wizard/informa-wizard/internal/model"
)

// TestNewClaudeModelPickerStateForPreset_KnownPresets verifies that each
// named preset (balanced/performance/economy) seeds the picker with the
// matching per-phase assignment map and the correct Preset value.
func TestNewClaudeModelPickerStateForPreset_KnownPresets(t *testing.T) {
	cases := []struct {
		name        string
		presetName  string
		wantPreset  ClaudeModelPreset
		wantPhases  map[string]model.ClaudeModelAlias
	}{
		{
			name:       "balanced",
			presetName: "balanced",
			wantPreset: ClaudePresetBalanced,
			wantPhases: model.ClaudeModelPresetBalanced(),
		},
		{
			name:       "performance",
			presetName: "performance",
			wantPreset: ClaudePresetPerformance,
			wantPhases: model.ClaudeModelPresetPerformance(),
		},
		{
			name:       "economy",
			presetName: "economy",
			wantPreset: ClaudePresetEconomy,
			wantPhases: model.ClaudeModelPresetEconomy(),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := NewClaudeModelPickerStateForPreset(tc.presetName)

			if got.Preset != tc.wantPreset {
				t.Errorf("Preset = %q, want %q", got.Preset, tc.wantPreset)
			}
			if got.InCustomMode {
				t.Errorf("InCustomMode = true, want false")
			}
			if !reflect.DeepEqual(got.CustomAssignments, tc.wantPhases) {
				t.Errorf("CustomAssignments mismatch:\n got = %v\nwant = %v", got.CustomAssignments, tc.wantPhases)
			}
		})
	}
}

// TestNewClaudeModelPickerStateForPreset_UnknownOrEmpty verifies that
// unknown presets, empty strings, AND "custom" all fall back to the
// balanced default — never to custom mode. Custom-mode re-entry is not
// supported because per-phase assignments are not persisted.
func TestNewClaudeModelPickerStateForPreset_UnknownOrEmpty(t *testing.T) {
	cases := []struct {
		name       string
		presetName string
	}{
		{"empty", ""},
		{"random", "random"},
		{"custom", "custom"},
	}

	want := NewClaudeModelPickerState()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := NewClaudeModelPickerStateForPreset(tc.presetName)

			if got.Preset != ClaudePresetBalanced {
				t.Errorf("Preset = %q, want %q (balanced fallback)", got.Preset, ClaudePresetBalanced)
			}
			if got.InCustomMode {
				t.Errorf("InCustomMode = true, want false (must NOT enter custom mode for %q)", tc.presetName)
			}
			if !reflect.DeepEqual(got.CustomAssignments, want.CustomAssignments) {
				t.Errorf("CustomAssignments mismatch for %q:\n got = %v\nwant = %v", tc.presetName, got.CustomAssignments, want.CustomAssignments)
			}
		})
	}
}

// TestIndexOfClaudePreset verifies the cursor index returned for each preset
// matches its display position, with unknown values defaulting to 0.
func TestIndexOfClaudePreset(t *testing.T) {
	cases := []struct {
		presetName string
		want       int
	}{
{"balanced", 0},
		{"performance", 1},
		{"economy", 2},
		// "custom" returns 0 (balanced) so cursor matches what
		// NewClaudeModelPickerStateForPreset("custom") seeds. The custom map is
		// not persisted anywhere recoverable, so re-entry falls back to balanced.
		{"custom", 0},
		{"", 0},
		{"unknown", 0},
	}

	for _, tc := range cases {
		t.Run(tc.presetName, func(t *testing.T) {
			got := IndexOfClaudePreset(tc.presetName)
			if got != tc.want {
				t.Errorf("IndexOfClaudePreset(%q) = %d, want %d", tc.presetName, got, tc.want)
			}
		})
	}
}
