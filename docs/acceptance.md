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
