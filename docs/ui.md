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

React/TypeScript, bundled with esbuild and embedded in the Go binary. Standalone AllMyThemeControls component with persisted appearance preference; private shared-source parity remains to be checked. Poll every 10 seconds only while visible, refresh on demand, label stale data after errors, never store session or enrollment secrets in local storage. No remote-control actions until implemented.
