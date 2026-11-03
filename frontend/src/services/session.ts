import { getServerUrl } from './server';

// The session token lives in memory while the app runs and is also kept in
// localStorage, so reloading the page (or restarting the app) does not sign
// the person out. The server decides how long a token is valid.

const KEY = 'gothermo.session';

let authToken = '';

const save = (token: string) => {
  try {
    if (token) localStorage.setItem(KEY, JSON.stringify({ server: getServerUrl(), token }));
    else localStorage.removeItem(KEY);
  } catch {
    /* storage can be unavailable; the session then lasts until reload */
  }
};

export const setAuthToken = (token: string) => {
  authToken = token;
  save(token);
};

export const getAuthToken = () => authToken;

export const clearAuthToken = () => {
  authToken = '';
  save('');
};

// The saved token, but only when it was issued by the server in use now: a
// token from another server is useless there.
export function savedAuthToken(): string {
  try {
    const raw = localStorage.getItem(KEY);
    if (!raw) return '';
    const saved = JSON.parse(raw) as { server?: string; token?: string };
    return saved.token && saved.server === getServerUrl() ? saved.token : '';
  } catch {
    return '';
  }
}

// Signs the person back in with the saved token. `check` is the server's
// CheckAuth: it answers the user as JSON, or an empty string when the token is
// no longer valid. Resolves to the username, or null (and forgets a token the
// server refused). A network failure keeps the token for the next try.
export async function restoreSession(check: (token: string) => Promise<string>): Promise<string | null> {
  const token = savedAuthToken();
  if (!token) return null;
  try {
    const json = await check(token);
    const user = json ? (JSON.parse(json) as { username?: string }) : null;
    if (!user?.username) {
      clearAuthToken();
      return null;
    }
    authToken = token;
    return user.username;
  } catch {
    return null;
  }
}
