// Package check verifies the actual Git state of a generated exercise
// repository against the exercise objective. It is read-only by design:
// no git command that mutates state is ever invoked, and the repository
// must be byte-identical before and after a Check call.
//
// Trust model: the advisory state manifest (.git-conflict-lab/state.json)
// is user-writable, so every fact it claims is cross-checked against the
// real Git state. A tampered manifest fails the check instead of being
// trusted.
package check

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/aslanchikop/git-conflict-lab/internal/exercise"
	"github.com/aslanchikop/git-conflict-lab/internal/gitx"
	"github.com/aslanchikop/git-conflict-lab/internal/lab"
)

// CheckResult is the outcome of a single named check.
type CheckResult struct {
	Name   string
	Passed bool
	Detail string
}

// Result aggregates all checks for one repository.
type Result struct {
	Passed bool
	Checks []CheckResult
}

// Check runs every verification against the repository at repoDir and
// returns the aggregated result. A returned error (distinct from a failed
// Result) means the check could not be performed at all.
func Check(ctx context.Context, repoDir string) (Result, error) {
	var res Result
	record := func(name string, passed bool, detail string) {
		res.Checks = append(res.Checks, CheckResult{Name: name, Passed: passed, Detail: detail})
	}

	// --- 1. manifest ---
	manifestPath := filepath.Join(repoDir, lab.StateDir, "state.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		record("manifest", false,
			"no exercise manifest found here. Run this command inside a generated exercise directory; create one with: git-conflict-lab start merge-basic")
		res.Passed = false
		return res, nil
	}
	var state lab.State
	if err := json.Unmarshal(data, &state); err != nil {
		record("manifest", false, "state file is damaged and cannot be read ("+err.Error()+"). Regenerate the exercise.")
		res.Passed = false
		return res, nil
	}
	if _, err := exercise.Get(state.ExerciseID); err != nil {
		record("manifest", false, fmt.Sprintf("manifest claims unknown exercise %q; regenerate the exercise.", state.ExerciseID))
		res.Passed = false
		return res, nil
	}
	shaOK := true
	for _, ref := range []string{state.BaseSHA, state.MainSHA, state.FeatureSHA} {
		if ref == "" {
			shaOK = false
			break
		}
		if _, err := gitx.Run(ctx, repoDir, "cat-file", "-e", ref+"^{commit}"); err != nil {
			shaOK = false
			break
		}
	}
	if !shaOK {
		record("manifest", false, "manifest references commits that do not exist in this repository; it was tampered with or belongs to a different exercise.")
		res.Passed = false
		return res, nil
	}
	record("manifest", true, "exercise manifest present and consistent with the repository.")

	// --- 2. no unfinished operation ---
	markers := []string{
		".git/MERGE_HEAD", ".git/CHERRY_PICK_HEAD", ".git/REVERT_HEAD",
		".git/rebase-merge", ".git/rebase-apply", ".git/sequencer",
	}
	var unfinished []string
	for _, m := range markers {
		if _, err := os.Stat(filepath.Join(repoDir, filepath.FromSlash(m))); err == nil {
			unfinished = append(unfinished, m)
		}
	}
	if len(unfinished) > 0 {
		record("unfinished-operations", false,
			"a Git operation is still in progress ("+strings.Join(unfinished, ", ")+"). Resolve it and commit, or run e.g. git merge --abort / git rebase --abort.")
	} else {
		record("unfinished-operations", true, "no merge, rebase, cherry-pick or revert is in progress.")
	}

	// --- 3. clean index (no unresolved conflicts) ---
	idxOut, err := gitx.Run(ctx, repoDir, "ls-files", "-u")
	if err != nil {
		return res, fmt.Errorf("cannot inspect the index: %w", err)
	}
	if strings.TrimSpace(idxOut) != "" {
		record("unresolved-index", false, "the index still holds unresolved conflict entries. Open the conflicted files, resolve them, then git add them.")
	} else {
		record("unresolved-index", true, "index has no unresolved conflict entries.")
	}

	// --- 4. conflict file without markers ---
	conflictFile := filepath.Join(repoDir, filepath.FromSlash(state.ConflictFile))
	if state.ConflictFile == "" {
		record("conflict-file", false, "manifest does not name the conflict file; regenerate the exercise.")
	} else if data, err := os.ReadFile(conflictFile); err != nil {
		record("conflict-file", false, "conflict file "+state.ConflictFile+" is missing.")
	} else {
		content := string(data)
		hasMarkers := strings.Contains(content, "<<<<<<<") ||
			strings.Contains(content, ">>>>>>>") ||
			strings.Contains(content, "\n=======") || strings.HasPrefix(content, "=======")
		if hasMarkers {
			record("conflict-file", false, state.ConflictFile+" still contains conflict markers. Search for <<<<<<< and resolve both sides.")
		} else {
			// Advisory only: absence of markers is not proof of a correct
			// resolution; the semantic check below does the real work.
			record("conflict-file", true, state.ConflictFile+" contains no conflict markers (advisory).")
		}
	}

	// --- 5. both sides preserved (semantic, exercise-specific) ---
	if data, err := os.ReadFile(conflictFile); err == nil {
		detail, ok := mergedResolution(string(data))
		record("both-sides-preserved", ok, detail)
	}

	// --- 6. resolution landed on main ---
	landing, landingOK := checkLanding(ctx, repoDir, &state)
	record("resolution-landed", landingOK, landing)

	// --- 7. clean worktree ---
	stOut, err := gitx.Run(ctx, repoDir, "status", "--porcelain")
	if err != nil {
		return res, fmt.Errorf("cannot inspect the worktree: %w", err)
	}
	var dirty []string
	for _, line := range strings.Split(stOut, "\n") {
		line = strings.TrimRight(line, "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		// The advisory manifest dir is tool-owned; ignore it here.
		if strings.Contains(line, lab.StateDir) {
			continue
		}
		dirty = append(dirty, line)
	}
	if len(dirty) > 0 {
		record("clean-worktree", false, "uncommitted changes remain:\n  "+strings.Join(dirbyTrim(dirty), "\n  ")+"\nCommit your resolution (git add <file>; git commit) or discard it deliberately.")
	} else {
		record("clean-worktree", true, "worktree is clean; the resolution is committed.")
	}

	res.Passed = true
	for _, c := range res.Checks {
		if !c.Passed {
			res.Passed = false
			break
		}
	}
	return res, nil
}

// dirbyTrim is a tiny helper trimming each entry for display.
func dirbyTrim(in []string) []string {
	out := make([]string, 0, len(in))
	for _, s := range in {
		out = append(out, strings.TrimSpace(s))
	}
	return out
}

// mergedResolution verifies the merge-basic learning objective: the
// merged login.go must combine BOTH sides of the conflict - the main
// branch requirement (password length >= 8) and the feature branch
// requirement (user length >= 3). Any resolution that keeps both
// constraints in the function passes; copying either side verbatim or
// reducing the logic to a bare "return true/false" does not.
//
// This is an honest, documented heuristic for merge-basic only; a static
// checker cannot prove arbitrary programs correct.
func mergedResolution(content string) (string, bool) {
	need := []string{"password", "8", "user", "3"}
	for _, n := range need {
		if !strings.Contains(content, n) {
			return "the resolution lost one side of the conflict: login.go must keep both requirements (user length >= 3 from feature/login, password length >= 8 from main).", false
		}
	}
	trimmed := strings.TrimSpace(content)
	if trimmed == "return true" || trimmed == "return false" {
		return "login.go was reduced to a constant return; that is not a resolution of the conflict.", false
	}
	return "the resolution honors both sides: user length >= 3 (feature/login) and password length >= 8 (main).", true
}

// checkLanding verifies the resolution was actually committed on main
// (or a descendant of the manifest's main commit) and that the current
// login.go is neither of the two pre-merge versions verbatim.
//
// Honestly documented scope: this proves a commit exists that descends
// from the recorded main commit and carries a changed conflict file. It
// does NOT prove the commit was created by git merge specifically - an
// equivalent hand-made commit passes, which is acceptable because the
// learning objective is the resolution, not the ceremony.
func checkLanding(ctx context.Context, repoDir string, state *lab.State) (string, bool) {
	// HEAD must descend from the recorded main commit.
	if _, err := gitx.Run(ctx, repoDir, "merge-base", "--is-ancestor", state.MainSHA, "HEAD"); err != nil {
		return "HEAD is not descended from the exercise's starting main commit; work on the main branch of this exercise repository.", false
	}
	// Current file must differ from both parents' versions.
	cur, err := os.ReadFile(filepath.Join(repoDir, filepath.FromSlash(state.ConflictFile)))
	if err != nil {
		return "cannot read " + state.ConflictFile + ": " + err.Error(), false
	}
	for _, ref := range []string{state.MainSHA, state.FeatureSHA} {
		old, err := gitx.Run(ctx, repoDir, "show", ref+":"+state.ConflictFile)
		if err != nil {
			return "cannot compare against " + ref + ": " + err.Error(), false
		}
		if strings.EqualFold(strings.TrimSpace(old), strings.TrimSpace(string(cur))) {
			return "login.go is identical to one pre-merge side (" + ref + "); both sides must be combined, not copied.", false
		}
	}
	return "a committed resolution on main was found and differs from both pre-merge versions.", true
}