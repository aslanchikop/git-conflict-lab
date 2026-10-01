// diagnose.go powers the doctor command: it collects environment facts
// (Git availability, labs directory health, existing attempts) without
// changing anything on disk beyond a short-lived write probe.
package lab

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/aslanchikop/git-conflict-lab/internal/gitx"
)

// Diagnosis is the environment report behind the doctor command.
type Diagnosis struct {
	// LabsRoot is the resolved labs directory.
	LabsRoot string
	// LabsRootExists reports whether the labs directory exists.
	LabsRootExists bool
	// LabsRootWritable reports a successful create-and-delete probe.
	LabsRootWritable bool
	// GitFound reports git on PATH.
	GitFound bool
	// GitVersion is the first line of `git --version` (e.g. "git version 2.43.0").
	GitVersion string
	// Attempts lists directories under the labs root (empty when it does not exist).
	Attempts []Attempt
}

// Diagnose collects environment facts for the doctor command. It never
// fails hard: problems are reported through the Diagnosis fields so the
// caller can print every finding at once.
func Diagnose(ctx context.Context, labsRoot string) Diagnosis {
	report := Diagnosis{LabsRoot: labsRoot}
	if out, err := gitx.Run(ctx, "", "--version"); err == nil {
		report.GitFound = true
		if i := strings.IndexByte(out, '\n'); i >= 0 {
			out = out[:i]
		}
		report.GitVersion = strings.TrimSpace(out)
	}
	if info, err := os.Stat(labsRoot); err == nil && info.IsDir() {
		report.LabsRootExists = true
		probe := filepath.Join(labsRoot, ".git-conflict-lab-doctor-probe")
		if err := os.WriteFile(probe, nil, 0o644); err == nil {
			report.LabsRootWritable = true
			_ = os.Remove(probe)
		}
	}
	if report.LabsRootExists {
		report.Attempts, _ = ListAttempts(labsRoot)
	}
	return report
}
