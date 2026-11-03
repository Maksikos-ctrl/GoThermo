import { describe, it, expect, vi, beforeEach } from 'vitest';

const get = vi.fn();
vi.mock('./transport', () => ({ GetIceServers: (...a: unknown[]) => get(...a) }));
vi.mock('./session', () => ({ getAuthToken: () => 'tok' }));

import { iceServers, prefetchIceServers, resetIceServers } from './ice';

describe('ice servers', () => {
  beforeEach(() => {
    get.mockReset();
    resetIceServers();
  });

  it('starts with STUN only', () => {
    expect(iceServers()).toEqual([{ urls: 'stun:stun.l.google.com:19302' }]);
  });

  it('uses what the server returns and does not refetch while fresh', async () => {
    const list = [{ urls: ['stun:a'] }, { urls: ['turn:b'], username: 'u', credential: 'c' }];
    get.mockResolvedValue(JSON.stringify(list));
    await prefetchIceServers();
    await prefetchIceServers();
    expect(iceServers()).toEqual(list);
    expect(get).toHaveBeenCalledTimes(1);
    expect(get).toHaveBeenCalledWith('tok');
  });

  it('keeps STUN when the server fails or sends junk', async () => {
    get.mockRejectedValueOnce('boom');
    await prefetchIceServers();
    get.mockResolvedValueOnce('not json');
    await prefetchIceServers(true);
    get.mockResolvedValueOnce('[]');
    await prefetchIceServers(true);
    expect(iceServers()).toEqual([{ urls: 'stun:stun.l.google.com:19302' }]);
  });
});
