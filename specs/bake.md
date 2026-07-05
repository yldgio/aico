# Spec: `aico bake` — Snapshot a Container into a Pushable Image

> Goal: A single non-interactive command that bakes a fully-configured aico container into a taggable, pushable OCI image.
> Date: 2026-07-05
> Status: Complete (implemented 2026-07-05)

---

## What & Why

Today the only way to get a reusable image out of aico is to manually `docker
commit`/`tag` a container you first had to launch interactively with `run` or
`exec`. `aico bake` collapses that into one **non-interactive** command that
produces a **taggable, pushable OCI image** from a fully-configured aico
container — so you can snapshot a customized environment (and optionally the
project itself) and hand it to a registry, CI, or another machine.

The value over a plain `docker commit` is that aico already knows how to
construct the correctly-configured container (base image, mounts, env,
entrypoint, agent) and does it without ever dropping you into the agent UI.

## Done Looks Like

- `aico bake pi ./folder -t my/img:tag` produces a local OCI image capturing the
  container's filesystem (installed tools, configs) **without launching the
  agent UI**.
- If a container for `(pi, ./folder)` already exists, bake commits **that**
  container's current state; otherwise bake **creates** one (via `docker
  create`, not `run`) and commits it. `--new` forces a fresh container first.
- A bake-created container is a normal `aico-<agent>-<hash>` container — a later
  `aico run` resumes it.
- `-w/--include-workspace` additionally copies the project folder into the
  image (honoring `.dockerignore`) and sets `WORKDIR` to the project path, so
  `docker run <image> <agent>` starts inside the code.
- **Auth/login is never baked in**: auth lives in named volumes and env vars,
  which `docker commit` excludes by construction. Bake prints a caution and the
  image is safe to push to a public registry.
- The resulting image carries the user's `-t` tag; they `docker push` it
  themselves. Registry push is out of scope.
- Missing/failed prerequisites (no runtime, bad path, missing tag) fail with a
  clear `cause + fix:` message, matching existing aico style.

---

## Scope

### In Scope

- `aico bake <agent> [path] -t <tag>` — new subcommand.
- `-t/--tag` (required, single tag) — the resulting image name.
- `-w/--include-workspace` — copy the project folder into the image, honoring
  `.dockerignore`, and set `WORKDIR`.
- `--new` — destroy any existing container, create fresh, then bake.
- `--image <tag>` — custom **base** image used only when bake must *create* the
  container (ignored if the container already exists).
- `--runtime <binary>` / `AICO_RUNTIME` — select the container runtime.
- `--dry-run` — print the exact commit/build/tag plan without executing.
- `--verbose` — extra signal (e.g. "container already existed, baking current
  state").
- Shared container identity with `run`/`exec`: `aico-<agent>-<hash>`.
- Commit an existing container **in place**, regardless of running/stopped state.
- New `internal/runtime` methods for commit + build; no docker-specific logic in
  `cmd`.

### Out of Scope

- **Pushing to a registry** — *the user runs `docker push` themselves; keeps
  aico out of the network/auth path.*
- **Baking auth/login into the image** — *serious secret-leak risk on public
  registries; contradicts aico's "auth never leaves volumes" posture.*
- **Multiple tags in one bake** — *single tag for v1; simplicity.*
- **`.gitignore` / `.aicoignore` support** — *only `.dockerignore` for v1;
  a dedicated ignore file is a future decision.*
- **`--share-config`** — *read-only config mounts aren't committed; irrelevant.*
- **Multi-agent images** — *one agent per container/image by design.*

---

## Constraints & Assumptions

### Hard Constraints

- **Shell out to the runtime CLI** — commit and build go through
  `internal/runtime`; never a container SDK.
- **No docker-specific strings in `cmd/bake.go`** — all runtime literals stay in
  `internal/runtime`.
- **Container identity is pure** — `aico-<agent>-<hash>`; no lockfiles/labels
  written to the project.
- **Auth is never placed in the image** — bake relies on `docker commit`
  excluding named volumes and bind mounts; it never copies auth into the FS.
- **Cross-platform path logic lives only in `internal/platform`** — bake reuses
  existing path resolution; no new `runtime.GOOS` branches elsewhere.

### Assumptions

| Assumption | What happens if wrong |
|---|---|
| `docker commit` excludes named volumes and bind mounts (documented behavior). | If a runtime deviates, auth could leak; the deterministic "auth absent" check catches it before release. |
| All designed agent auth lives in named volumes (pi/opencode/copilot keyring) or host env (codex/claude), never the container's own FS. | An agent caching a credential elsewhere in the FS would be captured; mitigated by the printed caution, not a hard guarantee. |
| `docker build` honors `.dockerignore` from the build-context root. | If context resolution differs, ignored files could ship; the `.dockerignore` deterministic check catches it. |
| `docker create` yields a container `docker commit` can snapshot without starting it. | If commit needs a started container, Task 3 flips to create+start-then-stop; discovered during T3. |

---

## Decisions Already Made

| Decision | Choice | Rationale |
|---|---|---|
| Command name | `bake` | Conveys "produce an artifact"; non-interactive. |
| Tag interface | Required `-t/--tag` flag, single tag | Keeps positionals identical to `run`/`exec`; no ambiguity; matches `docker build -t`; required = explicit intent. |
| Workspace shortcut | `-w` for `--include-workspace` | `-t` taken by tag; `-w` is free and mnemonic. |
| Content model | Container FS customizations; workspace only with `-w` | Auth excluded by design; workspace opt-in. |
| Ignore rules | `.dockerignore` only | Native to `docker build`; other ignore files deferred. |
| Container identity | Shared `aico-<agent>-<hash>` with run/exec | A bake-created container is reusable by `run`. |
| Missing container | `docker create` (not started), then commit | Non-interactive; no agent UI. |
| Existing container | Commit current state **in place**, any state | Simpler; no "please stop first". |
| Bake-created container | **Left** in place | Consistent with run/exec creating one. |
| `--new` | Remove existing, create fresh, then bake | Matches existing `--new` contract. |
| Workspace copy mechanics | commit → intermediate image → `docker build` (`FROM intermediate` + `COPY . <abs>` + `WORKDIR <abs>`) → remove intermediate | `.dockerignore` is a build-context feature; `docker cp` ignores it. |
| WORKDIR | Set to the workspace abs path, only with `-w` | Makes a workspace image self-contained; no meaningful WORKDIR without `-w`. |
| Auth safety | Rely on volume/bind exclusion + print caution; test auth-volume contents absent | The guaranteeable, testable property; honest about rogue in-FS caching. |
| Registry push | Out of scope | User runs `docker push`; aico stays out of network/auth. |

---

## Task Breakdown

### Task 1: `bake` command scaffold

- **Depends on**: none
- **Description**: Add `cmd/bake.go`; wire into `cmd/root.go`. Cobra command
  `bake <agent> [path]` with flags `-t/--tag` (required), `-w/--include-workspace`,
  `--new`, `--image`, `--runtime`, `--dry-run`, `--verbose`. Validate `<agent>`
  against the registry (reuse existing validation). Stub logic prints the plan.
- **Done when**: `aico bake --help` shows correct usage including `-t` and `-w`;
  `aico bake fakeagent -t x` exits non-zero naming valid agents; `aico bake pi`
  (no `-t`) exits non-zero saying `-t/--tag` is required.

### Task 2: Runtime `Commit` + build methods

- **Depends on**: Task 1
- **Description**: Add to `internal/runtime`: `Commit(container, tag string)
  error` (`docker commit`) and a build-from-dir capability (new method or reuse
  `images/image.go` plumbing) for the `-w` path. No docker-specific logic
  outside `internal/runtime`.
- **Done when**: unit tests assert the commit/build argv; `--dry-run` prints
  them.

### Task 3: Container resolution & creation (no start)

- **Depends on**: Task 2
- **Description**: Resolve `aico-<agent>-<hash>`. If it exists → use as-is. If
  missing → `docker create` with the same config `run` builds (mounts, auth
  volumes, env, entrypoint), do **not** start it. `--new` → remove then create.
- **Done when**: baking a missing container creates exactly one
  `aico-<agent>-<hash>` in `Created` state; a later `aico run` resumes it (still
  one container for that path); `--new` yields a new container ID.

### Task 4: Bake without workspace

- **Depends on**: Task 3
- **Description**: Commit the resolved container directly to the `-t` tag. Print
  the caution line about not committing secrets.
- **Done when**: `aico bake pi -t t/x` yields image `t/x`;
  `docker run --rm t/x pi --version` exits 0; auth-volume contents are absent
  from the image.

### Task 5: Bake with `--include-workspace`

- **Depends on**: Task 4
- **Description**: Commit → intermediate image; generate `FROM <intermediate>` +
  `COPY . <abs-path>` + `WORKDIR <abs-path>`; `docker build` with the workspace
  as build context (honors `.dockerignore`) → final `-t` tag; remove the
  intermediate image.
- **Done when**: a `.dockerignore`'d file is absent from the image; project
  files are present at the abs path; image `WORKDIR` == abs path; no dangling
  intermediate image remains.

### Task 6: Documentation

- **Depends on**: Tasks 4, 5
- **Description**: Add a README section for `bake` and its flags table entry;
  write cobra `Short`/`Long`/flag help. Document that push is out of scope (show
  the `docker push` follow-up) and the secret-caution.
- **Done when**: README and `--help` accurately describe bake; a first-time user
  can bake an image and push it using the README alone.

---

## Evaluation Criteria

### Deterministic Checks

| Check | Task | How to run | Pass condition |
|---|---|---|---|
| Help works | T1 | `aico bake --help` | Exit 0; shows `<agent>`, `[path]`, `-t`, `-w` |
| Missing tag rejected | T1 | `aico bake pi` | Exit non-zero; message says `-t/--tag` required |
| Unknown agent rejected | T1 | `aico bake fakeagent -t x` | Exit non-zero; names valid agents |
| Commit argv correct | T2 | unit test on `runtime.Commit` | argv == `commit <container> <tag>` |
| Dry-run prints plan | T2/T4/T5 | `aico bake pi -t t/x --dry-run` | Prints commit (and build, with `-w`) commands; no execution |
| Missing container created (not started) | T3 | `aico bake pi /tmp/p -t t/x`; inspect | One `aico-pi-<hash>` in `Created` state |
| Run resumes bake-created container | T3 | bake then `aico run pi /tmp/p`; count containers | Exactly 1 container for that path |
| `--new` replaces | T3 | bake, then `--new`; compare IDs | New ID; old gone |
| Bake (no workspace) runnable | T4 | `aico bake pi -t t/x`; `docker run --rm t/x pi --version` | Image exists; exit 0 |
| Auth volume absent from image | T4 | seed a marker in the auth volume; bake; `docker run --rm t/x ls <authpath>` | Marker absent |
| `.dockerignore` respected | T5 | add ignored file; `aico bake pi -t t/x -w`; inspect image | Ignored file absent |
| Workspace + WORKDIR baked | T5 | inspect image FS + config | Project files at abs path; `WORKDIR` == abs path |
| Intermediate image removed | T5 | after `-w` bake; `docker images` | No dangling intermediate left by bake |

### LLM-as-Judge Criteria

| Criterion | Task | Question | Evidence to examine | Scale | Pass boundary |
|---|---|---|---|---|---|
| Error message clarity | T1/T3 | Do bake's errors state cause + a `fix:` next step? | Trigger: missing tag, unknown agent, no runtime, bad path; read each message | 1–5: 5 = cause + fix + example; 1 = generic string | ≥ 4 |
| README usability | T6 | Can a new user bake an image and push it using only the README? | Read the bake section end-to-end, simulate a first run | 1–5: 5 = zero ambiguity incl. push follow-up + secret caution; 1 = missing critical steps | ≥ 4 |
| Runtime abstraction quality | T2 | Are commit/build fully inside `internal/runtime` with no docker literals leaking into `cmd`? | Read `cmd/bake.go` and grep for `docker`/`podman` strings | 1–5: 5 = none leak; 1 = scattered | ≥ 4 |

### Verification Protocol

- **Adversarial**: the verifying model MUST be different from the implementing
  model.
- **Process**: the verifier evaluates every criterion above, produces pass/fail
  with evidence for each, and returns a prioritized list of issues for the
  implementer.

### Convergence

- **Quality floor**: all deterministic checks pass; all LLM-as-judge criteria
  score ≥ 4.
- **Diminishing returns**: stop when the last iteration improved less than 10%
  on any failing criterion.
- **Max iterations**: 3.
