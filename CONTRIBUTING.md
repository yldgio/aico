# Contributing to aico

Thanks for your interest in improving `aico`! This document explains how to set
up, make changes, and submit them.

## Code of Conduct

This project follows the [Contributor Covenant](CODE_OF_CONDUCT.md). By
participating, you agree to uphold it.

## Prerequisites

- **Go 1.26+**
- **Docker** or **Podman** (only needed for end-to-end runs, not for unit tests)

## Getting started

```sh
git clone https://github.com/yldgio/aico
cd aico
go build -o aico .
go test ./...
./aico run pi --dry-run
```

## Project layout

See [`AGENTS.md`](AGENTS.md) for the full architecture map and the hard
constraints that contributions must respect. The specification in
[`specs/aico.md`](specs/aico.md) is the source of truth — read it before
proposing non-trivial changes.

## Before you open a pull request

All of the following must pass locally (CI enforces them too):

```sh
gofmt -l .          # must print nothing
go vet ./...        # must be clean
go test ./...       # must pass
# cross-compile all six release targets
for t in linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64 windows/arm64; do
  GOOS=${t%/*} GOARCH=${t#*/} go build -o /dev/null . || echo "FAIL $t"
done
```

## Commit style

We use **[Conventional Commits](https://www.conventionalcommits.org/)** with
**atomic** commits (one logical change per commit). The commit history feeds the
release changelog, so write clear subjects.

Common types: `feat`, `fix`, `docs`, `refactor`, `test`, `ci`, `chore`.
Use an optional scope matching a package, e.g. `feat(auth):`, `fix(runtime):`.

Examples:

```
feat(agents): add support for a new agent
fix(auth): forward env vars by name to keep secrets out of argv
docs: clarify resume behaviour in README
```

## Pull request guidelines

- Keep PRs focused; prefer several small PRs over one large one.
- Update `specs/aico.md` if you change behaviour, and `README.md` if you change
  user-facing usage.
- Add or update tests for the behaviour you change.
- Describe what and why in the PR body; link any related issue.

## Reporting bugs and requesting features

Use the GitHub issue templates. For security issues, **do not** open a public
issue — see [SECURITY.md](SECURITY.md).

## License

By contributing, you agree that your contributions will be licensed under the
[MIT License](LICENSE).
