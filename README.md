# AllMy Remote

A lightweight, self-hosted remote desktop and device management platform, built from scratch. One platform. All your devices.

**Status: Sprint 0 — architecture only.** No agent, server, installer, or production deployment is available yet. Features below are planned, not implemented.

## Goals

- Browser-based remote desktop for authorized devices, with keyboard, mouse, and multi-monitor support.
- Session-independent universal clipboard across enrolled devices, with explicit enrollment and per-device controls.
- File management, terminal, telemetry, and signed agent updates in later milestones.
- Responsive dashboard with light, dark, and system themes.
- Low idle CPU, memory, and network usage; no screen capture when a session is inactive.
- Self-hosted deployment using a single application and a configurable domain.

## Platform roadmap

| Platform | Target | Status |
| --- | --- | --- |
| Windows 11 x64 | First native host and web client | Planned |
| Linux x64 (Zorin OS) | Native host and web client | Planned |
| Linux ARMv6 | Lightweight management host; GUI conditional | Planned |
| iOS Safari | Browser client; explicit clipboard actions | Planned |
| Managed corporate devices | Only with organizational authorization | Out of initial scope |

## Architecture

Candidate stack: Go server and agents, React/TypeScript web client, SQLite WAL. Native operating system APIs and maintained protocol/codec libraries are allowed; existing remote desktop applications are not used as the project foundation.

See [architecture](docs/architecture.md), [security](SECURITY.md), and [roadmap](docs/roadmap.md).

## Installation

Not available yet. No public-facing service should be deployed from this planning baseline.

## License

MIT (see [LICENSE](LICENSE)). Third-party dependencies will be reviewed for compatibility and documented before distribution.
