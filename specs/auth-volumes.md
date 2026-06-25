# Spec: Auth via Per-Agent Login Volumes

> Goal: aico shares only what's needed to stay logged in across sessions — nothing else.
> Date: 2026-06-25
> Status: Complete (2026-06-25)

---

## What & Why

aico's current auth strategy bind-mounts whole host **config** folders read-only
(`~/.pi/agent`, `~/.config/opencode`, `~/.copilot`, `~/.config/gh`). This is wrong
on two counts, both confirmed by inspecting this host:

- It **clutters** the container with host settings/state the user explicitly does
  not want (e.g. `~/.copilot` is full of session DBs, logs, instructions).
- For some agents it **doesn't even mount the login**: opencode's real credential
  lives at `~/.local/share/opencode/auth.json`, which the old `~/.config/opencode`
  mount never touched.

New strategy: **persist login across sessions with a per-agent named volume**, the
same pattern proven in `/mnt/d/projects/copilot-devcontainer`. The user logs in once
*inside* the container; the volume keeps the login for every future `aico run` of
that agent. Nothing from the host is read by default, so no settings ever leak in.
Host config sharing becomes an explicit opt-in (`--share-config`).

Key insight that makes this clean: **login happens inside the Linux container**, so
each agent uses its *Linux* file-based credential store — exactly what a named volume
can persist. (Copilot is the one exception: even on Linux it stores its token in the
libsecret/gnome-keyring, not a file — so it is deferred to v2.)

## Done Looks Like

- `aico run pi` starts with no host folders mounted; the user runs the agent's
  `/login` once; on exit and re-run (same or different project) they are still logged in.
- `docker volume ls` shows one global `aico-auth-pi` volume reused by every pi container.
- No host settings/config appear in the container unless `--share-config` is passed.
- `aico run codex` still forwards `OPENAI_API_KEY` if the user prefers a key.
- `aico run <agent> --share-config` additionally exposes the host config dir, read-only.

---

## Scope

### In Scope

- Replace default read-only host-config bind mounts with **per-agent global named
  volumes** mounted at each agent's Linux login location.
- Agents covered in v1: **pi, opencode, codex, claude**.
- `--share-config` flag: opt-in, **read-only** bind mount of the agent's host config dir.
- Keep **API-key env forwarding** (`-e KEY`, name only) alongside the volumes.
- Update docs (README auth section, AGENTS.md, CHANGELOG).

### Out of Scope

- **copilot-cli login persistence** — *deferred to v2.* Requires gnome-keyring + dbus
  in the image, a headless daemon, an empty-password keyring pre-seed, and three
  volumes; "keyring headless as root" is unverified. Without the keyring the token
  would be stored as clear text, which we refuse to ship. copilot remains a selectable
  agent in v1 but does **not** persist login (re-login per container).
- **Seeding volumes from host credentials** — *rejected.* We never read or copy host
  secrets; login happens inside the container.
- **Per-project auth isolation** — *rejected.* Login is user-level; volumes are global
  per agent.
- Migrating away from any existing volumes (none exist yet; greenfield).

---

## Constraints & Assumptions

### Hard Constraints

- Shell out to the runtime CLI only (no Docker SDK); volumes via `-v name:target`.
- API keys forwarded by name only (`-e KEY`), never `-e KEY=VALUE`.
- aico never writes host auth; host config, when shared, is mounted read-only.
- Container runs as **root**; volume targets are under `/root`.
- All OS branching stays in `internal/platform`.

### Assumptions (verify during implementation)

- **Linux login file locations** (login happens inside the Linux container):
  - pi → `~/.pi/agent/auth.json` (dir also holds `settings.json`, `trust.json`)
  - opencode → `~/.local/share/opencode/auth.json` (dir may also hold session storage)
  - codex → `~/.codex/auth.json`
  - claude → `~/.claude/.credentials.json`
  *If any path is wrong, the volume target is wrong — verify by logging in and inspecting.*
- For pi/codex/claude the login directory has low, user-level churn, so mounting the
  whole dir as the global volume does not cause cross-project session bleed.
- For opencode, session storage may share the data dir; *if* concurrent same-agent runs
  bleed sessions, narrow the mount later. Acceptable for v1.
- Docker auto-creates a named volume on first `-v name:target`, so no explicit
  `volume create` step is required.

---

## Decisions Already Made

| Decision | Rationale |
|----------|-----------|
| Per-agent **global** named volume `aico-auth-<agent>` | Login is user-level; log in once, every project reuses it (Q3=1) |
| **Login bits only** in the volume; per-project session/state stays ephemeral | Avoid cross-project bleed and concurrent-write clashes (Q3=1a) |
| **Log-in-inside only**, no host seeding | Zero host secrets touched; uniform across agents; matches proven devcontainer (Q2=1) |
| **v1 = pi/opencode/codex/claude; copilot = v2** | Copilot needs keyring machinery; don't block four easy wins (Q4=1) |
| `--share-config`, **read-only**, default off | User opt-in to host settings; never mutate host (Q5) |
| Keep **env-var forwarding** by name | Key-based users still supported alongside interactive login (Q6) |
| Volume names have **no path hash** | Global per agent, not per project |

---

## Task Breakdown

### Task 1: Redefine the agent auth model (data)

- **Depends on**: none
- **Description**: In `internal/agents`, replace the folder-based `FileAuth` with:
  `AuthVolumes` (each = `{VolumeSuffix, Target}` → `-v aico-auth-<agent>-<suffix?>:<target>`,
  default one volume `aico-auth-<agent>`), keep `EnvVars`, and add `ConfigMounts`
  (each = `{Base, Rel, Target}`, used only with `--share-config`, mounted `:ro`).
  Populate the registry for pi/opencode/codex/claude with the Linux login targets above.
  copilot-cli: keep selectable, no `AuthVolumes` (no v1 persistence).
- **Done when**: `go build` passes; registry reflects the four agents' volume targets,
  env vars, and (for `--share-config`) host config dirs.

### Task 2: Build the mount plan in `internal/auth`

- **Depends on**: Task 1
- **Description**: `auth.Build(agent, shareConfig bool)` returns args:
  one `-v aico-auth-<agent>:<target>` per `AuthVolume`; `-e KEY` per env var **set in
  the environment**; and, when `shareConfig`, one `-v <hostConfigDir>:<target>:ro` per
  `ConfigMount` whose host dir exists (skip missing; warn only with `--verbose`).
- **Done when**: unit tests assert the exact arg list for each agent, with and without
  `--share-config`, and assert no `-e KEY=VALUE` form ever appears.

### Task 3: Wire `--share-config` into the run command

- **Depends on**: Task 2
- **Description**: Add `--share-config` (bool, default false) to `cmd/run.go`; pass it
  to `auth.Build`; include resulting args in the create command and in `--dry-run`.
- **Done when**: `aico run pi --share-config --dry-run` shows the extra read-only config
  mount; without the flag it does not.

### Task 4: Remove old host-folder default mounts + adjust warnings

- **Depends on**: Task 1
- **Description**: Delete the old default read-only `FileAuth` host bind mounts and the
  "missing host auth" warning path (we no longer read host auth by default). Keep a
  `--verbose` warning only for a missing host **config** dir under `--share-config`.
- **Done when**: a plain `aico run <agent> --dry-run` contains no host-path bind mount,
  only the `aico-auth-<agent>` volume (+ any env vars).

### Task 5: Update documentation

- **Depends on**: Tasks 1–4
- **Description**: Rewrite the README auth section (login-once-inside, volumes persist,
  `--share-config`, copilot=v2), update AGENTS.md auth model, add CHANGELOG entry.
- **Done when**: README documents the new model and the flag (incl. read-only + opt-in);
  AGENTS.md and CHANGELOG updated.

---

## Evaluation Criteria

### Deterministic Checks

| Check | Task | How to run | Pass condition |
|-------|------|------------|----------------|
| pi uses a volume, not host | T1,T4 | `aico run pi --dry-run` | contains `-v aico-auth-pi:/root/.pi/agent`; no host `.pi` bind mount |
| codex volume + key | T1,T2 | `aico run codex --dry-run` | contains `-v aico-auth-codex:/root/.codex` and `-e OPENAI_API_KEY` |
| claude volume + key | T1,T2 | `aico run claude --dry-run` | contains `-v aico-auth-claude:/root/.claude` and `-e ANTHROPIC_API_KEY` |
| opencode volume target | T1 | `aico run opencode --dry-run` | contains `-v aico-auth-opencode:/root/.local/share/opencode` |
| share-config adds RO mount | T3 | `aico run pi --share-config --dry-run` | shows a `...:ro` host-config mount absent without the flag |
| no secret in argv | T2 | `aico run codex --dry-run \| grep -c '='` within `-e` | no `-e KEY=VALUE` form present |
| global volume reused | T1 | `aico run pi <dirA> --dry-run` & `<dirB> --dry-run` | both reference the same `aico-auth-pi` |
| build/fmt/vet/test | all | `gofmt -l . && go vet ./... && go test ./...` | clean / green |
| login persists (integration) | T1–T4 | log into pi inside container, exit, `aico run pi` again | still logged in; `docker volume ls` shows `aico-auth-pi` |

### LLM-as-Judge Criteria

| Criterion | Task | Question | Evidence to examine | Scale | Pass boundary |
|-----------|------|----------|---------------------|-------|---------------|
| No host leakage by default | T4 | Does a default run avoid mounting any host config/settings? | dry-run output + `auth.Build` code | 1–5: 5 = only the auth volume + env vars, nothing host-derived | ≥ 4 |
| `--share-config` clarity | T5 | Is the flag's purpose, opt-in nature, and read-only behavior clear? | README + `--help` | 1–5: 5 = states what is shared, that it's off by default, and read-only | ≥ 4 |
| Login-persistence explanation | T5 | Would a new user understand they log in once inside and it persists? | README auth section | 1–5: 5 = explains log-in-inside, the volume, and copilot=v2 | ≥ 4 |

### Verification Protocol

- **Adversarial**: the verifying model MUST differ from the implementing model.
- **Process**: verifier runs every deterministic check, scores each judge criterion with
  evidence, and lists concrete issues for the implementer.

### Convergence

- **Quality floor**: all deterministic checks pass; every judge criterion ≥ 4.
- **Diminishing returns**: stop when the last iteration improves no criterion by ≥ 10%.
- **Max iterations**: 3.
