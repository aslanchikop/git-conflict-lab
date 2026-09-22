package check

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aslanchikop/git-conflict-lab/internal/gitx"
	"github.com/aslanchikop/git-conflict-lab/internal/lab"
)

// mergeEnv gives every commit-creating git call an identity, mirroring
// what users have via their own git config.
var mergeEnv = []string{
	"GIT_AUTHOR_NAME=Git Conflict Lab",
	"GIT_AUTHOR_EMAIL=git-conflict-lab@localhost",
	"GIT_COMMITTER_NAME=Git Conflict Lab",
	"GIT_COMMITTER_EMAIL=git-conflict-lab@localhost",
}

// genExercise generates a fresh merge-basic repo and returns its path.
func genExercise(t *testing.T) string {
	t.Helper()
	repo, err := lab.Generate(context.Background(), t.TempDir(), "merge-basic")
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}
	return repo
}

// resolveCorrectly simulates the correct user flow: merge, resolve with
// both requirements, commit.
func resolveCorrectly(t *testing.T, repo string) {
	t.Helper()
	ctx := context.Background()
	if _, err := gitx.RunWithEnv(ctx, repo, mergeEnv, "merge", "feature/login"); err == nil {
		t.Fatal("expected the merge to conflict")
	}
	resolved := `package auth

// Login checks the given credentials and reports validity.
func Login(user, password string) bool {
	if user == "" || password == "" {
		return false
	}
	if len(user) < 3 {
		return false
	}
	return len(password) >= 8
}
`
	if err := os.WriteFile(filepath.Join(repo, "login.go"), []byte(resolved), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := gitx.RunWithEnv(ctx, repo, mergeEnv, "add", "login.go"); err != nil {
		t.Fatal(err)
	}
	if _, err := gitx.RunWithEnv(ctx, repo, mergeEnv, "commit", "-m", "merge: combine both login requirements"); err != nil {
		t.Fatal(err)
	}
}

func TestCorrectResolutionPasses(t *testing.T) {
	repo := genExercise(t)
	resolveCorrectly(t, repo)

	before := repoFingerprint(t, repo)
	res, err := Check(context.Background(), repo)
	if err != nil {
		t.Fatalf("Check error: %v", err)
	}
	if !res.Passed {
		t.Fatalf("correct resolution must pass; results:\n%s", dumpResults(res))
	}
	if repoFingerprint(t, repo) != before {
		t.Fatal("Check modified the repository")
	}
}

func TestUnresolvedMergeFails(t *testing.T) {
	repo := genExercise(t)
	ctx := context.Background()
	if _, err := gitx.RunWithEnv(ctx, repo, mergeEnv, "merge", "feature/login"); err == nil {
		t.Fatal("expected the merge to conflict")
	}
	res, err := Check(context.Background(), repo)
	if err != nil {
		t.Fatalf("Check error: %v", err)
	}
	if res.Passed {
		t.Fatal("unresolved merge must not pass")
	}
	if !hasFailedCheck(res, "unfinished-operations") && !hasFailedCheck(res, "unresolved-index") {
		t.Errorf("expected unfinished-operations or unresolved-index to fail; results:\n%s", dumpResults(res))
	}
}

func TestMarkersLeftInFileFails(t *testing.T) {
	repo := genExercise(t)
	ctx := context.Background()
	_, _ = gitx.RunWithEnv(ctx, repo, mergeEnv, "merge", "feature/login")
	// "Resolve" by committing the file with markers intact.
	if _, err := gitx.RunWithEnv(ctx, repo, mergeEnv, "add", "login.go"); err != nil {
		t.Fatal(err)
	}
	if _, err := gitx.RunWithEnv(ctx, repo, mergeEnv, "commit", "-m", "bad: keep markers"); err != nil {
		t.Fatal(err)
	}
	res, _ := Check(context.Background(), repo)
	if res.Passed {
		t.Fatal("markers in file must not pass")
	}
	if !hasFailedCheck(res, "conflict-file") {
		t.Errorf("expected conflict-file check to fail; results:\n%s", dumpResults(res))
	}
}

func TestOneSideLostFails(t *testing.T) {
	repo := genExercise(t)
	ctx := context.Background()
	_, _ = gitx.RunWithEnv(ctx, repo, mergeEnv, "merge", "feature/login")
	// Take the feature side verbatim: file equals feature version.
	featureFile, err := gitx.Run(ctx, repo, "show", "feature/login:login.go")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "login.go"), []byte(featureFile), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := gitx.RunWithEnv(ctx, repo, mergeEnv, "add", "login.go"); err != nil {
		t.Fatal(err)
	}
	if _, err := gitx.RunWithEnv(ctx, repo, mergeEnv, "commit", "-m", "bad: feature side verbatim"); err != nil {
		t.Fatal(err)
	}
	res, _ := Check(context.Background(), repo)
	if res.Passed {
		t.Fatal("one-side-verbatim resolution must not pass")
	}
	if !hasFailedCheck(res, "both-sides-preserved") && !hasFailedCheck(res, "resolution-landed") {
		t.Errorf("expected both-sides-preserved or resolution-landed to fail; results:\n%s", dumpResults(res))
	}
}

func TestCheckOutsideRepoFailsFriendly(t *testing.T) {
	dir := t.TempDir()
	res, err := Check(context.Background(), dir)
	if err != nil {
		t.Fatalf("Check must return a result, not an error, outside a repo: %v", err)
	}
	if res.Passed {
		t.Fatal("empty dir must not pass")
	}
	if !hasFailedCheck(res, "manifest") || !strings.Contains(failedDetail(res, "manifest"), "git-conflict-lab start") {
		t.Errorf("manifest failure must carry actionable guidance; got: %s", dumpResults(res))
	}
}

func TestTamperedManifestFails(t *testing.T) {
	repo := genExercise(t)
	resolveCorrectly(t, repo)
	manifest := filepath.Join(repo, lab.StateDir, "state.json")
	data, err := os.ReadFile(manifest)
	if err != nil {
		t.Fatal(err)
	}
	bad := strings.Replace(string(data), "main_sha", "main_sha_tampered", 1)
	if err := os.WriteFile(manifest, []byte(bad), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := Check(context.Background(), repo)
	if err != nil {
		// A damaged manifest may surface as a hard error; both are acceptable
		// as long as it does not pass.
		t.Logf("tampered manifest returned error: %v", err)
		return
	}
	if res.Passed {
		t.Fatal("tampered manifest must not pass")
	}
}

func TestUncommittedResolutionFails(t *testing.T) {
	repo := genExercise(t)
	ctx := context.Background()
	_, _ = gitx.RunWithEnv(ctx, repo, mergeEnv, "merge", "feature/login")
	resolved := "package auth\n\nfunc Login(user, password string) bool {\n\treturn len(user) >= 3 && len(password) >= 8\n}\n"
	if err := os.WriteFile(filepath.Join(repo, "login.go"), []byte(resolved), 0o644); err != nil {
		t.Fatal(err)
	}
	res, _ := Check(context.Background(), repo)
	if res.Passed {
		t.Fatal("uncommitted resolution must not pass")
	}
	if !hasFailedCheck(res, "clean-worktree") {
		t.Errorf("expected clean-worktree to fail; results:\n%s", dumpResults(res))
	}
}

// repoFingerprint captures git status output as a cheap mutation guard.
func repoFingerprint(t *testing.T, repo string) string {
	t.Helper()
	out, err := gitx.Run(context.Background(), repo, "status", "--porcelain")
	if err != nil {
		t.Fatalf("git status failed: %v", err)
	}
	return out
}

func hasFailedCheck(res Result, name string) bool {
	for _, c := range res.Checks {
		if c.Name == name && !c.Passed {
			return true
		}
	}
	return false
}

func failedDetail(res Result, name string) string {
	for _, c := range res.Checks {
		if c.Name == name {
			return c.Detail
		}
	}
	return ""
}

func dumpResults(res Result) string {
	var b strings.Builder
	for _, c := range res.Checks {
		status := "ok  "
		if !c.Passed {
			status = "FAIL"
		}
		b.WriteString(status + " " + c.Name + ": " + c.Detail + "\n")
	}
	return b.String()
}
