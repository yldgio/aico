# aico

[![CI](https://github.com/yldgio/aico/actions/workflows/ci.yml/badge.svg)](https://github.com/yldgio/aico/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/yldgio/aico)](https://goreportcard.com/report/github.com/yldgio/aico)

**One command to launch an AI coding agent in an isolated, pre-authenticated container.**

`aico` puts your AI coding agent inside a container, mounts your project folder, and forwards the credentials you already have on your host — so the agent is ready to use immediately. Run it again on the same folder and it resumes the same container, state intact.

```sh
aico run pi          # launch (or resume) pi in the current folder
aico run claude ~/work/api   # ...or any folder
```

Supported agents: **pi**, **opencode**, **copilot-cli**, **codex**, **claude**.

---

## Why

Starting an agent in a clean environment usually means: build an image, mount the right folder, copy in credentials, remember which container was which. `aico` collapses all of that into a single command, and isolates each run in its own container as a side effect.

---

## Install

### With `go install` (works today)

If you have Go 1.26+:

```sh
go install github.com/yldgio/aico@latest
```

This builds `aico` and installs it to `$(go env GOPATH)/bin` (make sure that's on your `PATH`).

### From a release

Download the prebuilt binary for your platform from the [releases page](https://github.com/yldgio/aico/releases), unpack it, and put `aico` on your `PATH`:

```sh
# example: Linux x86_64
curl -sL https://github.com/yldgio/aico/releases/latest/download/aico_*_linux_amd64.tar.gz | tar xz
sudo mv aico /usr/local/bin/
```

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
5. Your host credentials for that agent are mounted read-only (or forwarded as env vars).
6. You land directly in the agent.

When you exit the agent, the container stops. The next `aico run` on the same folder resumes it with all its state.

---

## Authentication

`aico` never creates or asks for credentials — it forwards what you already have. If a credential source is missing it is silently skipped (run with `--verbose` to see what was skipped).

| Agent | What is forwarded |
|---|---|
| `pi` | `~/.pi/agent/` (read-only mount) |
| `opencode` | `~/.config/opencode/` (read-only mount) + `OPENAI_API_KEY`, `ANTHROPIC_API_KEY` if set |
| `copilot-cli` | `~/.copilot/` and `~/.config/gh/` (read-only mounts) |
| `codex` | `OPENAI_API_KEY` environment variable |
| `claude` | `ANTHROPIC_API_KEY` environment variable |

On Windows native, the equivalent locations under `%USERPROFILE%` and `%APPDATA%` are used automatically.

Examples:

```sh
# codex picks up your key from the environment
OPENAI_API_KEY=sk-... aico run codex

# pi reuses the login you already did on the host
aico run pi
```

---

## Usage

```
aico run <agent> [path] [flags]
```

- `<agent>` — one of `pi`, `opencode`, `copilot-cli`, `codex`, `claude`
- `[path]` — project folder to mount (defaults to the current directory)

### Checking the version

```sh
aico --version     # one line, e.g. "aico v0.1.2"
aico version       # detailed: version, commit, build date, Go, os/arch
```

### Flags

| Flag | Description |
|---|---|
| `--new` | Discard any existing container for this agent+folder and create a fresh one. |
| `--image <tag>` | Use a custom image instead of the built-in agent image. Skips the built-in build entirely. |
| `--runtime <bin>` | Force a specific container runtime (e.g. `podman`). Overrides auto-detection. |
| `--verbose` | Print warnings, e.g. when host credentials are not found. |
| `--dry-run` | Print what would run, without creating a container. |

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
```

---

## How it works

- **Container identity** is `aico-<agent>-<sha256(abspath)[:8]>` — deterministic, derived purely from the agent name and absolute folder path. No lockfiles are written into your project.
- **Resume** re-attaches a running container, or restarts a stopped one. `--new` removes it first.
- **Runtime independence**: `aico` only ever shells out to a container CLI, so Docker, Podman, or any OCI-compatible drop-in works. Auto-detection order is `docker`, then `podman`.
- **One shared image** holds all agents; the agent to launch is chosen at run time.

---

## Scope

This is v1. Intentionally **not** included yet: a guided `setup` command, composing a subset of agents into one image, and keeping containers running after the agent exits (for editor attach). See `specs/aico.md` for the full specification.

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
