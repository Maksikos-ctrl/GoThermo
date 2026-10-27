import { useSyncExternalStore } from 'react';
import { Message } from '../types';

// ───────── search inside a thread ─────────

// Replies whose text or author contains the query (case does not matter).
// An empty query keeps everything.
export function filterReplies(replies: Message[], query: string): Message[] {
  const q = query.trim().toLowerCase();
  if (!q) return replies;
  return replies.filter(r => (r.text ?? '').toLowerCase().includes(q) || r.user.toLowerCase().includes(q));
}

// ───────── unread replies ─────────

// How many replies of each thread (by parent message id) you have not seen.
// It lives outside React so a message deep in the list can show its badge
// without the number being passed through every component in between.
let unread: Record<string, number> = {};
const listeners = new Set<() => void>();
const emit = () => listeners.forEach(l => l());

export const threadUnread = {
  get(parentId: string): number {
    return unread[parentId] ?? 0;
  },
  // The server's answer for a channel replaces what was known.
  setAll(map: Record<string, number> | null | undefined): void {
    unread = { ...(map ?? {}) };
    emit();
  },
  clear(parentId: string): void {
    if (!(parentId in unread)) return;
    const next = { ...unread };
    delete next[parentId];
    unread = next;
    emit();
  },
  subscribe(listener: () => void): () => void {
    listeners.add(listener);
    return () => {
      listeners.delete(listener);
    };
  },
};

export function useThreadUnread(parentId: string): number {
  return useSyncExternalStore(
    threadUnread.subscribe,
    () => threadUnread.get(parentId),
    () => 0
  );
}
