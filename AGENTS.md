# AGENTS.md

Guidance for AI coding agents (and humans) working in this repository. Keep this
file up to date when project structure, commands, or conventions change.

## What this project is

`aico` is a single-command CLI that launches or resumes an isolated container
for an AI coding agent, mounts the current folder, and keeps your agent login
persisted across runs. It supports five agents: `pi`, `opencode`, `copilot-cli`,
`codex`, and `claude`.

The authoritative specification lives in [`specs/aico.md`](specs/aico.md); the
product rationale is in [`GOAL.md`](GOAL.md). Read the spec before making
non-trivial changes.

## Architecture (where things live)

```
main.go                  Entry point; calls cmd.Execute().
cmd/                     Cobra CLI. root.go wires commands; run.go orchestrates.
internal/agents/         Registry of the 5 supported agents + their auth sources.
internal/runtime/        Vendor-independent wrapper over the container CLI.
internal/container/      Deterministic container identity (aico-<agent>-<hash>).
internal/auth/           Builds login volumes + env-var forwarding + opt-in config mounts.
internal/platform/       OS-specific path resolution (Windows vs Unix).
images/                  Embedded Dockerfile (all agents) + on-demand build.
specs/aico.md            The specification. Source of truth.
```

## Hard architectural constraints

These are deliberate decisions. Do not violate them without updating the spec.

1. **Shell out to the runtime CLI — never use a container SDK.** All container
   operations go through `internal/runtime`, which execs `docker`/`podman`.
   This keeps aico vendor-independent. The CLI resolves its own per-OS socket;
   aico must not manage sockets or `DOCKER_HOST` itself.
2. **Container identity is pure.** `aico-<agent>-<sha256(abspath)[:8]>`. No
   lockfiles, labels, or state written into the user's project.
3. **Login persists in a per-agent volume; aico never seeds it from the host.**
   Each agent's login lives in a global named volume `aico-auth-<agent>`; the
   user logs in once inside the container and stays logged in. Nothing from the
   host is read by default. API keys are forwarded **by name only** (`-e KEY`,
   never `-e KEY=VALUE`) so secrets never appear in `argv`. Host config is shared
   read-only only with `--share-config`. copilot-cli login persistence is
   deferred to v2 (its token lives in the keyring, not a file).
4. **One shared image holds all agents.** The agent to launch is chosen at run
   time; the entrypoint is not baked per-agent.
5. **Cross-platform path logic lives only in `internal/platform`.** No other
   package should branch on `runtime.GOOS`.

## Commands

```sh
# Build
go build -o aico .

# Run all tests
go test ./...

# Static analysis (must pass before commit)
go vet ./...

# Format (must be clean before commit)
gofmt -l .          # lists files needing formatting; should print nothing
gofmt -w .          # apply

# Cross-compile sanity check (all release targets)
for t in linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64 windows/arm64; do
  GOOS=${t%/*} GOARCH=${t#*/} go build -o /dev/null . || echo "FAIL $t"
done

# Try the CLI without touching Docker
./aico run pi --dry-run
```

## Conventions

- **Go version:** 1.26+ (see `go.mod`).
- **Formatting:** `gofmt`; keep imports grouped (stdlib, then third-party, then
  local `github.com/yldgio/aico/...`).
- **Errors:** user-facing errors must state the cause and a `fix:` line with an
  actionable next step (and an example where useful). Match the existing style
  in `internal/runtime` and `cmd/run.go`.
- **Tests:** OS-specific logic is made testable by parameterising helpers on
  `goos` + an env lookup (see `internal/platform`). Prefer pure, table-style
  unit tests so Windows branches are verifiable on any host.
- **Commits:** Conventional Commits, atomic (one logical change per commit).
  Examples: `feat(auth): ...`, `fix(runtime): ...`, `docs: ...`,
  `ci: ...`, `chore: ...`.

## Verifying a change

Before opening a PR, all of the following must pass:

1. `gofmt -l .` prints nothing.
2. `go vet ./...` is clean.
3. `go test ./...` passes.
4. The cross-compile loop above succeeds for all six targets.
5. `./aico run <agent> --dry-run` still prints a sensible plan.

## Things not to do

- Don't add a dependency on the Docker/Podman Go SDK.
- Don't write files into the user's mounted project to track state.
- Don't put secret values into command arguments.
- Don't branch on the operating system outside `internal/platform`.
- Don't expand scope without updating `specs/aico.md` first. Out of scope for
  v1: an `aico setup` wizard, composing agent subsets into one image, and
  keeping containers running after the agent exits.
