// Where the backend lives. Empty means "this computer" (the Go code inside
// the app, reached through Wails). A URL means a shared GoThermo server.

const KEY = 'gothermo.serverUrl';

let serverUrl = '';
try {
  serverUrl = localStorage.getItem(KEY) || '';
} catch {
  /* storage can be unavailable; fall back to local mode */
}

// Some servers only let people register who know an invite code.
const INVITE_KEY = 'gothermo.inviteCode';
let inviteCode = '';
try {
  inviteCode = localStorage.getItem(INVITE_KEY) || '';
} catch {
  /* ignore */
}

export const getInviteCode = () => inviteCode;

export function setInviteCode(code: string) {
  inviteCode = code.trim();
  try {
    if (inviteCode) localStorage.setItem(INVITE_KEY, inviteCode);
    else localStorage.removeItem(INVITE_KEY);
  } catch {
    /* ignore */
  }
}

export const getServerUrl = () => serverUrl;
export const isRemote = () => serverUrl !== '';

export function setServerUrl(url: string) {
  serverUrl = url;
  try {
    if (url) localStorage.setItem(KEY, url);
    else localStorage.removeItem(KEY);
  } catch {
    /* ignore */
  }
}

export type ParsedServer = { url: string; error?: undefined } | { url?: undefined; error: string };

const isLocalHost = (h: string) =>
  h === 'localhost' || h.endsWith('.local') || /^\d{1,3}(\.\d{1,3}){3}$/.test(h) || h.startsWith('[');

// Turns what a person types ("chat.example.com", "192.168.1.5:8081") into a
// clean base URL. Public names default to https, local addresses to http.
export function parseServerInput(input: string): ParsedServer {
  const raw = input.trim();
  if (!raw) return { url: '' };

  let withScheme = raw;
  if (!/^[a-z][a-z0-9+.-]*:\/\//i.test(raw)) {
    const host = raw.split('/')[0].replace(/:\d+$/, '');
    withScheme = (isLocalHost(host) ? 'http://' : 'https://') + raw;
  }

  let u: URL;
  try {
    u = new URL(withScheme);
  } catch {
    return { error: 'That does not look like a server address.' };
  }
  if (u.protocol !== 'http:' && u.protocol !== 'https:') {
    return { error: 'Use an http:// or https:// address.' };
  }
  if (!u.hostname) return { error: 'That does not look like a server address.' };
  return { url: `${u.protocol}//${u.host}` };
}

// http -> ws, https -> wss, plus the path the backend listens on.
export function wsUrl(token: string): string {
  if (!serverUrl) {
    return `ws://127.0.0.1:8081/ws?token=${encodeURIComponent(token)}`;
  }
  return `${serverUrl.replace(/^http/, 'ws')}/ws?token=${encodeURIComponent(token)}`;
}

// Quick check that an address really is a GoThermo server.
export async function pingServer(url: string, timeoutMs = 6000): Promise<string | null> {
  const ctl = new AbortController();
  const timer = setTimeout(() => ctl.abort(), timeoutMs);
  try {
    const res = await fetch(`${url}/healthz`, { signal: ctl.signal });
    const body = await res.json();
    if (body && body.app === 'gothermo') return null;
    return 'That address answered, but it is not a GoThermo server.';
  } catch {
    return 'Cannot reach the server. Check the address and that it is running.';
  } finally {
    clearTimeout(timer);
  }
}
