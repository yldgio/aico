# Spec: `aico` — Instant Isolated Agent Launcher

> Goal: A single CLI command that creates or resumes an isolated container for an AI coding agent, with automatic auth forwarding from the host.
> Date: 2026-06-24
> Status: Complete (implemented & verified 2026-06-24)

---

## What & Why

Launching an AI coding agent in a clean, auth'd environment today requires too many manual steps. `aico` collapses that to one command: `aico run <agent> [path]`. It creates a container on first use, resumes it on subsequent calls, mounts the project folder, and automatically forwards host auth configs so the agent is ready immediately.

It is a public OSS CLI binary — any developer should be able to clone the repo, build, and use it without hidden personal setup.

---

## Done Looks Like

- `aico run pi` from any folder starts (or resumes) a container with pi running, the current folder mounted, and pi's auth forwarded from the host — in one command
- Running the same command again on the same path re-attaches the existing stopped container without data loss
- `aico run pi --new` replaces the old container with a fresh one
- The same binary works for: `pi`, `opencode`, `copilot-cli`, `codex`, `claude`
- Any developer can clone the repo, build, and use it — no hidden personal setup required
- Host auth configs are detected and mounted into the container automatically
- The user lands directly in the agent's UI — no intermediate shell
- Exiting the agent stops the container; the next `aico run` resumes it
- Works on Linux, macOS, Windows native, and WSL2

---

## Scope

### In Scope

- `aico run <agent> [path]` — the single subcommand for v1
- `--new` flag — destroy existing container and create a fresh one
- `--image <tag>` flag — use a custom image, bypassing the built-in Dockerfile
- `--runtime <binary>` flag + `AICO_RUNTIME` env var — select container runtime explicitly
- `--verbose` flag — enable warnings (e.g. missing auth paths)
- Auto-detect container runtime: `docker` → `podman` → error
- One shared Docker image (`aico-agents:latest`) with all 5 agents installed; auto-built if not present
- Container identity: `aico-<agent>-<sha256-of-abspath[:8]>` — deterministic, no lockfile
- Resume strategy: re-attach stopped container; fall back to named volume if container is gone
- Default on existing container: resume; `--new` to replace
- Auth forwarding — per-agent map (see Constraints)
- Working directory bind-mounted into container at the same absolute path
- Cross-platform binary: Linux (amd64, arm64), macOS (amd64, arm64), Windows (amd64, arm64)
- `goreleaser` build pipeline + GitHub Actions CI with tagged releases

### Out of Scope

- `aico setup` subcommand — *deferred to v2*
- Composable / stackable agent subsets per container — *future feature, needs separate design*
- `--detach` / keep-container-alive-on-exit — *future, useful for VSCode attach scenarios*
- Multi-agent containers — *one agent per container by design*
- Cloud / remote containers — *local Docker/Podman only*
- GUI, TUI, or web interface — *CLI only*
- Auth creation or login flows — *`aico` forwards existing auth, never creates it*
- Isolation / sandboxing as a hard security requirement — *side-effect of containers, not a design driver*

---

## Constraints & Assumptions

### Hard Constraints

- **Language**: Go — single compiled binary, no runtime dependency
- **Container interface**: shell out to the runtime CLI (never use Docker SDK) — ensures vendor independence
- **Runtime support**: any OCI-compatible CLI (`docker`, `podman`, etc.) selected via auto-detect or override
- **Windows support**: must handle named pipe socket (`//./pipe/docker_engine`), Windows config paths (`%APPDATA%`, `%USERPROFILE%`), and produce a `.exe` binary
- **No lockfiles**: container identity is fully derived from agent name + absolute path hash — `aico` writes nothing to the project folder
- **Auth is read-only**: file-based auth mounts are always `ro` — `aico` never writes to host auth dirs

### Assumptions

| Assumption | Risk if wrong |
|---|---|
| Auth config paths are at standard OS locations | Platform-specific path resolution needed; handled in Task 6 |
| opencode auth lives at `~/.config/opencode/` and is read-only-safe | May need write access; discovered during Task 5, handled by switching to copy-on-create for that agent |
| Node.js LTS is sufficient as the base for all 5 agents | A specific agent may need a different Node version; handled per-agent in the Dockerfile |
| All 5 agents accept a non-interactive install via `npm i -g` | If an agent needs interactive setup, the Dockerfile build will fail visibly |
| The container has outbound internet access to reach AI APIs | If not (air-gapped), auth forwarding still works but API calls will fail — `aico`'s problem surface doesn't change |

---

## Decisions Already Made

| Decision | Choice | Rationale |
|---|---|---|
| Language | Go | Single binary, fast to ship, great stdlib for exec/fs/crypto |
| Container interface | Shell out to CLI | Vendor-independent; `docker` and `podman` have identical CLI surface |
| Runtime auto-detect order | `docker` → `podman` → error | Docker is most common; podman is the OSS fallback |
| Image strategy | One image, all 5 agents, entrypoint varies | Simplest ops; composability is a future problem |
| Container identity scheme | `aico-<agent>-<hash-of-abspath[:8]>` | Deterministic, no lockfile, no labels needed |
| Resume default | Re-attach stopped container | `--new` must be explicit; destruction is opt-in |
| Exit behaviour | Container stops; resumable | Consistent with resume model; `--detach` is future |
| Landing UX | Straight into agent UI | Container process IS the agent — no intermediate shell |
| Auth (file-based) | Read-only bind mount, silently skip if missing | `aico` launches regardless; auth failure is the agent's problem |
| Auth (env-based) | Forward from host env automatically | codex → `OPENAI_API_KEY`; claude → `ANTHROPIC_API_KEY` |
| Missing auth verbosity | Silent by default; warn with `--verbose` | Don't surprise users with noise; power users can opt in |
| Platforms | Linux + macOS + Windows native + WSL2 | Full OSS reach; WSL2 is covered by Linux target |
| Distribution | goreleaser + GitHub Actions | Standard Go release tooling; zero manual steps |

### Auth Config Map

| Agent | File-based mount (read-only) | Env vars forwarded |
|---|---|---|
| pi | `~/.pi/agent/` → `/root/.pi/agent/` | — |
| copilot-cli | `~/.copilot/` → `/root/.copilot/`<br>`~/.config/gh/` → `/root/.config/gh/` | — |
| opencode | `~/.config/opencode/` → `/root/.config/opencode/` | Provider API keys if present |
| codex | — | `OPENAI_API_KEY` |
| claude | — | `ANTHROPIC_API_KEY` |

---

## Task Breakdown

### Task 1: Project Scaffold

- **Depends on**: nothing
- **Description**: `go mod init github.com/<org>/aico`, add `cobra` dependency, wire up `aico run <agent> [path]` command with all flags (`--new`, `--image`, `--runtime`, `--verbose`). Validate `<agent>` against the known list and exit non-zero with a helpful message for unknown agents. All logic stubs print "not implemented".
- **Done when**: `go build ./...` passes; `aico run --help` shows correct usage; `aico run fakeagent` exits non-zero with a message naming the valid agents.

### Task 2: Runtime Abstraction

- **Depends on**: Task 1
- **Description**: Implement a `runtime` package with a thin wrapper around `exec.Command`. Auto-detect `docker` then `podman` on `$PATH` (or Windows equivalent). Honour `AICO_RUNTIME` env var and `--runtime` flag as overrides. Expose methods: `Run(args ...string)`, `Inspect(name string)`, `Start(name string)`, `Stop(name string)`, `Remove(name string)`. No Docker-specific logic outside this package.
- **Done when**: With docker present, `aico run pi --dry-run` resolves to `docker`; `AICO_RUNTIME=podman aico run pi --dry-run` resolves to `podman`; with neither on PATH, `aico run pi` exits non-zero with a message naming the missing runtime.

### Task 3: Container Identity + Lifecycle

- **Depends on**: Task 2
- **Description**: Implement path hashing (SHA-256 of absolute path, first 8 hex chars) and container naming (`aico-<agent>-<hash>`). Implement the full lifecycle: check if container exists (running → attach; stopped → start then attach; missing → create then start then attach). `--new` destroys existing container and proceeds to create. Exit of the agent process stops the container.
- **Done when**: `aico run pi /tmp/testproject` twice produces exactly one container named `aico-pi-<hash>`; `--new` produces a new container ID; container name is identical across separate invocations with the same path.

### Task 4: Agent Dockerfile + Image Build

- **Depends on**: Task 2
- **Description**: Write a single `images/Dockerfile` — Node.js LTS base, installs all 5 agents globally via npm (`@earendil-works/pi-coding-agent`, `opencode-ai`, `@github/copilot-cli` or gh extension, `@openai/codex`, `@anthropic-ai/claude-code`). `aico` checks for `aico-agents:latest` at run time; if absent, builds it automatically from the embedded Dockerfile (`go:embed`). `--image <tag>` bypasses this entirely.
- **Done when**: `docker build -t aico-agents:latest images/` succeeds; `docker run --rm aico-agents:latest <agent> --version` exits 0 for all 5 agents; `aico run pi --image ubuntu:latest --dry-run` uses `ubuntu:latest` with no build attempted.

### Task 5: Workspace Mount + Auth Forwarding

- **Depends on**: Tasks 3, 4
- **Description**: Bind-mount the resolved project path into the container at the same absolute path, set as the working directory. Apply the per-agent auth map: add read-only bind mounts for each file-based config dir that exists on the host; forward env vars (`OPENAI_API_KEY`, `ANTHROPIC_API_KEY`) if set in the host environment. Silently skip missing auth paths; print a warning to stderr if `--verbose` is set.
- **Done when**: Project files are visible inside the container at the correct path; auth mounts are `ro` (write attempt fails); `OPENAI_API_KEY=xyz aico run codex /tmp/p` makes `OPENAI_API_KEY` visible inside; removing `~/.pi/agent/` does not crash `aico run pi`; `--verbose` prints a warning when auth is missing.

### Task 6: Cross-Platform Path Handling

- **Depends on**: Tasks 2, 5
- **Description**: Implement a `platform` package that resolves auth config dirs per OS: on Windows use `%APPDATA%` and `%USERPROFILE%` equivalents; on Linux/macOS use `$HOME`. Resolve Docker socket path per OS (`//./pipe/docker_engine` on Windows, `/var/run/docker.sock` on Unix). Ensure path normalisation works for WSL2 paths. All platform-specific logic lives in this package only.
- **Done when**: On Windows, `aico run pi` resolves auth paths to Windows-native locations, not `~`; docker socket path is correct per platform; binary produced by `GOOS=windows go build` runs without errors on Windows.

### Task 7: Distribution

- **Depends on**: Task 6
- **Description**: Add `.goreleaser.yml` targeting linux/amd64, linux/arm64, darwin/amd64, darwin/arm64, windows/amd64, windows/arm64. Add GitHub Actions workflow: run tests + build on every push; on `v*` tags, run goreleaser and attach binaries to a GitHub Release. Add `README.md` covering: install, first run, auth setup per agent, `--new` / `--image` / `--runtime` flags.
- **Done when**: `goreleaser build --snapshot --clean` produces 6 binaries; pushing a `v*` tag triggers the Actions workflow and attaches release assets; README allows a first-time user to go from clone to running agent without external docs.

---

## Evaluation Criteria

### Deterministic Checks

| Check | Task | How to run | Pass condition |
|---|---|---|---|
| Binary builds clean | T1 | `go build ./...` | Exit 0, no errors or warnings |
| Help flag works | T1 | `aico run --help` | Exit 0, output includes `<agent>` and `[path]` |
| Unknown agent rejected | T1 | `aico run fakeagent` | Exit non-zero, message names valid agents |
| Runtime auto-detect | T2 | `which docker && aico run pi --dry-run` | Resolves to `docker` |
| Runtime env override | T2 | `AICO_RUNTIME=podman aico run pi --dry-run` | Resolves to `podman` |
| No runtime = clear error | T2 | Unset PATH entries, `aico run pi` | Exit non-zero, message names missing runtime |
| Container name deterministic | T3 | `aico run pi /tmp/p` × 2, inspect name | Same `aico-pi-<hash>` both times |
| Resume = no new container | T3 | Run twice same path, `docker ps -a \| grep aico` | Exactly 1 container for that path+agent |
| `--new` replaces container | T3 | Run, then `--new`, compare container IDs | New ID, old container gone |
| All 5 agents in image | T4 | `docker run --rm aico-agents:latest <agent> --version` × 5 | All exit 0 |
| `--image` bypasses build | T4 | `aico run pi --image ubuntu:latest --dry-run` | Uses `ubuntu:latest`, no build step |
| Workspace mounted | T5 | Create file in project dir, `ls` inside container | File visible at correct path |
| Auth mount is read-only | T5 | Inside container: `touch ~/.pi/agent/test` | Permission denied |
| Env vars forwarded | T5 | `OPENAI_API_KEY=xyz aico run codex /tmp/p --dry-run` | `OPENAI_API_KEY=xyz` visible inside container |
| Missing auth = no crash | T5 | Remove `~/.pi/agent/`, `aico run pi /tmp/p` | Container starts, exit 0 |
| Missing auth + `--verbose` warns | T5 | Same + `--verbose` | Warning on stderr, container still starts |
| Windows paths resolve | T6 | Run on Windows, inspect mount source paths | Uses `%APPDATA%`/`%USERPROFILE%`, not `~` |
| Goreleaser snapshot | T7 | `goreleaser build --snapshot --clean` | 6 binaries produced, all named correctly |
| CI passes on tag | T7 | Push `v0.1.0` tag | Actions workflow green, release assets attached |

### LLM-as-Judge Criteria

| Criterion | Task | Question | Evidence to examine | Scale | Pass |
|---|---|---|---|---|---|
| Error message clarity | T1–T6 | Do error messages explain what went wrong and what the user should do next? | Trigger 5 failure scenarios (unknown agent, no runtime, missing docker socket, bad path, no auth); read each message | 1–5: 5 = cause + fix + example shown, 1 = generic error string | ≥ 4 |
| README usability | T7 | Can a developer who has never used `aico` install it, run their first agent, and understand auth setup from the README alone — without reading any other file? | Read README top to bottom, simulate a first-time user journey | 1–5: 5 = zero ambiguity, covers all agents' auth, 1 = missing critical steps | ≥ 4 |
| Runtime abstraction quality | T2 | Is the runtime abstraction genuinely decoupled — could you add `nerdctl` support by editing only the runtime package? | Read runtime package and grep for docker/podman-specific strings outside it | 1–5: 5 = single-package change, 1 = runtime-specific code scattered throughout | ≥ 4 |

### Verification Protocol

- **Adversarial**: The verifying model MUST be different from the implementing model.
- **Process**: Verifier evaluates every criterion above, produces pass/fail with evidence for each, and returns a prioritised list of issues for the implementer to address.

### Convergence

- **Quality floor**: All deterministic checks pass. All LLM-as-judge criteria score ≥ 4.
- **Diminishing returns**: Stop when the last iteration improved less than 10% on any failing criterion.
- **Max iterations**: 3
