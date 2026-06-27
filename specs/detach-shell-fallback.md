# Spec: Detached run drops into a container shell on agent exit

> Goal: Under `aico run -d`, quitting the agent lands you in a `bash` shell **inside the container** (not back on the host), so you get one continuous agent ⇄ shell ⇄ agent session.
> Date: 2026-06-27
> Status: Complete (2026-06-27)

---

## What & Why

Today `aico run -d` runs the agent via `docker exec -it <c> <agent>` on top of a
`sleep infinity` container. When the agent quits, the exec session ends and the
user is thrown back to the **host** shell; to do anything in the container they
must run `aico exec` separately.

Users want the same flow they get when they manually `docker exec` into a
running container and start an agent: quit the agent and you are still **inside
the container**, free to install tools, inspect files, run builds, then relaunch
the agent — all without leaving and re-entering. This makes `-d` a real
"working session in a box" instead of a one-shot agent launch.

## Done Looks Like

1. `aico run pi -d` → the agent starts, attached to the terminal.
2. The user works in the agent.
3. The user quits the agent (its `/exit`, `Ctrl-C`, `Ctrl-D` — however that agent quits).
4. The user lands at a `bash` prompt **inside the container**, in the mounted workdir.
5. The user runs container commands (`apt-get install ...`, inspect files, builds).
6. The user relaunches the agent by typing its binary (e.g. `pi`) and continues; quitting it again returns to the same shell.
7. Exiting the **shell** itself (`exit` / `Ctrl-D`) returns to the host, and the container **stays running** (resumable with `aico run` / `aico exec`).

Verified scope of "relaunch" (step 6) by agent:

- **pi, opencode** (file auth in mounted volume) — relaunch works.
- **codex, claude** (env auth via `-e KEY`) — relaunch works (`-e` vars reach fresh `exec` sessions and the fallback shell; verified).
- **copilot-cli** (gnome-keyring/dbus) — first launch → drop-to-shell works; **relaunching by typing `copilot` does not re-establish the keyring** (its dbus address lived in the exited agent process). This is handled by a *separate* spec (see Out of Scope).

---

## Scope

### In Scope

- Change the `-d` **interactive (TTY)** launch so the agent runs inside a shell that falls back to an interactive `bash` when the agent exits. Applies to both code paths: the fresh-create exec and the resume exec of an already-running detached container.
- A one-line hint printed when landing in the fallback shell (states: you are in the container shell; how to leave; that the container keeps running).
- Update `--dry-run` output to show the new exec form.
- Update docs: README (`-d` flag row, detach section, examples), `aico run` `--help`, `specs/aico.md` decisions table, `specs/detach-exec.md`.

### Out of Scope

- **copilot-cli keyring on relaunch** — *separate spec*: making the dbus session-bus address shared across all of a container's exec sessions (also fixes today's `aico exec` shells, independent of this feature). Not solved here.
- **Non-`-d` runs** — *unchanged*: there the agent is the container's PID 1; quitting it stops the container, so a shell fallback does not fit the model.
- **Non-interactive `-d` runs** (no TTY, e.g. `aico run pi -d -- -p "..." > out`) — *unchanged*: the agent runs and exits with no shell fallback, preserving scripting/piping and the existing non-interactive execution feature.
- **Detaching from a *still-running* agent** (tmux/dtach-style background) — *not this goal*; the agent runs inside the shell, so no signal/detach-key handling is added.
- **Auto-relaunch loop** — *out*: landing in the shell is enough; we do not re-run the agent automatically after each quit.

---

## Constraints & Assumptions

### Hard Constraints

- No image change (Dockerfile / embedded scripts) — the mechanism is a `docker exec` argv only. (Per the chosen approach #1.)
- Shell out to the runtime CLI only; go through `internal/runtime` (no SDK).
- The fallback shell is `bash` (matches `aico exec`; present in `node:22-bookworm-slim`, verified).
- Preserve non-interactive behavior exactly (TTY-gated).

### Assumptions

- Every agent binary is on `PATH` in the container so the user can relaunch by name — *verified for `pi`; the others install via `npm i -g` to the same prefix. If wrong, the relaunch hint should name the full path.*
- `bash -c '"$@"; ... exec bash' aico <agent> <args>` passes args as argv with no quoting issues — *verified empirically (args with spaces, non-zero exit codes, fall-through all correct).*

---

## Decisions Already Made

| Decision | Rationale |
|----------|-----------|
| Mechanism: inline `bash -c '"$@"; <hint>; exec bash' aico <agent> <args>` | Smallest change that delivers the goal; no image rebuild; verified for args/exit-codes/relaunch |
| TTY-gated: wrapper only when stdin is a terminal; non-TTY keeps `exec <agent>` directly | Preserves non-interactive execution / piping |
| Apply to both fresh-create and resume exec paths | Consistent experience whether the container is new or resumed |
| Fallback shell is `bash` | Consistent with `aico exec`; available in the image |
| copilot-cli relaunch keyring deferred to its own spec | Orthogonal, pre-existing problem; keeps this spec tight |
| Build the exec argv in a pure, unit-tested helper | Lets us verify composition without Docker |

---

## Task Breakdown

### Task 1: Pure command-builder helper

- **Depends on**: none
- **Description**: Add a pure function in `cmd/run.go` (e.g. `agentExecCmd(agentCmd []string, tty bool) []string`) that returns the bash-wrapped slice (`bash -c '"$@"; <hint>; exec bash' aico <agentCmd...>`) when `tty` is true, and `agentCmd` unchanged when false. The hint is a single `echo`/`printf` line.
- **Done when**: function exists and is referenced by the run paths; `go build` passes.

### Task 2: Wire the helper into both `-d` exec paths

- **Depends on**: Task 1
- **Description**: Replace the two `rt.Exec(name, isTTY(), agentCmd...)` calls in the `-d` fresh-create path and the resume path with `rt.Exec(name, isTTY(), agentExecCmd(agentCmd, isTTY())...)`. Non-`-d` paths untouched.
- **Done when**: with a TTY, quitting the agent drops into a container `bash`; without a TTY, behavior is byte-for-byte the previous behavior.

### Task 3: Dry-run output

- **Depends on**: Task 2
- **Description**: Update `printDryRunDetach` so the `exec:` line reflects the wrapped command for the interactive case (and the plain agent command for non-TTY).
- **Done when**: `aico run pi -d --dry-run` prints the `bash -c '"$@"; ... exec bash' aico pi` exec line.

### Task 4: Docs + `--help`

- **Depends on**: Task 2
- **Description**: Update README (`-d` row, "Detach mode" section, examples), the `aico run` cobra `Long`, `specs/aico.md` (Landing UX / Exit behaviour rows), and `specs/detach-exec.md` to describe the agent→shell fallback. Note the copilot-cli relaunch caveat and that it is handled separately.
- **Done when**: `gofmt -l .` clean and docs reflect the behavior; no stale "returns to host on agent exit" language for `-d`.

---

## Evaluation Criteria

### Deterministic Checks

| Check | Task | How to run | Pass condition |
|-------|------|------------|----------------|
| Unit: TTY builds wrapper | 1 | `go test ./cmd/` on `agentExecCmd(["pi"], true)` | slice equals `["bash","-c","\"$@\"; …; exec bash","aico","pi"]` (hint substring allowed) |
| Unit: non-TTY passthrough | 1 | `go test ./cmd/` on `agentExecCmd(["pi","-p","x"], false)` | returns `["pi","-p","x"]` unchanged |
| Build/vet/test | 1–4 | `go build ./...`, `go vet ./...`, `go test ./...` | all pass |
| gofmt | 4 | `gofmt -l .` | prints nothing |
| Cross-compile | 2 | the six-target loop in AGENTS.md | all six succeed |
| Dry-run shows wrapper | 3 | `aico run pi -d --dry-run` | `exec:` line contains `bash -c` … `exec bash` … `aico pi` |
| Smoke: fall-through (sim TTY off) | 2 | `docker run -d --name t <img> sleep infinity` then `docker exec -i t bash -c '"$@"; echo MARK' aico echo hi` | prints `hi` then `MARK` |
| Smoke: relaunch env for codex/claude | 2 | create container with `-e OPENAI_API_KEY`; `docker exec -i t bash -c '"$@"; echo K=$OPENAI_API_KEY' aico true` | fallback shell shows the key |
| Non-interactive unchanged | 2 | `echo x \| aico run pi -d -- -p -` style path (no TTY) | runs agent directly, no shell fallback, same exit semantics as before |

### LLM-as-Judge Criteria

| Criterion | Task | Question | Evidence to examine | Scale | Pass boundary |
|-----------|------|----------|---------------------|-------|---------------|
| Journey fidelity | 2 | Does quitting the agent under `-d` land the user in a container shell, and does exiting that shell return to host with the container still running? | Manual run-through of "Done Looks Like" steps 1–7 (or a transcript) | 1–5 (5 = all 7 steps behave exactly as written) | ≥ 4 |
| Surgical change | 1–4 | Does every changed line trace to this feature, with non-`-d` and non-TTY paths untouched? | `git diff` | 1–5 (5 = no unrelated edits, non-interactive path byte-identical) | ≥ 4 |
| Docs honesty | 4 | Do the docs accurately state the new behavior and the copilot-cli relaunch caveat without overclaiming? | README, `--help`, `specs/*` | 1–5 (5 = accurate, includes caveat, no stale claims) | ≥ 4 |

### Verification Protocol

- **Adversarial**: The verifying model MUST differ from the implementing model.
- **Process**: The verifier runs every deterministic check, scores each LLM-as-judge criterion with evidence (pass/fail), and lists concrete issues for the implementer to fix.

### Convergence

- **Quality floor**: All deterministic checks pass; every LLM-as-judge criterion ≥ 4.
- **Diminishing returns**: Stop when the last iteration improves no judge criterion by ≥ 1 point and all deterministic checks already pass.
- **Max iterations**: 3.
