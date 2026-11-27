export type Device = {
  id: string;
  name: string;
  platform: "mock" | "windows";
  icon: string;
  status: "online" | "offline" | "revoked";
  last_seen: number | null;
  metrics?: {cpu_percent?: number; ram_percent?: number; disk_percent?: number};
  internal_ip_available?: boolean;
  external_ip_available?: boolean;
};

export type Grant = { grant: string; expires_at: number };

export class APIError extends Error {
  constructor(readonly status: number) {
    super(status === 401 ? "Sign in required or invalid credentials." :
      status === 429 ? "Too many attempts. Try again shortly." :
      status === 409 ? "Request conflicts with the current limit." :
      status === 403 ? "Request not permitted." : "Request failed. Try again.");
  }
}

async function request<T>(path: string, method = "GET", body?: unknown, signal?: AbortSignal): Promise<T> {
  const response = await fetch("/api/v1" + path, {
    method,
    credentials: "same-origin",
    cache: "no-store",
    headers: body === undefined ? {} : { "Content-Type": "application/json" },
    body: body === undefined ? undefined : JSON.stringify(body),
    signal,
  });
  if (!response.ok) throw new APIError(response.status);
  return await response.json() as T;
}

export const api = {
  session: (signal?: AbortSignal) => request<{status: string}>("/session", "GET", undefined, signal),
  login: (password: string) => request<{status: string}>("/session/login", "POST", {username: "admin", password}),
  logout: () => request<{status: string}>("/session/logout", "POST"),
  devices: (signal?: AbortSignal) => request<Device[]>("/devices", "GET", undefined, signal),
  revealIP: (id: string, kind: "internal" | "external", signal?: AbortSignal) => request<{ip: string}>("/devices/" + encodeURIComponent(id) + "/ip", "POST", {kind}, signal),
  grant: (name: string) => request<Grant>("/enrollment/grants", "POST", {name}),
  revoke: (id: string) => request<{status: string}>("/devices/" + encodeURIComponent(id) + "/revoke", "POST"),
  editDevice: (id: string, name: string, icon: string) => request<{status: string}>("/devices/" + encodeURIComponent(id), "PATCH", {name, icon}),
  deleteDevice: (id: string) => request<{status: string}>("/devices/" + encodeURIComponent(id), "DELETE"),
};

export function message(error: unknown): string {
  return error instanceof APIError ? error.message : "Connection unavailable. Try again.";
}
