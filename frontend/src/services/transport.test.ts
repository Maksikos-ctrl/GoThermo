import { describe, it, expect, beforeEach, vi } from 'vitest';
import { setServerUrl, setInviteCode } from './server';

const local = vi.fn();
vi.mock('../../wailsjs/go/main/App', () => {
  const names = ['Register', 'GetChannels', 'SendMessage', 'Logout', 'ExportChannel', 'LoginWithGoogle', 'SaveFileToDisk', 'CancelGoogleLogin'];
  const o: Record<string, unknown> = {};
  for (const n of names) o[n] = (...a: unknown[]) => local(n, ...a);
  return o;
});

import { Register, GetChannels, SendMessage, Logout, ExportChannel, LoginWithGoogle } from './transport';

const reply = (body: unknown, ok = true, status = 200) =>
  vi.fn().mockResolvedValue({ ok, status, json: async () => body });

beforeEach(() => {
  local.mockReset();
  local.mockResolvedValue('local-result');
  setServerUrl('');
});

describe('local mode', () => {
  it('calls the Wails binding and never the network', async () => {
    const f = vi.fn();
    vi.stubGlobal('fetch', f);
    expect(await GetChannels('tok')).toBe('local-result');
    expect(local).toHaveBeenCalledWith('GetChannels', 'tok');
    expect(f).not.toHaveBeenCalled();
  });
});

describe('server mode', () => {
  beforeEach(() => setServerUrl('https://chat.example.com'));

  it('posts the arguments as a JSON array to /rpc/<Method>', async () => {
    const f = reply({ result: [{ name: 'general' }] });
    vi.stubGlobal('fetch', f);
    expect(await GetChannels('tok')).toEqual([{ name: 'general' }]);
    expect(local).not.toHaveBeenCalled();
    const [url, init] = f.mock.calls[0];
    expect(url).toBe('https://chat.example.com/rpc/GetChannels');
    expect(init.method).toBe('POST');
    expect(JSON.parse(init.body)).toEqual(['tok']);
  });

  it('keeps argument order and types', async () => {
    const f = reply({ result: 'id1' });
    vi.stubGlobal('fetch', f);
    await SendMessage('t', 'alice', 'hi', 'general');
    expect(JSON.parse(f.mock.calls[0][1].body)).toEqual(['t', 'alice', 'hi', 'general']);
  });

  it('rejects with the server error text, like Wails does', async () => {
    vi.stubGlobal('fetch', reply({ result: null, error: 'incorrect email or password' }));
    await expect(Logout('x')).rejects.toBe('incorrect email or password');
  });

  it('sends the invite code with Register only', async () => {
    setInviteCode('abc123');
    const f = reply({ result: null });
    vi.stubGlobal('fetch', f);
    await Register('a@b.c', 'pw');
    await GetChannels('t');
    expect(f.mock.calls[0][1].headers['X-Invite-Code']).toBe('abc123');
    expect(f.mock.calls[1][1].headers['X-Invite-Code']).toBeUndefined();
    setInviteCode('');
    await Register('a@b.c', 'pw');
    expect(f.mock.calls[2][1].headers['X-Invite-Code']).toBeUndefined();
  });

  it('turns a network failure into a readable message', async () => {
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new TypeError('Failed to fetch')));
    await expect(GetChannels('t')).rejects.toBe('Cannot reach the server at https://chat.example.com');
  });

  it('reports a non-JSON answer', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: false, status: 502, json: async () => { throw new Error('x'); } }));
    await expect(GetChannels('t')).rejects.toMatch(/502/);
  });

  it('returns null for methods without a result', async () => {
    vi.stubGlobal('fetch', reply({ result: null }));
    expect(await Logout('t')).toBeNull();
  });

  it('blocks desktop-only methods without touching network or Wails', async () => {
    const f = vi.fn();
    vi.stubGlobal('fetch', f);
    expect(typeof ExportChannel).toBe('function'); // export now works over a server
    expect(typeof LoginWithGoogle).toBe('function'); // Google sign-in now works over a server
    expect(f).not.toHaveBeenCalled();
    expect(local).not.toHaveBeenCalled();
  });
});
