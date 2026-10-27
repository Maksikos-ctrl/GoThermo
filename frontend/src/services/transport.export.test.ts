import { describe, it, expect, beforeEach, vi } from 'vitest';
import { setServerUrl } from './server';

const local = vi.fn();
vi.mock('../../wailsjs/go/main/App', () => {
  const names = ['ExportChannel', 'SaveTextFile', 'SaveFileToDisk'];
  const o: Record<string, unknown> = {};
  for (const n of names) o[n] = (...a: unknown[]) => local(n, ...a);
  return o;
});

import { ExportChannel, SaveFileToDisk } from './transport';

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
});

describe('channel export', () => {
  it('local mode is the Wails call, nothing else', async () => {
    const f = vi.fn();
    vi.stubGlobal('fetch', f);
    expect(await ExportChannel('t', 'general', 'ann', 'json')).toBe('local-result');
    expect(local).toHaveBeenCalledWith('ExportChannel', 't', 'general', 'ann', 'json');
    expect(f).not.toHaveBeenCalled();
  });

  it('on a server: built there, saved here', async () => {
    setServerUrl('https://chat.example.com');
    const f = reply({ result: { filename: 'gothermo-general-2026-10-27.json', content: '{"a":1}' } });
    vi.stubGlobal('fetch', f);
    local.mockResolvedValue('/Users/me/Downloads/gothermo-general-2026-10-27.json');

    const path = await ExportChannel('t', 'general', 'ann', 'json');

    expect(path).toBe('/Users/me/Downloads/gothermo-general-2026-10-27.json');
    expect(f.mock.calls[0][0]).toBe('https://chat.example.com/rpc/BuildChannelExport');
    expect(JSON.parse(f.mock.calls[0][1].body)).toEqual(['t', 'general', 'ann', 'json']);
    expect(local).toHaveBeenCalledTimes(1);
    expect(local).toHaveBeenCalledWith('SaveTextFile', 'gothermo-general-2026-10-27.json', '{"a":1}');
  });

  it('does not open a save dialog when the server refuses', async () => {
    setServerUrl('https://chat.example.com');
    vi.stubGlobal('fetch', reply({ error: 'you are not a participant of this chat' }));
    await expect(ExportChannel('t', 'dm_a_b', 'eve', 'json')).rejects.toMatch(/not a participant/);
    expect(local).not.toHaveBeenCalled();
  });

  it('closing the save dialog gives an empty path', async () => {
    setServerUrl('https://chat.example.com');
    vi.stubGlobal('fetch', reply({ result: { filename: 'x.txt', content: 'hi' } }));
    local.mockResolvedValue('');
    expect(await ExportChannel('t', 'general', 'ann', 'txt')).toBe('');
  });

  it('saving received files is still blocked over a server', async () => {
    setServerUrl('https://chat.example.com');
    await expect(SaveFileToDisk('t', 'f1')).rejects.toMatch(/not available/);
    expect(local).not.toHaveBeenCalled();
  });
});
