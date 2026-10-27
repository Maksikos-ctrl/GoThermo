// Pure helpers for the typing hint and for @mention notices, kept free of
// React so the rules can be tested alone.

// ───────── who is typing ─────────

// channel -> user -> the time (ms) the hint expires
export type TypingMap = Record<string, Record<string, number>>;

export const TYPING_TTL_MS = 5000; // a hint lives this long unless refreshed
export const TYPING_SEND_GAP_MS = 3000; // how often our own typing is announced

export function noteTyping(map: TypingMap, channel: string, user: string, now: number, ttl: number = TYPING_TTL_MS): TypingMap {
  return { ...map, [channel]: { ...(map[channel] ?? {}), [user]: now + ttl } };
}

// A user who sent their message is no longer typing.
export function clearTyping(map: TypingMap, channel: string, user: string): TypingMap {
  const inChannel = map[channel];
  if (!inChannel || !(user in inChannel)) return map;
  const rest = { ...inChannel };
  delete rest[user];
  const next = { ...map };
  if (Object.keys(rest).length === 0) delete next[channel];
  else next[channel] = rest;
  return next;
}

// Drops expired hints. Returns the same object when nothing expired, so React
// does not re-render for nothing.
export function pruneTyping(map: TypingMap, now: number): TypingMap {
  let changed = false;
  const next: TypingMap = {};
  for (const [channel, users] of Object.entries(map)) {
    const alive: Record<string, number> = {};
    for (const [user, until] of Object.entries(users)) {
      if (until > now) alive[user] = until;
      else changed = true;
    }
    if (Object.keys(alive).length > 0) next[channel] = alive;
    else changed = true;
  }
  return changed ? next : map;
}

export function typingUsers(map: TypingMap, channel: string, now: number, exclude?: string): string[] {
  return Object.entries(map[channel] ?? {})
    .filter(([user, until]) => until > now && user !== exclude)
    .map(([user]) => user)
    .sort();
}

export function typingLabel(users: string[]): string {
  if (users.length === 0) return '';
  if (users.length === 1) return `${users[0]} is typing…`;
  if (users.length === 2) return `${users[0]} and ${users[1]} are typing…`;
  if (users.length === 3) return `${users[0]}, ${users[1]} and ${users[2]} are typing…`;
  return 'Several people are typing…';
}

// Announce our own typing at most once per gap.
export function shouldSendTyping(lastSentAt: number, now: number, gap: number = TYPING_SEND_GAP_MS): boolean {
  return now - lastSentAt >= gap;
}

// ───────── @mentions ─────────

// Same rule as the highlight in a message and as the server: the @ starts a
// word and the name ends it (or is followed by . , ! ?).
export function mentionsUser(text: string, username: string): boolean {
  if (!username) return false;
  const escaped = username.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
  return new RegExp(`(?:^|\\s)@${escaped}(?:\\s|$|[.,!?])`).test(text);
}

// "alice mentioned you in #general"
export function mentionNotice(from: string, channelTag: string): string {
  return channelTag.startsWith('@') ? `${from} mentioned you in a direct message` : `${from} mentioned you in ${channelTag}`;
}
