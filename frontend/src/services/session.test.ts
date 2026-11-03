import { describe, it, expect, beforeEach, vi } from 'vitest';
import { clearAuthToken, getAuthToken, restoreSession, savedAuthToken, setAuthToken } from './session';
import { setServerUrl } from './server';

const store: Record<string, string> = {};
vi.stubGlobal('localStorage', {
  getItem: (k: string) => (k in store ? store[k] : null),
  setItem: (k: string, v: string) => { store[k] = String(v); },
  removeItem: (k: string) => { delete store[k]; },
});

beforeEach(() => {
  for (const k of Object.keys(store)) delete store[k];
  setServerUrl('');
  clearAuthToken();
});

describe('saved session', () => {
  it('survives a reload', () => {
    setAuthToken('tok-1');
    expect(getAuthToken()).toBe('tok-1');
    expect(savedAuthToken()).toBe('tok-1');
  });

  it('is forgotten on sign-out', () => {
    setAuthToken('tok-1');
    clearAuthToken();
    expect(getAuthToken()).toBe('');
    expect(savedAuthToken()).toBe('');
  });

  it('is not offered to a different server', () => {
    setServerUrl('https://a.example.com');
    setAuthToken('tok-a');
    setServerUrl('https://b.example.com');
    expect(savedAuthToken()).toBe('');
    setServerUrl('https://a.example.com');
    expect(savedAuthToken()).toBe('tok-a');
  });

  it('ignores damaged storage', () => {
    store['gothermo.session'] = '{not json';
    expect(savedAuthToken()).toBe('');
  });
});

describe('restoreSession', () => {
  it('signs back in when the server accepts the token', async () => {
    setAuthToken('tok-1');
    const check = vi.fn().mockResolvedValue(JSON.stringify({ username: 'ann' }));
    // simulate a fresh page: memory is empty, storage still has the token
    store['gothermo.session'] = JSON.stringify({ server: '', token: 'tok-1' });
    expect(await restoreSession(check)).toBe('ann');
    expect(check).toHaveBeenCalledWith('tok-1');
    expect(getAuthToken()).toBe('tok-1');
  });

  it('does nothing without a saved token', async () => {
    const check = vi.fn();
    expect(await restoreSession(check)).toBeNull();
    expect(check).not.toHaveBeenCalled();
  });

  it('forgets a token the server no longer knows', async () => {
    setAuthToken('old');
    expect(await restoreSession(vi.fn().mockResolvedValue(''))).toBeNull();
    expect(savedAuthToken()).toBe('');
  });

  it('keeps the token when the server cannot be reached', async () => {
    setAuthToken('tok-1');
    expect(await restoreSession(vi.fn().mockRejectedValue('Cannot reach the server'))).toBeNull();
    expect(savedAuthToken()).toBe('tok-1');
  });
});
