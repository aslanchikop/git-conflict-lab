// attempts.go manages the exercise attempt directories under the labs
// root: listing them, removing them, and regenerating them. Every removal
// path revalidates the target through resolveTargetDir and additionally
// requires the lab manifest, so the tool never deletes a directory it did
// not create itself.
package lab

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// ErrNotAnExercise reports a directory that lacks the lab manifest, so it
// is not a generated exercise attempt and must not be deleted.
var ErrNotAnExercise = errors.New("directory is not a git-conflict-lab exercise")

// Attempt describes one directory under the labs root.
type Attempt struct {
	// Folder is the directory name under the labs root.
	Folder string
	// Path is the absolute path of the attempt directory.
	Path string
	// ExerciseID comes from the manifest; empty when unrecognized.
	ExerciseID string
	// Valid reports a present, parseable manifest with an exercise id.
	Valid bool
}

// ListAttempts describes every directory under labsRoot. Directories with
// a parseable manifest are Valid; everything else (user files, unrelated
// directories) is reported with Valid=false so callers can show it
// without touching it.
func ListAttempts(labsRoot string) ([]Attempt, error) {
	root, err := filepath.Abs(labsRoot)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	var attempts []Attempt
	for _, entry := range entries {
		// IsDir is false for symlinks, so symlinked directories are
		// reported as unrecognized rather than followed.
		if !entry.IsDir() {
			continue
		}
		path := filepath.Join(root, entry.Name())
		attempt := Attempt{Folder: entry.Name(), Path: path}
		if data, readErr := os.ReadFile(filepath.Join(path, stateDir, "state.json")); readErr == nil {
			var state State
			if jsonUnmarshal(data, &state) == nil && state.ExerciseID != "" {
				attempt.Valid = true
				attempt.ExerciseID = state.ExerciseID
			}
		}
		attempts = append(attempts, attempt)
	}
	sort.Slice(attempts, func(i, j int) bool { return attempts[i].Folder < attempts[j].Folder })
	return attempts, nil
}

// resolveExistingAttempt validates folder like ResolveTargetDir does, but
// requires an existing directory that carries the lab manifest.
func resolveExistingAttempt(labsRoot, folder string) (string, error) {
	dir, err := resolveTargetDir(labsRoot, folder, true)
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(filepath.Join(dir, stateDir, "state.json")); err != nil {
		return "", fmt.Errorf("%w: %s", ErrNotAnExercise, folder)
	}
	return dir, nil
}

// RemoveAttempt safely deletes one exercise attempt directory. It refuses
// directories without the lab manifest (ErrNotAnExercise) and keeps all
// path-safety rules from resolveTargetDir.
func RemoveAttempt(labsRoot, folder string) error {
	dir, err := resolveExistingAttempt(labsRoot, folder)
	if err != nil {
		return err
	}
	return os.RemoveAll(dir)
}

// Reset deletes the existing attempt and regenerates it from scratch. The
// returned path is the freshly generated repository.
func Reset(ctx context.Context, labsRoot, id, folder string) (string, error) {
	if _, ok := LookupScenario(id); !ok {
		return "", fmt.Errorf("unknown exercise %q", id)
	}
	if err := RemoveAttempt(labsRoot, folder); err != nil {
		return "", err
	}
	return GenerateAs(ctx, labsRoot, id, folder)
}

// CleanAll removes every valid attempt under labsRoot and returns the
// removed folder names. Unrecognized directories are left untouched.
func CleanAll(labsRoot string) ([]string, error) {
	attempts, err := ListAttempts(labsRoot)
	if err != nil {
		return nil, err
	}
	var removed []string
	for _, attempt := range attempts {
		if !attempt.Valid {
			continue
		}
		if err := RemoveAttempt(labsRoot, attempt.Folder); err != nil {
			return removed, err
		}
		removed = append(removed, attempt.Folder)
	}
	return removed, nil
}
