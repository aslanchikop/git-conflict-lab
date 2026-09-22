# Git Conflict Lab

A free, open-source, offline CLI training utility for practicing real Git conflict resolution in safely generated, isolated repositories.

> Status: Milestone C (verification engine). Generation and the solution checker for `merge-basic` are live; hints arrive in upcoming milestones tracked in GitHub Issues.
>
> Checker limits (honest): `check` verifies documented invariants for `merge-basic` - no unfinished Git operation, no unresolved index entries, no conflict markers, both sides of the conflict preserved, resolution committed on main, clean worktree. A static checker cannot infer every possible valid workflow; equivalent hand-made resolutions pass, and `check` never modifies your repository.
>
> Platforms: Windows is verified locally and in CI; Linux is verified in CI; macOS support is planned but not yet verified.

## Prerequisites

- [Git](https://git-scm.com) available on your `PATH`
- [Go 1.24+](https://go.dev) only if you build from source

## Usage

What works **today**:

- `git-conflict-lab list` - show the exercises in the embedded catalog
- `git-conflict-lab start merge-basic` - create an isolated exercise repository under `~/git-conflict-lab-labs/merge-basic` (override the location with `GIT_CONFLICT_LAB_LABS`)
- `git-conflict-lab check` - verify your resolution inside an exercise repository
- `git-conflict-lab version` - show the tool version
- `git-conflict-lab help` - show usage and the command list

`start` generates a real Git repository with `main` and `feature/login` branches whose merge conflicts on purpose. Solve it with real Git commands, then `git-conflict-lab check` verifies your resolution.

What arrives **later** (the commands already exist and report their milestone):

| Command | Delivered in | What it will do |
|---------|--------------|-----------------|
| `hint <exercise-id>` | Milestone D | Reveal the next hint |

Exit codes: `0` success, `1` runtime failure, `2` usage error, `4` command known but not implemented yet.

## Planned session

```
git-conflict-lab list
git-conflict-lab start merge-basic
cd merge-basic
# ... use real Git to resolve the conflict ...
git-conflict-lab check
```

## Development

```
go build ./...
go test ./... -count=1
go vet ./...
```

Requires Go 1.24 or newer. Pull requests welcome.

## License

[MIT](LICENSE) - chosen because it is a permissive, widely understood license suitable for a small reusable developer tool.
