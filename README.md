# aico

[![CI](https://github.com/yldgio/aico/actions/workflows/ci.yml/badge.svg)](https://github.com/yldgio/aico/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/yldgio/aico)](https://goreportcard.com/report/github.com/yldgio/aico)

**One command to launch an AI coding agent in an isolated container with persistent login.**

`aico` puts your AI coding agent inside a container, mounts your project folder, and keeps you logged in across runs — so the agent is ready to use immediately. Run it again on the same folder and it resumes the same container, state intact.

```sh
aico run pi          # launch (or resume) pi in the current folder
aico run claude ~/work/api   # ...or any folder
```

Supported agents: **pi**, **opencode**, **copilot-cli**, **codex**, **claude**.

---

## Why

Starting an agent in a clean environment usually means: build an image, mount the right folder, log in again, remember which container was which. `aico` collapses all of that into a single command, keeps your login across runs, and isolates each run in its own container as a side effect.

---

## Install

### One-liner (recommended)

**Linux / macOS:**

```sh
curl -sSfL https://raw.githubusercontent.com/yldgio/aico/main/install.sh | sh
```

**Windows (PowerShell):**

```powershell
irm https://raw.githubusercontent.com/yldgio/aico/main/install.ps1 | iex
```

Installs to `/usr/local/bin` (if writable) or `~/.local/bin` on Unix, and `%USERPROFILE%\.local\bin` on Windows (added to User PATH automatically). Override with `INSTALL_DIR`:

```sh
curl -sSfL https://raw.githubusercontent.com/yldgio/aico/main/install.sh | INSTALL_DIR=~/bin sh
```

### Uninstall

```sh
aico uninstall
```

Removes the binary, all aico containers, the agent image, and auth volumes.
Use `--keep-data` to preserve your login volumes (so you stay logged in if you reinstall).

### Remove & Purge

```sh
aico rm myapi              # remove a single container by name
aico rm pi ~/projects/api  # remove by agent + path
aico rm myapi --volumes    # also remove its auth volumes
aico purge                 # nuclear reset: all containers + volumes + image (binary stays)
```

### Upgrade

```sh
aico upgrade
```

Downloads and replaces the binary with the latest release from GitHub.
The agent image rebuilds automatically on the next run if the Dockerfile changed.

### With `go install`

If you have Go 1.26+:

```sh
go install github.com/yldgio/aico@latest
```

This builds `aico` and installs it to `$(go env GOPATH)/bin` (make sure that's on your `PATH`).

### From a release

Download the prebuilt binary for your platform from the [releases page](https://github.com/yldgio/aico/releases), unpack it, and put `aico` on your `PATH`.

### From source

```sh
git clone https://github.com/yldgio/aico
cd aico
go build -o aico .
# move ./aico somewhere on your PATH
```

Requires Go 1.26+ to build.

### Prerequisites

- A container runtime: **Docker** or **Podman**. `aico` auto-detects whichever is installed.

---

## Quick start

```sh
cd ~/my-project
aico run pi
```

On first run for a given agent, `aico` builds a one-time shared image containing all agents (this takes a few minutes). After that, launches are fast.

What happens:

1. `aico` detects your container runtime (Docker, then Podman).
2. It computes a deterministic container name from the agent + folder path (`aico-pi-<hash>`).
3. If a container for that agent+folder already exists, it **resumes** it. Otherwise it **creates** one.
4. Your folder is mounted into the container and set as the working directory. On Linux/macOS it is mounted at the same path it has on the host; on Windows it is mounted at `/workspace` (a Windows path like `D:\proj` is not a valid Linux directory).
5. Your login for that agent is preserved across runs in a per-agent volume; API keys you set in your environment are forwarded by name.
6. You land directly in the agent.

When you exit the agent, the container stops. The next `aico run` on the same folder resumes it with all its state.

---

## Authentication

You log in **once, inside the container**, and stay logged in for every future run.
`aico` keeps each agent's login in a per-agent **named volume** (`aico-auth-<agent>`)
that is global across your projects — so logging into `pi` once means every project
using `pi` is already authenticated. Nothing from your host is read by default, so
your host settings never leak into the container.

| Agent | How login is preserved |
|---|---|
| `pi` | volume `aico-auth-pi` → `/root/.pi/agent` |
| `opencode` | volume `aico-auth-opencode` → `/root/.local/share/opencode` + `OPENAI_API_KEY` / `ANTHROPIC_API_KEY` if set |
| `codex` | volume `aico-auth-codex` → `/root/.codex` + `OPENAI_API_KEY` if set |
| `claude` | volume `aico-auth-claude` → `/root/.claude` + `ANTHROPIC_API_KEY` if set |
| `copilot-cli` | volumes `aico-auth-copilot-cli` → `/root/.copilot`, `…-gh` → `/root/.config/gh`, `…-keyring` → `/root/.local/share/keyrings` (token stored via gnome-keyring/libsecret) |

**API keys** are forwarded **by name only** (`-e KEY`, never `-e KEY=VALUE`), so the
value never appears in the runtime's argument list. Set the variable in your shell and
`aico` passes it through if present.

**Importing host config** is opt-in. Pass `--import-config` on first run to copy
your host config directory into the container. The copy is one-time — subsequent
runs don't overwrite, so changes you make inside the container persist.
`aico` never mounts your host config read-write.

Examples:

```sh
# codex picks up your key from the environment
OPENAI_API_KEY=sk-... aico run codex

# pi: log in once inside the container; every later run stays logged in
aico run pi

# also import your host opencode config (one-time copy)
aico run opencode --import-config
```

---

## Usage

```
aico run <agent> [path] [flags] [-- agent-args...]
```

- `<agent>` — one of `pi`, `opencode`, `copilot-cli`, `codex`, `claude`
- `[path]` — project folder to mount (defaults to the current directory)
- `[-- args]` — everything after `--` is forwarded to the agent command

### Checking the version

```sh
aico --version     # one line, e.g. "aico v0.1.2"
aico version       # detailed: version, commit, build date, Go, os/arch
```

### Flags

| Flag | Description |
|---|---|
| `-d`, `--detach` | Keep the container running after the agent exits. Re-run `aico run` to re-attach, or `aico exec` to open a shell. A container's mode is fixed when it is created: if you pass `-d` for a container that already exists in interactive mode, aico asks for confirmation before destroying and recreating it (or errors with a `--new` hint when there is no terminal to prompt on). |
| `--name <name>` | Assign a short name to the container (default: folder basename). Use the name with `aico run <name>` or `aico exec <name>`. |
| `--new` | Discard any existing container for this agent+folder and create a fresh one. |
| `--image <tag>` | Use a custom image instead of the built-in agent image. Skips the built-in build entirely. |
| `--runtime <bin>` | Force a specific container runtime (e.g. `podman`). Overrides auto-detection. |
| `--verbose` | Print warnings, e.g. when an `--import-config` source directory is missing. |
| `--dry-run` | Print what would run, without creating a container. |
| `--import-config` | Copy host config into the container on first run (one-time; does not overwrite on resume). |

You can also set the runtime via the `AICO_RUNTIME` environment variable:

```sh
AICO_RUNTIME=podman aico run pi
```

### Examples

```sh
aico run pi                       # current folder, resume if it exists
aico run claude ~/work/api        # a specific folder
aico run codex --new              # force a fresh container
aico run pi --image my/custom:tag # bring your own image
aico run opencode --dry-run       # see what would happen

# Detach mode: container stays alive after agent exits
aico run pi -d                    # interactive session, container persists
aico run pi                       # re-attach to the same container
aico exec pi                      # open a shell alongside the agent
# Note: a container's mode is set at creation. Passing -d for a container
# created in interactive mode prompts to recreate it (destructive); use --new
# to recreate without the prompt.

# Pass args to the agent
aico run pi -- -p "fix the tests" # forward args after --
```

### Scripting & automation

`aico` auto-detects when stdin is not a terminal and runs non-interactively
(no TTY allocated). Output streams directly to stdout/stderr for piping and
capture. Exit codes pass through from the agent (125 = aico infrastructure error).

```sh
# Capture agent output to a file
aico run pi ~/proj -d -- -p "explain the auth module" > docs/auth.md

# Pipe input to the agent
echo "fix the failing tests" | aico run pi ~/proj -d -- -p -

# Check exit code in a script
aico run codex ~/proj -d -- "update dependencies"
if [ $? -ne 0 ]; then echo "agent failed"; fi

# Parallel execution on multiple repos
aico run pi repo1 -d -- -p "lint fix" &
aico run codex repo2 -d -- "update deps" &
wait

# Cron job
0 3 * * * aico run pi /srv/api -d -- -p "daily maintenance" >> /var/log/aico.log 2>&1
```

### `aico ls` — list containers

```sh
aico ls
```

Shows all aico containers with their name, agent, project path, and status:

```
NAME    AGENT  PATH                       STATUS
myapi   pi     /home/dev/projects/api     running
front   codex  /home/dev/projects/front   stopped
```

Containers are auto-named from the project folder basename. Override with `--name`.
Use the name to access containers directly:

```sh
aico run myapi                    # re-attach by name
aico exec myapi                   # shell by name
```

### `aico exec` — shell into a running container

```sh
aico exec <agent|name> [path]
```

Opens an interactive bash shell in a running container (started with `-d`).
Useful for exploring the filesystem, debugging, or running commands alongside
the agent.

```sh
aico exec pi                      # shell into the pi container for cwd
aico exec myapi                   # shell by name
```

### `aico rm` — remove a container

```sh
aico rm <name|agent> [path]       # remove by name or agent+path
aico rm myapi --volumes           # also remove auth volumes (re-login needed)
```

### `aico purge` — nuclear reset

```sh
aico purge                        # remove ALL containers + volumes + image
```

The binary stays. Use `aico uninstall` to also remove it.

### `aico upgrade` — self-update

```sh
aico upgrade                      # download + replace with latest release
```

### `aico uninstall` — remove everything

```sh
aico uninstall                    # purge + delete binary
aico uninstall --keep-data        # keep login volumes
```

---

## How it works

- **Container identity** is `aico-<agent>-<sha256(abspath)[:8]>` — deterministic, derived purely from the agent name and absolute folder path. No lockfiles are written into your project.
- **Resume** re-attaches a running container, or restarts a stopped one. `--new` removes it first.
- **After upgrading `aico`**: if an agent's startup command changed between versions, existing containers still use the old command (Docker bakes it at creation time). Run with `--new` once after upgrading to recreate them.
- **Runtime independence**: `aico` only ever shells out to a container CLI, so Docker, Podman, or any OCI-compatible drop-in works. Auto-detection order is `docker`, then `podman`.
- **One shared image** holds all agents; the agent to launch is chosen at run time.

---

## Scope

See `specs/aico.md` for the full specification. Intentionally **not** included
yet: a guided `setup` wizard, composing a subset of agents into one image, and
a built-in workflow orchestrator.

---

## Contributing

Contributions are welcome! Please read [CONTRIBUTING.md](CONTRIBUTING.md) for
setup, conventions, and the PR checklist, and [AGENTS.md](AGENTS.md) for the
architecture map and hard constraints. By participating you agree to the
[Code of Conduct](CODE_OF_CONDUCT.md).

Found a security issue? Please follow [SECURITY.md](SECURITY.md) and do **not**
open a public issue.

---

## License

MIT — see [LICENSE](LICENSE).
