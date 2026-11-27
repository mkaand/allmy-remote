// Small local vector icons; user-selected presentation, never platform attestation.
export const iconOptions = [
  ["auto", "Automatic"], ["desktop", "Desktop"], ["laptop", "Laptop"],
  ["macbook", "MacBook"], ["mac", "Mac desktop"], ["windows", "Windows"],
  ["linux", "Linux"], ["mini-pc", "Mini PC"], ["server", "Server"],
  ["raspberry-pi", "Raspberry Pi"], ["phone", "Phone"], ["tablet", "Tablet"],
] as const;

export function DeviceIcon({ value, platform }: {value: string; platform: string}) {
  const kind = value === "auto" ? (platform === "windows" ? "windows" : "desktop") : value;
  const monitor = <><rect x="3" y="3" width="18" height="13" rx="2"/><path d="M12 16v5m-5 0h10"/></>;
  const paths = kind === "windows" ? <><path d="M3 5l8-1v7H3zM13 4l8-1v8h-8zM3 13h8v7l-8-1zM13 13h8v8l-8-1z" fill="currentColor" stroke="none"/></> :
    kind === "laptop" || kind === "macbook" ? <><rect x="4" y="4" width="16" height="13" rx="2"/><path d="M4 17l-2 3h20l-2-3"/>{kind === "macbook" && <path d="M10 17h4"/>}</> :
    kind === "mac" ? <><rect x="3" y="3" width="18" height="14" rx="2"/><path d="M3 14h18m-9 3v4m-4 0h8"/></> :
    kind === "linux" ? <><path d="M7 11c0-5 2-8 5-8s5 3 5 8c3 4 3 7 0 9H7c-3-2-3-5 0-9z"/><path d="M9 11l3 2 3-2M8 20l-3 1m11-1 3 1"/><circle cx="10" cy="8" r=".5"/><circle cx="14" cy="8" r=".5"/></> :
    kind === "mini-pc" ? <><rect x="3" y="7" width="18" height="11" rx="3"/><path d="M6 11h7m-7 3h3m7 0h2"/></> :
    kind === "server" ? <><rect x="3" y="3" width="18" height="8" rx="2"/><rect x="3" y="13" width="18" height="8" rx="2"/><path d="M6 7h1m3 0h1m-5 10h1m3 0h1"/></> :
    kind === "raspberry-pi" ? <><rect x="3" y="5" width="18" height="14" rx="2"/><rect x="8" y="9" width="8" height="6" rx="1"/><path d="M6 5V3m4 2V3m4 2V3m4 2V3M6 19v2m4-2v2m4-2v2m4-2v2"/></> :
    kind === "phone" ? <><rect x="7" y="2" width="10" height="20" rx="3"/><path d="M10 5h4m-3 14h2"/></> :
    kind === "tablet" ? <><rect x="4" y="2" width="16" height="20" rx="2"/><path d="M11 19h2"/></> : monitor;
  return <svg aria-hidden="true" focusable="false" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" strokeLinejoin="round" data-device-icon={kind}>{paths}</svg>;
}
