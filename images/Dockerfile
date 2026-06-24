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

WORKDIR /workspace

# No ENTRYPOINT: aico supplies the agent command at `run` time.
CMD ["bash"]
