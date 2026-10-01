# Git Conflict Lab

An offline trainer for resolving **real Git merge conflicts** in isolated repositories. Practice in your terminal, follow progress in a local browser interface, and verify the result.

![Git Conflict Lab interface](docs/interface.png)

[See the one-minute walkthrough](docs/DEMO.md) for the learning flow and what the checker verifies.

## Quick start

Requirements: Git on `PATH`; Go 1.24+ to build from source.

Download a binary from the [Releases page](https://github.com/aslanchikop/git-conflict-lab/releases), or build from source:

```sh
go build -o git-conflict-lab ./cmd/git-conflict-lab
./git-conflict-lab ui
```

On Windows PowerShell, build with `go build -o git-conflict-lab.exe ./cmd/git-conflict-lab` and run `./git-conflict-lab.exe ui`. Open the localhost URL printed by the command. The interface serves its assets locally and requires no account or network connection. Keep the command running while using the page; stop it with Ctrl+C.

The browser interface includes a short learning path, a concept and knowledge check for each scenario, exercise search and difficulty filters, a guided workspace with copyable terminal commands, a four-version conflict view, progressive hints, solution feedback, seven Git skills, and seven achievements. The knowledge checks give immediate explanations and do not block hands-on practice. Skills, progress, and achievements are reconstructed from completed Git repositories when the page reloads; they do not require an account. Motion effects respect the operating system's reduced-motion setting.

For terminal-only use:

```sh
./git-conflict-lab list
./git-conflict-lab start merge-basic
```

`start` prints the full path to the created repository. Change to that directory, then run these commands. For the final `git-conflict-lab check`, put the built executable on your `PATH` or invoke it by its full path.

```sh
git log --oneline --all --graph
git merge feature/login
# Open login.go, resolve the conflict, then:
git add login.go
git commit
git-conflict-lab check
```

Run `git-conflict-lab hint merge-basic` inside the exercise for gradual hints. `check` gives a next step if the exercise is incomplete. It verifies the original branch history, a genuine merge commit, a clean worktree, and the scenario-specific result. For `merge-basic`, it parses the Go source rather than running learner code. The accepted solution is a small Go function with the original empty-input guard and a final expression equivalent to requiring a username of at least 3 characters and a password of at least 12 characters.

Exercises are created under `~/git-conflict-lab-labs/` by default. Set `GIT_CONFLICT_LAB_LABS` to choose another location. Existing attempts are never replaced. Use `start merge-basic --attempt second` for another copy, or click **Start challenge** again in the browser interface. The interface lists saved attempts when reopened.

## Exercises

| Exercise | Conflict | Resolution goal |
| --- | --- | --- |
| `merge-basic` | Both branches edit the same line | Preserve the username and password requirements |
| `add-add` | Both branches create `notes.txt` | Keep both notes |
| `modify-delete` | Main edits a file that the feature branch deletes | Accept the deliberate deletion |
| `rebase-basic` | A feature commit conflicts while rebasing onto `main` | Preserve the intended setting in a linear history |
| `merge-multi` | Two files conflict during one merge | Resolve both files and keep both review methods |
| `cherry-pick` | A selected commit conflicts when replayed on `main` | Preserve both audit rules in a linear history |

## Commands

| Command | Purpose |
| --- | --- |
| `list` | Show available exercises |
| `start <id>` | Create an isolated practice repository |
| `check [id]` | Check progress from inside a generated repository |
| `hint <id>` | Reveal the next hint from inside that repository |
| `ui` | Serve the local browser interface |
| `version` | Show the version |
| `help` | Show usage |

Exit codes: `0` success, `1` runtime failure or incomplete exercise, `2` usage error.

## Current scope

This release focuses on merge conflicts and the Git commands needed to resolve them. It is not a general Git course. Windows and Linux are exercised in CI; macOS is included in the CI matrix but still needs a successful CI run before it can be described as verified.

## Development

```sh
go test ./... -count=1
go vet ./...
go build ./...
```

The generator lives in `internal/lab/generate.go`, the checker in `internal/lab/check.go`, the local UI in `internal/webui/`, and the embedded exercise catalog in `internal/exercise/catalog/`. Each lab has a synthetic **repository-local** commit identity so beginners can practice without configuring Git first; global Git configuration is untouched. The exercise manifest is advisory: the checker cross-checks it against actual Git history. Release archives and SHA-256 checksums are built by the tag workflow.

## License

[MIT](LICENSE).
