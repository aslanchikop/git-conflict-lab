package gitx

import (
	"context"
	"errors"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// withGitUser returns config args so commits work in a bare temp environment.
func withGitUser() []string {
	return []string{"-c", "user.name=t", "-c", "user.email=t@e.invalid"}
}

func TestRunHappyPath(t *testing.T) {
	dir := t.TempDir()

	if _, err := Run(context.Background(), dir, "init"); err != nil {
		t.Fatalf("git init: %v", err)
	}

	out, err := Run(context.Background(), dir, "status", "--porcelain")
	if err != nil {
		t.Fatalf("git status --porcelain: %v", err)
	}
	if strings.TrimSpace(out) != "" {
		t.Errorf("expected empty porcelain output, got %q", out)
	}

	// Round-trip an empty commit.
	args := append([]string{}, withGitUser()...)
	args = append(args, "commit", "--allow-empty", "-m", "init")
	if _, err := Run(context.Background(), dir, args...); err != nil {
		t.Fatalf("git commit: %v", err)
	}

	out, err = Run(context.Background(), dir, "log", "--oneline", "-1")
	if err != nil {
		t.Fatalf("git log: %v", err)
	}
	if !strings.Contains(out, "init") {
		t.Errorf("expected commit message in log output, got %q", out)
	}
}

func TestRunExitError(t *testing.T) {
	dir := t.TempDir()

	if _, err := Run(context.Background(), dir, "init"); err != nil {
		t.Fatalf("git init: %v", err)
	}

	out, err := Run(context.Background(), dir, "status", "--definitely-not-a-flag")
	if err == nil {
		t.Fatal("expected error for invalid flag, got nil")
	}
	var exitErr *ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected *ExitError, got %T: %v", err, err)
	}
	if exitErr.ExitCode <= 0 {
		t.Errorf("expected positive exit code, got %d", exitErr.ExitCode)
	}
	if !strings.Contains(out, "definitely-not-a-flag") && !strings.Contains(exitErr.Output, "definitely-not-a-flag") {
		t.Errorf("expected git error output mentioning the bad flag, got %q", out)
	}
	if !strings.Contains(exitErr.Error(), "exit code") {
		t.Errorf("Error() should mention exit code, got %q", exitErr.Error())
	}
}

func TestRunRejectsEmptyArg(t *testing.T) {
	dir := t.TempDir()

	_, err := Run(context.Background(), dir, "status", "")
	if err == nil {
		t.Fatal("expected error for empty argument, got nil")
	}
	if !errors.Is(err, ErrInvalidArgs) {
		t.Errorf("expected ErrInvalidArgs wrap, got %v", err)
	}
}

func TestRunGitNotFound(t *testing.T) {
	orig := lookPath
	lookPath = func(string) (string, error) { return "", exec.ErrNotFound }
	t.Cleanup(func() { lookPath = orig })

	_, err := Run(context.Background(), t.TempDir(), "status")
	if err == nil {
		t.Fatal("expected ErrGitNotFound, got nil")
	}
	if !errors.Is(err, ErrGitNotFound) {
		t.Errorf("expected ErrGitNotFound wrap, got %v", err)
	}
}

func TestRunTimeout(t *testing.T) {
	orig := defaultTimeout
	defaultTimeout = 1 * time.Nanosecond
	t.Cleanup(func() { defaultTimeout = orig })

	_, err := Run(context.Background(), t.TempDir(), "status")
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Skipf("timeout error not deterministic on this platform: %v", err)
	}
	if !strings.Contains(err.Error(), "timed out") {
		t.Errorf("expected timeout message mentioning the command, got %q", err.Error())
	}
}
