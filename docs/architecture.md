# Architecture — Sprint 0

Status: Sprint 0 design baseline. See [technical specification](specification.md) for implementation contracts; performance and platform choices remain subject to profiling.

## Components

- Web: React + TypeScript, responsive browser client, accessible keyboard/touch controls, system/light/dark theme.
- Core: Go control plane, enrollment, device registry, capability negotiation, session coordination, bounded relay, audit metadata; SQLite WAL for durable metadata. Keep protocol and platform packages separate.
- Agents: Go with minimal native adapters where required. Windows x64 first. Linux x64 and ARMv6 follow; avoid coupling ARM telemetry to GUI dependencies.
- Deployment: one self-hosted service behind HTTPS; outbound agent connections; no inbound ports required on target devices. Local configuration and secrets remain outside the repository.

## First vertical slice

1. Local development core with authenticated operator and one-time enrollment.
2. Windows agent explicitly installed by its authorized operator, with visible status and stop/unpair controls.
3. Device heartbeat and minimal capability/telemetry dashboard.
4. View-only desktop proof of concept, then authorized input on the interactive desktop.
5. Session teardown, idle resource recovery, and manual acceptance test.

## Media and input

Windows capture candidates: DXGI Desktop Duplication or Windows Graphics Capture; evaluate driver compatibility and monitor selection. Input uses supported native APIs within authorized sessions. Secure desktop and privilege boundaries are not bypassed. Separate interactive user-session capture from any service lifecycle. Start with a bounded binary WebSocket proof of concept, evaluate JPEG changed-region transport versus hardware-assisted standard video encoding and WebRTC using measured latency, CPU, and traffic. Do not invent cryptographic protocols. Production media must satisfy the security design before exposure. Capture only during authorized sessions; backpressure drops stale frames rather than accumulating them. Negotiate resolution, frame rate, and quality.

## Clipboard

Independent of remote desktop sessions. Start with UTF-8 text and URLs, then image and rich text formats, then file transfer integration. Explicitly enable each device and content type; distinguish remote writes from local clipboard events to avoid sync loops. Use origin IDs, bounded payloads, deduplication, short expiry, and defined latest-event conflict semantics. Do not persist content on the server by default. Offline delivery, if implemented, must have bounded expiry and ciphertext-only handling. Browser clipboard requires user interaction where mandated by browser/OS. Linux clipboard requires a compatible active session; headless devices do not have a native desktop clipboard.

## Performance gates (proposed, not measured)

- Idle CPU target below 1% on Windows and Linux x64, with measurement method documented.
- Idle agent RAM targets: Windows <50 MiB, Linux x64 <30 MiB, ARMv6 <15 MiB (validate on real hardware).
- Capture/encode off while idle. Bounded queues, log rotation, no server transcoding by default.
- Record capture/encode/display latency, frame rate, bandwidth, agent/server memory, and CPU on a repeatable workload. No performance claim without measurements.

## Repository layout (planned)

`cmd/server`, `cmd/agent`, `internal/`, `web/`, `docs/`, `deploy/`.

## Open design decisions

Transport and codecs, identity/key lifecycle, signed updater, dependency versions, Windows service/session model, Wayland/X11 support, and exact resource limits require implementation review. Do not publish a production release until acceptance and security gates pass.
