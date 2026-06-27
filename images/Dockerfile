# aico agent image — a single image containing every supported agent.
# aico selects which agent to launch at run time; the image is shared across
# all agents for a given host.
FROM node:22-bookworm-slim

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
