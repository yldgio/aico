# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.5.1] - 2026-06-25

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

[0.5.1]: https://github.com/yldgio/aico/compare/v0.5.0...v0.5.1
[0.5.0]: https://github.com/yldgio/aico/compare/v0.4.0...v0.5.0
[0.4.0]: https://github.com/yldgio/aico/compare/v0.3.0...v0.4.0
[0.3.0]: https://github.com/yldgio/aico/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/yldgio/aico/compare/v0.1.2...v0.2.0
[0.1.2]: https://github.com/yldgio/aico/compare/v0.1.1...v0.1.2
[0.1.1]: https://github.com/yldgio/aico/compare/v0.1.0...v0.1.1

