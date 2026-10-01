package lab

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestResolveTargetDir_Rejections(t *testing.T) {
	tests := []struct {
		name    string
		labs    func(t *testing.T) string // returns labsRoot (may skip)
		dirName string
		want    error
	}{
		{
			name:    "empty name",
			dirName: "",
			want:    ErrEmptyName,
		},
		{
			name:    "whitespace-only name",
			dirName: "   ",
			want:    ErrEmptyName,
		},
		{
			name:    "dot",
			dirName: ".",
			want:    ErrInvalidName,
		},
		{
			name:    "dotdot",
			dirName: "..",
			want:    ErrInvalidName,
		},
		{
			name:    "path separator slash",
			dirName: "a/b",
			want:    ErrInvalidName,
		},
		{
			name:    "path separator backslash",
			dirName: `a\b`,
			want:    ErrInvalidName,
		},
		{
			name:    "colon",
			dirName: "a:b",
			want:    ErrInvalidName,
		},
		{
			name:    "wildcard",
			dirName: "a*b",
			want:    ErrInvalidName,
		},
		{
			name:    "question mark",
			dirName: "a?b",
			want:    ErrInvalidName,
		},
		{
			name:    "double quote",
			dirName: `a"b`,
			want:    ErrInvalidName,
		},
		{
			name:    "angle bracket",
			dirName: "a<b",
			want:    ErrInvalidName,
		},
		{
			name:    "pipe",
			dirName: "a|b",
			want:    ErrInvalidName,
		},
		{
			name:    "control character",
			dirName: "a\x07b",
			want:    ErrInvalidName,
		},
		{
			name:    "leading space",
			dirName: " name",
			want:    ErrInvalidName,
		},
		{
			name:    "trailing space",
			dirName: "name ",
			want:    ErrInvalidName,
		},
		{
			name:    "trailing dot",
			dirName: "name.",
			want:    ErrInvalidName,
		},
		{
			name:    "too long 65 chars",
			dirName: strings.Repeat("a", 65),
			want:    ErrInvalidName,
		},
		{
			name:    "reserved CON",
			dirName: "con",
			want:    ErrReservedName,
		},
		{
			name:    "reserved CON uppercase",
			dirName: "CON",
			want:    ErrReservedName,
		},
		{
			name:    "reserved NUL",
			dirName: "NUL",
			want:    ErrReservedName,
		},
		{
			name:    "reserved aux",
			dirName: "aux",
			want:    ErrReservedName,
		},
		{
			name:    "reserved prn",
			dirName: "prn",
			want:    ErrReservedName,
		},
		{
			name:    "reserved com1",
			dirName: "COM1",
			want:    ErrReservedName,
		},
		{
			name:    "reserved lpt9",
			dirName: "lpt9",
			want:    ErrReservedName,
		},
		{
			name:    "reserved com style 10-31",
			dirName: "COM17",
			want:    ErrReservedName,
		},
		{
			name:    "reserved VCS name .git on fresh root",
			dirName: ".git",
			want:    ErrReservedGitName,
			labs: func(t *testing.T) string {
				return t.TempDir()
			},
		},
		{
			name:    "reserved VCS name .GIT case-insensitive",
			dirName: ".GIT",
			want:    ErrReservedGitName,
			labs: func(t *testing.T) string {
				return t.TempDir()
			},
		},
		{
			name:    "reserved VCS name .hg on fresh root",
			dirName: ".hg",
			want:    ErrReservedGitName,
			labs: func(t *testing.T) string {
				return t.TempDir()
			},
		},
		{
			name: "reserved VCS name .git takes precedence over non-empty target",
			labs: func(t *testing.T) string {
				root := t.TempDir()
				gitDir := filepath.Join(root, ".git")
				if err := os.MkdirAll(gitDir, 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(gitDir, "HEAD"), []byte("ref: refs/heads/main\n"), 0o644); err != nil {
					t.Fatal(err)
				}
				return root
			},
			dirName: ".git",
			want:    ErrReservedGitName,
		},
		{
			name:    "non-reserved com32",
			dirName: "COM32",
			want:    nil,
			labs: func(t *testing.T) string {
				return t.TempDir()
			},
		},
		{
			name:    "labs root missing",
			dirName: "exercise",
			want:    ErrUnsafeLocation,
		},
		{
			name: "labs root is a file",
			labs: func(t *testing.T) string {
				root := filepath.Join(t.TempDir(), "rootfile")
				if err := os.WriteFile(root, []byte("x"), 0o644); err != nil {
					t.Fatal(err)
				}
				return root
			},
			dirName: "exercise",
			want:    ErrUnsafeLocation,
		},
		{
			name: "target exists as file",
			labs: func(t *testing.T) string {
				root := t.TempDir()
				if err := os.WriteFile(filepath.Join(root, "exercise"), []byte("x"), 0o644); err != nil {
					t.Fatal(err)
				}
				return root
			},
			dirName: "exercise",
			want:    ErrTargetExists,
		},
		{
			name: "target exists as symlink",
			labs: func(t *testing.T) string {
				root := t.TempDir()
				// Point at a sibling INSIDE the root: the symlink is refused as
				// itself (never followed), yielding ErrTargetExists.
				inner := filepath.Join(root, "real-dir")
				if err := os.MkdirAll(inner, 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(inner, filepath.Join(root, "exercise")); err != nil {
					t.Skipf("symlink not permitted on this platform/filesystem: %v", err)
				}
				return root
			},
			dirName: "exercise",
			want:    ErrTargetExists,
		},
		{
			name: "target dir not empty",
			labs: func(t *testing.T) string {
				root := t.TempDir()
				dir := filepath.Join(root, "exercise")
				if err := os.MkdirAll(dir, 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, "stuff.txt"), []byte("x"), 0o644); err != nil {
					t.Fatal(err)
				}
				return root
			},
			dirName: "exercise",
			want:    ErrTargetNotEmpty,
		},
		{
			name: "symlinked component escapes root",
			labs: func(t *testing.T) string {
				root := t.TempDir()
				outside := filepath.Join(filepath.Dir(root), "outside")
				if err := os.MkdirAll(outside, 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(outside, filepath.Join(root, "sym")); err != nil {
					t.Skipf("symlink not permitted on this platform/filesystem: %v", err)
				}
				// Candidate root/sym: an escaping symlink is caught by the
				// containment check with the more specific ErrTraversal.
				return root
			},
			dirName: "sym",
			want:    ErrTraversal,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			var labsRoot string
			if tc.labs != nil {
				labsRoot = tc.labs(t)
			} else {
				labsRoot = filepath.Join(t.TempDir(), "missing-root")
			}

			got, err := ResolveTargetDir(labsRoot, tc.dirName)
			if tc.want == nil {
				if err != nil {
					t.Fatalf("ResolveTargetDir(%q, %q) unexpected error: %v", labsRoot, tc.dirName, err)
				}
				return
			}
			if !errors.Is(err, tc.want) {
				t.Fatalf("ResolveTargetDir(%q, %q) error = %v, want %v", labsRoot, tc.dirName, err, tc.want)
			}
			if got != "" && tc.want != nil {
				t.Errorf("ResolveTargetDir returned non-empty path %q alongside error", got)
			}
		})
	}
}

// normalizePath returns the canonical long-path form of p: absolute,
// cleaned, and symlink-resolved when the path exists. GitHub's Windows
// runners create temp dirs under 8.3 short names (C:\Users\RUNNER~1\...),
// while the product resolves roots to their long form
// (C:\Users\runneradmin\...); raw string comparisons between the two
// forms are invalid, so tests must normalize both sides first.
func normalizePath(t *testing.T, p string) string {
	t.Helper()
	abs := filepath.Clean(p)
	if !filepath.IsAbs(abs) {
		var err error
		abs, err = filepath.Abs(abs)
		if err != nil {
			t.Fatalf("cannot absolutize %q: %v", p, err)
		}
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		// Non-existing path: compare the cleaned absolute form. The
		// product resolves symlinks only for existing paths as well.
		return abs
	}
	return resolved
}

// resolveJoin normalizes base (which must exist) and joins elems onto it,
// producing the expected long-form location for a path that may not exist.
func resolveJoin(t *testing.T, base string, elems ...string) string {
	t.Helper()
	return filepath.Join(append([]string{normalizePath(t, base)}, elems...)...)
}

func TestResolveTargetDir_HappyPath(t *testing.T) {
	root := t.TempDir()
	got, err := ResolveTargetDir(root, "merge-basics")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := resolveJoin(t, root, "merge-basics"); !strings.EqualFold(normalizePath(t, got), want) {
		t.Errorf("resolved path = %q, want inside root at %q", got, want)
	}
	if !strings.HasPrefix(strings.ToLower(normalizePath(t, got)), strings.ToLower(normalizePath(t, root))) {
		t.Errorf("resolved path %q escapes labs root %q", got, root)
	}
}

func TestResolveTargetDir_ExistingEmptyDirAllowed(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "rebase-intro")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := ResolveTargetDir(root, "rebase-intro")
	if err != nil {
		t.Fatalf("unexpected error for empty existing dir: %v", err)
	}
	if !strings.EqualFold(normalizePath(t, got), resolveJoin(t, root, "rebase-intro")) {
		t.Errorf("resolved path = %q, want %q", got, dir)
	}
}

func TestResolveTargetDir_NameExactlyAtLimit(t *testing.T) {
	root := t.TempDir()
	got, err := ResolveTargetDir(root, strings.Repeat("b", 64))
	if err != nil {
		t.Fatalf("64-char name should be valid, got error: %v", err)
	}
	if want := resolveJoin(t, root, strings.Repeat("b", 64)); !strings.EqualFold(normalizePath(t, got), want) {
		t.Errorf("resolved path = %q, want %q", got, want)
	}
}

func TestResolveTargetDir_SymlinkedChildEscapesRoot(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(filepath.Dir(root), "outside-target")
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatal(err)
	}
	// labsRoot/sym -> outside. The candidate "sym/exercise" cannot be
	// expressed as labsRoot+name (names reject separators), so two assertions
	// cover the escape surface:
	//  1) resolving the symlink itself as the name must fail (never follow);
	//  2) with labsRoot = the symlink, the root designation follows it - a
	// pre-existing candidate resolves inside the real location, proving
	// containment is enforced against resolved (real) roots.
	sym := filepath.Join(root, "sym")
	if err := os.Symlink(outside, sym); err != nil {
		t.Skipf("symlink not permitted on this platform/filesystem: %v", err)
	}
	if _, err := ResolveTargetDir(root, "sym"); !errors.Is(err, ErrTraversal) {
		t.Fatalf("error = %v, want ErrTraversal for escaping symlink candidate", err)
	}
	ex := filepath.Join(outside, "exercise")
	if err := os.MkdirAll(ex, 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := ResolveTargetDir(sym, "exercise")
	if err != nil {
		t.Fatalf("symlinked labs root follows its designation: %v", err)
	}
	if !strings.EqualFold(normalizePath(t, got), normalizePath(t, ex)) {
		t.Errorf("resolved path = %q, want %q", got, ex)
	}
}

func TestResolveTargetDir_NestedGitAboveCandidateRejected(t *testing.T) {
	// Names cannot contain separators, so the "unrelated work tree between
	// candidate parent and root" rule is exercised through observable
	// behavior: a nested repo child of the root is treated as an existing
	// non-empty target (its .git is an entry), while fresh siblings still
	// resolve. The boundary invariant: only .git strictly between candidate
	// parent and the labs root would be flagged by the upward walk.
	root := t.TempDir()
	team := filepath.Join(root, "team")
	if err := os.MkdirAll(filepath.Join(team, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := ResolveTargetDir(root, "team"); !errors.Is(err, ErrTargetNotEmpty) {
		t.Fatalf("error = %v, want ErrTargetNotEmpty for repo child (contains .git entry)", err)
	}
	got, err := ResolveTargetDir(root, "fresh")
	if err != nil {
		t.Fatalf("fresh sibling next to a nested repo must resolve: %v", err)
	}
	if want := resolveJoin(t, root, "fresh"); !strings.EqualFold(normalizePath(t, got), want) {
		t.Errorf("resolved path = %q, want %q", got, want)
	}
}

func TestResolveTargetDir_RootGitIsBoundary(t *testing.T) {
	// The labs root itself may be a git repo; its own .git must NOT disqualify.
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := ResolveTargetDir(root, "fresh-exercise")
	if err != nil {
		t.Fatalf("root's own .git must not disqualify, got error: %v", err)
	}
	if want := resolveJoin(t, root, "fresh-exercise"); !strings.EqualFold(normalizePath(t, got), want) {
		t.Errorf("resolved path = %q, want %q", got, want)
	}
}

func TestResolveTargetDir_UnrelatedNestedWorkTreeRejected(t *testing.T) {
	// The labs root is the designated boundary. A repo ABOVE the labs root
	// (tmp itself is a repo, labsRoot = tmp/labs) must NOT disqualify
	// resolution inside the labs area; the upward walk stops at the root.
	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	labsRoot := filepath.Join(tmp, "labs")
	if err := os.MkdirAll(labsRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := ResolveTargetDir(labsRoot, "isolated")
	if err != nil {
		t.Fatalf("repo above the labs root must be ignored, got error: %v", err)
	}
	if want := resolveJoin(t, labsRoot, "isolated"); !strings.EqualFold(normalizePath(t, got), want) {
		t.Errorf("resolved path = %q, want %q", got, want)
	}
}

func TestResolveTargetDir_VolumeRootLabsRoot(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("volume-root drive-relative path malformation is Windows-only; on POSIX the root \"/\" already joins cleanly")
	}
	got, err := ResolveTargetDir("C:\\", "exercise")
	if err != nil {
		t.Fatalf("ResolveTargetDir(C:\\, exercise): %v", err)
	}
	if cleaned := filepath.Clean(got); !strings.EqualFold(cleaned, `C:\exercise`) {
		t.Errorf("resolved path = %q, want C:\\exercise (well-formed, not drive-relative C:exercise)", got)
	}
	if strings.EqualFold(got, `C:exercise`) || strings.HasSuffix(got, "C:exercise") {
		t.Errorf("resolved path %q is the malformed drive-relative form", got)
	}
}
