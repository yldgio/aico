# Goal: `aico` — Instant Isolated Agent Launcher

### What & Why
A public OSS CLI binary (Rust or Go) called `aico` that eliminates the friction of going from a project folder to a running AI coding agent. Today, launching an agent in a clean environment requires too many manual steps. `aico run <agent> [path]` collapses that into one command — creating a container on first use, resuming it on subsequent calls, mounting the project folder, and automatically forwarding host auth configs so the agent is ready to use immediately.

### Done Looks Like
- `aico run pi` from any folder starts (or resumes) a container with pi running, the current folder mounted, and pi's auth forwarded from the host — in one command
- Running the same command again on the same path re-attaches the existing stopped container without any data loss
- `aico run pi --new` replaces the old container with a fresh one
- The same binary works for: `pi`, `opencode`, `copilot-cli`, `codex`, `claude`
- Any developer can clone the repo, build, and use it — no hidden personal setup required
- Host auth configs (~/.pi, ~/.copilot, ~/.config/opencode, etc.) are detected and mounted into the container automatically

### Boundaries
- **Out of scope:** a setup wizard / `aico setup` subcommand (not in v1)
- **Out of scope:** multi-agent containers (one agent per container)
- **Out of scope:** cloud/remote containers — local Docker only
- **Out of scope:** GUI, TUI, or web interface
- **Out of scope:** auth *creation* — `aico` forwards existing auth, never sets it up
- **Not a constraint:** isolation/sandboxing as a hard security requirement — it's a welcome side-effect, not a design driver

### Decisions Record
| # | Decision | Choice | Reason |
|---|---|---|---|
| 1 | Primary problem | Eliminate launch friction | "Simplest CLI" phrasing; isolation is additive |
| 2 | Speed vs. isolation | Speed primary | Isolation is a container side-effect, not a goal |
| 3 | Audience | Public OSS | Should work for any developer, not just personal use |
| 4 | CLI form | Compiled binary (Rust or Go) | Feels like a real tool; doubles as a learning vehicle |
| 5 | v1 scope | `run` only | Auth-sharing lands early; `setup` subcommand deferred |
| 6 | Auth sharing priority | High — early, not deferred | This is where most of the value lives |
| 7 | Resume strategy | Re-attach stopped container; fallback to named volume | Preserves state naturally |
| 8 | Container identity | `aico-<agent>-<hash-of-path>` | Deterministic, no lockfile, no labels needed |
| 9 | Default on existing container | Resume; `--new` to replace | Destruction must be explicit |
