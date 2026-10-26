import { describe, it, expect, beforeEach, vi } from 'vitest';
import { parseServerInput, setServerUrl, getServerUrl, isRemote, wsUrl, pingServer } from './server';

describe('parseServerInput', () => {
  it('treats empty input as the local backend', () => {
    expect(parseServerInput('   ')).toEqual({ url: '' });
  });
  it('uses https for public names', () => {
    expect(parseServerInput('chat.example.com')).toEqual({ url: 'https://chat.example.com' });
  });
  it('uses http for localhost and IP addresses, keeping the port', () => {
    expect(parseServerInput('localhost:8081')).toEqual({ url: 'http://localhost:8081' });
    expect(parseServerInput('192.168.1.5:8081')).toEqual({ url: 'http://192.168.1.5:8081' });
  });
  it('keeps an explicit scheme and drops path and trailing slash', () => {
    expect(parseServerInput('https://chat.example.com/some/path/')).toEqual({ url: 'https://chat.example.com' });
    expect(parseServerInput('http://10.0.0.2:9000/')).toEqual({ url: 'http://10.0.0.2:9000' });
  });
  it('rejects other schemes and garbage', () => {
    expect(parseServerInput('ftp://example.com').error).toBeTruthy();
    expect(parseServerInput('http://').error).toBeTruthy();
  });
});

describe('server url state', () => {
  beforeEach(() => setServerUrl(''));
  it('is local by default and remote once set', () => {
    expect(isRemote()).toBe(false);
    setServerUrl('https://chat.example.com');
    expect(isRemote()).toBe(true);
    expect(getServerUrl()).toBe('https://chat.example.com');
  });
  it('builds the websocket address for both modes', () => {
    expect(wsUrl('a b')).toBe('ws://127.0.0.1:8081/ws?token=a%20b');
    setServerUrl('https://chat.example.com');
    expect(wsUrl('t')).toBe('wss://chat.example.com/ws?token=t');
    setServerUrl('http://10.0.0.2:8081');
    expect(wsUrl('t')).toBe('ws://10.0.0.2:8081/ws?token=t');
  });
  it('remembers the choice in localStorage', () => {
    // Some Node versions ship their own half-working localStorage, so use a
    // plain in-memory one and the test does not depend on the environment.
    const store = new Map<string, string>();
    vi.stubGlobal('localStorage', {
      getItem: (k: string) => store.get(k) ?? null,
      setItem: (k: string, v: string) => void store.set(k, v),
      removeItem: (k: string) => void store.delete(k),
    });
    setServerUrl('https://x.example.com');
    expect(store.get('gothermo.serverUrl')).toBe('https://x.example.com');
    setServerUrl('');
    expect(store.has('gothermo.serverUrl')).toBe(false);
    vi.unstubAllGlobals();
  });
  it('still works when localStorage is missing', () => {
    vi.stubGlobal('localStorage', undefined);
    expect(() => setServerUrl('https://y.example.com')).not.toThrow();
    expect(getServerUrl()).toBe('https://y.example.com');
    vi.unstubAllGlobals();
  });
});

describe('pingServer', () => {
  it('accepts a GoThermo health answer', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ json: async () => ({ ok: true, app: 'gothermo' }) }));
    expect(await pingServer('https://x')).toBeNull();
  });
  it('rejects other servers and unreachable ones', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ json: async () => ({ hello: 1 }) }));
    expect(await pingServer('https://x')).toMatch(/not a GoThermo/);
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new Error('net')));
    expect(await pingServer('https://x')).toMatch(/Cannot reach/);
  });
});
