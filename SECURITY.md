# Security policy

AllMy Remote is a planned remote administration platform for devices you own or are authorized to manage. No production-ready release exists.

## Required security properties

- Explicit enrollment with short-lived one-time tokens, unique device identity, revocation, and authenticated sessions.
- Least privilege, separate operator and device credentials, capability-based authorization, visible session status, and local disable/unpair controls.
- HTTPS/TLS, secure transport and a reviewed end-to-end encryption design before production media or clipboard use; never claim relay-blind encryption before it is implemented and verified.
- Do not bypass operating system security boundaries or organizational controls. Managed devices require organizational authorization.
- Clipboard synchronization is opt-in by device and data type; no clipboard contents, file contents, tokens, passwords, or keystrokes in audit logs. Password-manager exclusion where supported; do not claim perfect secret detection.
- Signed updates, artifact integrity verification, rollback, and fail-closed behavior before automatic update rollout.
- Strict message limits, rate limits, safe file paths, and dependency review. Audit session initiation, termination, enrollment, revocation, and privileged operations without sensitive payloads.
- Never commit real infrastructure identifiers, credentials, private configuration, or personal contact details. Use generic examples.

Report potential vulnerabilities privately through GitHub's private vulnerability reporting feature when enabled. Do not post secrets or exploitable details in public issues.
