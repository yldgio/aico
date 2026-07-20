# Spec: Opt-In Shared Agent Root Volumes

> Goal: Make agent root state project-isolated by default, with global per-agent root sharing available only via an explicit flag.
> Date: 2026-07-20
> Status: Complete (implemented & self-verified 2026-07-20)

---

## What & Why

`aico` currently persists agent root state in global per-agent named volumes by default
(e.g. `aico-auth-pi`, `aico-auth-codex`, `aico-auth-copilot-cli`). That means
separate projects using the same agent share the same root config, login, and
other agent state unless the user does something special.

That violates the isolation principle. A fresh project container should not inherit
another project's agent root state by default. The default must be isolated, and
shared root state must be an explicit opt-in.

This change keeps persistence, but changes its scope:
- default persistence becomes **per-project**
- optional persistence becomes **global per-agent** via `--shared-root`

This solves cross-project leakage while preserving a deliberate escape hatch for
users who want one shared root per agent.

## Done Looks Like

The finished behavioral state is:

- Running `aico run pi` in project A and project B uses different root volumes,
  so config/login/session state from A does not appear in B by default.
- Running `aico run pi --shared-root` in project A and B uses the same global
  per-agent root volume, preserving intentionally shared behavior.
- Existing global per-agent root volumes are preserved and reused only when
  `--shared-root` is passed.
- `aico bake` follows the same root-volume rules as `aico run`.
- `--dry-run` explicitly shows whether the root mode is project-isolated or
  shared-root, and prints the exact root volume names.
- `aico rm` keeps root volumes by default; `aico rm --volumes` removes the
  relevant root volumes for the selected mode.
- `--import-config` remains a separate one-time copy feature and does not change
  which root volume model is used.
- README and `--help` make the default/opt-in behavior obvious.

---

## Scope

### In Scope

- Change default agent root persistence from global per-agent volumes to
  deterministic per-project volumes.
- Add `--shared-root` to opt into the existing global per-agent root-volume
  behavior.
- Apply the new root-volume model to all supported agents: `pi`, `opencode`,
  `copilot-cli`, `codex`, `claude`.
- Apply the new root-volume model to both `aico run` and `aico bake`.
- Preserve existing shared volumes and reuse them only in shared-root mode.
- Make `--dry-run` print the selected root mode and exact root volume names.
- Detect legacy containers created under the old shared-by-default behavior and
  require recreate before default isolated mode can apply.
- Keep `aico rm` semantics: keep volumes by default; remove them with
  `--volumes`.
- Update tests, README, cobra help text, and any architectural/spec docs needed
  to reflect the new contract.

### Out of Scope

- Host bind-mounting of agent root directories — *this feature is about named
  volumes, not host-path sharing*.
- Automatic migration or copying from old shared volumes into new per-project
  volumes — *too risky and unnecessary for the first version*.
- Changing `--import-config` into a sharing flag — *copy-once and shared-root are
  different behaviors and stay separate*.
- Changing container identity (`aico-<agent>-<hash>`) — *container identity stays
  pure and unchanged*.
- Writing any new state files into the user's project directory — *forbidden by
  the architecture*.

---

## Constraints & Assumptions

### Hard Constraints

- All container operations must still shell out through `internal/runtime`; no
  container SDK may be introduced.
- Container identity remains `aico-<agent>-<sha256(abspath)[:8]>`; this change is
  only about root-volume selection.
- The default run must not use a global shared root volume.
- Shared-root mode must be explicit, user-facing, and off by default.
- Shared-root mode keeps the current global per-agent volume names so existing
  data remains available.
- Default per-project root volumes must be named deterministically from agent +
  project path hash.
- API-key env vars must still be forwarded by name only (`-e KEY`, never
  `-e KEY=VALUE`).
- `--import-config` remains a one-time copy operation and must not silently turn
  into root sharing.
- Cross-platform path logic stays confined to `internal/platform`.
- User-facing docs and `--help` text must be updated alongside the behavior.

### Assumptions

- Each supported agent's effective root state can be represented by one or more
  explicit container targets:
  - `pi` → `/root/.pi/agent`
  - `opencode` → `/root/.config/opencode` and `/root/.local/share/opencode`
  - `copilot-cli` → `/root/.copilot`, `/root/.config/gh`, and `/root/.local/share/keyrings`
  - `codex` → `/root/.codex`
  - `claude` → `/root/.claude`
  *If any target is incomplete or wrong, the registry/tests must be corrected
  before shipping.*
- Reusing the current shared volume names for shared-root mode will preserve old
  user state without needing migration.
- Legacy containers created before this change can be identified either by their
  mounted volume names, by missing/new labels, or both.
- `docker commit` / `podman commit` continue to exclude mounted named volumes, so
  changing the root-volume model will not bake agent root state into images.

---

## Decisions Already Made

| Decision | Rationale |
|----------|-----------|
| Default root persistence is **per-project persistent named volumes** | Preserves convenience without leaking state across projects |
| Opt-in flag name is `--shared-root` | Names the behavior directly and keeps it separate from `--import-config` |
| Shared-root mode uses **one global volume set per agent**, created on first use and reused later | Keeps the old convenience model available explicitly |
| Shared-root mode keeps current volume names like `aico-auth-pi` | Preserves existing user data without migration |
| Default per-project volume names are deterministic from agent + project hash | Matches the repo's pure-identity design and makes cleanup/verifiability straightforward |
| Applies to **all five agents** | One mental model and one documentation model is simpler and safer |
| Applies to both `run` and `bake` | `bake` also creates agent containers and must honor the same isolation contract |
| `--dry-run` explicitly shows root mode and exact volume names | Users must be able to verify isolation vs sharing before running |
| Legacy shared-by-default containers must be recreated to get isolated default behavior | Resuming them silently would violate the new default contract |
| `aico rm` keeps volumes by default; `aico rm --volumes` removes them | Preserves existing CLI expectations |
| `--import-config` stays as one-time copy only | Copying host config and choosing root-sharing scope are different concerns |

---

## Task Breakdown

### Task 1: Redefine agent root-volume metadata

- **Depends on**: none
- **Description**: Update the agent metadata model so each agent declares the full
  root/state volume targets that participate in isolation/shared-root selection.
  Provide helpers for two naming modes:
  - shared: existing names like `aico-auth-<agent>[-<suffix>]`
  - project-isolated: deterministic names like `aico-auth-<agent>-<hash>[-<suffix>]`
  Ensure the agent registry covers all five agents consistently.
- **Done when**: there is a single source of truth for root targets and volume
  naming for shared vs project-isolated mode, and tests cover all agents.

### Task 2: Build a root-volume plan from agent + path + mode

- **Depends on**: Task 1
- **Description**: Replace the current auth-volume-only planning with a plan that
  accepts agent, absolute project path, and a shared-root boolean, then returns:
  - runtime args for the selected volume set
  - the selected root mode (`project-isolated` or `shared-root`)
  - the exact resolved volume names for dry-run/reporting
  Env-var forwarding remains by-name only. `--import-config` stays separate.
- **Done when**: unit tests prove that the same agent on two project paths gets
  different default root volumes, while `--shared-root` resolves to the same
  existing global per-agent names.

### Task 3: Wire `--shared-root` into `aico run`

- **Depends on**: Task 2
- **Description**: Add a new visible `--shared-root` flag to `cmd/run.go` and
  use it when creating the root-volume plan. Update `--dry-run` output to print
  the selected root mode and exact volume names before the runtime command.
  `--import-config` remains available and independent.
- **Done when**: `aico run <agent> --dry-run` clearly shows project-isolated
  mode by default, and `aico run <agent> --shared-root --dry-run` clearly shows
  shared-root mode with the reused global volume names.

### Task 4: Handle legacy containers and mode mismatches safely

- **Depends on**: Task 3
- **Description**: Detect containers created under the old shared-by-default
  behavior. When a user runs without `--shared-root`, do not silently resume a
  legacy shared-root container. Instead require recreate (`--new` or prompt,
  matching the command's interactive/non-interactive behavior). For new
  containers, record enough metadata (e.g. labels and/or inferable mounts) to
  distinguish project-isolated vs shared-root mode reliably.
- **Done when**: default runs cannot accidentally continue using legacy shared
  root state without an explicit recreate step, and automated tests cover both
  interactive and non-interactive outcomes.

### Task 5: Apply the same model to `aico bake`

- **Depends on**: Tasks 2, 4
- **Description**: Add `--shared-root` to `cmd/bake.go` and make bake-created
  containers use the same project-isolated vs shared-root selection logic and
  dry-run reporting as `aico run`.
- **Done when**: `aico bake <agent> ... --dry-run` reports the correct root mode
  and volume names, and bake-created containers do not bypass the new default
  isolation contract.

### Task 6: Update removal and cleanup behavior

- **Depends on**: Tasks 1, 3, 5
- **Description**: Keep current default removal behavior (volumes preserved), but
  make `aico rm --volumes` remove the correct root volumes for the selected
  container mode:
  - project-isolated container → remove that project's root volumes only
  - shared-root container → remove the global per-agent shared root volumes
  Purge/uninstall should continue to remove all `aico-auth-*` volumes.
  Documentation/help must warn that removing shared-root volumes affects every
  project using that agent in shared mode.
- **Done when**: rm/purge/uninstall behavior matches the new model and tests
  cover both isolated and shared-root cleanup paths.

### Task 7: Update docs and authoritative specs

- **Depends on**: Tasks 3–6
- **Description**: Update README, cobra help text, and architecture/spec docs to
  describe:
  - isolated-by-default root behavior
  - `--shared-root`
  - preserved existing shared volumes
  - legacy-container recreate requirement
  - `--import-config` staying separate
  - `rm --volumes` effects in both modes
  Update `AGENTS.md` if the repository guidance still describes global per-agent
  root volumes as the default.
- **Done when**: a first-time user can understand default isolation, explicit
  shared-root behavior, and cleanup consequences from `README.md` and `--help`
  alone.

---

## Evaluation Criteria

### Deterministic Checks

| Check | Task | How to run | Pass condition |
|-------|------|------------|----------------|
| Default run uses project-isolated volume | T2,T3 | `aico run pi /tmp/p1 --dry-run` | output states `root mode: project-isolated` and includes `aico-auth-pi-<hash>` rather than `aico-auth-pi` |
| Different projects get different default root volumes | T2 | `aico run pi /tmp/p1 --dry-run` and `aico run pi /tmp/p2 --dry-run` | reported default root volume names differ |
| Shared-root reuses existing global volume | T2,T3 | `aico run pi /tmp/p1 --shared-root --dry-run` and `/tmp/p2 --shared-root --dry-run` | both report `root mode: shared-root` and the same `aico-auth-pi` volume |
| All agents resolve root volumes correctly | T1,T2 | unit tests over registry + plan builder | each of `pi`, `opencode`, `copilot-cli`, `codex`, `claude` gets the expected target set in both modes |
| No host-path root mounts by default | T2,T3 | inspect dry-run output / unit-test plan args | root-state mounts use named volumes only; no host path is used for root sharing |
| `--import-config` stays independent | T3 | `aico run opencode /tmp/p --import-config --dry-run` and `... --shared-root --dry-run` | import flag does not alter reported root mode; shared-root only changes root volume selection |
| Legacy default-shared container requires recreate | T4 | create or simulate legacy container, then run without `--shared-root` | command prompts or errors with a clear `--new` fix instead of silently resuming shared-root state |
| Bake follows the same root model | T5 | `aico bake pi /tmp/p -t test:latest --dry-run` and `... --shared-root --dry-run` | bake reports the same root modes/volume names as run |
| `rm --volumes` removes isolated volumes only | T6 | create project-isolated container, run `aico rm <agent> <path> --volumes`, inspect runtime volumes | only that project's root volumes are removed |
| `rm --volumes` removes shared-root global volumes | T6 | create shared-root container, run `aico rm <agent> <path> --shared-root?` or remove by name with `--volumes`, inspect runtime volumes | the per-agent shared root volumes are removed |
| Purge still clears all root volumes | T6 | create isolated + shared-root containers, run `aico purge`, inspect `volume ls` | all `aico-auth-*` volumes are removed |
| Docs/help updated | T7 | `aico run --help`, `aico bake --help`, `aico rm --help`, README review | all mention isolated default, `--shared-root`, and cleanup implications |
| Repo checks stay green | all | `gofmt -l . && go vet ./... && go test ./...` | formatting clean; vet/tests pass |

### LLM-as-Judge Criteria

| Criterion | Task | Question | Evidence to examine | Scale | Pass boundary |
|-----------|------|----------|---------------------|-------|---------------|
| Isolation contract clarity | T3,T7 | Does the CLI make it obvious that shared root is opt-in and default behavior is isolated per project? | `--help`, dry-run output, README examples | 1–5 where 5 = impossible to mistake the default for shared behavior | ≥ 4 |
| Legacy safety | T4 | Does the design avoid silently preserving old shared-root behavior when the user expects new isolated defaults? | legacy-container handling code/tests and resulting messages | 1–5 where 5 = safe by default with explicit recovery path | ≥ 4 |
| Cleanup clarity | T6,T7 | Would a user understand what `rm --volumes` does for project-isolated vs shared-root containers, including cross-project consequences? | `rm --help`, README cleanup section, any warnings/errors | 1–5 where 5 = consequences are explicit and hard to miss | ≥ 4 |

### Verification Protocol

- **Adversarial**: The verifying model MUST be different from the implementing model.
- **Process**: Verifier evaluates every deterministic and LLM-as-judge criterion,
  produces pass/fail with evidence for each, and identifies issues the
  implementer should address before acceptance.

### Convergence

- **Quality floor**: All deterministic checks pass. All LLM-as-judge criteria
  meet their pass boundary.
- **Diminishing returns**: Stop when the last iteration improved less than 10%
  on any failing criterion.
- **Max iterations**: 3
