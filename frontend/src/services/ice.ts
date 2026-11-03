import { GetIceServers } from './transport';
import { getAuthToken } from './session';

// The call hooks need the ICE server list synchronously when they create a
// peer connection, so it is fetched ahead of time and kept here. If the server
// does not answer, calls still work through STUN alone.

const STUN: RTCIceServer[] = [{ urls: 'stun:stun.l.google.com:19302' }];
const REFRESH_MS = 6 * 60 * 60 * 1000; // credentials last 12 hours

let current: RTCIceServer[] = STUN;
let fetchedAt = 0;
let inflight: Promise<void> | null = null;

export const iceServers = (): RTCIceServer[] => current;

/** Fetches the list if it is missing or old. Never rejects. */
export function prefetchIceServers(force = false): Promise<void> {
  if (!force && fetchedAt && Date.now() - fetchedAt < REFRESH_MS) return Promise.resolve();
  if (inflight) return inflight;
  const token = getAuthToken();
  if (!token) return Promise.resolve();
  inflight = GetIceServers(token)
    .then((json: string) => {
      const list = JSON.parse(json);
      if (Array.isArray(list) && list.length > 0) {
        current = list as RTCIceServer[];
        fetchedAt = Date.now();
      }
    })
    .catch(() => {
      /* keep what we have */
    })
    .finally(() => {
      inflight = null;
    });
  return inflight;
}

/** For tests and sign-out. */
export function resetIceServers(): void {
  current = STUN;
  fetchedAt = 0;
  inflight = null;
}
