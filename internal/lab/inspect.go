package lab

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/aslanchikop/git-conflict-lab/internal/gitx"
)

// FileVersion is a bounded, read-only view of one side of an exercise conflict.
type FileVersion struct {
	Exists  bool   `json:"exists"`
	Content string `json:"content"`
}

// Inspection explains the starting versions and the learner's current file.
type Inspection struct {
	File    string      `json:"file"`
	Files   []string    `json:"files"`
	Branch  string      `json:"branch"`
	Base    FileVersion `json:"base"`
	Main    FileVersion `json:"main"`
	Feature FileVersion `json:"feature"`
	Current FileVersion `json:"current"`
	Check   CheckResult `json:"check"`
}

var commitHash = regexp.MustCompile(`^[0-9a-f]{40}([0-9a-f]{24})?$`)

// Inspect reads the original Git objects and the current working file. It never
// changes the lab and never executes code from the learner's repository.
func Inspect(ctx context.Context, dir string) (Inspection, error) {
	return InspectFile(ctx, dir, "")
}

// InspectFile selects one of the conflict files in a generated exercise.
func InspectFile(ctx context.Context, dir, selected string) (Inspection, error) {
	state, root, err := readState(dir)
	if err != nil {
		return Inspection{}, err
	}
	sc, ok := LookupScenario(state.ExerciseID)
	if !ok || state.ConflictFile != sc.File || state.FeatureBranch != sc.Branch || !commitHash.MatchString(state.BaseSHA) || !commitHash.MatchString(state.MainSHA) || !commitHash.MatchString(state.FeatureSHA) {
		return Inspection{}, fmt.Errorf("invalid exercise state")
	}
	available := []string{sc.File}
	file := sc.File
	if len(sc.Files) > 1 {
		if len(state.ConflictFiles) != len(sc.Files) {
			return Inspection{}, fmt.Errorf("invalid exercise file list")
		}
		for i, name := range sc.Files {
			if state.ConflictFiles[i] != name {
				return Inspection{}, fmt.Errorf("invalid exercise file list")
			}
		}
		available = append([]string(nil), sc.Files...)
	}
	if selected != "" {
		found := false
		for _, allowed := range available {
			if selected == allowed {
				found = true
			}
		}
		if !found {
			return Inspection{}, fmt.Errorf("unknown exercise file")
		}
		file = selected
	}
	check, err := Check(ctx, root)
	if err != nil {
		return Inspection{}, err
	}
	view := Inspection{File: file, Files: available, Branch: state.FeatureBranch, Check: check}
	readCommit := func(sha string, absent bool) (FileVersion, error) {
		output, err := gitx.Run(ctx, root, "show", sha+":"+file)
		if err != nil {
			if absent {
				return FileVersion{}, nil
			}
			return FileVersion{}, err
		}
		if len(output) > 16*1024 {
			return FileVersion{}, fmt.Errorf("exercise file is too large to preview")
		}
		return FileVersion{Exists: true, Content: output}, nil
	}
	view.Base, err = readCommit(state.BaseSHA, sc.BaseAbsent)
	if err != nil {
		return Inspection{}, err
	}
	view.Main, err = readCommit(state.MainSHA, false)
	if err != nil {
		return Inspection{}, err
	}
	view.Feature, err = readCommit(state.FeatureSHA, sc.FeatureAbsent)
	if err != nil {
		return Inspection{}, err
	}
	path := filepath.Join(root, file)
	info, err := os.Lstat(path)
	if err == nil {
		if !info.Mode().IsRegular() || info.Size() > 16*1024 {
			return Inspection{}, fmt.Errorf("current exercise file cannot be previewed")
		}
		reader, err := os.Open(path)
		if err != nil {
			return Inspection{}, err
		}
		defer func() { _ = reader.Close() }()
		data, err := io.ReadAll(io.LimitReader(reader, 16*1024+1))
		if err != nil {
			return Inspection{}, err
		}
		if len(data) > 16*1024 {
			return Inspection{}, fmt.Errorf("current exercise file is too large to preview")
		}
		view.Current = FileVersion{Exists: true, Content: string(data)}
	} else if !os.IsNotExist(err) {
		return Inspection{}, err
	}
	// A worktree with conflict markers is shown verbatim so learners can connect
	// the source branches to the file they are editing.
	view.Current.Content = strings.ReplaceAll(view.Current.Content, "\r\n", "\n")
	return view, nil
}
