# Threat model — initial design

Protect the operator identity, enrolled devices, remote session confidentiality/integrity, clipboard, files, local secrets and update supply chain.

| Threat | Required mitigation |
| --- | --- |
| Unauthorized enrollment | Single-use short-lived grant, server-side hash, atomic consumption, rate limit, audit, local confirmation |
| Stolen operator session | TLS, secure cookie, CSRF/origin validation, expiry, revocation, MFA before production |
| Stolen device credential | Unique revocable credential, protected OS storage, no credentials in logs or URLs; no cross-device privilege |
| Confused deputy or device spoofing | Authenticate transport identity, bind actions to authorized session and device, validate each operation |
| Malicious relay or interception | TLS; reviewed E2EE and key lifecycle before production media/clipboard; do not advertise E2EE prematurely |
| Unexpected remote control | Visible local consent, explicit capabilities, local stop, disconnect cleanup; no unattended access by default |
| Clipboard disclosure | Local opt-in, per-device controls, no payload logging/persistence, bounded expiry, pause, restricted managed devices |
| Resource exhaustion | Message/connection/rate/queue bounds, timeouts, bounded logs, frame dropping and input validation |
| Compromised update | Signed manifests and binaries, pinned trusted verification key, downgrade protection, atomic rollback before updater rollout |
| Public repository leakage | Generic examples only, secret scan staged changes and CI logs, no deployment config, private credentials or device identifiers |

Trust boundaries: browser/operator to core; enrolled device to core; core to storage; release pipeline to installed agent. Local OS and browser permission constraints are mandatory. Assume authorized operators can make harmful changes; high-impact capabilities require separate authorization and audit. Corporate use requires organizational approval regardless of local admin privileges.

Sprint 1 excludes screen capture, input injection, remote shell, file transfer, real clipboard payloads, unattended access and automatic updates. Production gate: review auth, consent, revocation, transport confidentiality, security tests, real-device acceptance and deployment isolation. Critical/high unresolved issues block production release.

Live telemetry is untrusted self-reported device data, not attestation. It is bounded to utilization percentages and a validated local source address, negotiated only on an authenticated Windows channel, and held in active connection memory. Operator-only same-origin reveal excludes raw IP from registry listings; no payload logging or persistence. Forwarded client IP headers are not accepted without a verified deployment trust chain. Presence and telemetry do not authorize remote commands.
