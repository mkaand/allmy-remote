# Isolated development preview

This is not a production deployment. It has one administrator, one active operator session and mock devices only. Do not expose its HTTP port through a public proxy, public DNS route, or public firewall rule. No native agent, remote desktop, file management, terminal, clipboard or MFA is available yet.

## Prerequisites and isolation

- Use a dedicated project checkout and Compose project. Confirm the source commit and disk capacity before building. Build only this project, never prune unrelated Docker images, volumes or containers.
- Docker Engine and Compose v2 on Linux. Use the included compose.preview.yml. Its host binding is fixed to 127.0.0.1; ALLMY_PREVIEW_PORT selects only the host port (default 18080). Check for a port conflict before starting.
- The volume is mounted at /data; SQLite lives in the private child /data/state (0700, UID 65532). The child is seeded from the image on first use. The application continues to reject unsafe data-directory and database permissions. Do not chmod the volume root recursively. This is a fresh-deployment layout; if an earlier experimental volume contains /data/core.sqlite, stop and back it up before a separately reviewed migration. Never initialize a new empty database over existing data.
- Separate persistent named volume, non-root UID 65532, read-only root filesystem, all Linux capabilities dropped, no-new-privileges, bounded logs. Do not mount the Docker socket, host root, or other project directories.
- The preview has a 256 MiB container ceiling and 64 PID limit; these are safety caps, not measured consumption. Keep configuration and credentials outside Git.
- Access from the server itself or an authorized SSH local-forwarding tunnel to localhost using the same local port. Use the exact configured origin. A localhost browser may accept Secure cookies over loopback HTTP; if the browser rejects them, use separately configured private trusted HTTPS, not insecure cookies or public exposure. Real Safari/iOS acceptance remains pending.
- Use a strong unique password from a password manager. Initialize it only once with the init-admin command using protected stdin, before starting the service; no default password, no command-line password, no password in a file or transcript. For example from a trusted Bash terminal, read without echo, pass through printf, and unset the variable immediately. Do not enable shell tracing. Run the initializer against the same named volume.
- Start with docker compose -f compose.preview.yml build, then initialize the administrator using docker compose -f compose.preview.yml run --rm -T --interactive --no-deps server init-admin with the protected input described above. Start with docker compose -f compose.preview.yml up -d --no-build. Check /health and /ready from loopback. Do not reuse the CI fixture or its credentials.
- Use docker compose -f compose.preview.yml stop for shutdown. Do not use down -v for normal maintenance; it deletes persisted data. The container is disposable, the volume is not.
- For a backup, stop the preview first, then copy the entire private volume (including SQLite WAL/SHM if present) to a protected location. Verify restore on a separate volume before deleting the only usable backup or changing schema. Do not back up other projects or store backups in Git.

## Manual acceptance after isolated deployment

1. Access from the approved local path, sign in with the initialized administrator. Expected: dashboard, empty or actual mock-only registry.
2. Toggle the AllMy light/dark switch and Auto; reload. Expected: manual choice persists and Auto follows the system.
3. Verify mobile-width layout without horizontal overflow; real iPhone Safari is a separate acceptance step when a private access method is available.
4. Generate a mock enrollment grant; expected: single-use secret and five-minute expiration. Only pass to an authorized test client. Do not paste secrets in issues or chat transcripts.
5. Verify online/offline and revoke with an authorized mock client if one is available; CI already tests these paths. Native Windows acceptance is Sprint 2.
6. Sign out; expected: login screen and protected API returns unauthorized.
7. Verify service restart, volume persistence, no public bind, and measured idle CPU/RAM on the actual deployment. Report measurements, not assumptions.

## Gates

Automated CI includes Go race tests, dependency checks, browser acceptance and a disposable Compose persistence fixture. CI success is not equivalent to server deployment or manual acceptance. Production requires further security review, MFA, trusted TLS and reviewed confidentiality/consent designs before enabling remote access.
