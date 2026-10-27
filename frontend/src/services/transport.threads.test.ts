import { describe, it, expect, beforeEach, vi } from 'vitest';
import { setServerUrl } from './server';

const local = vi.fn();
vi.mock('../../wailsjs/go/main/App', () => {
  const names = ['BuildThreadExport', 'SaveTextFile', 'GetThreadUnread', 'MarkThreadRead'];
  const o: Record<string, unknown> = {};
  for (const n of names) o[n] = (...a: unknown[]) => local(n, ...a);
  return o;
});

import { ExportThread, GetThreadUnread, MarkThreadRead } from './transport';

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
  setServerUrl('');
});

describe('thread export', () => {
  it('local: the backend builds it, the save dialog writes it', async () => {
    local.mockImplementation(async (name: string) =>
      name === 'BuildThreadExport' ? { filename: 't.txt', content: 'hello' } : '/Users/me/t.txt');
    const f = vi.fn();
    vi.stubGlobal('fetch', f);

    expect(await ExportThread('tok', 'general', 'p1', 'txt')).toBe('/Users/me/t.txt');
    expect(local).toHaveBeenNthCalledWith(1, 'BuildThreadExport', 'tok', 'general', 'p1', 'txt');
    expect(local).toHaveBeenNthCalledWith(2, 'SaveTextFile', 't.txt', 'hello');
    expect(f).not.toHaveBeenCalled();
  });

  it('on a server: built there, saved here', async () => {
    setServerUrl('https://chat.example.com');
    const f = reply({ result: { filename: 'gothermo-general-thread.json', content: '{}' } });
    vi.stubGlobal('fetch', f);
    local.mockResolvedValue('/Users/me/x.json');

    expect(await ExportThread('tok', 'general', 'p1', 'json')).toBe('/Users/me/x.json');
    expect(f.mock.calls[0][0]).toBe('https://chat.example.com/rpc/BuildThreadExport');
    expect(JSON.parse(f.mock.calls[0][1].body)).toEqual(['tok', 'general', 'p1', 'json']);
    expect(local).toHaveBeenCalledTimes(1);
    expect(local).toHaveBeenCalledWith('SaveTextFile', 'gothermo-general-thread.json', '{}');
  });

  it('no save dialog when the server refuses', async () => {
    setServerUrl('https://chat.example.com');
    vi.stubGlobal('fetch', reply({ error: 'you are not a participant of this chat' }));
    await expect(ExportThread('tok', 'secret', 'p1', 'txt')).rejects.toMatch(/not a participant/);
    expect(local).not.toHaveBeenCalled();
  });
});

describe('unread calls follow the mode', () => {
  it('go to the server when one is set', async () => {
    setServerUrl('https://chat.example.com');
    const f = reply({ result: { p1: 2 } });
    vi.stubGlobal('fetch', f);
    expect(await GetThreadUnread('tok', 'general')).toEqual({ p1: 2 });
    await MarkThreadRead('tok', 'general', 'p1');
    expect(f.mock.calls.map(c => c[0])).toEqual([
      'https://chat.example.com/rpc/GetThreadUnread',
      'https://chat.example.com/rpc/MarkThreadRead',
    ]);
    expect(local).not.toHaveBeenCalled();
  });

  it('stay local otherwise', async () => {
    local.mockResolvedValue({});
    await GetThreadUnread('tok', 'general');
    expect(local).toHaveBeenCalledWith('GetThreadUnread', 'tok', 'general');
  });
});
