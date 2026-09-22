package lab

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// DefaultLabsRoot returns the default labs root directory, creating it if
// necessary: <os.UserConfigDir()>/git-conflict-lab/labs.
func DefaultLabsRoot() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve user config dir: %w", err)
	}
	root := filepath.Join(base, "git-conflict-lab", "labs")
	if err := os.MkdirAll(root, 0o755); err != nil {
		return "", fmt.Errorf("create labs root: %w", err)
	}
	return resolveExistingDir(root)
}

// InitLabsRoot validates (and creates, if missing) an explicitly configured
// labs root. Empty paths are rejected. The resolved directory must not live
// inside any git repository (any ancestor strictly above it containing a
// .git entry is rejected) so lab exercises never nest inside user repos.
func InitLabsRoot(dir string) (string, error) {
	trimmed := strings.TrimSpace(dir)
	if trimmed == "" {
		return "", errors.New("labs root must not be empty")
	}
	abs, err := filepath.Abs(trimmed)
	if err != nil {
		return "", fmt.Errorf("resolve labs root: %w", err)
	}
	if err := os.MkdirAll(abs, 0o755); err != nil {
		return "", fmt.Errorf("create labs root: %w", err)
	}
	resolved, err := resolveExistingDir(abs)
	if err != nil {
		return "", err
	}
	if err := ensureNotInsideRepo(resolved); err != nil {
		return "", err
	}
	return resolved, nil
}

// resolveExistingDir returns the absolute, symlink-resolved path of dir and
// verifies that it exists and is a directory.
func resolveExistingDir(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve path: %w", err)
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", fmt.Errorf("resolve %s: %w", abs, err)
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return "", fmt.Errorf("stat %s: %w", resolved, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("%s is not a directory", resolved)
	}
	return resolved, nil
}

// ensureNotInsideRepo rejects root if any ancestor strictly above it contains
// a .git entry (directory or worktree-style file).
func ensureNotInsideRepo(root string) error {
	abs, err := filepath.Abs(root)
	if err != nil {
		return fmt.Errorf("resolve labs root: %w", err)
	}
	parent := filepath.Dir(abs)
	for {
		if dirHasGitMeta(parent) {
			return fmt.Errorf(
				"labs root %s is inside a git repository (%s contains .git); choose a location outside any repository",
				abs, parent)
		}
		next := filepath.Dir(parent)
		if next == parent {
			return nil
		}
		parent = next
	}
}

// dirHasGitMeta reports whether dir directly contains a .git entry.
func dirHasGitMeta(dir string) bool {
	info, err := os.Stat(filepath.Join(dir, ".git"))
	return err == nil && (info.IsDir() || info.Mode().IsRegular())
}
