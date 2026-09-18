# aico agent image — a single image containing every supported agent.
# aico selects which agent to launch at run time; the image is shared across
# all agents for a given host.
#
# This Dockerfile has two build targets sharing one base stage:
#   - `base`   (default, last-but-one stage): the plain agent image, tagged
#              aico-agents:latest. Built with `docker build --target base`.
#   - `devenv` (opt-in, FROM base): adds Nix + the devenv CLI, tagged
#              aico-agents-devenv:latest. Built with `docker build --target devenv`.
# `docker build` with no --target builds the LAST stage in the file, so both
# aico.image.go build paths must always pass an explicit --target.
FROM node:22-bookworm-slim AS base

# Common tooling agents tend to rely on.
RUN apt-get update && apt-get install -y --no-install-recommends \
      git ca-certificates curl ripgrep less \
    && rm -rf /var/lib/apt/lists/*

# Install all supported agents globally. Pinning is intentionally left loose for
# v1 so the image tracks each agent's latest release at build time.
RUN npm install -g \
      @earendil-works/pi-coding-agent \
      opencode-ai \
      @github/copilot \
      @openai/codex \
      @anthropic-ai/claude-code \
    && npm cache clean --force

# Keyring support for copilot-cli: it stores its OAuth token in the system
# keyring (libsecret), not a file. gnome-keyring + dbus provide a headless
# secrets service so the token is encrypted at rest inside the container.
# This layer is separate from the base apt layer so the expensive npm install
# above stays cached when these packages are added/updated.
RUN apt-get update && apt-get install -y --no-install-recommends \
      gnome-keyring dbus-x11 libsecret-tools \
    && rm -rf /var/lib/apt/lists/*

# Pre-seed the gnome-keyring with an empty-password login collection.
# This auto-unlocks headlessly (no GUI prompter needed).
RUN mkdir -p /root/.local/share/keyrings \
    && printf '[keyring]\ndisplay-name=login\nctime=0\nmtime=0\nlock-on-idle=false\nlock-after=false\n' \
       > /root/.local/share/keyrings/login.keyring \
    && chmod 600 /root/.local/share/keyrings/login.keyring \
    && echo 'login' > /root/.local/share/keyrings/default

# Entrypoint for copilot-cli: starts dbus + gnome-keyring, then exec's copilot.
COPY copilot-entrypoint.sh /usr/local/bin/copilot-entrypoint.sh
RUN chmod +x /usr/local/bin/copilot-entrypoint.sh

# Python runtime for agent tasks. uv (and uvx) manage Python and packages; we
# also install the latest CPython as the global default so `python`/`python3`
# work directly on PATH without `uv run`. uv, the unversioned executables, and
# the interpreters all live under /root/.local/bin, which we put on PATH so
# non-login shells (e.g. `docker exec`) resolve them. Version pinning is left
# loose for v1 (matches the npm agents above): the build tracks the latest
# uv + CPython 3.14 at build time.
ENV PATH="/root/.local/bin:${PATH}"
RUN curl -LsSf https://astral.sh/uv/install.sh | sh -s -- --no-modify-path \
    && uv python install --default 3.14 \
    && uv cache clean

WORKDIR /workspace

# No ENTRYPOINT: aico supplies the agent command at `run` time.
CMD ["bash"]

# ---------------------------------------------------------------------------
# devenv stage — opt-in, built only via `docker build --target devenv`.
# Adds Nix + the devenv CLI on top of the base stage. Kept as a separate,
# late stage so plain (non-devenv) users never pull these ~1GB+ layers, and so
# the expensive npm/apt layers above stay cached when this stage changes.
# ---------------------------------------------------------------------------
FROM base AS devenv

# xz-utils is required by the Nix installer to unpack its store tarball; it
# isn't needed by the base image, so it's installed only in this stage.
RUN apt-get update && apt-get install -y --no-install-recommends xz-utils \
    && rm -rf /var/lib/apt/lists/*

# Single-user, no-daemon, root install (verified: devenv.sh's own "Windows
# (WSL2)" instructions use this exact form for containers without systemd).
# Single-user is required here, not multi-user/daemon: it puts the Nix store
# at /nix owned by root with no nix-daemon service to manage, which is what
# lets a later `docker run -v aico-nix:/nix` bind-mount the store directly.
# Two root-specific quirks the installer hits in a plain container, worked
# around here: (1) it shells out to `sudo` to create /nix even when already
# root (no sudo in this image, so /nix is pre-created); (2) nix.conf still
# defaults to `build-users-group = nixbld` even in single-user mode and
# refuses to install unless that group exists and has a member other than
# root (single-user mode never actually uses it to sandbox builds), so an
# unrelated placeholder user fills that membership requirement.
RUN mkdir -m 0755 /nix \
    && groupadd -r nixbld \
    && useradd -r -G nixbld -M -s /usr/sbin/nologin nixbld1 \
    && curl -fsSL https://nixos.org/nix/install | sh -s -- --no-daemon

# The single-user installer creates a profile under /root/.nix-profile and a
# system-wide default profile under /nix/var/nix/profiles/default; put both
# bin dirs on PATH so `nix`/`devenv` resolve in every exec form (`docker run`,
# `docker exec`, login and non-login shells alike). devenv requires the
# nix-command and flakes experimental features, which are opt-in upstream.
ENV PATH="/root/.nix-profile/bin:/nix/var/nix/profiles/default/bin:${PATH}" \
    NIX_CONFIG="experimental-features = nix-command flakes"

RUN nix profile install nixpkgs#devenv
