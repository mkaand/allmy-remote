# Roadmap

| Sprint | Deliverable | Status |
| --- | --- | --- |
| 0 | Architecture, threat model, technical specification, repository bootstrap | Design baseline complete |
| 1 | Core, web dashboard skeleton, authenticated device enrollment | In progress (1D interactive dashboard) |
| 2 | Windows agent and telemetry | Planned |
| 3 | Windows view-only desktop, then authorized input | Planned |
| 4 | Universal clipboard: text first, then richer formats | Planned |
| 5 | Zorin OS native agent | Planned |
| 6 | File transfer and file manager | Planned |
| 7 | Terminal and device monitoring | Planned |
| 8 | ARMv6 lightweight agent | Planned |
| 9 | Mobile Safari and PWA | Planned |
| 10 | Signed updates and rollback | Planned |
| 11 | Security and performance hardening | Planned |
| 12 | Authorized managed-device integration | Deferred |

## Acceptance policy

Use scope-specific automated tests, code review, and manual device acceptance. Every update documents what changed, what to test, and expected results. Do not repeat unrelated tests unless regression risk warrants it. Do not describe a planned feature as implemented. Update README whenever user-visible features change.

Initial test target is a single Windows 11 x64 host; additional platforms must not block the initial vertical slice. Production deployments must wait for security and acceptance gates.
