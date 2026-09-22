# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased] - 0.1.0-alpha.3

### Added

- `check` is live: semantic verification of the merge-basic resolution (no unfinished operations, no unresolved index entries, no conflict markers, both sides preserved, resolution committed, clean worktree) with actionable failure diagnostics.
- `internal/check`: read-only checker package; the repository is provably unmodified by a check run (test-asserted).
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
