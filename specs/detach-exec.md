# Spec: Detach Mode + Exec Command

> Goal: Let users keep containers running after the agent exits (`-d`) and open a shell in running containers (`aico exec`).
> Date: 2026-06-25
> Status: Complete (2026-06-25)

---

## What & Why

Today, containers stop when the agent exits — every `aico run` is a blocking
session. Users want to keep the environment alive (tooling state, running
processes, agent context) and re-enter later, or open a shell alongside the
agent to explore the filesystem, debug, or run manual commands.

## Done Looks Like

- `aico run pi -d` → interactive agent session; when the user exits, the
  container stays running.
- `aico run pi -d -- "fix the auth tests"` → agent runs the command, exits;
  container stays running.
- `aico run pi` again (container already running from `-d`) → new interactive
  agent session in the same container.
- `aico exec pi` → opens a bash shell in the running container.
- Without `-d`, behavior is unchanged: container stops when agent exits.

---

## Scope

### In Scope

- `-d` flag on `aico run`: creates the container with `sleep infinity` as the
  main process and runs the agent via `docker exec`. Container stays alive
  after the agent exits. In an interactive (TTY) session the agent runs inside
  a shell wrapper, so quitting the agent drops the user into a `bash` shell
  inside the container (agent -> shell -> agent); non-interactive `-d` runs the
  agent directly with no shell fallback. Relaunching the agent from that shell
  works for pi/opencode/codex/claude; copilot-cli needs its keyring re-
  established (tracked separately).
- Resume logic updated: if a `-d` container is already running, `aico run`
  re-execs the agent (no flag needed on subsequent runs).
- Mode-conflict handling: a container's mode is fixed at creation. Passing `-d`
  for a container that already exists in interactive mode prompts the user to
  destroy and recreate it (honoring "destruction is opt-in"). With no TTY to
  prompt on, it errors with a `--new` hint instead of silently ignoring `-d`.
- `aico exec <agent> [path]` subcommand: opens `bash` in a running container.
  Errors if the container is not running.
- Passing extra args to the agent: `aico run pi -- "fix tests"` forwards the
  trailing args to the agent command.
- Podman compatibility (same OCI primitives: `run -d`, `exec -it`, `sleep`).
- Dry-run output reflects the new flags.
- Documentation: README, AGENTS.md, CHANGELOG.

### Out of Scope

- **`aico stop` / `aico rm` commands** — *users can `docker stop/rm` directly;
  add convenience wrappers later if needed.*
- **Running a different agent inside a container** — *each container has
  agent-specific auth volumes; cross-agent exec would have wrong auth.*
- **Background output streaming / logs** — *the agent runs interactively via
  exec; no log capture needed.*
- **Auto-stop idle containers** — *the user decides when to stop.*

---

## Constraints & Assumptions

### Hard Constraints

- Shell out to runtime CLI only (no Docker SDK).
- Podman must be supported (same CLI flags).
- Non-`-d` behavior must not change (no breaking changes).
- All OS branching in `internal/platform` only.

### Assumptions

- `sleep infinity` is available in the `node:22-bookworm-slim` base image
  (coreutils provides it). *If wrong: fall back to `tail -f /dev/null`.*
- `docker exec -it` works on a container started with `docker run -d`. *Known
  true for Docker and Podman.*
- A container created with `-d` (CMD=`sleep infinity`) can be distinguished
  from a non-`-d` container (CMD=agent) by inspecting the container's command.
  *Needed so `aico run` without `-d` on a `-d` container uses exec, not attach.*

---

## Decisions Already Made

| Decision | Rationale |
|----------|-----------|
| `-d` means "keep alive after exit", not "start in background" | Matches user intent: persistent environment, not fire-and-forget |
| `sleep infinity` as main process for `-d` containers | Clean, simple; agent runs via `exec`; container never exits |
| `aico exec`, not `aico open` | Matches container ecosystem terminology; `open` is ambiguous and conflicts with macOS `open` |
| Agent args via `--` separator | Standard convention; avoids flag parsing conflicts |
| No cross-agent exec | Auth volumes are per-agent; running wrong agent = wrong auth |

---

## Task Breakdown

### Task 1: Support trailing agent args (`--`)

- **Depends on**: none
- **Description**: `aico run pi -- "fix tests"` forwards `"fix tests"` to the
  agent command. Update `cmd/run.go` to capture args after `--` and append them
  to `agent.Command` in `createArgs`. Works for both `-d` and non-`-d` modes.
- **Done when**: `aico run pi /tmp -- --help --dry-run` shows the agent command
  with `--help` appended; `go test` passes.

### Task 2: `-d` flag — detached container creation

- **Depends on**: Task 1
- **Description**: When `-d` is passed:
  1. Create the container with `docker run -d --name <name> <volumes...> <image> sleep infinity`
  2. Then immediately `docker exec -it <name> <agent-command> [args...]`
  
  The container's main process is `sleep infinity`; the agent runs as an exec.
  When the agent exits, the exec returns but the container stays alive.
  Without `-d`: current `docker run -it` behavior unchanged.
- **Done when**: `aico run pi -d --dry-run` shows the two-step command;
  actually running it keeps the container alive after agent exit.

### Task 3: Resume logic for `-d` containers

- **Depends on**: Task 2
- **Description**: Update the resume path in `runAgent`:
  - Container running → `docker exec -it <name> <agent-command> [args...]`
    (not `docker attach`, which would attach to `sleep infinity`).
  - Container stopped + was `-d` → `docker start <name>` (background, no
    `-ai`) then `docker exec -it`.
  - Container stopped + was not `-d` → `docker start -ai` (current behavior).
  
  Detect `-d` containers by inspecting the container's command (contains
  `sleep`). Add `runtime.ContainerCommand(name)` helper.
- **Done when**: exit agent in a `-d` container, re-run `aico run pi` → new
  agent session starts; container stayed alive between sessions.

### Task 4: `aico exec` subcommand

- **Depends on**: Task 2
- **Description**: New cobra command `aico exec <agent> [path]` that runs
  `docker exec -it <container-name> bash`. Errors with a clear message if the
  container is not running. Uses the same `container.Name(agent, absPath)` for
  identity.
- **Done when**: `aico exec pi --dry-run` shows `docker exec -it <name> bash`;
  running it on a live `-d` container opens a shell.

### Task 5: Documentation

- **Depends on**: Tasks 1–4
- **Description**: Update README (new `-d` flag, `aico exec` command, examples),
  AGENTS.md, CHANGELOG.
- **Done when**: README documents both features with examples; `--help` output
  is clear.

---

## Evaluation Criteria

### Deterministic Checks

| Check | Task | How to run | Pass condition |
|-------|------|------------|----------------|
| trailing args in dry-run | T1 | `aico run pi /tmp -- --help --dry-run` | agent command includes `--help` |
| -d dry-run shows two steps | T2 | `aico run pi /tmp -d --dry-run` | shows `run -d ... sleep infinity` + `exec -it ... <agent>` |
| -d container stays alive | T2 | start with `-d`, exit agent, `docker ps` | container still running |
| non-d unchanged | T2 | `aico run pi /tmp --dry-run` (no -d) | same output as before (single `run -it`) |
| resume -d uses exec | T3 | start `-d`, exit, `aico run pi /tmp --dry-run` | shows `exec -it`, not `attach` |
| exec dry-run | T4 | `aico exec pi /tmp --dry-run` | shows `docker exec -it <name> bash` |
| exec errors when not running | T4 | `aico exec pi /nonexistent` (no container) | clear error message |
| build/fmt/vet/test | all | `gofmt -l . && go vet ./... && go test ./...` | clean / green |

### LLM-as-Judge Criteria

| Criterion | Task | Question | Evidence to examine | Scale | Pass boundary |
|-----------|------|----------|---------------------|-------|---------------|
| No breaking change | T2,T3 | Does the non-`-d` path remain identical to before? | `cmd/run.go` diff, dry-run comparison | 1–5: 5 = zero behavioral change without `-d` | ≥ 5 |
| `-d` + exec clarity | T5 | Would a new user understand when to use `-d` vs not, and how `exec` relates? | README | 1–5: 5 = clear examples showing the workflow | ≥ 4 |

### Verification Protocol

- **Adversarial**: the verifying model MUST differ from the implementing model.
- **Process**: verifier runs every deterministic check, scores each judge
  criterion with evidence, and lists concrete issues for the implementer.

### Convergence

- **Quality floor**: all deterministic checks pass; every judge criterion ≥ pass boundary.
- **Diminishing returns**: stop when the last iteration improves no criterion by ≥ 10%.
- **Max iterations**: 3.
