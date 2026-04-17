package planner

import (
	"errors"
	"reflect"
	"testing"

	"gitlab.informa.tools/ai/wizard/informa-wizard/internal/model"
)

func TestTopologicalSortOrdersDependenciesFirst(t *testing.T) {
	deps := map[model.ComponentID][]model.ComponentID{
		model.ComponentSkills:   {model.ComponentSDD},
		model.ComponentSDD:      {model.ComponentEngram},
		model.ComponentEngram:   nil,
		model.ComponentContext7: nil,
	}

	ordered, err := TopologicalSort(deps)
	if err != nil {
		t.Fatalf("TopologicalSort() returned error: %v", err)
	}

	if !reflect.DeepEqual(ordered, []model.ComponentID{
		model.ComponentContext7,
		model.ComponentEngram,
		model.ComponentSDD,
		model.ComponentSkills,
	}) {
		t.Fatalf("TopologicalSort() order = %v", ordered)
	}
}

func TestApplySoftOrderingReordersWithoutAddingDependencies(t *testing.T) {
	ordered := []model.ComponentID{
		model.ComponentContext7,
		model.ComponentEngram,
		model.ComponentPersona,
		model.ComponentSDD,
	}

	result := applySoftOrdering(ordered, [][2]model.ComponentID{{model.ComponentPersona, model.ComponentEngram}})

	if !reflect.DeepEqual(result, []model.ComponentID{
		model.ComponentContext7,
		model.ComponentPersona,
		model.ComponentEngram,
		model.ComponentSDD,
	}) {
		t.Fatalf("applySoftOrdering() = %v", result)
	}

	// If the first component is absent, nothing should be added.
	result = applySoftOrdering([]model.ComponentID{model.ComponentEngram}, [][2]model.ComponentID{{model.ComponentPersona, model.ComponentEngram}})
	if !reflect.DeepEqual(result, []model.ComponentID{model.ComponentEngram}) {
		t.Fatalf("applySoftOrdering() should not add missing components (first absent), got %v", result)
	}
}

func TestApplySoftOrderingEdgeCases(t *testing.T) {
	pair := [][2]model.ComponentID{{model.ComponentPersona, model.ComponentEngram}}

	// Second absent — no-op, no panic
	result := applySoftOrdering([]model.ComponentID{model.ComponentPersona}, pair)
	if !reflect.DeepEqual(result, []model.ComponentID{model.ComponentPersona}) {
		t.Fatalf("second absent: expected [persona], got %v", result)
	}

	// Both absent — no-op
	result = applySoftOrdering([]model.ComponentID{model.ComponentSDD}, pair)
	if !reflect.DeepEqual(result, []model.ComponentID{model.ComponentSDD}) {
		t.Fatalf("both absent: expected [sdd], got %v", result)
	}

	// Already correct order — no-op (must not mutate)
	already := []model.ComponentID{model.ComponentPersona, model.ComponentEngram}
	result = applySoftOrdering(already, pair)
	if !reflect.DeepEqual(result, []model.ComponentID{model.ComponentPersona, model.ComponentEngram}) {
		t.Fatalf("already correct: expected [persona, engram], got %v", result)
	}

	// Input slice must NOT be mutated
	input := []model.ComponentID{model.ComponentEngram, model.ComponentPersona}
	_ = applySoftOrdering(input, pair)
	if !reflect.DeepEqual(input, []model.ComponentID{model.ComponentEngram, model.ComponentPersona}) {
		t.Fatalf("input slice was mutated")
	}
}

func TestApplySoftOrderingMVPPairWithFullSelection(t *testing.T) {
	// Simulates the real scenario: topo gives [context7, sdd, engram, skills]
	// The remaining soft pair {SDD, Engram} should result in SDD before Engram.
	ordered := []model.ComponentID{
		model.ComponentContext7,
		model.ComponentEngram,
		model.ComponentSDD,
		model.ComponentSkills,
	}

	result := applySoftOrdering(ordered, SoftOrderingConstraints())

	engramIdx, sddIdx := -1, -1
	for i, c := range result {
		switch c {
		case model.ComponentEngram:
			engramIdx = i
		case model.ComponentSDD:
			sddIdx = i
		}
	}

	if engramIdx < 0 || sddIdx < 0 {
		t.Fatalf("missing components in result: %v", result)
	}
	// Soft ordering: SDD must be before Engram (SDD writes base, Engram appends)
	if sddIdx > engramIdx {
		t.Fatalf("SDD (%d) must be before Engram (%d) after soft reorder, got %v", sddIdx, engramIdx, result)
	}
	// Skills must remain last
	if result[len(result)-1] != model.ComponentSkills {
		t.Fatalf("Skills must remain last, got %v", result)
	}
}

func TestTopologicalSortDetectsCycles(t *testing.T) {
	deps := map[model.ComponentID][]model.ComponentID{
		model.ComponentEngram: {model.ComponentSDD},
		model.ComponentSDD:    {model.ComponentEngram},
	}

	_, err := TopologicalSort(deps)
	if err == nil {
		t.Fatalf("TopologicalSort() expected cycle error")
	}

	if !errors.Is(err, ErrDependencyCycle) {
		t.Fatalf("TopologicalSort() error = %v, want ErrDependencyCycle", err)
	}
}
