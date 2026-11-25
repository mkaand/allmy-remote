import { useEffect, useState } from "react";

type Theme = "system" | "light" | "dark";
const storageKey = "allmy.theme";

function getTheme(): Theme {
  try {
    const value = localStorage.getItem(storageKey);
    if (value === "light" || value === "dark") return value;
  } catch { /* Storage may be disabled. */ }
  return "system";
}

// Standalone reusable implementation; no private shared source dependency.
export function AllMyThemeControls() {
  const [theme, setTheme] = useState<Theme>(getTheme);
  useEffect(() => {
    document.documentElement.dataset.theme = theme;
    try { localStorage.setItem(storageKey, theme); } catch { /* Optional preference. */ }
  }, [theme]);

  return <fieldset className="theme-controls">
    <legend className="sr-only">Appearance</legend>
    {(["system", "light", "dark"] as const).map(value =>
      <button key={value} type="button" aria-pressed={theme === value}
        onClick={() => setTheme(value)}>{value[0].toUpperCase() + value.slice(1)}</button>
    )}
  </fieldset>;
}
