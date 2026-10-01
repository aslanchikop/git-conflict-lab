# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased] - 0.1.0-alpha.3

### Added

- A working `check` command that verifies merge history, conflict resolution, and exercise-specific outcomes without executing learner code.
- Progressive `hint` delivery for generated labs.
- `add-add`, `modify-delete`, and `rebase-basic` exercises with end-to-end Git tests.
- Named repeat attempts via `start <id> --attempt <name>`.
- A synthetic repository-local Git identity so first-time users can merge and commit without global Git setup.
- An offline localhost browser interface (`ui`) for choosing exercises, managing attempts, revealing hints, and checking solutions.
- A redesigned responsive interface with search, difficulty filters, loading states, completion animation, and five achievements based on verified Git progress.
- A read-only four-version conflict inspector, contextual solution review, a beginner learning path, and a portfolio walkthrough.
- Scenario-specific concepts, self-check questions, explanations, and common mistakes in the browser workspace.
- Two advanced exercises: a two-file merge and a conflicting cherry-pick, each with history verification, hints, and a browser walkthrough.
- A seven-skill progress map derived from verified completed repositories, plus two new achievements.
- macOS CI coverage and a tag-triggered multi-platform release workflow.

### Changed

- `start` now prints the full path to the generated repository and scenario-specific merge instructions.
- README and help reflect the working commands and current scope.

## [Unreleased] - 0.1.0-alpha.2

### Added

- `start <exercise-id>` is now live for `merge-basic`: generates a deterministic, isolated Git repository under `~/git-conflict-lab-labs/` (override with `GIT_CONFLICT_LAB_LABS`) with `main` and `feature/login` branches that genuinely conflict on merge.
- `internal/lab` generation engine: TOCTOU re-validation, repo-local inline identity (no global config writes), pinned commit dates for reproducible SHAs, and an advisory state manifest (`.git-conflict-lab/state.json`) for the Milestone C checker.
## [Unreleased] - 0.1.0-alpha.1

### Added

- CLI skeleton (`cmd/git-conflict-lab` + `internal/cli`) with `list`, `version` and `help` commands, friendly usage text and stable exit codes (`0` success, `1` runtime failure, `2` usage error, `4` command known but not implemented yet).
- `start`, `check` and `hint` command surface: argument validation and exercise-id lookup today; their implementations report the milestone that delivers them (B, C and D respectively).
- `internal/gitx`: thin, safe git adapter (argument arrays only, timeout, typed errors).
- `internal/exercise`: embedded, validated exercise catalog shipping its first exercise, `merge-basic` (easy).
- `internal/lab`: filesystem safety boundary - target directory resolution with reserved-name, symlink, traversal and nested-repository protections.
- Continuous integration on Ubuntu and Windows: go vet and go test on every push and pull request targeting `main`.
