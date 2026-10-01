package lab

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aslanchikop/git-conflict-lab/internal/exercise"
)

// skipSlow skips end-to-end tests that generate real Git repositories in
// short mode (go test -short): they dominate the package runtime.
func skipSlow(t *testing.T) {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping slow end-to-end Git test in -short mode")
	}
}

// TestScenarioRegistryMatchesCatalog keeps the Git-side scenario registry
// in sync with the embedded exercise catalog.
func TestScenarioRegistryMatchesCatalog(t *testing.T) {
	catalog := exercise.List()
	if len(catalog) != len(scenarios) {
		t.Fatalf("catalog has %d exercises, scenario registry has %d", len(catalog), len(scenarios))
	}
	seen := map[string]bool{}
	for _, sc := range scenarios {
		if seen[sc.ID] {
			t.Errorf("duplicate scenario id %q", sc.ID)
		}
		seen[sc.ID] = true
		if sc.Branch == "" || sc.File == "" || sc.BaseRef == "" || sc.Strategy == "" {
			t.Errorf("scenario %q has empty mechanics fields: %+v", sc.ID, sc)
		}
		if len(sc.Files) > 0 && sc.Files[0] != sc.File {
			t.Errorf("scenario %q: Files[0]=%q does not match File=%q", sc.ID, sc.Files[0], sc.File)
		}
	}
	for _, spec := range catalog {
		if !seen[spec.ID] {
			t.Errorf("exercise %q has no scenario in the registry", spec.ID)
		}
	}
}

// TestGenerateCleansUpOnFailure verifies that a failed generation leaves
// no partial directory behind, so a retry starts clean (review finding
// B-003).
func TestGenerateCleansUpOnFailure(t *testing.T) {
	prev := generateScenario
	t.Cleanup(func() { generateScenario = prev })
	generateScenario = func(ctx context.Context, id, repo string) error {
		return fmt.Errorf("injected failure")
	}
	root := t.TempDir()
	_, err := Generate(context.Background(), root, "merge-basic")
	if err == nil || !strings.Contains(err.Error(), "injected failure") {
		t.Fatalf("err = %v, want injected failure", err)
	}
	if _, statErr := os.Stat(filepath.Join(root, "merge-basic")); !os.IsNotExist(statErr) {
		t.Fatal("partial exercise directory was left behind after a failed generation")
	}
}
