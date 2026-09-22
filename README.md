# Git Conflict Lab

A free, open-source, offline CLI training utility for practicing real Git conflict resolution in safely generated, isolated repositories.

> Status: Milestone A (foundation). The CLI skeleton, exercise catalog and safety groundwork are in place; exercise generation, checking and hints arrive in upcoming milestones tracked in GitHub Issues.
>
> Platforms: Windows is verified locally and in CI; Linux is verified in CI; macOS support is planned but not yet verified.

## Prerequisites

- [Git](https://git-scm.com) available on your `PATH`
- [Go 1.24+](https://go.dev) only if you build from source

## Usage

What works **today**:

- `git-conflict-lab list` - show the exercises in the embedded catalog
- `git-conflict-lab version` - show the tool version
- `git-conflict-lab help` - show usage and the command list

What arrives **later** (the commands already exist and report their milestone):

| Command | Delivered in | What it will do |
|---------|--------------|-----------------|
| `start <exercise-id>` | Milestone B | Generate an isolated exercise repository |
| `check` | Milestone C | Verify your conflict resolution |
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
