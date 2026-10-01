package lab

import (
	"context"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/aslanchikop/git-conflict-lab/internal/exercise"
	"github.com/aslanchikop/git-conflict-lab/internal/gitx"
)

// CheckResult describes the next useful action or a verified completion.
type CheckResult struct {
	ExerciseID string
	Passed     bool
	Message    string
}

func readState(dir string) (State, string, error) {
	root, err := filepath.Abs(dir)
	if err != nil {
		return State{}, "", err
	}
	for {
		data, readErr := os.ReadFile(filepath.Join(root, stateDir, "state.json"))
		if readErr == nil {
			var state State
			if err := json.Unmarshal(data, &state); err != nil {
				return State{}, "", fmt.Errorf("invalid exercise state: %w", err)
			}
			return state, root, nil
		}
		parent := filepath.Dir(root)
		if parent == root {
			return State{}, "", fmt.Errorf("no exercise found; run this command inside a generated lab")
		}
		root = parent
	}
}

// Check inspects Git state without changing the exercise repository.
func Check(ctx context.Context, dir string) (CheckResult, error) {
	state, root, err := readState(dir)
	if err != nil {
		return CheckResult{}, err
	}
	result := CheckResult{ExerciseID: state.ExerciseID}
	sc, ok := LookupScenario(state.ExerciseID)
	if !ok {
		return result, fmt.Errorf("exercise state is incomplete or unsupported")
	}
	if sc.Strategy == StrategyRebase {
		return checkRebase(ctx, state, root)
	}
	if sc.Strategy == StrategyCherryPick {
		return checkCherryPick(ctx, state, root)
	}
	if state.MainSHA == "" || state.FeatureSHA == "" || state.BaseSHA == "" {
		return result, fmt.Errorf("exercise state is incomplete or unsupported")
	}
	if state.FeatureBranch != sc.Branch || state.ConflictFile != sc.File {
		return result, fmt.Errorf("exercise state is incomplete or unsupported")
	}
	git := func(args ...string) (string, error) { return gitx.Run(ctx, root, args...) }
	gitRoot, err := git("rev-parse", "--show-toplevel")
	if err != nil || !sameDir(root, strings.TrimSpace(gitRoot)) {
		return result, fmt.Errorf("exercise state is not at the root of this Git repository")
	}
	for ref, want := range map[string]string{state.FeatureBranch: state.FeatureSHA} {
		got, err := git("rev-parse", "--verify", ref)
		if err != nil || strings.TrimSpace(got) != want {
			return result, fmt.Errorf("exercise Git history differs from its starting state (%s)", ref)
		}
	}
	base, err := git("merge-base", state.MainSHA, state.FeatureSHA)
	if err != nil || strings.TrimSpace(base) != state.BaseSHA {
		return result, fmt.Errorf("exercise Git history differs from its starting state (base commit)")
	}
	if _, err := git("cat-file", "-e", state.MainSHA+"^{commit}"); err != nil {
		return result, fmt.Errorf("original main commit is missing")
	}
	branch, err := git("symbolic-ref", "--short", "HEAD")
	if err != nil || strings.TrimSpace(branch) != "main" {
		result.Message = "Switch to the main branch before checking this exercise."
		return result, nil
	}
	status, err := git("status", "--porcelain")
	if err != nil {
		return result, err
	}
	unmerged, err := git("ls-files", "-u")
	if err != nil {
		return result, err
	}
	if strings.TrimSpace(unmerged) != "" {
		if state.ExerciseID == "merge-multi" {
			paths, _ := git("diff", "--name-only", "--diff-filter=U")
			result.Message = "The merge still has unresolved files: " + strings.Join(strings.Fields(paths), ", ") + ". Resolve and stage every file."
		} else if state.ExerciseID == "modify-delete" {
			result.Message = "The merge still has unresolved files. Use git rm legacy.txt to accept the deletion."
		} else {
			result.Message = fmt.Sprintf("The merge still has unresolved files. Resolve %s, then run git add %s.", state.ConflictFile, state.ConflictFile)
		}
		return result, nil
	}
	if _, err := os.Stat(filepath.Join(root, ".git", "MERGE_HEAD")); err == nil {
		result.Message = "The merge is still in progress. Commit the resolved file with git commit, then run check again."
		return result, nil
	}
	head, err := git("rev-parse", "HEAD")
	if err != nil {
		return result, err
	}
	if strings.TrimSpace(head) == state.MainSHA {
		result.Message = "The exercise has not been merged yet. Run git merge " + state.FeatureBranch + "."
		return result, nil
	}
	parents, err := git("rev-list", "--parents", "-n", "1", "HEAD")
	if err != nil {
		return result, err
	}
	parts := strings.Fields(parents)
	if len(parts) != 3 || parts[1] != state.MainSHA || parts[2] != state.FeatureSHA {
		result.Message = "The final commit must merge " + state.FeatureBranch + " into the original main branch."
		return result, nil
	}
	if strings.TrimSpace(status) != "" {
		result.Message = "Commit or remove remaining changes so git status is clean."
		return result, nil
	}
	if state.ExerciseID == "merge-multi" {
		config, err := os.ReadFile(filepath.Join(root, "config.txt"))
		if err != nil {
			return result, err
		}
		review, err := os.ReadFile(filepath.Join(root, "review.txt"))
		if err != nil {
			return result, err
		}
		if trimExerciseText(config) != "timeout=60" || strings.Contains(string(review), "<<<<<<<") || strings.Contains(string(review), ">>>>>>>") || !strings.Contains(string(review), "manual") || !strings.Contains(string(review), "automated") {
			result.Message = "Set config.txt to timeout=60 and keep both manual and automated review in review.txt."
			return result, nil
		}
		result.Passed = true
		result.Message = "Exercise complete: both conflicted files were resolved in a genuine merge commit."
		return result, nil
	}
	content, err := os.ReadFile(filepath.Join(root, state.ConflictFile))
	if state.ExerciseID == "modify-delete" {
		if err == nil {
			result.Message = "The obsolete legacy.txt should be removed in the final merge."
			return result, nil
		}
		if !os.IsNotExist(err) {
			return result, err
		}
		result.Passed = true
		result.Message = "Exercise complete: the merge commit includes both branches and removes the obsolete file."
		return result, nil
	}
	if err != nil {
		return result, err
	}
	if strings.Contains(string(content), "<<<<<<<") || strings.Contains(string(content), ">>>>>>>") {
		result.Message = "Remove the conflict markers from " + state.ConflictFile + " and commit the corrected result."
		return result, nil
	}
	if state.ExerciseID == "merge-basic" {
		if !validLoginResolution(content) {
			result.Message = "The final Login function must reject empty credentials, require a user name of at least 3 characters, and require a password of at least 12 characters."
			return result, nil
		}
	} else if !strings.Contains(string(content), "Main note: record the release checklist.") || !strings.Contains(string(content), "Feature note: document the review steps.") {
		result.Message = "notes.txt must contain both the main note and the feature note."
		return result, nil
	}
	result.Passed = true
	result.Message = "Exercise complete: the merge commit includes both branches and preserves the required content."
	return result, nil
}

func checkCherryPick(ctx context.Context, state State, root string) (CheckResult, error) {
	result := CheckResult{ExerciseID: state.ExerciseID}
	sc, ok := LookupScenario(state.ExerciseID)
	if !ok || sc.Strategy != StrategyCherryPick || state.FeatureBranch != sc.Branch || state.ConflictFile != sc.File || state.BaseSHA == "" || state.MainSHA == "" || state.FeatureSHA == "" {
		return result, fmt.Errorf("exercise state is incomplete or unsupported")
	}
	git := func(args ...string) (string, error) { return gitx.Run(ctx, root, args...) }
	gitRoot, err := git("rev-parse", "--show-toplevel")
	if err != nil || !sameDir(root, strings.TrimSpace(gitRoot)) {
		return result, fmt.Errorf("exercise state is not at the root of this Git repository")
	}
	feature, err := git("rev-parse", state.FeatureBranch)
	if err != nil || strings.TrimSpace(feature) != state.FeatureSHA {
		return result, fmt.Errorf("original feature branch differs from its starting state")
	}
	base, err := git("merge-base", state.MainSHA, state.FeatureSHA)
	if err != nil || strings.TrimSpace(base) != state.BaseSHA {
		return result, fmt.Errorf("exercise base commit differs from its starting state")
	}
	branch, err := git("symbolic-ref", "--short", "HEAD")
	if err != nil || strings.TrimSpace(branch) != "main" {
		result.Message = "Switch to main before cherry-picking feature/audit."
		return result, nil
	}
	if pick, err := os.ReadFile(filepath.Join(root, ".git", "CHERRY_PICK_HEAD")); err == nil {
		if strings.TrimSpace(string(pick)) != state.FeatureSHA {
			return result, fmt.Errorf("a different commit is being cherry-picked")
		}
		unmerged, err := git("ls-files", "-u")
		if err != nil {
			return result, err
		}
		if strings.TrimSpace(unmerged) != "" {
			result.Message = "The cherry-pick is paused at a conflict. Set policy.txt to audit=required+verbose, then run git add policy.txt."
		} else {
			result.Message = "The conflict is staged. Run git cherry-pick --continue."
		}
		return result, nil
	}
	head, err := git("rev-parse", "HEAD")
	if err != nil {
		return result, err
	}
	if strings.TrimSpace(head) == state.MainSHA {
		result.Message = "The feature commit has not been applied yet. Run git cherry-pick feature/audit."
		return result, nil
	}
	parents, err := git("rev-list", "--parents", "-n", "1", "HEAD")
	if err != nil {
		return result, err
	}
	parts := strings.Fields(parents)
	if len(parts) != 2 || parts[1] != state.MainSHA {
		result.Message = "The replayed commit must sit directly on the original main commit."
		return result, nil
	}
	subject, err := git("log", "-1", "--format=%s")
	if err != nil {
		return result, err
	}
	if strings.TrimSpace(subject) != "feat: add verbose audit" {
		result.Message = "Keep the selected feature commit message when continuing the cherry-pick."
		return result, nil
	}
	status, err := git("status", "--porcelain")
	if err != nil {
		return result, err
	}
	if strings.TrimSpace(status) != "" {
		result.Message = "Commit or remove remaining changes so git status is clean."
		return result, nil
	}
	data, err := os.ReadFile(filepath.Join(root, "policy.txt"))
	if err != nil {
		return result, err
	}
	if trimExerciseText(data) != "audit=required+verbose" {
		result.Message = "policy.txt must contain audit=required+verbose after the cherry-pick."
		return result, nil
	}
	result.Passed = true
	result.Message = "Exercise complete: the selected commit was replayed on main with the combined audit policy."
	return result, nil
}

func checkRebase(ctx context.Context, state State, root string) (CheckResult, error) {
	result := CheckResult{ExerciseID: state.ExerciseID}
	sc, ok := LookupScenario(state.ExerciseID)
	if !ok || sc.Strategy != StrategyRebase || state.FeatureBranch != sc.Branch || state.ConflictFile != sc.File ||
		state.MainSHA == "" || state.FeatureSHA == "" || state.BaseSHA == "" {
		return result, fmt.Errorf("exercise state is incomplete or unsupported")
	}
	git := func(args ...string) (string, error) { return gitx.Run(ctx, root, args...) }
	gitRoot, err := git("rev-parse", "--show-toplevel")
	if err != nil || !sameDir(root, strings.TrimSpace(gitRoot)) {
		return result, fmt.Errorf("exercise state is not at the root of this Git repository")
	}
	main, err := git("rev-parse", "main")
	if err != nil || strings.TrimSpace(main) != state.MainSHA {
		return result, fmt.Errorf("main branch differs from the exercise starting state")
	}
	base, err := git("merge-base", state.MainSHA, state.FeatureSHA)
	if err != nil || strings.TrimSpace(base) != state.BaseSHA {
		return result, fmt.Errorf("exercise base commit differs from its starting state")
	}
	if _, err := git("cat-file", "-e", state.FeatureSHA+"^{commit}"); err != nil {
		return result, fmt.Errorf("original feature commit is missing")
	}
	_, mergeStateErr := os.Stat(filepath.Join(root, ".git", "rebase-merge"))
	_, applyStateErr := os.Stat(filepath.Join(root, ".git", "rebase-apply"))
	if mergeStateErr == nil || applyStateErr == nil {
		unmerged, err := git("ls-files", "-u")
		if err != nil {
			return result, err
		}
		if strings.TrimSpace(unmerged) != "" {
			result.Message = "Rebase is paused at a conflict. Set settings.txt to mode=safe-fast, then run git add settings.txt."
		} else {
			result.Message = "The conflict is staged. Run git rebase --continue."
		}
		return result, nil
	}
	branch, err := git("symbolic-ref", "--short", "HEAD")
	if err != nil || strings.TrimSpace(branch) != state.FeatureBranch {
		result.Message = "Switch to feature/fast, then run git rebase main."
		return result, nil
	}
	feature, err := git("rev-parse", state.FeatureBranch)
	if err != nil {
		return result, err
	}
	if strings.TrimSpace(feature) == state.FeatureSHA {
		result.Message = "The feature branch has not been rebased yet. Run git rebase main."
		return result, nil
	}
	parents, err := git("rev-list", "--parents", "-n", "1", "HEAD")
	if err != nil {
		return result, err
	}
	parts := strings.Fields(parents)
	if len(parts) != 2 || parts[1] != state.MainSHA {
		result.Message = "The replayed feature commit must sit directly on top of main."
		return result, nil
	}
	subject, err := git("log", "-1", "--format=%s")
	if err != nil {
		return result, err
	}
	if strings.TrimSpace(subject) != "feat: enable fast mode" {
		result.Message = "Keep the feature commit message when continuing the rebase."
		return result, nil
	}
	status, err := git("status", "--porcelain")
	if err != nil {
		return result, err
	}
	if strings.TrimSpace(status) != "" {
		result.Message = "Commit or remove remaining changes so git status is clean."
		return result, nil
	}
	data, err := os.ReadFile(filepath.Join(root, "settings.txt"))
	if err != nil {
		return result, err
	}
	if trimExerciseText(data) != "mode=safe-fast" {
		result.Message = "settings.txt must contain mode=safe-fast after the rebase."
		return result, nil
	}
	result.Passed = true
	result.Message = "Exercise complete: the feature commit was replayed on main and the resolved setting is safe-fast."
	return result, nil
}

func trimExerciseText(data []byte) string {
	return strings.TrimSpace(strings.TrimPrefix(string(data), "\uFEFF"))
}

// validLoginResolution accepts the intended logic while allowing reordered
// conditions and equivalent boolean expressions. It only interprets a small,
// safe subset of Go expressions; it never executes learner code.
func validLoginResolution(src []byte) bool {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "login.go", src, 0)
	if err != nil || file.Name.Name != "auth" {
		return false
	}
	if _, err := (&types.Config{Importer: importer.Default()}).Check("auth", fset, []*ast.File{file}, nil); err != nil {
		return false
	}
	var fn *ast.FuncDecl
	count := 0
	for _, decl := range file.Decls {
		if f, ok := decl.(*ast.FuncDecl); ok && f.Name.Name == "Login" {
			fn = f
			count++
		}
	}
	if count != 1 || fn == nil || fn.Body == nil || len(fn.Body.List) != 2 {
		return false
	}
	if fn.Recv != nil || fn.Type.Params == nil || fn.Type.Results == nil || len(fn.Type.Results.List) != 1 {
		return false
	}
	var names []string
	for _, param := range fn.Type.Params.List {
		paramType, ok := param.Type.(*ast.Ident)
		if !ok || paramType.Name != "string" {
			return false
		}
		for _, name := range param.Names {
			names = append(names, name.Name)
		}
	}
	if len(names) != 2 || names[0] != "user" || names[1] != "password" {
		return false
	}
	resultType, ok := fn.Type.Results.List[0].Type.(*ast.Ident)
	if !ok || resultType.Name != "bool" {
		return false
	}
	guard, ok := fn.Body.List[0].(*ast.IfStmt)
	if !ok || guard.Else != nil || len(guard.Body.List) != 1 {
		return false
	}
	guardReturn, ok := guard.Body.List[0].(*ast.ReturnStmt)
	if !ok || len(guardReturn.Results) != 1 {
		return false
	}
	finalReturn, ok := fn.Body.List[1].(*ast.ReturnStmt)
	if !ok || len(finalReturn.Results) != 1 {
		return false
	}
	for _, user := range []string{"", "ab", "abc", "longname"} {
		for _, pass := range []string{"", "1234567", "12345678", "123456789012", "1234567890123"} {
			condition, ok := evalBool(guard.Cond, user, pass)
			if !ok {
				return false
			}
			guardValue, ok := evalBool(guardReturn.Results[0], user, pass)
			if !ok {
				return false
			}
			finalValue, ok := evalBool(finalReturn.Results[0], user, pass)
			if !ok {
				return false
			}
			got := finalValue
			if condition {
				got = guardValue
			}
			want := len(user) >= 3 && len(pass) >= 12
			if got != want {
				return false
			}
		}
	}
	return true
}

func evalBool(expr ast.Expr, user, pass string) (bool, bool) {
	switch e := expr.(type) {
	case *ast.ParenExpr:
		return evalBool(e.X, user, pass)
	case *ast.Ident:
		if e.Name == "true" {
			return true, true
		}
		if e.Name == "false" {
			return false, true
		}
	case *ast.UnaryExpr:
		if e.Op == token.NOT {
			v, ok := evalBool(e.X, user, pass)
			return !v, ok
		}
	case *ast.BinaryExpr:
		if e.Op == token.LAND || e.Op == token.LOR {
			a, aok := evalBool(e.X, user, pass)
			b, bok := evalBool(e.Y, user, pass)
			if !aok || !bok {
				return false, false
			}
			if e.Op == token.LAND {
				return a && b, true
			}
			return a || b, true
		}
		a, aok := evalValue(e.X, user, pass)
		b, bok := evalValue(e.Y, user, pass)
		if !aok || !bok {
			return false, false
		}
		switch e.Op {
		case token.EQL:
			return a == b, true
		case token.NEQ:
			return a != b, true
		case token.GEQ:
			return a >= b, true
		case token.GTR:
			return a > b, true
		case token.LEQ:
			return a <= b, true
		case token.LSS:
			return a < b, true
		}
	}
	return false, false
}

func evalValue(expr ast.Expr, user, pass string) (int, bool) {
	switch e := expr.(type) {
	case *ast.ParenExpr:
		return evalValue(e.X, user, pass)
	case *ast.BasicLit:
		if e.Kind == token.INT {
			n, err := strconv.Atoi(e.Value)
			return n, err == nil
		}
		if e.Kind == token.STRING {
			s, err := strconv.Unquote(e.Value)
			return len(s), err == nil
		}
	case *ast.CallExpr:
		if len(e.Args) == 1 {
			if id, ok := e.Fun.(*ast.Ident); ok && id.Name == "len" {
				if arg, ok := e.Args[0].(*ast.Ident); ok {
					if arg.Name == "user" {
						return len(user), true
					}
					if arg.Name == "password" {
						return len(pass), true
					}
				}
			}
		}
	case *ast.Ident:
		if e.Name == "user" {
			return len(user), true
		}
		if e.Name == "password" {
			return len(pass), true
		}
	}
	return 0, false
}

// NextHint returns hints in order and repeats the final hint on later calls.
func NextHint(dir, id string) (string, error) {
	state, root, err := readState(dir)
	if err != nil {
		return "", err
	}
	if state.ExerciseID != id {
		return "", fmt.Errorf("this repository contains %s, not %s", state.ExerciseID, id)
	}
	spec, err := exercise.Get(id)
	if err != nil {
		return "", err
	}
	path := filepath.Join(root, stateDir, "hint-index")
	index := 0
	if data, err := os.ReadFile(path); err == nil {
		if value, parseErr := strconv.Atoi(strings.TrimSpace(string(data))); parseErr == nil && value >= 0 {
			index = value
		}
	}
	if index >= len(spec.Hints) {
		index = len(spec.Hints) - 1
	}
	if err := os.WriteFile(path, []byte(strconv.Itoa(index+1)), 0o644); err != nil {
		return "", err
	}
	return fmt.Sprintf("Hint %d/%d: %s", index+1, len(spec.Hints), spec.Hints[index]), nil
}
