package lab

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeFakeAttempt creates a directory under root that looks like a
// generated attempt (manifest only, no Git history). It keeps tests that
// do not need a real repository fast.
func writeFakeAttempt(t *testing.T, root, folder, id string) {
	t.Helper()
	dir := filepath.Join(root, folder, stateDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := `{"exercise_id":"` + id + `"}`
	if err := os.WriteFile(filepath.Join(dir, "state.json"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestListAttemptsClassifiesDirectories(t *testing.T) {
	root := t.TempDir()
	writeFakeAttempt(t, root, "merge-basic", "merge-basic")
	if err := os.MkdirAll(filepath.Join(root, "plain-dir"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "notes.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	attempts, err := ListAttempts(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(attempts) != 2 {
		t.Fatalf("attempts = %+v, want 2 entries", attempts)
	}
	byFolder := map[string]Attempt{}
	for _, a := range attempts {
		byFolder[a.Folder] = a
	}
	if a := byFolder["merge-basic"]; !a.Valid || a.ExerciseID != "merge-basic" {
		t.Errorf("merge-basic = %+v, want valid attempt", a)
	}
	if a := byFolder["plain-dir"]; a.Valid || a.ExerciseID != "" {
		t.Errorf("plain-dir = %+v, want unrecognized", a)
	}
}

func TestRemoveAttemptRefusesNonLabDirectories(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "user-data"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := RemoveAttempt(root, "user-data"); !errorIs(err, ErrNotAnExercise) {
		t.Fatalf("RemoveAttempt(user-data) = %v, want ErrNotAnExercise", err)
	}
	if _, err := os.Stat(filepath.Join(root, "user-data")); err != nil {
		t.Fatalf("user-data must survive a refused removal: %v", err)
	}
	if err := RemoveAttempt(root, "missing"); !errorIs(err, ErrNotAnExercise) {
		t.Fatalf("RemoveAttempt(missing) = %v, want ErrNotAnExercise", err)
	}
}

func TestCleanAllRemovesOnlyManagedAttempts(t *testing.T) {
	root := t.TempDir()
	writeFakeAttempt(t, root, "add-add-review", "add-add")
	writeFakeAttempt(t, root, "merge-basic", "merge-basic")
	if err := os.MkdirAll(filepath.Join(root, "user-data"), 0o755); err != nil {
		t.Fatal(err)
	}

	removed, err := CleanAll(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(removed) != 2 || strings.Join(removed, " ") != "add-add-review merge-basic" {
		t.Fatalf("removed = %v, want both managed attempts", removed)
	}
	if _, err := os.Stat(filepath.Join(root, "user-data")); err != nil {
		t.Fatalf("user-data must survive clean: %v", err)
	}
	for _, folder := range []string{"add-add-review", "merge-basic"} {
		if _, err := os.Stat(filepath.Join(root, folder)); !os.IsNotExist(err) {
			t.Errorf("%s still exists after clean", folder)
		}
	}
}

func TestResetRegeneratesFreshCopy(t *testing.T) {
	if testing.Short() {
		t.Skip("generates a real Git repository")
	}
	root := t.TempDir()
	repo, err := Generate(context.Background(), root, "merge-basic")
	if err != nil {
		t.Fatal(err)
	}
	junk := filepath.Join(repo, "junk.txt")
	if err := os.WriteFile(junk, []byte("leftover"), 0o644); err != nil {
		t.Fatal(err)
	}

	created, err := Reset(context.Background(), root, "merge-basic", "merge-basic")
	if err != nil {
		t.Fatal(err)
	}
	if created != repo {
		t.Fatalf("created = %q, want the same path %q", created, repo)
	}
	if _, err := os.Stat(junk); !os.IsNotExist(err) {
		t.Error("reset kept files from the previous attempt")
	}
	if _, err := os.Stat(filepath.Join(repo, stateDir, "state.json")); err != nil {
		t.Errorf("manifest missing after reset: %v", err)
	}
}

func TestDiagnoseReportsEnvironment(t *testing.T) {
	root := t.TempDir()
	writeFakeAttempt(t, root, "merge-basic", "merge-basic")

	report := Diagnose(context.Background(), root)
	if !report.LabsRootExists || !report.LabsRootWritable {
		t.Fatalf("labs root = exists:%v writable:%v", report.LabsRootExists, report.LabsRootWritable)
	}
	if !report.GitFound || !strings.HasPrefix(report.GitVersion, "git version") {
		t.Errorf("git = found:%v version:%q", report.GitFound, report.GitVersion)
	}
	if len(report.Attempts) != 1 || !report.Attempts[0].Valid {
		t.Fatalf("attempts = %+v", report.Attempts)
	}
	if _, err := os.Stat(filepath.Join(root, ".git-conflict-lab-doctor-probe")); !os.IsNotExist(err) {
		t.Error("doctor probe file was left behind")
	}
}
