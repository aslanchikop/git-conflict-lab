package exercise

import (
	"errors"
	"strings"
	"testing"
)

// TestCatalogLoads verifies the embedded catalog parses and validates.
func TestCatalogLoads(t *testing.T) {
	if len(catalog()) == 0 {
		t.Fatal("catalog is empty")
	}
}

// TestListSortedAndCopy checks ordering and defensive copying of List.
func TestListSortedAndCopy(t *testing.T) {
	specs := List()
	if len(specs) < 1 {
		t.Fatal("List returned no exercises")
	}
	for i := 1; i < len(specs); i++ {
		if specs[i-1].ID >= specs[i].ID {
			t.Fatalf("List not sorted by ID: %q before %q", specs[i-1].ID, specs[i].ID)
		}
	}

	mutated := List()
	if len(mutated) == 0 {
		t.Fatal("second List returned no exercises")
	}
	mutated[0].ID = "mutated-id"
	mutated[0].Hints[0] = "mutated-hint"
	after := List()
	for _, spec := range after {
		if spec.ID == "mutated-id" {
			t.Fatal("List result aliases catalog state: ID mutation visible")
		}
		if len(spec.Hints) > 0 && spec.Hints[0] == "mutated-hint" {
			t.Fatal("List result aliases catalog state: hint mutation visible")
		}
	}
}

// TestGetMergeBasic verifies the known exercise with all fields populated.
func TestGetMergeBasic(t *testing.T) {
	spec, err := Get("merge-basic")
	if err != nil {
		t.Fatalf("Get(merge-basic): %v", err)
	}
	if spec.ID != "merge-basic" {
		t.Errorf("ID = %q, want merge-basic", spec.ID)
	}
	for _, field := range map[string]string{
		"title":       spec.Title,
		"difficulty":  spec.Difficulty,
		"objective":   spec.Objective,
		"description": spec.Description,
	} {
		if strings.TrimSpace(field) == "" {
			t.Error("Get(merge-basic): required field is empty")
		}
	}
	if spec.Difficulty != "easy" {
		t.Errorf("difficulty = %q, want easy", spec.Difficulty)
	}
	if len(spec.Hints) != 3 {
		t.Errorf("hints = %d, want 3", len(spec.Hints))
	}
}

// TestGetNotFound checks ErrNotFound semantics.
func TestGetNotFound(t *testing.T) {
	for _, id := range []string{"nope", ""} {
		if _, err := Get(id); !errors.Is(err, ErrNotFound) {
			t.Errorf("Get(%q) error = %v, want ErrNotFound", id, err)
		}
	}
}

// TestCatalogSpecsValid re-runs the same validation rules the package
// applies at init, to catch bad catalog data early in tests.
func TestCatalogSpecsValid(t *testing.T) {
	for _, spec := range List() {
		if err := spec.Validate(); err != nil {
			t.Errorf("spec %q failed validation: %v", spec.ID, err)
		}
	}
}
