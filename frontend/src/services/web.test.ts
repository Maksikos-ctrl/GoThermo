import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { getInviteCode, getServerUrl, isRemote, setInviteCode, setServerUrl, takeInviteFromUrl, wsUrl } from './server';

const local = vi.fn();
vi.mock('../../wailsjs/go/main/App', () => {
  const names = ['Login', 'SaveTextFile', 'SaveFileToDisk', 'LoginWithGoogle', 'CancelGoogleLogin', 'ExportChannel'];
  const o: Record<string, unknown> = {};
  for (const n of names) o[n] = (...a: unknown[]) => local(n, ...a);
  return o;
});

const popup = vi.fn();
vi.mock('./googleWeb', () => ({ requestGoogleAccessToken: (id: string) => popup(id) }));

import { ExportChannel, LoginWithGoogle, Login, SaveFileToDisk, hasGoogleWeb } from './transport';
import { downloadText, openExternal, saveText } from './save';

const store: Record<string, string> = {};
vi.stubGlobal('localStorage', {
  getItem: (k: string) => (k in store ? store[k] : null),
  setItem: (k: string, v: string) => { store[k] = String(v); },
  removeItem: (k: string) => { delete store[k]; },
});

const reply = (body: unknown) => vi.fn().mockResolvedValue({ ok: true, status: 200, json: async () => body });

beforeEach(() => {
  local.mockReset();
  local.mockResolvedValue('local-result');
  setServerUrl('');
  setInviteCode('');
});

afterEach(() => {
  vi.unstubAllEnvs();
  vi.restoreAllMocks();
});

describe('browser build', () => {
  it('uses the address of the page as the server', () => {
    vi.stubEnv('VITE_WEB', '1');
    expect(getServerUrl()).toBe(window.location.origin);
    expect(isRemote()).toBe(true);
    expect(wsUrl('tok')).toBe(`${window.location.origin.replace(/^http/, 'ws')}/ws?token=tok`);
  });

  it('ignores a server address saved earlier and cannot be pointed elsewhere', () => {
    setServerUrl('https://old.example.com');
    vi.stubEnv('VITE_WEB', '1');
    setServerUrl('https://other.example.com');
    expect(getServerUrl()).toBe(window.location.origin);
    vi.unstubAllEnvs();
    expect(getServerUrl()).toBe('https://old.example.com');
  });

  it('the desktop app is unchanged', () => {
    expect(getServerUrl()).toBe('');
    expect(isRemote()).toBe(false);
    expect(wsUrl('tok')).toBe('ws://127.0.0.1:8081/ws?token=tok');
  });

  it('sends calls to the page address, never to Wails', async () => {
    vi.stubEnv('VITE_WEB', '1');
    const f = reply({ result: { username: 'ann' } });
    vi.stubGlobal('fetch', f);
    await Login('ann@example.com', 'pw');
    expect(f.mock.calls[0][0]).toBe(`${window.location.origin}/rpc/Login`);
    expect(local).not.toHaveBeenCalled();
  });

  it('keeps desktop-only calls out of the browser', async () => {
    vi.stubEnv('VITE_WEB', '1');
    vi.stubGlobal('fetch', vi.fn());
    await expect(SaveFileToDisk('t', 'f')).rejects.toMatch(/not available/);
    expect(local).not.toHaveBeenCalled();
  });

  it('export from the browser is a download, not a save dialog', async () => {
    vi.stubEnv('VITE_WEB', '1');
    vi.stubGlobal('fetch', reply({ result: { filename: 'general.json', content: '{"a":1}' } }));
    const click = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => {});
    URL.createObjectURL = vi.fn(() => 'blob:x');
    URL.revokeObjectURL = vi.fn();

    expect(await ExportChannel('t', 'general', 'ann', 'json')).toBe('general.json');
    expect(click).toHaveBeenCalledTimes(1);
    expect(local).not.toHaveBeenCalled();
  });
});

describe('invite link', () => {
  it('takes the code from the address and removes it from the bar', () => {
    window.history.replaceState(null, '', '/app/?invite=abc123&x=1');
    takeInviteFromUrl();
    expect(getInviteCode()).toBe('abc123');
    expect(window.location.search).toBe('?x=1');
    expect(window.location.pathname).toBe('/app/');
  });

  it('leaves everything alone when there is no code', () => {
    setInviteCode('kept');
    window.history.replaceState(null, '', '/app/');
    takeInviteFromUrl();
    expect(getInviteCode()).toBe('kept');
  });
});

describe('save helpers', () => {
  it('downloadText hands the text to the browser under the given name', () => {
    const click = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => {});
    let seen: Blob | null = null;
    URL.createObjectURL = vi.fn((b: Blob) => { seen = b; return 'blob:x'; });
    URL.revokeObjectURL = vi.fn();
    expect(downloadText('a.txt', 'hello')).toBe('a.txt');
    expect(click).toHaveBeenCalledTimes(1);
    expect(seen).toBeInstanceOf(Blob);
    expect((seen as unknown as Blob).size).toBe(5);
  });

  it('saveText uses the Wails dialog on the desktop', async () => {
    local.mockResolvedValue('/Users/me/a.txt');
    expect(await saveText('a.txt', 'x')).toBe('/Users/me/a.txt');
    expect(local).toHaveBeenCalledWith('SaveTextFile', 'a.txt', 'x');
  });

  it('openExternal uses the system browser in the app and a new tab on the web', () => {
    const rt = { BrowserOpenURL: vi.fn() };
    (window as any).runtime = rt;
    const open = vi.spyOn(window, 'open').mockImplementation(() => null);

    openExternal('https://a.example.com');
    expect(rt.BrowserOpenURL).toHaveBeenCalledWith('https://a.example.com');
    expect(open).not.toHaveBeenCalled();

    vi.stubEnv('VITE_WEB', '1');
    openExternal('https://b.example.com');
    expect(rt.BrowserOpenURL).toHaveBeenCalledTimes(1);
    expect(open).toHaveBeenCalledWith('https://b.example.com', '_blank', 'noopener,noreferrer');
    delete (window as any).runtime;
  });
});

describe('Google sign-in in the browser', () => {
  beforeEach(() => popup.mockReset());

  it('asks the server for its web client, opens the popup, and signs in with the token', async () => {
    vi.stubEnv('VITE_WEB', '1');
    popup.mockResolvedValue('tok-123');
    const f = vi.fn()
      .mockResolvedValueOnce({ ok: true, status: 200, json: async () => ({ result: 'web-client-id' }) })
      .mockResolvedValueOnce({ ok: true, status: 200, json: async () => ({ result: { username: 'ann' } }) });
    vi.stubGlobal('fetch', f);

    const user = await LoginWithGoogle(true);

    expect(popup).toHaveBeenCalledWith('web-client-id');
    expect(f.mock.calls[0][0]).toMatch(/\/rpc\/GoogleWebClientID$/);
    expect(f.mock.calls[1][0]).toMatch(/\/rpc\/LoginWithGoogleToken$/);
    expect(JSON.parse(f.mock.calls[1][1].body)).toEqual(['tok-123', true]);
    expect(user).toEqual({ username: 'ann' });
    expect(local).not.toHaveBeenCalled();
  });

  it('says so when the server has no Google client', async () => {
    vi.stubEnv('VITE_WEB', '1');
    vi.stubGlobal('fetch', reply({ result: '' }));
    await expect(LoginWithGoogle(false)).rejects.toMatch(/not set up/);
    expect(popup).not.toHaveBeenCalled();
  });

  it('shows the button only when the server has a web client', async () => {
    vi.stubEnv('VITE_WEB', '1');
    vi.stubGlobal('fetch', reply({ result: 'cid' }));
    expect(await hasGoogleWeb()).toBe(true);
    vi.stubGlobal('fetch', reply({ result: '' }));
    expect(await hasGoogleWeb()).toBe(false);
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new Error('offline')));
    expect(await hasGoogleWeb()).toBe(false);
  });

  it('the desktop app always has the button', async () => {
    expect(await hasGoogleWeb()).toBe(true);
  });
});

describe('default server of a release build', () => {
  const fresh = async () => {
    vi.resetModules();
    return import('./server');
  };

  beforeEach(() => {
    for (const k of Object.keys(store)) delete store[k];
  });

  it('is used until the person chooses something else', async () => {
    vi.stubEnv('VITE_DEFAULT_SERVER', 'https://team.example.org');
    const m = await fresh();
    expect(m.getServerUrl()).toBe('https://team.example.org');
    expect(m.isRemote()).toBe(true);
  });

  it('a stored choice wins, including "this computer"', async () => {
    vi.stubEnv('VITE_DEFAULT_SERVER', 'https://team.example.org');
    store['gothermo.serverUrl'] = 'https://other.example.org';
    expect((await fresh()).getServerUrl()).toBe('https://other.example.org');

    store['gothermo.serverUrl'] = '';
    expect((await fresh()).getServerUrl()).toBe('');
  });

  it('choosing "this computer" is remembered over the default', async () => {
    vi.stubEnv('VITE_DEFAULT_SERVER', 'https://team.example.org');
    const m = await fresh();
    m.setServerUrl('');
    expect(store['gothermo.serverUrl']).toBe('');
    expect((await fresh()).getServerUrl()).toBe('');
  });

  it('without a default the app starts on this computer', async () => {
    vi.stubEnv('VITE_DEFAULT_SERVER', '');
    expect((await fresh()).getServerUrl()).toBe('');
  });
});
