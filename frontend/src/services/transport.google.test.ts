import { describe, it, expect, beforeEach, vi } from 'vitest';
import { setServerUrl, setInviteCode } from './server';

const local = vi.fn();
vi.mock('../../wailsjs/go/main/App', () => {
  const names = ['LoginWithGoogle', 'CancelGoogleLogin', 'GoogleAccessToken'];
  const o: Record<string, unknown> = {};
  for (const n of names) o[n] = (...a: unknown[]) => local(n, ...a);
  return o;
});

import { LoginWithGoogle, CancelGoogleLogin } from './transport';

const reply = (body: unknown) => vi.fn().mockResolvedValue({ ok: true, status: 200, json: async () => body });

// The test environment may not have a usable localStorage.
const store: Record<string, string> = {};
vi.stubGlobal('localStorage', {
  getItem: (k: string) => (k in store ? store[k] : null),
  setItem: (k: string, v: string) => { store[k] = String(v); },
  removeItem: (k: string) => { delete store[k]; },
});

beforeEach(() => {
  local.mockReset();
  local.mockResolvedValue('local-result');
  setServerUrl('');
  setInviteCode('');
});

describe('Google sign-in', () => {
  it('local mode runs the whole flow in Wails', async () => {
    const f = vi.fn();
    vi.stubGlobal('fetch', f);
    expect(await LoginWithGoogle(true)).toBe('local-result');
    expect(local).toHaveBeenCalledWith('LoginWithGoogle', true);
    expect(f).not.toHaveBeenCalled();
  });

  it('on a server: browser step here, then the server signs the user in', async () => {
    setServerUrl('https://chat.example.com');
    setInviteCode('abc');
    local.mockResolvedValue('google-access-token');
    const f = reply({ result: { username: 'ann', token: 'sess' } });
    vi.stubGlobal('fetch', f);

    const user = await LoginWithGoogle(true);

    expect(local).toHaveBeenCalledTimes(1);
    expect(local).toHaveBeenCalledWith('GoogleAccessToken');
    expect(user).toEqual({ username: 'ann', token: 'sess' });
    const [url, init] = f.mock.calls[0];
    expect(url).toBe('https://chat.example.com/rpc/LoginWithGoogleToken');
    expect(JSON.parse(init.body)).toEqual(['google-access-token', true]);
    expect(init.headers['X-Invite-Code']).toBe('abc');
  });

  it('never calls the server when the browser step fails', async () => {
    setServerUrl('https://chat.example.com');
    local.mockRejectedValue('Google sign-in cancelled');
    const f = vi.fn();
    vi.stubGlobal('fetch', f);
    await expect(LoginWithGoogle(false)).rejects.toBe('Google sign-in cancelled');
    expect(f).not.toHaveBeenCalled();
  });

  it('shows the server error text', async () => {
    setServerUrl('https://chat.example.com');
    local.mockResolvedValue('tok');
    vi.stubGlobal('fetch', reply({ error: 'no GoThermo account for a@b.c yet, please use Sign Up first' }));
    await expect(LoginWithGoogle(false)).rejects.toMatch(/Sign Up first/);
  });

  it('cancel always stays on this computer', async () => {
    setServerUrl('https://chat.example.com');
    const f = vi.fn();
    vi.stubGlobal('fetch', f);
    await CancelGoogleLogin();
    expect(local).toHaveBeenCalledWith('CancelGoogleLogin');
    expect(f).not.toHaveBeenCalled();
  });
});
