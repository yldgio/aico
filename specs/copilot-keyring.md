# Spec: Copilot-CLI Login Persistence (v2 Auth)

> Goal: Persist copilot-cli's OAuth login across container sessions by running gnome-keyring headlessly inside the aico image, so users log in once and stay logged in — without ever storing a clear-text token.
> Date: 2026-06-25
> Status: Complete (2026-06-25)

---

## What & Why

In v1 (`specs/auth-volumes.md`) copilot-cli was intentionally left without login
persistence because its token lives in the system keyring (libsecret), not a file.
Storing a clear-text token in a volume was rejected as insecure. The other four
agents persist login via simple named volumes.

This spec closes that gap: install gnome-keyring + dbus in the shared image, run
the keyring daemon headlessly via an entrypoint script, and persist the keyring
data in a named volume — exactly the approach proven in
`/mnt/d/projects/copilot-devcontainer`.

## Done Looks Like

- `aico run copilot-cli` → user runs `/login` once → exits → `aico run copilot-cli`
  again on the same or a different project → still logged in.
- `gh auth login` also persists (gh is useful inside the container).
- `docker volume ls` shows three `aico-auth-copilot-cli*` volumes.
- No clear-text OAuth token appears in any volume — the token is encrypted by
  gnome-keyring in `~/.local/share/keyrings/`.
- Other agents (pi, opencode, codex, claude) are completely unaffected at runtime
  — no dbus/keyring startup cost for them.

---

## Scope

### In Scope

- Add `gnome-keyring` + `dbus-x11` packages to the shared Dockerfile.
- Pre-seed the empty-password login keyring at image **build time** (so it
  auto-unlocks headlessly with no GUI prompter).
- Create `/usr/local/bin/copilot-entrypoint.sh` that starts dbus-launch +
  gnome-keyring-daemon, then `exec`s `copilot`. Baked into the image.
- Change copilot-cli's `Command` in the agent registry from `["copilot"]` to
  `["copilot-entrypoint.sh"]`.
- Add 3 `AuthVolumes` to copilot-cli:
  - `aico-auth-copilot-cli` → `/root/.copilot`
  - `aico-auth-copilot-cli-gh` → `/root/.config/gh`
  - `aico-auth-copilot-cli-keyring` → `/root/.local/share/keyrings`
- Verify that gnome-keyring works **headlessly as root** (the one untested
  assumption from v1).
- Update README (copilot now persists login), AGENTS.md, CHANGELOG.

### Out of Scope

- **Setup wizard / plugin management / BYOK / offline mode** — *aico's job is
  "run the agent"; configuration is the agent's own UX.*
- **`clear-auth` helper** — *nice-to-have, not required for login persistence;
  users can `docker volume rm` the volumes.*
- **Concurrent same-agent session bleed** — *known limitation, same as opencode;
  `~/.copilot/session-store.db` may clash if two copilot containers run
  simultaneously against the same volumes. Acceptable for v1.*
- **Non-root user support** — *aico image runs as root; no ownership chown dance
  needed.*

---

## Constraints & Assumptions

### Hard Constraints

- Single shared image (one Dockerfile for all 5 agents).
- Entrypoint script only affects copilot — other agents must not pay
  dbus/keyring startup cost.
- Shell out to runtime CLI only (no Docker SDK).
- Token must be stored via the standard libsecret API in the keyring volume,
  not dumped to a raw file by copilot directly. With an empty-password keyring
  (required for headless operation) the token is readable inside the keyring
  file — this is the same tradeoff the reference devcontainer accepts.

### Assumptions

- **gnome-keyring-daemon works headlessly as root with an empty-password
  keyring** — proven as `vscode` user in the devcontainer; untested as root.
  *If wrong:* may need to create a non-root user in the container, or find an
  alternative secret store. Verify early (Task 1).
- **`dbus-launch` succeeds inside a container without `--privileged`** — true
  in the devcontainer and standard Docker.
- **copilot CLI stores its token via libsecret D-Bus interface** — confirmed by
  the devcontainer's keyring approach working.
- **~15–25 MB image size increase** from gnome-keyring + dbus-x11 is acceptable.

---

## Decisions Already Made

| Decision | Rationale |
|----------|-----------|
| Entrypoint script (`copilot-entrypoint.sh`), not `.bashrc` | Isolated to copilot; no side effects on other agents (Q2) |
| 3 separate volumes (copilot + gh + keyring) | Matches proven devcontainer; clear single responsibility per volume (Q3) |
| Include `gh-auth` volume | gh and git are important inside the container (Q4) |
| Shared image, accept size increase | One image principle; 15–25 MB negligible (Q5) |
| Pre-seed keyring at build time | Root user → no ownership issues; avoids runtime first-run complexity |
| No clear-text fallback | Rejected in v1 spec; if keyring fails, login simply doesn't persist |

---

## Task Breakdown

### Task 1: Verify keyring-as-root (spike)

- **Depends on**: none
- **Description**: Build a minimal test container with gnome-keyring + dbus as
  root, pre-seed the empty-password keyring, run dbus-launch + gnome-keyring-daemon,
  and use `secret-tool` to store/retrieve a value. Proves the mechanism works before
  touching aico code.
- **Done when**: `secret-tool store` + `secret-tool lookup` round-trips a value
  inside a `--rm` container running as root with no `--privileged`.

### Task 2: Dockerfile + keyring pre-seed

- **Depends on**: Task 1 (must prove keyring-as-root works)
- **Description**: Add `gnome-keyring dbus-x11 libsecret-tools` to the Dockerfile's
  `apt-get install` line. After install, pre-seed `/root/.local/share/keyrings/login.keyring`
  with the empty-password descriptor and write `login` to the `default` file — same
  content as the devcontainer's `setup.sh`.
- **Done when**: `docker build` succeeds; the keyring files exist in the built image.

### Task 3: copilot-entrypoint.sh

- **Depends on**: Task 1
- **Description**: Create `images/copilot-entrypoint.sh`, install it to
  `/usr/local/bin/copilot-entrypoint.sh` in the Dockerfile. The script:
  1. Starts `dbus-launch` (or reuses `DBUS_SESSION_BUS_ADDRESS` if already set).
  2. Starts `gnome-keyring-daemon --start --components=secrets` if the secrets
     service isn't already on D-Bus.
  3. `exec copilot "$@"` — so signals propagate correctly and the container stops
     when copilot exits.
  Script must be idempotent (safe to re-run if the container is resumed).
- **Done when**: Running the entrypoint in a fresh container prints no errors and
  launches copilot; `dbus-send` confirms `org.freedesktop.secrets` is on the bus.

### Task 4: Agent registry + auth volumes

- **Depends on**: Task 2, Task 3
- **Description**: In `internal/agents/agents.go`, update `copilot-cli`:
  - `Command`: `["copilot-entrypoint.sh"]`
  - `AuthVolumes`: 3 entries (copilot, gh, keyring) with appropriate suffixes and targets.
  Update tests in `agents_test.go` to reflect the new volumes.
- **Done when**: `go test ./internal/agents/...` passes; `aico run copilot-cli --dry-run`
  shows 3 volume mounts and the entrypoint command.

### Task 5: Integration test — login persists

- **Depends on**: Task 4
- **Description**: End-to-end test: build the image, run `copilot-entrypoint.sh`
  in a container with the 3 named volumes, use `secret-tool store` to simulate a
  token being written to the keyring, stop and remove the container, start a new
  container with the same volumes, use `secret-tool lookup` to confirm the value
  survived. (Full copilot `/login` requires interactive OAuth, so we test the
  persistence mechanism, not the OAuth flow itself.)
- **Done when**: Token value round-trips across two separate container lifecycles
  using the same named volumes.

### Task 6: Documentation

- **Depends on**: Tasks 4–5
- **Description**: Update README (copilot-cli row in the auth table: now shows
  3 volumes + "persisted via keyring"), remove the "not persisted yet (v2)" caveat,
  update AGENTS.md, add CHANGELOG entry.
- **Done when**: README no longer mentions copilot as a limitation; auth table
  shows the 3 volumes.

---

## Evaluation Criteria

### Deterministic Checks

| Check | Task | How to run | Pass condition |
|-------|------|------------|----------------|
| keyring-as-root | T1 | `docker run --rm` test container: `secret-tool store/lookup` | round-trip succeeds, no errors |
| image builds | T2 | `docker build -t aico-agents:latest images/` | exit 0; keyring files in image |
| entrypoint starts keyring | T3 | run entrypoint in container; `dbus-send ... ListNames` | `org.freedesktop.secrets` present |
| dry-run shows 3 volumes | T4 | `aico run copilot-cli --dry-run` | output contains `aico-auth-copilot-cli:/root/.copilot`, `-gh:/root/.config/gh`, `-keyring:/root/.local/share/keyrings` |
| entrypoint command | T4 | `aico run copilot-cli --dry-run` | command ends with `copilot-entrypoint.sh` |
| token persists across containers | T5 | store → rm container → new container → lookup | value matches |
| no raw token file in volume | T5 | `find` copilot volume for files named `*token*` or containing bare token outside keyring format | no raw token file; token only in `login.keyring` via libsecret |
| build/fmt/vet/test | all | `gofmt -l . && go vet ./... && go test ./...` | clean / green |
| README copilot updated | T6 | `grep "not persisted" README.md` | no match (caveat removed) |

### LLM-as-Judge Criteria

| Criterion | Task | Question | Evidence to examine | Scale | Pass boundary |
|-----------|------|----------|---------------------|-------|---------------|
| Entrypoint robustness | T3 | Is the entrypoint idempotent, does it handle resume (dbus already running), and does it propagate signals via exec? | `copilot-entrypoint.sh` source | 1–5: 5 = handles all three correctly | ≥ 4 |
| No side effects on other agents | T2,T3 | Do other agents (pi, codex, claude, opencode) remain unaffected — no dbus startup, no extra volumes? | `aico run pi --dry-run`, agent registry code | 1–5: 5 = zero copilot-specific artifacts in other agents' runs | ≥ 5 |
| Security: token via libsecret | T5 | Is the OAuth token stored only via the standard libsecret/keyring API (not dumped to a raw file by copilot)? | keyring volume contents, entrypoint code | 1–5: 5 = token only accessible via secret-tool/libsecret, stored in keyring file format | ≥ 4 |

### Verification Protocol

- **Adversarial**: the verifying model MUST differ from the implementing model.
- **Process**: verifier runs every deterministic check, scores each judge criterion
  with evidence, and lists concrete issues for the implementer.

### Convergence

- **Quality floor**: all deterministic checks pass; every judge criterion ≥ pass boundary.
- **Diminishing returns**: stop when the last iteration improves no criterion by ≥ 10%.
- **Max iterations**: 3.
