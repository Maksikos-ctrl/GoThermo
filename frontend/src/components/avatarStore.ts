// Pictures people uploaded, keyed by lowercase username. A tiny external
// store so any <Avatar> updates live without passing props around.
import { useSyncExternalStore } from 'react';

let pics: Record<string, string> = {};
const listeners = new Set<() => void>();

function emit() {
  listeners.forEach(l => l());
}

const key = (u: string) => u.toLowerCase();

export function setAvatar(username: string, url: string | undefined) {
  const k = key(username);
  const next = { ...pics };
  if (url) next[k] = url;
  else delete next[k];
  if ((pics[k] ?? '') === (next[k] ?? '')) return;
  pics = next;
  emit();
}

// Replace everything we know from a fresh user list.
export function setAvatarsFromUsers(users: { username: string; avatar?: string }[]) {
  const next: Record<string, string> = {};
  for (const u of users) if (u.avatar) next[key(u.username)] = u.avatar;
  pics = next;
  emit();
}

export function getAvatar(username: string): string | undefined {
  return pics[key(username)];
}

function subscribe(l: () => void) {
  listeners.add(l);
  return () => {
    listeners.delete(l);
  };
}

export function useAvatar(username: string): string | undefined {
  return useSyncExternalStore(subscribe, () => getAvatar(username));
}
