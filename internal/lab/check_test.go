package lab

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aslanchikop/git-conflict-lab/internal/gitx"
)

func TestCheckExerciseJourney(t *testing.T) {
	skipSlow(t)
	repo := generateInto(t)
	ctx := context.Background()
	check := func(wantPass bool, wantText string) {
		t.Helper()
		result, err := Check(ctx, repo)
		if err != nil {
			t.Fatal(err)
		}
		if result.Passed != wantPass || !strings.Contains(result.Message, wantText) {
			t.Fatalf("check = %+v, want passed=%v and %q", result, wantPass, wantText)
		}
	}
	check(false, "git merge")
	mergeEnv := []string{"GIT_AUTHOR_NAME=Lab", "GIT_AUTHOR_EMAIL=lab@localhost", "GIT_COMMITTER_NAME=Lab", "GIT_COMMITTER_EMAIL=lab@localhost"}
	if _, err := gitx.RunWithEnv(ctx, repo, mergeEnv, "merge", "feature/login"); err == nil {
		t.Fatal("expected conflict")
	}
	check(false, "unresolved")
	wrong := `package auth
func Login(user, password string) bool {
 if user == "" || password == "" { return false }
 return len(password) >= 8
}
`
	path := filepath.Join(repo, "login.go")
	if err := os.WriteFile(path, []byte(wrong), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := gitx.Run(ctx, repo, "add", "login.go"); err != nil {
		t.Fatal(err)
	}
	check(false, "git commit")
	if _, err := gitx.RunWithEnv(ctx, repo, mergeEnv, "commit", "-m", "Resolve conflict"); err != nil {
		t.Fatal(err)
	}
	check(false, "at least 3")
	// Amend the merge commit with the correct solution and verify its two parents.
	correct := `package auth
func Login(user, password string) bool {
 if user == "" || password == "" { return false }
 return len(user) >= 3 && len(password) >= 12
}
`
	if err := os.WriteFile(path, []byte(correct), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := gitx.Run(ctx, repo, "add", "login.go"); err != nil {
		t.Fatal(err)
	}
	if _, err := gitx.RunWithEnv(ctx, repo, mergeEnv, "commit", "--amend", "--no-edit"); err != nil {
		t.Fatal(err)
	}
	check(true, "Exercise complete")
}

func TestHintsAdvanceAndStop(t *testing.T) {
	skipSlow(t)
	repo := generateInto(t)
	for i := 1; i <= 4; i++ {
		got, err := NextHint(repo, "merge-basic")
		if err != nil {
			t.Fatal(err)
		}
		want := i
		if want > 3 {
			want = 3
		}
		if !strings.Contains(got, "Hint "+string(rune('0'+want))+"/3") {
			t.Fatalf("hint %d: %q", i, got)
		}
	}
}

func TestAddAddJourney(t *testing.T) {
	skipSlow(t)
	repo, err := Generate(context.Background(), t.TempDir(), "add-add")
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	mergeEnv := []string{"GIT_AUTHOR_NAME=Lab", "GIT_AUTHOR_EMAIL=lab@localhost", "GIT_COMMITTER_NAME=Lab", "GIT_COMMITTER_EMAIL=lab@localhost"}
	if _, err := gitx.RunWithEnv(ctx, repo, mergeEnv, "merge", "feature/notes"); err == nil {
		t.Fatal("expected add/add conflict")
	}
	result, err := Check(ctx, repo)
	if err != nil || result.Passed || !strings.Contains(result.Message, "unresolved") {
		t.Fatalf("check during conflict = %+v, %v", result, err)
	}
	content := "Main note: record the release checklist.\nFeature note: document the review steps.\n"
	if err := os.WriteFile(filepath.Join(repo, "notes.txt"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := gitx.Run(ctx, repo, "add", "notes.txt"); err != nil {
		t.Fatal(err)
	}
	if _, err := gitx.RunWithEnv(ctx, repo, mergeEnv, "commit", "-m", "Combine notes"); err != nil {
		t.Fatal(err)
	}
	result, err = Check(ctx, repo)
	if err != nil || !result.Passed {
		t.Fatalf("completed add/add check = %+v, %v", result, err)
	}
}

func TestModifyDeleteJourney(t *testing.T) {
	skipSlow(t)
	repo, err := Generate(context.Background(), t.TempDir(), "modify-delete")
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	mergeEnv := []string{"GIT_AUTHOR_NAME=Lab", "GIT_AUTHOR_EMAIL=lab@localhost", "GIT_COMMITTER_NAME=Lab", "GIT_COMMITTER_EMAIL=lab@localhost"}
	if _, err := gitx.RunWithEnv(ctx, repo, mergeEnv, "merge", "feature/cleanup"); err == nil {
		t.Fatal("expected modify/delete conflict")
	}
	result, err := Check(ctx, repo)
	if err != nil || result.Passed || !strings.Contains(result.Message, "git rm") {
		t.Fatalf("check during conflict = %+v, %v", result, err)
	}
	if _, err := gitx.Run(ctx, repo, "rm", "legacy.txt"); err != nil {
		t.Fatal(err)
	}
	if _, err := gitx.RunWithEnv(ctx, repo, mergeEnv, "commit", "-m", "Remove obsolete file"); err != nil {
		t.Fatal(err)
	}
	result, err = Check(ctx, repo)
	if err != nil || !result.Passed {
		t.Fatalf("completed modify/delete check = %+v, %v", result, err)
	}
}

func TestLoginResolutionRejectsInvalidSignature(t *testing.T) {
	src := []byte(`package auth
func Login() bool {
 if user == "" || password == "" { return false }
 return len(user) >= 3 && len(password) >= 12
}`)
	if validLoginResolution(src) {
		t.Fatal("accepted a function that cannot compile")
	}
}

func TestRebaseJourney(t *testing.T) {
	skipSlow(t)
	repo, err := Generate(context.Background(), t.TempDir(), "rebase-basic")
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	check := func(wantPass bool, text string) {
		t.Helper()
		got, err := Check(ctx, repo)
		if err != nil || got.Passed != wantPass || !strings.Contains(got.Message, text) {
			t.Fatalf("check=%+v error=%v; want passed=%v and %q", got, err, wantPass, text)
		}
	}
	check(false, "Switch")
	if _, err := gitx.Run(ctx, repo, "switch", "feature/fast"); err != nil {
		t.Fatal(err)
	}
	check(false, "git rebase")
	rebaseEnv := []string{"GIT_AUTHOR_NAME=Lab", "GIT_AUTHOR_EMAIL=lab@localhost", "GIT_COMMITTER_NAME=Lab", "GIT_COMMITTER_EMAIL=lab@localhost", "GIT_EDITOR=true"}
	if _, err := gitx.RunWithEnv(ctx, repo, rebaseEnv, "rebase", "main"); err == nil {
		t.Fatal("expected rebase conflict")
	}
	check(false, "paused at a conflict")
	if err := os.WriteFile(filepath.Join(repo, "settings.txt"), []byte("mode=safe-fast\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := gitx.Run(ctx, repo, "add", "settings.txt"); err != nil {
		t.Fatal(err)
	}
	check(false, "rebase --continue")
	if out, err := gitx.RunWithEnv(ctx, repo, rebaseEnv, "rebase", "--continue"); err != nil {
		t.Fatalf("continue: %v\n%s", err, out)
	}
	check(true, "Exercise complete")
}

func TestMergeMultiJourney(t *testing.T) {
	skipSlow(t)
	repo, err := Generate(context.Background(), t.TempDir(), "merge-multi")
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if _, err := gitx.Run(ctx, repo, "merge", "feature/release"); err == nil {
		t.Fatal("expected two-file conflict")
	}
	check := func(pass bool, text string) {
		t.Helper()
		got, err := Check(ctx, repo)
		if err != nil || got.Passed != pass || !strings.Contains(got.Message, text) {
			t.Fatalf("check=%+v, err=%v", got, err)
		}
	}
	check(false, "config.txt")
	if err := os.WriteFile(filepath.Join(repo, "config.txt"), []byte("\uFEFFtimeout=60\r\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := gitx.Run(ctx, repo, "add", "config.txt"); err != nil {
		t.Fatal(err)
	}
	check(false, "review.txt")
	if err := os.WriteFile(filepath.Join(repo, "review.txt"), []byte("review=manual\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := gitx.Run(ctx, repo, "add", "review.txt"); err != nil {
		t.Fatal(err)
	}
	check(false, "git commit")
	if _, err := gitx.Run(ctx, repo, "commit", "-m", "Resolve release merge"); err != nil {
		t.Fatal(err)
	}
	check(false, "both manual and automated")
	if err := os.WriteFile(filepath.Join(repo, "review.txt"), []byte("review=manual+automated\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := gitx.Run(ctx, repo, "add", "review.txt"); err != nil {
		t.Fatal(err)
	}
	if _, err := gitx.Run(ctx, repo, "commit", "--amend", "--no-edit"); err != nil {
		t.Fatal(err)
	}
	check(true, "Exercise complete")
}

func TestCherryPickJourney(t *testing.T) {
	skipSlow(t)
	repo, err := Generate(context.Background(), t.TempDir(), "cherry-pick")
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	check := func(pass bool, text string) {
		t.Helper()
		got, err := Check(ctx, repo)
		if err != nil || got.Passed != pass || !strings.Contains(got.Message, text) {
			t.Fatalf("check=%+v, err=%v", got, err)
		}
	}
	check(false, "git cherry-pick")
	if _, err := gitx.Run(ctx, repo, "cherry-pick", "feature/audit"); err == nil {
		t.Fatal("expected cherry-pick conflict")
	}
	check(false, "paused at a conflict")
	if err := os.WriteFile(filepath.Join(repo, "policy.txt"), []byte("\uFEFFaudit=required+verbose\r\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := gitx.Run(ctx, repo, "add", "policy.txt"); err != nil {
		t.Fatal(err)
	}
	check(false, "cherry-pick --continue")
	if out, err := gitx.RunWithEnv(ctx, repo, []string{"GIT_EDITOR=true"}, "cherry-pick", "--continue"); err != nil {
		t.Fatalf("continue: %v\n%s", err, out)
	}
	check(true, "Exercise complete")
}
