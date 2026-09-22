// Package lab is the security boundary between the Git Conflict Lab tool
// and the user's filesystem. Milestone A scope: resolve and validate the
// target directory for an exercise. Generation lands in Milestone B.
package lab

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Sentinel errors returned by ResolveTargetDir. Use errors.Is to classify.
var (
	ErrEmptyName       = errors.New("exercise name is empty")
	ErrInvalidName     = errors.New("exercise name contains invalid characters or format")
	ErrReservedName    = errors.New("exercise name is a reserved Windows device name")
	ErrReservedGitName = errors.New("exercise name is reserved by Git/version-control systems")
	ErrTargetExists    = errors.New("target path already exists as a file or symlink")
	ErrTargetNotEmpty  = errors.New("target directory already exists and is not empty")
	ErrTraversal       = errors.New("target path escapes the labs root via symlink")
	ErrUnsafeLocation  = errors.New("target location is unsafe")
)

// NotAvailableError reports a CLI command that exists on the command surface
// but has no implementation yet in the current milestone.
type NotAvailableError struct {
	Command   string
	Milestone string
}

func (e *NotAvailableError) Error() string {
	return fmt.Sprintf("%s is not implemented yet - it arrives in Milestone %s.", e.Command, e.Milestone)
}

func isReservedDeviceName(name string) bool {
	base := strings.ToUpper(name)
	if i := strings.IndexByte(base, '.'); i >= 0 {
		base = base[:i]
	}
	switch base {
	case "CON", "PRN", "AUX", "NUL":
		return true
	}
	if len(base) > 3 && (strings.HasPrefix(base, "COM") || strings.HasPrefix(base, "LPT")) {
		n := 0
		for _, r := range base[3:] {
			if r < '0' || r > '9' {
				return false
			}
			n = n*10 + int(r-'0')
		}
		return n >= 1 && n <= 31
	}
	return false
}

// isReservedVCSName reports whether name collides with a version-control
// metadata directory (".git", ".hg"). Comparison is case-insensitive: a
// Windows filesystem treats ".GIT" and ".git" as the same directory.
func isReservedVCSName(name string) bool {
	return strings.EqualFold(name, ".git") || strings.EqualFold(name, ".hg")
}

func containsControlChar(s string) bool {
	for _, r := range s {
		if r <= 0x1F || r == 0x7F {
			return true
		}
	}
	return false
}

func dirContainsGitEntry(dir string) bool {
	_, err := os.Lstat(filepath.Join(dir, ".git"))
	return err == nil
}

// contains reports whether dir is root itself or a path inside root,
// comparing case-insensitively (Windows volume-qualified paths).
func contains(root, dir string) bool {
	if strings.EqualFold(root, dir) {
		return true
	}
	rel, err := filepath.Rel(root, dir)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// sameDir reports whether two paths denote the same directory, resolving
// symlinks where possible and falling back to case-insensitive comparison.
func sameDir(a, b string) bool {
	if strings.EqualFold(a, b) {
		return true
	}
	ea, errA := filepath.EvalSymlinks(a)
	eb, errB := filepath.EvalSymlinks(b)
	if errA == nil && errB == nil {
		return strings.EqualFold(ea, eb)
	}
	return false
}

// ResolveTargetDir validates the exercise ID (name, untrusted input) and
// resolves it to a safe directory path under labsRoot.
//
// labsRoot is the designated directory that will contain exercise
// directories; it must already exist. The labs root itself may live inside
// a git repository - only directories strictly between the candidate and
// the labs root are checked for unrelated nested work trees.
func ResolveTargetDir(labsRoot, name string) (string, error) {
	// TOCTOU boundary: filesystem state can change between validation here
	// and directory creation when Milestone B generation lands; the generation
	// step must re-verify the resolved path at creation time.
	// (a) Empty or whitespace-only name.
	if strings.TrimSpace(name) == "" {
		return "", ErrEmptyName
	}

	// (b) Invalid characters/format.
	switch {
	case strings.ContainsAny(name, `/\:*?"<>|`):
		return "", ErrInvalidName
	case containsControlChar(name):
		return "", ErrInvalidName
	case name == "." || name == "..":
		return "", ErrInvalidName
	case isReservedVCSName(name):
		// ".git"/".hg" collide with version-control metadata directories.
		// Reject them here, before any path-join or existing-target logic,
		// so a repo-like labs root yields the dedicated error instead of
		// ErrTargetNotEmpty.
		return "", fmt.Errorf("%w: %q", ErrReservedGitName, name)
	case name != strings.TrimSpace(name):
		return "", ErrInvalidName
	case len(name) > 64:
		return "", ErrInvalidName
	case strings.HasSuffix(name, ".") || strings.HasSuffix(name, " "):
		return "", ErrInvalidName
	}

	// (c) Windows reserved device names (case-insensitive, extension stripped).
	if isReservedDeviceName(name) {
		return "", fmt.Errorf("%w: %q", ErrReservedName, name)
	}

	// (d) labsRoot must exist as a directory; resolve it canonically.
	// Stat (not Lstat): a symlinked labs root is followed and then fully
	// resolved below, so containment is enforced against the real location.
	if info, err := os.Stat(labsRoot); err != nil || !info.IsDir() {
		return "", fmt.Errorf("%w: labs root is not initialized: %s", ErrUnsafeLocation, labsRoot)
	}
	absRoot, err := filepath.Abs(labsRoot)
	if err != nil {
		return "", fmt.Errorf("%w: cannot resolve labs root: %v", ErrUnsafeLocation, err)
	}
	resolvedRoot, err := filepath.EvalSymlinks(absRoot)
	if err != nil {
		return "", fmt.Errorf("%w: cannot resolve labs root: %v", ErrUnsafeLocation, err)
	}
	// A volume root ("C:\", "/") must keep its trailing separator: trimming
	// it would turn Join("C:", name) into the drive-relative form "C:name".
	if len(resolvedRoot) > 3 {
		resolvedRoot = strings.TrimSuffix(resolvedRoot, string(filepath.Separator))
	}

	// (e) Ancestor symlink containment. If any component of the candidate's
	// parent chain (up to and including resolvedRoot) is a symlink or reparse
	// point, the fully resolved parent must remain within resolvedRoot.
	candidate := filepath.Join(resolvedRoot, name)
	parent := filepath.Dir(candidate)

	if err := enforceParentContainment(resolvedRoot, parent); err != nil {
		return "", err
	}

	// The candidate itself may be a symlink planted by an attacker. Resolve it
	// without following: if its resolution escapes the labs root, that is a
	// traversal attempt; if it stays inside, rule (f) still refuses to follow
	// it (never overwrite through user symlinks).
	if info, err := os.Lstat(candidate); err == nil && info.Mode()&os.ModeSymlink != 0 {
		if resolved, evalErr := filepath.EvalSymlinks(candidate); evalErr == nil && !contains(resolvedRoot, resolved) {
			return "", fmt.Errorf("%w: %q resolves to %q, outside labs root %q", ErrTraversal, candidate, resolved, resolvedRoot)
		}
	}

	// (f) Existing target checks. Lstat so symlinks are detected as themselves
	// and never followed into overwrites.
	if info, err := os.Lstat(candidate); err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("%w: refusing to follow symlink at %s", ErrTargetExists, candidate)
		}
		if !info.IsDir() {
			return "", fmt.Errorf("%w: %s is not a directory", ErrTargetExists, candidate)
		}
		entries, readErr := os.ReadDir(candidate)
		if readErr != nil {
			return "", fmt.Errorf("%w: cannot inspect existing directory %s: %v", ErrUnsafeLocation, candidate, readErr)
		}
		if len(entries) > 0 {
			return "", fmt.Errorf("%w: %s", ErrTargetNotEmpty, candidate)
		}
		// An empty existing directory is allowed; return its resolved path.
		resolvedCandidate, evalErr := filepath.EvalSymlinks(candidate)
		if evalErr != nil {
			return "", fmt.Errorf("%w: cannot resolve existing directory %s: %v", ErrUnsafeLocation, candidate, evalErr)
		}
		if !contains(resolvedRoot, resolvedCandidate) {
			return "", fmt.Errorf("%w: resolved directory %q escapes labs root %q", ErrTraversal, resolvedCandidate, resolvedRoot)
		}
		return resolvedCandidate, nil
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("%w: cannot stat candidate %s: %v", ErrUnsafeLocation, candidate, err)
	}

	// (g) Walk up from the candidate's parent to resolvedRoot: reject if an
	// unrelated git work tree sits in between. The labs root's own .git does
	// not disqualify - the root is the designated labs area.
	for dir := parent; ; dir = filepath.Dir(dir) {
		if dirContainsGitEntry(dir) && !sameDir(dir, resolvedRoot) {
			return "", fmt.Errorf("%w: target is inside an unrelated Git work tree", ErrUnsafeLocation)
		}
		if sameDir(dir, resolvedRoot) {
			break
		}
		if next := filepath.Dir(dir); next == dir {
			break // reached the volume root; resolvedRoot was never hit
		}
	}

	return candidate, nil
}

// enforceParentContainment checks each existing ancestor of dir, from dir up
// to and including root, for symlink/reparse-point components and verifies
// the resolved chain stays inside root.
func enforceParentContainment(root, dir string) error {
	if !contains(root, dir) {
		return fmt.Errorf("%w: path %q is outside labs root %q", ErrTraversal, dir, root)
	}

	// Find the deepest existing ancestor; components above it cannot be
	// symlinks because they do not exist.
	existing := dir
	for {
		if _, err := os.Lstat(existing); err == nil {
			break
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("%w: cannot inspect path %s: %v", ErrUnsafeLocation, existing, err)
		}
		next := filepath.Dir(existing)
		if sameDir(next, existing) || !contains(root, next) {
			break
		}
		existing = next
	}

	resolved, err := filepath.EvalSymlinks(existing)
	if err != nil {
		return fmt.Errorf("%w: cannot resolve path %s: %v", ErrUnsafeLocation, existing, err)
	}
	if !contains(root, resolved) {
		return fmt.Errorf("%w: resolved parent %q escapes labs root %q", ErrTraversal, resolved, root)
	}
	return nil
}
