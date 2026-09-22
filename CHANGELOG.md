# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased] - 0.1.0-alpha.1

### Added

- CLI skeleton (`cmd/git-conflict-lab` + `internal/cli`) with `list`, `version` and `help` commands, friendly usage text and stable exit codes (`0` success, `1` runtime failure, `2` usage error, `4` command known but not implemented yet).
- `start`, `check` and `hint` command surface: argument validation and exercise-id lookup today; their implementations report the milestone that delivers them (B, C and D respectively).
- `internal/gitx`: thin, safe git adapter (argument arrays only, timeout, typed errors).
- `internal/exercise`: embedded, validated exercise catalog shipping its first exercise, `merge-basic` (easy).
- `internal/lab`: filesystem safety boundary - target directory resolution with reserved-name, symlink, traversal and nested-repository protections.
- Continuous integration on Ubuntu and Windows: go vet and go test on every push and pull request targeting `main`.
