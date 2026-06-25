# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.1.1] - 2026-06-24

### Fixed

- Windows: mount the project folder at `/workspace` inside the container instead
  of reusing the host path, which produced `the working directory 'D:\...' is
  invalid, it needs to be an absolute path` because a Windows path is not a valid
  Linux working directory.

## [0.1.0] - 2026-06-24

### Added

- `aico run <agent> [path]` — create or resume an isolated container for an AI
  coding agent, mounting the current folder and forwarding host credentials.
- Support for five agents: `pi`, `opencode`, `copilot-cli`, `codex`, `claude`.
- Vendor-independent container runtime abstraction with auto-detection
  (`docker`, then `podman`), overridable via `--runtime` or `AICO_RUNTIME`.
- Deterministic container identity (`aico-<agent>-<hash>`) derived purely from
  the agent and absolute project path.
- Read-only auth bind mounts and by-name environment forwarding for API keys.
- Embedded all-agents image with on-demand build on first run.
- Cross-platform path handling (Linux, macOS, Windows, WSL2).
- `--new`, `--image`, `--runtime`, `--verbose`, and `--dry-run` flags.
- GoReleaser configuration and GitHub Actions release pipeline for six targets.

[Unreleased]: https://github.com/yldgio/aico/compare/v0.1.1...HEAD
[0.1.1]: https://github.com/yldgio/aico/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/yldgio/aico/releases/tag/v0.1.0
