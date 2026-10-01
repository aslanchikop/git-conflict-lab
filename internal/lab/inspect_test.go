package lab

import (
	"context"
	"os"
	"strings"
	"testing"
)

func TestInspectExerciseVersions(t *testing.T) {
	skipSlow(t)
	for _, tc := range []struct {
		id            string
		baseExists    bool
		featureExists bool
	}{
		{"merge-basic", true, true},
		{"add-add", false, true},
		{"modify-delete", true, false},
		{"rebase-basic", true, true},
		{"merge-multi", true, true},
		{"cherry-pick", true, true},
	} {
		t.Run(tc.id, func(t *testing.T) {
			root := t.TempDir()
			path, err := Generate(context.Background(), root, tc.id)
			if err != nil {
				t.Fatal(err)
			}
			view, err := Inspect(context.Background(), path)
			if err != nil {
				t.Fatal(err)
			}
			if view.Base.Exists != tc.baseExists || view.Feature.Exists != tc.featureExists || !view.Main.Exists {
				t.Fatalf("unexpected versions: %+v", view)
			}
			if view.File == "" || view.Branch == "" || view.Check.Passed {
				t.Fatalf("unexpected inspection: %+v", view)
			}
			if _, err := os.Stat(path); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestInspectSelectsOnlyKnownConflictFiles(t *testing.T) {
	skipSlow(t)
	repo, err := Generate(context.Background(), t.TempDir(), "merge-multi")
	if err != nil {
		t.Fatal(err)
	}
	view, err := InspectFile(context.Background(), repo, "review.txt")
	if err != nil || view.File != "review.txt" || len(view.Files) != 2 || !strings.Contains(view.Main.Content, "manual") || !strings.Contains(view.Feature.Content, "automated") {
		t.Fatalf("review inspection=%+v, err=%v", view, err)
	}
	if _, err := InspectFile(context.Background(), repo, "../state.json"); err == nil {
		t.Fatal("accepted file outside exercise")
	}
}
