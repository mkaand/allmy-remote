# AllMy Remote

A lightweight, self-hosted remote desktop and device management platform, built from scratch. One platform. All your devices.

**Status: Sprint 1B — local development preview.** The Go core includes a static shell, health/readiness, SQLite WAL storage and single-operator authentication API. No browser login UI, device enrollment, agent, remote access or production deployment is available. Features below remain planned.

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

Baseline stack: Go server and agents, React/TypeScript web client, SQLite WAL. Native operating system APIs and maintained protocol/codec libraries are allowed; existing remote desktop applications are not used as the project foundation.

See [architecture](docs/architecture.md), [technical specification](docs/specification.md), [threat model](docs/threat-model.md), [acceptance criteria](docs/acceptance.md), [UI guidelines](docs/ui.md), [security](SECURITY.md), and [roadmap](docs/roadmap.md).

## Installation

Local development only; do not expose this preview to the Internet. Production MFA and additional security gates are pending.

With Go 1.26.11:

```sh
go run ./cmd/server
```

Open http://localhost:8080. Status endpoints: /health and /ready. Run `go test -race ./...` for scoped checks.

Container preview (loopback host binding only):

```sh
docker build -t allmy-remote:dev .
docker run --rm --read-only --tmpfs /data:rw,noexec,nosuid,size=16m,uid=65532,gid=65532,mode=0700 --cap-drop ALL --security-opt no-new-privileges -p 127.0.0.1:8080:8080 allmy-remote:dev
```

The non-loopback container listener is explicitly enabled inside the image; never publish its port on a public interface. The embedded shell is intentionally plain HTML/CSS; React/TypeScript and theme controls will arrive in Sprint 1D. SQLite uses a CGO-free driver and Argon2id uses a maintained Go library. The container example uses ephemeral data; persistent storage requires a private directory or volume owned by UID 65532.

### Operator authentication API

Initialize the single administrator before starting the service with `go run ./cmd/server init-admin`; supply a password (12–256 bytes) via protected standard input, never in command arguments or shell history. For local testing, use the default origin `http://localhost:8080`. The server requires an exact Origin header on login/logout and issues a Secure, HttpOnly, SameSite=Strict cookie. Use HTTPS or a localhost client supporting secure cookies. Endpoints: POST /api/v1/session/login (JSON username and password), GET /api/v1/session, POST /api/v1/session/logout. The username is admin; no default password. Each successful login revokes the previous session; sessions expire after 12 hours. Browser UI is deferred.

## License

MIT (see [LICENSE](LICENSE)). Third-party dependencies will be reviewed for compatibility and documented before distribution.
