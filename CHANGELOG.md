# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.15.1] - 2026-06-27

### Fixed

- **install:** resolve arch via env fallback when RuntimeInformation is null

## [0.15.0] - 2026-06-27

### Added

- **image:** bundle uv + Python 3.14 as the global default

### Documentation

- promote changelog [0.15.0]

## [0.14.0] - 2026-06-27

### Added

- **run:** drop into a container shell when the agent exits under -d

### Documentation

- **agents:** note AGENTS.md is a living document
- promote changelog [0.14.0]

## [0.13.0] - 2026-06-27

### Added

- **run:** confirm before recreating a container to honor -d

### Documentation

- **agents:** document release process and Conventional Commits
- promote changelog [0.13.0]

## [0.12.1] - 2026-06-26

### Documentation

- add rm/purge to README, fix stale --verbose help text
- **agents:** add mandatory documentation rule
- promote changelog [0.12.1]

## [0.12.0] - 2026-06-26

### Added

- **cli:** add aico rm and aico purge commands

### Documentation

- promote changelog [0.12.0]

## [0.11.1] - 2026-06-26

### Documentation

- promote changelog [0.11.1]

### Fixed

- **cli:** auto-name includes agent to avoid collisions

## [0.11.0] - 2026-06-26

### Added

- **cli:** add aico ls, named containers, and name-based access

### Documentation

- promote changelog [0.11.0]

## [0.10.0] - 2026-06-26

### Added

- **cli:** replace --share-config with --import-config (one-time copy)

### Documentation

- promote changelog [0.10.0]

## [0.9.1] - 2026-06-25

### Documentation

- promote changelog [0.9.1]

### Fixed

- **docs:** remove stale 'forwarding host credentials' language

## [0.9.0] - 2026-06-25

### Added

- **cli:** non-interactive execution mode (auto-detect TTY)

### Documentation

- promote changelog [0.9.0]

### Fixed

- **ci:** update goreleaser config for v2 archive format syntax

## [0.8.0] - 2026-06-25

### Added

- **cli:** add aico upgrade command

### Documentation

- promote changelog [0.8.0]

## [0.7.0] - 2026-06-25

### Added

- **cli:** add aico uninstall command

### Documentation

- promote changelog [0.7.0]

## [0.6.0] - 2026-06-25

### Added

- **cli:** add -d (detach) flag and aico exec command

### Documentation

- promote changelog [0.6.0]

## [0.5.2] - 2026-06-25

### Documentation

- note --new needed after upgrade when agent command changes
- promote changelog [0.5.2]

### Fixed

- **image:** auto-rebuild stale image and include entrypoint in build context

## [0.5.1] - 2026-06-25

### Documentation

- promote changelog [0.5.1]

### Fixed

- **copilot:** use absolute path for entrypoint script

## [0.5.0] - 2026-06-25

### Added

- add one-command installer scripts for all platforms

### Documentation

- promote changelog [0.5.0]

## [0.4.0] - 2026-06-25

### Added

- **ci:** add multi-OS test matrix and automated release

## [0.3.0] - 2026-06-25

### Added

- **copilot:** persist login via gnome-keyring in named volumes

### Documentation

- copilot login persistence now shipped, remove v2 caveat
- promote changelog [0.3.0]

## [0.2.0] - 2026-06-25

### Added

- **auth:** persist agent login in per-agent named volumes

### Documentation

- document login-volume auth model and --share-config
- **agents:** add behavioral guidelines to reduce coding mistakes
- promote changelog [0.2.0]

## [0.1.2] - 2026-06-25

### Added

- **cli:** add version command and --version flag

## [0.1.1] - 2026-06-25

### Documentation

- mark 0.1.1 release in changelog

### Fixed

- **platform:** mount workspace at /workspace on Windows

## [0.1.0] - 2026-06-24

### Added

- **platform:** add cross-platform path resolution
- **runtime:** add vendor-independent container runtime abstraction
- **container:** add deterministic container identity
- **agents:** add registry of supported AI coding agents
- **auth:** add read-only mount and by-name env auth forwarding
- **image:** add embedded all-agents image with on-demand build
- **cli:** add run command for container create/resume lifecycle

### Documentation

- add product goal and specification
- add README, agent guidance, and license
- add community health files and GitHub templates
- add go install instructions and clarify release downloads
- mark 0.1.0 release in changelog

[0.15.1]: https://github.com/yldgio/aico/compare/v0.15.0...v0.15.1
[0.15.0]: https://github.com/yldgio/aico/compare/v0.14.0...v0.15.0
[0.14.0]: https://github.com/yldgio/aico/compare/v0.13.0...v0.14.0
[0.13.0]: https://github.com/yldgio/aico/compare/v0.12.1...v0.13.0
[0.12.1]: https://github.com/yldgio/aico/compare/v0.12.0...v0.12.1
[0.12.0]: https://github.com/yldgio/aico/compare/v0.11.1...v0.12.0
[0.11.1]: https://github.com/yldgio/aico/compare/v0.11.0...v0.11.1
[0.11.0]: https://github.com/yldgio/aico/compare/v0.10.0...v0.11.0
[0.10.0]: https://github.com/yldgio/aico/compare/v0.9.1...v0.10.0
[0.9.1]: https://github.com/yldgio/aico/compare/v0.9.0...v0.9.1
[0.9.0]: https://github.com/yldgio/aico/compare/v0.8.0...v0.9.0
[0.8.0]: https://github.com/yldgio/aico/compare/v0.7.0...v0.8.0
[0.7.0]: https://github.com/yldgio/aico/compare/v0.6.0...v0.7.0
[0.6.0]: https://github.com/yldgio/aico/compare/v0.5.2...v0.6.0
[0.5.2]: https://github.com/yldgio/aico/compare/v0.5.1...v0.5.2
[0.5.1]: https://github.com/yldgio/aico/compare/v0.5.0...v0.5.1
[0.5.0]: https://github.com/yldgio/aico/compare/v0.4.0...v0.5.0
[0.4.0]: https://github.com/yldgio/aico/compare/v0.3.0...v0.4.0
[0.3.0]: https://github.com/yldgio/aico/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/yldgio/aico/compare/v0.1.2...v0.2.0
[0.1.2]: https://github.com/yldgio/aico/compare/v0.1.1...v0.1.2
[0.1.1]: https://github.com/yldgio/aico/compare/v0.1.0...v0.1.1

