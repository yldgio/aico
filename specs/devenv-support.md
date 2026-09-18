# Spec: Project-level devenv support

> Goal: When a project folder contains `devenv.nix`, `aico run` launches the agent inside that project's devenv environment — built and cached inside the container, with zero new host requirements.
> Date: 2026-09-16
> Status: Active

---

## What & Why

Many projects already declare their toolchain in [devenv](https://devenv.sh/) (`devenv.nix` + optional `devenv.yaml`). Today `aico` drops the agent into a generic container, so the agent lacks the project's compilers, linters, and tools and can produce code that doesn't build or test in the project's real environment. With this feature, aico detects the devenv config and runs the agent inside it — one command, same as today, but the agent works with the project's actual toolchain.

Host requirements stay exactly as they are: a Go binary plus a container runtime. Nix and devenv live **inside** the image; nothing Nix-related is needed on the host (which also keeps Windows-native support intact).

---

## Done Looks Like

- `aico run pi` in a folder containing `devenv.nix` starts the container, builds the project's devenv environment inside it (first run only), and lands the user in the agent **inside `devenv shell`** — the project's tools are on PATH for the agent.
- Running the same command again on the same path resumes instantly: the Nix store is cached in a shared volume, so no rebuild occurs when `devenv.nix` is unchanged.
- `aico run pi --no-devenv` in the same folder launches the plain agent environment, ignoring the devenv config.
- `aico run pi` in a folder **without** `devenv.nix` behaves exactly as today — no Nix tooling, no extra volume, no behavior change.
- The first environment build prints a clear notice and streams progress, so a multi-minute first build is never a silent hang.
- A developer can verify the whole feature from a dry-run: image, volume, and wrapped command are all visible without touching Docker.

---

## Scope

### In Scope

- Auto-detection of devenv projects via `devenv.nix` in the project root (`devenv.yaml` inputs are honored implicitly by devenv itself).
- `--no-devenv` flag — per-run opt-out when a `devenv.nix` is present.
- A lazily-built second image `aico-agents-devenv:latest` — a `--target devenv` stage of the existing embedded Dockerfile that adds Nix + the devenv CLI on top of the base image. Built automatically on first devenv-project run, same staleness/hash mechanism as the base image.
- A single global named volume `aico-nix` mounted at `/nix` in devenv-mode containers — content-addressed store shared across projects for cross-project cache reuse.
- Launching the agent via `devenv shell <agent> [args]` (verified upstream: `devenv shell` accepts a command, runs it in the environment, and exits).
- Devenv/non-devenv mode conflict handling on an existing container, following the existing `-d` precedent: a container's mode is fixed at creation; a conflicting run prompts to recreate (confirm-then-recreate, `--new` skips the prompt, error with hint when no TTY). Mode is recorded via a new `aico.devenv` container label.
- `--image` bypass: an explicit `--image` wins over devenv mode (user takes full control; a stderr notice states devenv was skipped).
- Docs: README section + flags table, `--help` text, AGENTS.md, and a cross-reference from `specs/aico.md`.

### Out of Scope

- **Services/process orchestration (`devenv up`, process-compose)** — *v1 delivers the shell environment only; full services support is deferred to a follow-up spec (roadmap). The agent or user can run `devenv up` manually inside the container.*
- **Host-side `devenv container build`** — *requires Nix+devenv on the host, breaks Windows-native, contradicts the "Go binary + runtime only" philosophy.*
- **Per-project Nix volumes** — *the store is content-addressed and immutable; sharing is safe by design and gives cross-project reuse. Trade-off accepted: no automatic GC (manual `docker volume rm aico-nix`).*
- **Nix garbage collection / volume size management** — *documented manual cleanup only.*
- **devenv profiles** (`--profile`) — *not exposed as an aico flag in v1; projects relying on non-default profiles are out of scope for now.*
- **Building deployable artifacts (`devenv build`, `devenv container`)** — *aico is a dev-environment launcher, not a packaging tool.*

---

## Constraints & Assumptions

### Hard Constraints

- All aico architectural constraints from `specs/aico.md` still apply (shell out to runtime CLI, pure container identity, no aico state written to the project, OS branching only in `internal/platform`).
- No new host dependencies: Nix/devenv exist only inside the image.
- Non-devenv projects must see **zero** behavior change and zero new mounts/volumes.
- devenv-mode detection must be pure Go (`os.Stat` on the resolved absolute path) — no shelling out on the host.
- API-key env forwarding and file auth mounts keep working identically in devenv mode (devenv shell inherits the container's environment).

### Assumptions

| Assumption | What happens if wrong |
|---|---|
| Nix can be installed single-user (root, `--no-daemon`) in the `node:22-bookworm-slim` image and devenv installs via `nix profile` | The image build fails visibly; fix by pinning a Nix release or switching the installer (Determinate Systems installer) |
| `devenv shell <cmd>` runs a non-interactive command in the environment and propagates its exit code (documented upstream) | Agent never starts; caught by the Task 3 smoke test with a trivial devenv.nix |
| Agent binaries (npm global installs) remain on PATH inside `devenv shell` | Devenv prepends but does not clear PATH; if a project's devenv.nix overrides PATH hostilely, the agent launch fails visibly — document as known limitation |
| Devenv's own state dir (`.devenv/`) written into the project folder is acceptable | This is devenv's own convention (devenv users already gitignore it); aico itself still writes nothing into the project. Explicit decision below. |
| First-run environment builds fit in reasonable time/disk (minutes, ~GBs) | If a project's env is huge, the streamed output makes the cost visible; `--no-devenv` is the escape hatch |

---

## Decisions Already Made

| Decision | Choice | Rationale |
|---|---|---|
| Trigger | Presence of `devenv.nix` in project root | devenv's own convention; unambiguous; `devenv.yaml` alone is not meaningful |
| Opt-out | `--no-devenv` flag | Matches aico's flag style; tri-state bool avoided |
| Realization | Nix+devenv baked into image; `devenv shell` at container start | Zero new host deps; keeps Windows-native support |
| Image strategy | Lazy second image `aico-agents-devenv` via Dockerfile `--target devenv` | Non-devenv users never pay the ~1GB Nix cost; base image unchanged; one Dockerfile, two targets |
| Nix store persistence | One global volume `aico-nix` mounted at `/nix` | Content-addressed + immutable = safe to share; cross-project cache reuse; matches `aico-auth-<agent>` global-volume precedent; docker pre-populates the volume from image `/nix` on first mount |
| Mode conflict | `aico.devenv` label + confirm-then-recreate | Follows the existing `-d` mode precedent; destruction stays opt-in |
| `--image` + devenv project | `--image` wins; devenv skipped with stderr notice | Explicit user override always wins |
| First build UX | stderr notice ("devenv.nix detected, building environment…") + streamed build output | Minutes-long silent hangs are unacceptable UX |
| `.devenv/` state in project | Allowed (devenv's own convention) | The "no state in project" rule governs aico's own bookkeeping; devenv state belongs to the project |
| Services (`devenv up`) | Out of scope for v1, on roadmap | Keep v1 small and verifiable |

---

## Task Breakdown

### Task 1: Detection + `--no-devenv` flag

- **Depends on**: none
- **Description**: Add pure-Go detection of `<abspath>/devenv.nix`. Add the `--no-devenv` flag to `runOpts`/`newRunCmd` with accurate help text. Wire the decision into `runAgent` as an explicit `devenvMode bool` (detected && !noDevenv && image == ""). Unit tests: table-style, covering present/absent file, flag override, `--image` override.
- **Done when**: `aico run pi --dry-run` in a folder with `devenv.nix` prints a devenv-mode plan line; in a folder without it prints today's plan unchanged; `--no-devenv` flips a devenv project back to the plain plan; `--image x --dry-run` in a devenv project prints the skip notice.
- **Evaluation**:
  - **Deterministic check**: `go test ./...` passes, including new table tests for detection logic; the three dry-run scenarios above produce the expected output → exit 0, expected plan lines present.
  - **LLM-as-judge**: --help clarity → 1–5 scale (5 = flag purpose, default auto-detect behavior, and interaction with `--image` all clear from help text alone) → Pass ≥ 4

### Task 2: Devenv image stage

- **Depends on**: none (parallel with Task 1)
- **Description**: Add a `devenv` build target to the embedded `images/Dockerfile`: install Nix (single-user, no-daemon, as root) and `devenv` CLI on top of the base stage. Extend the images package with a `DevenvTag` (`aico-agents-devenv:latest`) and a build path that runs `docker build --target devenv` with the same content-hash staleness label mechanism as the base image. Base image build must remain bit-identical for non-devenv use.
- **Done when**: `docker build --target devenv -t aico-agents-devenv:latest images/` succeeds; `docker run --rm aico-agents-devenv:latest devenv version` exits 0; `docker run --rm aico-agents-devenv:latest pi --version` (and the other 4 agents) still exit 0; the base image build is untouched.
- **Evaluation**:
  - **Deterministic check**: the four docker commands above → all exit 0; `go test ./...` passes for the images package (hash/staleness tests extended for the new tag).
  - **LLM-as-judge**: Dockerfile layer hygiene → 1–5 scale (5 = Nix/devenv layers isolated after the expensive npm layer, build-cache friendly, comments explain non-obvious install choices) → Pass ≥ 4

### Task 3: Run wiring — volume, command wrapping, mode conflicts

- **Depends on**: Tasks 1, 2
- **Description**: When `devenvMode` is true: select `images.DevenvTag` (lazy-build on demand), add `-v aico-nix:/nix` to common args, add the `aico.devenv=true` label, and wrap the agent command as `devenv shell <agent> [args]` on **all** launch paths (interactive `run`, `create`+`start`, and detached `exec` including the `agentExecCmd` bash wrapper). Implement the mode-conflict check on resume: existing container's `aico.devenv` label vs current mode mismatch → confirm-then-recreate (same UX as `-d` conflicts; `--new` skips; no-TTY errors with a `fix:` hint). Update `printDryRunDetach` to show image, nix volume, and wrapped command. On fresh devenv runs, print the "building environment" notice to stderr before the agent starts.
- **Done when**: Dry-run in a devenv project shows the devenv image, the `aico-nix:/nix` mount, and `devenv shell <agent>` as the command on every path (interactive, `-d`, resume-exec). A mode conflict (create plain, then run without `--no-devenv` after adding a devenv.nix, or vice-versa) triggers the confirm/error path. Smoke test: a temp project with `devenv.nix` containing `packages = [ pkgs.hello ];` — `aico run <agent>` lands inside the env where `hello` resolves, and a second run resumes without rebuild.
- **Evaluation**:
  - **Deterministic check**: `go test ./...` (argv-assertion tests via the runtime test-stub pattern for every launch path); `go vet ./...` clean; `gofmt -l .` empty; the six-target cross-compile loop passes; the dry-run outputs above match expectations; the docker smoke test with `pkgs.hello` → `command -v hello` succeeds inside, second run starts without a rebuild.
  - **LLM-as-judge**: error/notice message quality → 1–5 scale (5 = mode-conflict, skip-notice, and first-build messages each state cause + consequence + fix, matching existing `cmd/run.go` style) → Pass ≥ 4

### Task 4: Documentation

- **Depends on**: Task 3
- **Description**: README: new "devenv support" section (detection, `--no-devenv`, first-build behavior, `aico-nix` volume + manual GC, services out of scope with roadmap note, PATH-collision known limitation) and flags-table entry. `--help` Long text updated in `cmd/run.go`. AGENTS.md: architecture note (second image target, global nix volume, devenv label). `specs/aico.md`: cross-reference to this spec for the devenv extension.
- **Done when**: A first-time reader can understand from README alone: what triggers devenv mode, how to opt out, where the cache lives, and what is not supported (services). Help text matches actual behavior.
- **Evaluation**:
  - **Deterministic check**: every new flag/behavior named in this spec appears in both README and `--help` output → grep finds each.
  - **LLM-as-judge**: README usability → 1–5 scale (5 = a devenv user learns trigger, opt-out, caching, and limitations without reading code) → Pass ≥ 4

---

## Evaluation Criteria

> Each task carries its own criteria inline; this section aggregates them for the verifying agent.

### Deterministic Checks

| Check | Task | How to run | Pass condition |
|---|---|---|---|
| Unit tests | T1, T3 | `go test ./...` | Exit 0; detection table tests + argv-stub tests included |
| Dry-run: devenv plan | T1, T3 | `aico run pi --dry-run` in devenv project | Shows `aico-agents-devenv` image, `aico-nix:/nix`, `devenv shell pi` |
| Dry-run: plain unchanged | T1 | `aico run pi --dry-run` in non-devenv project | Identical to current output (no nix volume, no devenv image) |
| Dry-run: opt-out | T1 | `aico run pi --no-devenv --dry-run` in devenv project | Plain plan |
| Dry-run: --image wins | T1 | `aico run pi --image x --dry-run` in devenv project | Uses `x`; stderr skip notice |
| Devenv image builds | T2 | `docker build --target devenv -t aico-agents-devenv:latest images/` | Exit 0 |
| devenv CLI in image | T2 | `docker run --rm aico-agents-devenv:latest devenv version` | Exit 0 |
| All 5 agents still work | T2 | `docker run --rm aico-agents-devenv:latest <agent> --version` × 5 | All exit 0 |
| Smoke: env active | T3 | devenv project with `pkgs.hello`; run agent; `command -v hello` inside | Resolves |
| Smoke: warm resume | T3 | Second `aico run` on same project | No rebuild; attaches/resumes |
| Mode conflict guard | T3 | Create plain, add devenv.nix, run without `--new` | Confirm prompt (TTY) or error with `fix:` hint (no TTY) |
| Static analysis | T3 | `go vet ./...` + `gofmt -l .` | Clean / empty |
| Cross-compile | T3 | six-target build loop from AGENTS.md | All succeed |
| Docs coverage | T4 | grep README + `--help` for `no-devenv`, `devenv.nix`, `aico-nix` | All present |

### LLM-as-Judge Criteria

| Criterion | Task | Question | Evidence to examine | Scale | Pass boundary |
|---|---|---|---|---|---|
| Help text clarity | T1 | Does `--help` explain `--no-devenv`, the auto-detect default, and the `--image` interaction without ambiguity? | `aico run --help` output | 1–5: 5 = all three facts unambiguous | ≥ 4 |
| Dockerfile hygiene | T2 | Are the Nix/devenv layers cache-friendly and explained? | `images/Dockerfile` diff | 1–5: 5 = layers after npm layer, non-obvious choices commented | ≥ 4 |
| Error/notice quality | T3 | Do the mode-conflict, `--image`-skip, and first-build messages state cause + consequence + fix? | Trigger each path, read messages | 1–5: 5 = all three match existing `cmd/run.go` error style | ≥ 4 |
| README usability | T4 | Can a devenv user learn trigger, opt-out, caching, and limitations from README alone? | README devenv section | 1–5: 5 = zero ambiguity | ≥ 4 |
| Constraint compliance | T1–T3 | Does the change preserve the hard constraints (CLI shell-out, pure identity, no aico state in project, OS branching only in internal/platform)? | Full diff | 1–5: 5 = zero violations | ≥ 4 |

### Verification Protocol

- **Adversarial**: The verifying model MUST be different from the implementing model.
- **Process**: Verifier evaluates every criterion above, produces pass/fail with evidence for each, and returns a prioritised issue list for the implementer.

### Convergence

- **Quality floor**: All deterministic checks pass. All LLM-as-judge criteria score ≥ 4.
- **Diminishing returns**: Stop when the last iteration improved less than 10% on any failing criterion.
- **Max iterations**: 3
