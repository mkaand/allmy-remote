import { useCallback, useEffect, useRef, useState, type FormEvent } from "react";
import { createRoot } from "react-dom/client";
import { api, APIError, message, type Device, type Grant } from "./api";
import { AllMyThemeControls } from "./AllMyThemeControls";
import { useThemeAdapter } from "./theme";

type SessionState = "checking" | "signed-out" | "signed-in" | "unavailable";

function Login({ onLogin }: {onLogin: () => void}) {
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (busy) return;
    setBusy(true);
    setError("");
    try {
      await api.login(password);
      setPassword("");
      onLogin();
    } catch (error) {
      setPassword("");
      setError(message(error));
    } finally {
      setBusy(false);
    }
  }
  return <section className="panel login" aria-labelledby="login-title">
    <p className="eyebrow">OPERATOR ACCESS</p>
    <h2 id="login-title">Sign in</h2>
    <p>Use the administrator account initialized on this server.</p>
    <form onSubmit={submit}>
      <label htmlFor="username">Username</label>
      <input id="username" autoComplete="username" value="admin" readOnly />
      <label htmlFor="password">Password</label>
      <input id="password" type="password" autoComplete="current-password" required
        maxLength={256} value={password} onChange={e => setPassword(e.target.value)} />
      {error && <p className="error" role="alert">{error}</p>}
      <button className="primary" type="submit" disabled={busy}>{busy ? "Signing in…" : "Sign in"}</button>
    </form>
  </section>;
}

function DeviceCard({ device, onRevoke, busy }: {device: Device; onRevoke: (id: string) => void; busy: boolean}) {
  const [confirm, setConfirm] = useState(false);
  const lastSeen = device.last_seen === null ? "Never" : new Date(device.last_seen * 1000).toLocaleString();
  return <article className="panel device" aria-label={device.name}>
    <div className="device-head">
      <div className="device-icon" aria-hidden="true">▣</div>
      <div className="device-name"><h3>{device.name}</h3><p>Mock device · Preview</p></div>
      <span className={"status " + device.status}><span aria-hidden="true" className="dot" />{device.status}</span>
    </div>
    <p className="last-seen">Last seen: {lastSeen}</p>
    <dl className="metrics">
      <div><dt>CPU</dt><dd>—</dd></div><div><dt>RAM</dt><dd>—</dd></div><div><dt>Disk</dt><dd>—</dd></div>
    </dl>
    <p className="small">Telemetry and remote access are not available in this preview.</p>
    <div className="actions">
      <button disabled title="Planned">Desktop</button><button disabled title="Planned">Files</button>
      <button disabled title="Planned">Terminal</button>
    </div>
    {device.status !== "revoked" && (confirm ?
      <div className="confirm"><p>Revoke this device and disconnect it?</p>
        <button className="danger" disabled={busy} onClick={() => { setConfirm(false); onRevoke(device.id); }}>Confirm revoke</button>
        <button disabled={busy} onClick={() => setConfirm(false)}>Cancel</button></div> :
      <button className="quiet" disabled={busy} onClick={() => setConfirm(true)}>Revoke device</button>)}
  </article>;
}

function Dashboard({ onUnauthorized }: {onUnauthorized: () => void}) {
  const [devices, setDevices] = useState<Device[] | null>(null);
  const [error, setError] = useState("");
  const [name, setName] = useState("");
  const [grant, setGrant] = useState<Grant | null>(null);
  const [copyStatus, setCopyStatus] = useState("");
  const [busy, setBusy] = useState(false);
  const [seconds, setSeconds] = useState(0);
  const abort = useRef<AbortController | null>(null);
  const alive = useRef(false);

  const handleError = useCallback((err: unknown) => {
    if (!alive.current || (err instanceof DOMException && err.name === "AbortError")) return;
    if (err instanceof APIError && err.status === 401) {
      onUnauthorized();
      return;
    }
    setError(message(err));
  }, [onUnauthorized]);

  const refresh = useCallback(async () => {
    if (!alive.current) return;
    abort.current?.abort();
    const controller = new AbortController();
    abort.current = controller;
    try {
      const result = await api.devices(controller.signal);
      if (alive.current && !controller.signal.aborted) {
        setDevices(result);
        setError("");
      }
    } catch (err) {
      if (!controller.signal.aborted) handleError(err);
    } finally {
      if (abort.current === controller) abort.current = null;
    }
  }, [handleError]);

  useEffect(() => {
    alive.current = true;
    void refresh();
    const tick = window.setInterval(() => {
      if (document.visibilityState === "visible" && !abort.current) void refresh();
    }, 10000);
    const visible = () => {
      if (document.visibilityState === "visible") void refresh();
    };
    document.addEventListener("visibilitychange", visible);
    return () => {
      alive.current = false;
      window.clearInterval(tick);
      document.removeEventListener("visibilitychange", visible);
      abort.current?.abort();
    };
  }, [refresh]);

  useEffect(() => {
    if (!grant) return;
    const update = () => {
      const left = Math.max(0, Math.ceil(grant.expires_at - Date.now() / 1000));
      setSeconds(left);
      if (!left) { setGrant(null); setCopyStatus(""); }
    };
    update();
    const timer = window.setInterval(update, 1000);
    return () => window.clearInterval(timer);
  }, [grant]);

  async function enroll(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (busy) return;
    setBusy(true);
    setError("");
    setGrant(null);
    setCopyStatus("");
    try {
      const value = await api.grant(name.trim());
      if (alive.current) {
        setSeconds(Math.max(0, Math.ceil(value.expires_at - Date.now() / 1000)));
        setGrant(value);
        setName("");
      }
    } catch (err) { handleError(err); }
    finally { if (alive.current) setBusy(false); }
  }

  async function revoke(id: string) {
    if (busy) return;
    setBusy(true);
    try {
      await api.revoke(id);
      await refresh();
    } catch (err) { handleError(err); }
    finally { if (alive.current) setBusy(false); }
  }

  async function copy() {
    if (!grant || grant.expires_at <= Date.now() / 1000) return;
    try {
      await navigator.clipboard.writeText(grant.grant);
      setCopyStatus("Copied.");
    } catch { setCopyStatus("Select the code and copy it manually."); }
  }

  const enrolled = devices?.filter(d => d.status !== "revoked");
  const online = enrolled?.filter(d => d.status === "online").length;
  return <>
    <section className="summary" aria-label="Overview">
      <article className="panel"><h2>Enrolled devices</h2><p className="value">{enrolled?.length ?? "—"}</p><p>Authorized registry</p></article>
      <article className="panel"><h2>Online</h2><p className="value">{online ?? "—"}</p><p>Connected mock devices</p></article>
      <article className="panel"><h2>Active sessions</h2><p className="value">—</p><p>Remote desktop pending</p></article>
    </section>
    <section className="panel enrollment" aria-labelledby="enroll-title">
      <div><h2 id="enroll-title">Enroll a device</h2><p>Generate a single-use grant for a mock client. Valid for five minutes.</p></div>
      <form onSubmit={enroll}>
        <label htmlFor="device-name">Device name</label>
        <div className="form-row"><input id="device-name" required maxLength={96} value={name}
          onChange={e => setName(e.target.value)} placeholder="My test device" />
          <button className="primary" type="submit" disabled={busy || !name.trim()}>Generate grant</button></div>
      </form>
      {grant && seconds > 0 && <div className="grant">
        <label htmlFor="grant-code">Enrollment grant</label>
        <div className="form-row"><input id="grant-code" readOnly spellCheck={false} autoComplete="off"
          value={grant.grant} onFocus={e => e.target.select()} /><button onClick={() => void copy()}>Copy</button></div>
        <p className="small">Expires in {seconds}s. Share only with your authorized test client.</p>
        <p className="small" role="status">{copyStatus}</p>
        <button className="quiet" onClick={() => { setGrant(null); setCopyStatus(""); }}>Hide grant</button>
      </div>}
    </section>
    <section aria-labelledby="devices-title">
      <div className="section-head"><div><h2 id="devices-title">Devices</h2><p>Live registry · refreshes every 10 seconds while visible</p></div>
        <button onClick={() => void refresh()}>Refresh</button></div>
      {error && <p className="error panel" role="alert">{error}{devices !== null ? " Displayed data may be stale." : ""}</p>}
      {devices === null ? <p role="status">Loading devices…</p> : devices.length === 0 ?
        <div className="panel empty"><h3>No devices enrolled</h3><p>Generate a grant to connect an authorized mock client.</p></div> :
        <div className="device-grid">{devices.map(d => <DeviceCard key={d.id} device={d} onRevoke={revoke} busy={busy} />)}</div>}
    </section>
    <section className="panel clipboard"><h2>Universal clipboard</h2><p>Planned · Not enabled. No clipboard data is collected.</p></section>
  </>;
}

function App() {
  const theme = useThemeAdapter();
  const [state, setState] = useState<SessionState>("checking");
  const [logoutError, setLogoutError] = useState("");
  const [loggingOut, setLoggingOut] = useState(false);
  const unauthorized = useCallback(() => setState("signed-out"), []);
  const signedIn = useCallback(() => setState("signed-in"), []);
  const checkSession = useCallback(async (signal?: AbortSignal) => {
    setState("checking");
    try {
      await api.session(signal);
      if (!signal?.aborted) setState("signed-in");
    } catch (err) {
      if (signal?.aborted) return;
      setState(err instanceof APIError && err.status === 401 ? "signed-out" : "unavailable");
    }
  }, []);
  useEffect(() => {
    const controller = new AbortController();
    void checkSession(controller.signal);
    return () => controller.abort();
  }, [checkSession]);
  async function logout() {
    setLoggingOut(true);
    setLogoutError("");
    try {
      await api.logout();
      setState("signed-out");
    } catch (err) {
      if (err instanceof APIError && err.status === 401) setState("signed-out");
      else setLogoutError(message(err));
    } finally { setLoggingOut(false); }
  }
  return <main className="shell">
    <header className="topbar"><div className="brand"><span className="mark" aria-hidden="true">A</span><div>
      <p className="eyebrow">ALLMY REMOTE</p><p className="small">One platform. All your devices.</p>
    </div></div><div className="header-actions"><AllMyThemeControls adapter={theme} />
      {state === "signed-in" && <button onClick={() => void logout()} disabled={loggingOut}>Sign out</button>}</div></header>
    <div className="preview" role="note"><strong>Development preview</strong><span>Local testing only · Mock devices · Not production-ready</span></div>
    <div className="page-title"><h1>{state === "signed-in" ? "Dashboard" : "Welcome to AllMy Remote"}</h1>
      <p>{state === "signed-in" ? "Manage your authorized devices in one place." : "Secure operator access to your development workspace."}</p></div>
    {logoutError && state === "signed-in" && <p role="alert" className="error">{logoutError}</p>}
    {state === "checking" && <p role="status">Checking session…</p>}
    {state === "unavailable" && <section className="panel"><p role="alert">Server unavailable.</p><button onClick={() => void checkSession()}>Retry</button></section>}
    {state === "signed-out" && <Login onLogin={signedIn} />}
    {state === "signed-in" && <Dashboard onUnauthorized={unauthorized} />}
    <footer>AllMy Remote · Sprint 1E · Development preview</footer>
  </main>;
}

const root = document.getElementById("root");
if (!root) throw new Error("Missing application root");
createRoot(root).render(<App />);
