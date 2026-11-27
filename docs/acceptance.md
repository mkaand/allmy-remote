# Acceptance and performance criteria

All numbers are initial engineering budgets, not achieved benchmarks. Report hardware, OS, workload, duration, versions and measurement method. Use the first Windows host as the real-device baseline. Mock devices are labeled and never substitute for real hardware tests.

| Area | Proposed target or check |
| --- | --- |
| Core idle | <1% of one logical CPU, <100 MiB process RSS excluding page cache; measure rather than assert |
| Windows agent idle | <1% of one logical CPU and <50 MiB working set |
| Linux x64 / ARMv6 idle | <1% CPU, <30 / <15 MiB RSS respectively; defer until hardware exists |
| Idle screen pipeline | Zero capture/encode activity and zero media traffic |
| First desktop profile | 720p, 10 FPS target when content changes; record latency, FPS, bandwidth and CPU |
| Active desktop usability | Record p50/p95 input-to-visible-response latency and frame age under repeatable workload; set final threshold after baseline |
| Recovery | Disconnect, revoke or local stop releases input and resources; stale heartbeat transitions offline |
| Clipboard | No duplicate loop, no stale replay, bidirectional Unicode text; no payload logs; pause and revocation enforced |
| Resource bounds | Slow consumers, invalid messages and oversized payloads cannot create unbounded queues |
| Privacy | No private deployment data in diff, Git metadata (use GitHub noreply), workflow output or public assets |

Measure idle for at least 5 minutes after warmup, CPU as process CPU seconds / wall seconds * 100 (one logical core = 100%), memory as working set on Windows or RSS on Linux; report mean and peak. Test active desktop with idle screen, scrolling and window movement; measure on both local and remote path when available. No server transcoding by default. Adjust budgets only with documented measurements.

## Sprint 1 exit criteria

- Reproducible build, scoped automated checks, clean shutdown and health/readiness.
- Authentication and unauthorized-request rejection verified; expired/used enrollment rejected; revoked device cannot reconnect.
- Mock agent enrollment, heartbeat, offline transition and dashboard rendering pass.
- README lists actual implementation and limitations. No installer or production claim.
- Manual acceptance: operator signs in, sees clearly labeled mock device, verifies online/offline and revoke, checks mobile/desktop theme. Real Windows agent acceptance follows in Sprint 2.

Each change gets focused tests and a concise manual checklist when there is a user-facing feature. Avoid repetitive unrelated testing. A public stable release requires actual device acceptance and security gates.

## Isolated deployment candidate

Automated gates include a persistent Compose fixture (container removal/recreation, admin login, revoked-device state, loopback binding and runtime restrictions), and Chromium/WebKit browser checks via test-only loopback HTTPS. WebKit automation is not an actual iPhone Safari test. Follow [manual deployment acceptance](deployment-preview.md) after deploying to an approved isolated host. Only mark Sprint 1 accepted after these manual checks and recorded resource measurements; production remains blocked.

## Sprint 2B first hardware acceptance

Before upgrading, stop the preview and back up its private SQLite volume, verify restoration, and retain the previous image. Schema 3 preserves existing devices as mock and stores the declared Windows platform. A downgrade after migration requires restoring the pre-migration backup. Check a Windows x64 CI build and DPAPI round trip, then pair a single authorized NUC via hidden grant input, start the foreground agent, verify Windows/online and manual Refresh, wait for at least two heartbeat intervals, stop and verify offline, restart and verify online without re-pairing, revoke and verify reconnection is rejected, then unpair locally. Measure process CPU and working set for five minutes after warmup. Confirm no service, startup, elevation or remote-control capability. Cloudflare/browser access policy is separate from agent credentials. Do not weaken it to make a test pass.

## Sprint 2C1 acceptance

Verify transparent branding in light, dark and automatic themes on desktop and mobile. Delete only an explicitly revoked test entry after confirmation; cancel preserves the record. Active and offline authorized devices must reject deletion. Verify the deleted credential still cannot connect, the registry slot is freed, and an unrelated online device remains connected. No database migration is required for this increment.

## Device customization acceptance

Back up schema 3 before migration to schema 4. Verify existing IDs, names, platforms, credentials and revoked state are retained with Automatic icon default. Rename an online device and select Mini PC or another icon; verify live connection remains authorized, refresh and reload retain both values, cancel does not modify metadata, and revoked or unauthorized requests cannot edit. Test persistence after restart and responsive keyboard-accessible icon selection.


## Sprint 2C native Windows GUI acceptance

Stop the old console process without unpairing. Extract the two unsigned preview binaries to a stable user-owned directory; launch only the GUI. Verify existing pairing, server origin and online status without issuing a new grant. Verify the embedded icon in Explorer, window and notification area; close and minimize hide to tray, open restores, Disconnect goes offline, Connect reconnects, Exit terminates and releases the local lock. The console binary must continue working after GUI Exit. Verify startup is initially disabled, opt-in prompts for confirmation, the current user's Run entry is quoted, disabling removes it, and a normal sign-out/sign-in starts the GUI in the tray only when enabled. Do not sign out during an active remote management session. Verify pairing and unpair with a separate test record only; unpair disables startup, and dashboard revoke remains required. Confirm denial after revoke, reconnect after network disruption, no elevation or service. Measure idle CPU and working set for five minutes. CI checks the resource loader and startup command construction; it does not replace interactive Windows tests. No schema or server deployment change in this increment.
