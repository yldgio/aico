# Spec: Non-Interactive Execution Mode

> Goal: Make `aico run` callable from scripts, cron, CI, and other programs — without a TTY, with captured output, and a meaningful exit code.
> Date: 2026-06-25
> Status: Complete (2026-06-25)

---

## What & Why

Today `aico run` always uses `docker run -it` / `docker exec -it`, which
requires a TTY. This makes it impossible to use from scripts, cron jobs, CI
pipelines, or programmatic orchestration. By auto-detecting the absence of a
TTY and adjusting the Docker flags accordingly, aico becomes scriptable with
zero configuration — enabling all future orchestration, parallelism, and
automation patterns.

## Done Looks Like

- `aico run pi /tmp -- -p "explain X" > result.md` works in a script — output
  captured, no TTY errors.
- `echo "fix the auth bug" | aico run pi /tmp -- -p -` pipes input to the agent.
- Exit code reflects the agent's exit code; aico infra errors return 125.
- Multiple parallel runs work: `aico run pi repo1 -- -p "A" & aico run codex repo2 -- -p "B" & wait`
- Interactive terminal use is completely unchanged.

---

## Scope

### In Scope

- Auto-detect TTY: if `os.Stdin` is not a terminal, use `-i` (no `-t`) for
  `docker run` and `docker exec`.
- Pass-through output: agent stdout → aico stdout, agent stderr → aico stderr.
  Aico's own messages (image build, warnings) stay on stderr.
- Exit code contract: agent's exit code passes through; aico infrastructure
  errors (runtime not found, image build failed, container create failed)
  return exit code 125.
- Works with and without `-d` (orthogonal).
- Works for both `docker run` (fresh container) and `docker exec` (resume/-d).

### Out of Scope

- **`--rm` / ephemeral mode** — *future flag; container lifecycle unchanged.*
- **Structured JSON output** — *not needed; Unix pipes suffice.*
- **Orchestration / workflow engine** — *that's option 2; this is the prerequisite.*
- **Explicit `--non-interactive` flag** — *auto-detect is sufficient; add later if needed.*

---

## Constraints & Assumptions

### Hard Constraints

- Shell out to runtime CLI only.
- Interactive behavior must not change when stdin IS a TTY.
- Exit code 125 for aico infra errors (matches Docker convention).
- Aico messages to stderr only (never pollute stdout with non-agent output).

### Assumptions

- `docker run -i` (without `-t`) streams stdout/stderr correctly for
  non-interactive agent commands. *Known true.*
- `docker exec -i` (without `-t`) works the same way. *Known true (tested).*
- Agents' non-interactive modes (`pi -p`, `claude -p`, `codex --quiet`) write
  to stdout and exit with a meaningful code. *Assumed true; agent-specific.*
- `term.IsTerminal(int(os.Stdin.Fd()))` works on Linux, macOS, and Windows.
  *Standard Go library, known true.*

---

## Decisions Already Made

| Decision | Rationale |
|----------|-----------|
| Auto-detect from TTY, no flag | Unix convention; zero friction for scripters |
| Pass-through output (no framing) | Maximizes composability; `> file`, `\| jq` just work |
| Agent exit code passes through | Scripts can check `$?` meaningfully |
| 125 for aico infra errors | Docker convention; distinguishes "agent failed" from "aico failed" |
| `-i` without `-t` when no TTY | Keeps stdin open for piping; avoids "not a TTY" errors |
| Container lifecycle unchanged | `-d` is orthogonal to TTY; no implicit `--rm` |

---

## Task Breakdown

### Task 1: TTY detection helper

- **Depends on**: none
- **Description**: Add a helper `isTTY() bool` (using `golang.org/x/term` or
  `os.Stdin.Stat()`) that returns whether stdin is a terminal. Place in a
  shared location accessible by `cmd/run.go` and `cmd/exec.go`.
- **Done when**: unit test passes on both TTY and non-TTY (can test via the
  `os.Stdin.Stat()` mode-bit approach).

### Task 2: Conditional `-it` vs `-i` in container creation and exec

- **Depends on**: Task 1
- **Description**: In `cmd/run.go`, use `isTTY()` to decide:
  - TTY → `-it` (current behavior, unchanged)
  - No TTY → `-i` only
  
  Apply to: `docker run` (fresh non-d), `docker run -d` + `docker exec`
  (detached), and `docker exec` (resume). Also update `runtime.Exec()` to
  accept a `tty bool` parameter (or add `ExecNonInteractive`).
- **Done when**: `echo test | aico run pi /tmp --dry-run` shows `-i` without
  `-t`; `aico run pi /tmp --dry-run` (in a terminal) shows `-it`.

### Task 3: Exit code pass-through (125 for infra errors)

- **Depends on**: Task 2
- **Description**: Update `cmd.Execute()` and `runAgent` to:
  - Detect when the container/exec process exits with a non-zero code and pass
    it through to `os.Exit()`.
  - Return exit code 125 for aico's own errors (runtime not found, image build
    failed, path resolution, etc.).
  - `runtime.Run()` and `runtime.Exec()` must propagate the subprocess exit
    code (currently they return a generic error).
- **Done when**: `aico run pi /tmp -- --bad-flag; echo $?` returns the agent's
  non-zero exit code; a bogus `--runtime /nonexistent` returns 125.

### Task 4: Ensure aico messages stay on stderr

- **Depends on**: none
- **Description**: Audit all `fmt.Printf` / `fmt.Println` in `cmd/` that emit
  aico-level messages (image building, warnings, dry-run). Ensure they use
  `fmt.Fprintf(os.Stderr, ...)` so agent stdout is never polluted. Dry-run
  output should also go to stderr (it's aico metadata, not agent output).
- **Done when**: `aico run pi /tmp --dry-run 2>/dev/null` produces no output on
  stdout; `aico run pi /tmp --dry-run` still shows the plan on stderr.

### Task 5: Documentation + integration test

- **Depends on**: Tasks 1–4
- **Description**: Update README with scripting examples. Add a CI integration
  test that runs `echo "hello" | ./aico run pi /tmp --dry-run` and verifies
  no TTY errors and correct dry-run output on stderr.
- **Done when**: README documents the non-interactive behavior; CI test passes.

---

## Evaluation Criteria

### Deterministic Checks

| Check | Task | How to run | Pass condition |
|-------|------|------------|----------------|
| no-TTY uses -i only | T2 | `echo x \| ./aico run pi /tmp --dry-run 2>&1` | contains `-i` but not `-it` |
| TTY uses -it | T2 | `./aico run pi /tmp --dry-run` (in terminal) | contains `-it` |
| exit code pass-through | T3 | `./aico run pi /tmp -- --nonexistent-flag; echo $?` | non-zero (agent's code) |
| infra error = 125 | T3 | `./aico run pi /tmp --runtime /nope 2>/dev/null; echo $?` | 125 |
| dry-run on stderr | T4 | `./aico run pi /tmp --dry-run 2>/dev/null` | empty stdout |
| dry-run visible on stderr | T4 | `./aico run pi /tmp --dry-run 2>&1 >/dev/null` | shows dry-run output |
| build/fmt/vet/test | all | `gofmt -l . && go vet ./... && go test ./...` | clean / green |

### LLM-as-Judge Criteria

| Criterion | Task | Question | Evidence to examine | Scale | Pass boundary |
|-----------|------|----------|---------------------|-------|---------------|
| Interactive unchanged | T2 | Is the interactive (TTY) path completely untouched? | diff of cmd/run.go, dry-run in terminal | 1–5: 5 = zero change to TTY behavior | ≥ 5 |
| Scripting docs clarity | T5 | Would a user understand how to use aico from a script? | README scripting section | 1–5: 5 = clear examples with exit codes, piping, parallel | ≥ 4 |

### Verification Protocol

- **Adversarial**: the verifying model MUST differ from the implementing model.
- **Process**: verifier runs every deterministic check, scores each judge
  criterion with evidence, and lists concrete issues for the implementer.

### Convergence

- **Quality floor**: all deterministic checks pass; every judge criterion ≥ pass boundary.
- **Diminishing returns**: stop when the last iteration improves no criterion by ≥ 10%.
- **Max iterations**: 3.
