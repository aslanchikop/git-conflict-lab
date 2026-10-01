// scenario.go is the single source of truth for the Git-side facts of every
// exercise scenario: the feature branch to integrate, the conflict files,
// the common-ancestor ref at generation time, and the strategy (merge,
// rebase, or cherry-pick) the learner is expected to use. The exercise
// catalog (internal/exercise) carries the teaching content; this registry
// carries the repository mechanics, so the generator, checker, inspector,
// and CLI all agree on branch and file names.
package lab

// Strategy values for Scenario.Strategy: the Git command family the
// learner uses to surface and resolve the conflict.
const (
	// StrategyMerge integrates the feature branch with git merge.
	StrategyMerge = "merge"
	// StrategyRebase replays the feature branch with git rebase.
	StrategyRebase = "rebase"
	// StrategyCherryPick replays a selected commit with git cherry-pick.
	StrategyCherryPick = "cherry-pick"
)

// Scenario describes the Git-side facts of one exercise scenario.
type Scenario struct {
	// ID is the exercise id in the embedded catalog.
	ID string
	// Branch is the feature branch the learner integrates into main.
	Branch string
	// File is the primary conflict file.
	File string
	// Files lists every conflict file. It is longer than one file only
	// for multi-file scenarios; the manifest records it only then.
	Files []string
	// BaseRef is the rev-parse spec of the common ancestor commit at
	// generation time (used for the manifest's base SHA).
	BaseRef string
	// Strategy is one of StrategyMerge, StrategyRebase, StrategyCherryPick.
	Strategy string
	// BaseAbsent reports that the conflict file does not exist in the
	// common ancestor (add/add conflict).
	BaseAbsent bool
	// FeatureAbsent reports that the conflict file was deleted on the
	// feature branch (modify/delete conflict).
	FeatureAbsent bool
}

// scenarios lists every supported exercise scenario. IDs must match the
// embedded exercise catalog exactly.
var scenarios = []Scenario{
	{ID: "merge-basic", Branch: "feature/login", File: "login.go", BaseRef: "main~2", Strategy: StrategyMerge},
	{ID: "add-add", Branch: "feature/notes", File: "notes.txt", BaseRef: "main~", Strategy: StrategyMerge, BaseAbsent: true},
	{ID: "modify-delete", Branch: "feature/cleanup", File: "legacy.txt", BaseRef: "main~", Strategy: StrategyMerge, FeatureAbsent: true},
	{ID: "rebase-basic", Branch: "feature/fast", File: "settings.txt", BaseRef: "main~", Strategy: StrategyRebase},
	{ID: "merge-multi", Branch: "feature/release", File: "config.txt", Files: []string{"config.txt", "review.txt"}, BaseRef: "main~", Strategy: StrategyMerge},
	{ID: "cherry-pick", Branch: "feature/audit", File: "policy.txt", BaseRef: "main~", Strategy: StrategyCherryPick},
}

// LookupScenario returns the scenario registered for the exercise id.
func LookupScenario(id string) (Scenario, bool) {
	for _, s := range scenarios {
		if s.ID == id {
			return s, true
		}
	}
	return Scenario{}, false
}
