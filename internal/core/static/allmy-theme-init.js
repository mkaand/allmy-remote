(() => {
  const key = "allmy-theme";
  const legacyKey = "allmy.theme";
  const valid = new Set(["system", "light", "dark"]);
  let mode = "system";
  try {
    const stored = window.localStorage.getItem(key);
    if (valid.has(stored)) {
      mode = stored;
    } else {
      if (stored !== null) window.localStorage.removeItem(key);
      const legacy = window.localStorage.getItem(legacyKey);
      if (valid.has(legacy)) {
        mode = legacy;
        if (mode === "system") window.localStorage.removeItem(key);
        else window.localStorage.setItem(key, mode);
        window.localStorage.removeItem(legacyKey);
      } else if (legacy !== null) {
        window.localStorage.removeItem(legacyKey);
      }
    }
  } catch {
    // Inaccessible storage uses the system preference.
  }
  let prefersDark = false;
  try {
    prefersDark = Boolean(window.matchMedia?.("(prefers-color-scheme: dark)").matches);
  } catch {}
  const resolved = mode === "system" ? (prefersDark ? "dark" : "light") : mode;
  document.documentElement.dataset.theme = resolved;
  document.documentElement.dataset.themeMode = mode;
  const meta = document.querySelector('meta[name="theme-color"]');
  if (meta) meta.content = resolved === "dark" ? "#10151e" : "#f1f5f9";
})();
