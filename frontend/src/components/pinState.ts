import { Message } from '../types';

// Pure helpers for the pinned-messages state, so the rules are easy to test.

export interface PinnedMessage {
  message: Message;
  pinnedBy: string;
  pinnedAt: string; // RFC 3339
}

export const pinnedIdSet = (pins: PinnedMessage[]): Set<string> => new Set(pins.map(p => p.message.id));

export const isPinned = (pins: PinnedMessage[], id: string): boolean => pins.some(p => p.message.id === id);

// A pinned message was edited or reacted to: the pinned copy follows.
// Returns the same array when the message is not pinned, so React does not re-render.
export function applyMessageToPins(pins: PinnedMessage[], message: Message): PinnedMessage[] {
  if (!isPinned(pins, message.id)) return pins;
  return pins.map(p => (p.message.id === message.id ? { ...p, message } : p));
}

// The message was deleted: its pin goes with it.
export function removeMessageFromPins(pins: PinnedMessage[], id: string): PinnedMessage[] {
  return isPinned(pins, id) ? pins.filter(p => p.message.id !== id) : pins;
}

// The server sends the whole list; make sure it is a list and the newest pin is first.
export function normalizePins(raw: unknown): PinnedMessage[] {
  if (!Array.isArray(raw)) return [];
  return (raw as PinnedMessage[])
    .filter(p => p && p.message && p.message.id)
    .slice()
    .sort((a, b) => new Date(b.pinnedAt).getTime() - new Date(a.pinnedAt).getTime());
}

export function pinCountLabel(n: number): string {
  return n === 1 ? '1 pinned message' : `${n} pinned messages`;
}

// "Pinned by alice · 3 h ago"
export function pinnedByLabel(by: string, at: string, now: number = Date.now()): string {
  const t = new Date(at).getTime();
  if (Number.isNaN(t)) return `Pinned by ${by}`;
  const min = Math.floor((now - t) / 60000);
  let when: string;
  if (min < 1) when = 'just now';
  else if (min < 60) when = `${min} min ago`;
  else if (min < 60 * 24) when = `${Math.floor(min / 60)} h ago`;
  else when = new Date(t).toLocaleDateString();
  return `Pinned by ${by} · ${when}`;
}
