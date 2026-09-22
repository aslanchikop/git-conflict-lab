// Package gitx provides a thin, safe wrapper around invoking the git
// executable. It always uses argument arrays (never a shell), enforces a
// timeout, and returns typed errors for common failure modes.
package gitx

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// defaultTimeout is the maximum duration for a single git invocation.
// It is a package variable so tests can shorten it.
var defaultTimeout = 30 * time.Second

// lookPath is the function used to locate the git executable.
// It is a package variable so tests can inject a stub.
var lookPath = exec.LookPath

// ErrInvalidArgs is returned when an argument is empty.
var ErrInvalidArgs = errors.New("invalid git arguments")

// ErrGitNotFound is returned when the git executable cannot be located.
var ErrGitNotFound = errors.New("git executable not found on PATH")

// ExitError reports a non-zero exit from a git invocation.
type ExitError struct {
	Args     []string
	ExitCode int
	Output   string
}

// Error implements the error interface.
func (e *ExitError) Error() string {
	name := "git"
	if len(e.Args) > 0 {
		name = e.Args[0]
	}
	return fmt.Sprintf("git %s failed with exit code %d", name, e.ExitCode)
}

// RunWithEnv executes git like Run, but appends extra environment
// variables (KEY=VALUE) to the process environment. It is used by
// deterministic generation flows that must pin author/committer dates
// and disable system/global Git configuration per invocation, without
// ever touching the user's global Git setup.
func RunWithEnv(ctx context.Context, dir string, extraEnv []string, args ...string) (string, error) {
	for _, a := range args {
		if a == "" {
			return "", fmt.Errorf("%w: empty argument at position %d", ErrInvalidArgs, indexOf(args, a)+1)
		}
	}

	if _, err := lookPath("git"); err != nil {
		return "", fmt.Errorf("%w: %v", ErrGitNotFound, err)
	}

	runCtx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()

	cmd := exec.CommandContext(runCtx, "git", args...)
	cmd.Dir = dir
	if len(extraEnv) > 0 {
		cmd.Env = append(os.Environ(), extraEnv...)
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return string(output), &ExitError{
				Args:     args,
				ExitCode: exitErr.ExitCode(),
				Output:   string(output),
			}
		}
		if errors.Is(runCtx.Err(), context.Canceled) {
			return string(output), fmt.Errorf("git %s was canceled: %w", strings.Join(args, " "), runCtx.Err())
		}
		if errors.Is(runCtx.Err(), context.DeadlineExceeded) {
			return string(output), fmt.Errorf("git %s timed out after %s: %w", strings.Join(args, " "), defaultTimeout, runCtx.Err())
		}
		return string(output), fmt.Errorf("git %s failed to start: %w", strings.Join(args, " "), err)
	}
	return string(output), nil
}
// Run executes git with the given arguments in dir and returns combined
// stdout+stderr output. Arguments must be non-empty strings.
func Run(ctx context.Context, dir string, args ...string) (string, error) {
	for _, a := range args {
		if a == "" {
			return "", fmt.Errorf("%w: empty argument at position %d", ErrInvalidArgs, indexOf(args, a)+1)
		}
	}

	if _, err := lookPath("git"); err != nil {
		return "", fmt.Errorf("%w: %v", ErrGitNotFound, err)
	}

	runCtx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()

	cmd := exec.CommandContext(runCtx, "git", args...)
	cmd.Dir = dir

	output, err := cmd.CombinedOutput()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return string(output), &ExitError{
				Args:     args,
				ExitCode: exitErr.ExitCode(),
				Output:   string(output),
			}
		}
		if errors.Is(runCtx.Err(), context.Canceled) {
			return string(output), fmt.Errorf("git %s was canceled: %w", strings.Join(args, " "), runCtx.Err())
		}
		if errors.Is(runCtx.Err(), context.DeadlineExceeded) {
			return string(output), fmt.Errorf("git %s timed out after %s: %w", strings.Join(args, " "), defaultTimeout, runCtx.Err())
		}
		return string(output), fmt.Errorf("git %s failed to start: %w", strings.Join(args, " "), err)
	}
	return string(output), nil
}

func indexOf(args []string, target string) int {
	for i, a := range args {
		if a == target {
			return i
		}
	}
	return -1
}
