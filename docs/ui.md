# UI guidelines

Design target: clear, compact, responsive, accessible dashboard. No real device data in example screenshots.

- Theme modes: system, light, dark. Central CSS variables for surfaces, text, borders, accents and semantic states. Use native system font stack initially; avoid external font downloads.
- Dark reference palette: background #10151e, card #1b2432, border #334155, text #e2e8f0, muted #94a3b8, blue accent #60a5fa, green online #4ade80. Light palette uses equivalent accessible contrast, verified at implementation.
- Header: product, navigation, theme switch and settings. Summary: enrolled devices, online, active sessions. Device cards: name, platform, status, freshness, capabilities, CPU/RAM/disk if supported; unavailable values show a dash, not zero. Status includes text and icon, not color alone.
- Actions: Desktop, Files, Terminal, More only when implemented and authorized; unavailable capabilities show a clear reason. Explicitly distinguish mock devices. No fake telemetry.
- Clipboard: global state, per-device opt-in, supported content types, pause, security/availability status. Never display sensitive clipboard content by default.
- Responsive grid, consistent spacing and corner radius, keyboard navigation, visible focus, appropriate touch targets, reduced-motion support, readable contrast.
- Keep UI lightweight; no large animation libraries or unnecessary polling. Prefer event updates with bounded refresh. The first dashboard should be functional before ornamental.
- English UI first; externalize strings for later localization. Theme controls use a reusable interface without publishing private shared-source paths or dependencies.

## Sprint 1D implementation

React/TypeScript, bundled with esbuild and embedded in the Go binary. AllMy Theme Controls reuses the existing React component and CSS from Changd Fusion (source commit 2a2cf2b0b590b21866d85c9d6bec8870318922b3); the application adapter omits Ant Design, uses local palette metadata, and migrates the previous preview key. The canonical preference key is allmy-theme. Prepaint initialization uses an external script under the existing CSP. Auto follows OS changes; explicit modes override them; storage events synchronize tabs. Poll every 10 seconds only while visible, refresh on demand, label stale data after errors, never store session or enrollment secrets in local storage. No remote-control actions until implemented.

## Sprint 2A branding

Use the approved multi-device logo in the shared login/dashboard header and the matching app icon for browser tabs and Apple touch icons. Both optimized PNGs are embedded in the existing Go binary, with same-origin image loading under CSP. The logo has a light neutral backing for legibility in dark mode; the header wraps on small screens. No external image requests, new runtime dependencies or changes to the canonical theme controls. The app icon is also the README mark and the basis for later native-agent icons.

## Sprint 2C1 polish

Light and dark variants share the same logo geometry and transparent background; only the dark palette is adjusted for contrast. Theme resolution chooses the asset. Revoked device cards omit unavailable remote actions and expose a confirmed permanent delete action. Online and offline authorized devices must be revoked before deletion.

Device metadata editor: explicitly save a display name and choose from local vector icon presets. Default Automatic uses the existing declared platform. The selected visual icon is not a claim of platform support. Edits are server-persisted and do not change device credentials or capabilities. Revoked devices cannot be edited.

Native Windows GUI preview: standard Win32 controls and notification area, fixed system-DPI-scaled layout. The GUI shows server origin and live presence status without exposing credentials. Dashboard display names and hardware telemetry are not yet part of the agent protocol. Explicit Exit differs from minimize/close-to-tray. Use the operating system's native control appearance; custom GUI dark mode is not part of this increment.
