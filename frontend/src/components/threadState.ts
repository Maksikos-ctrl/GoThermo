import { Message } from '../types';

// Pure helpers for the thread side panel. Kept free of React so the rules
// (who is shown, in what order, what a late event does) can be tested alone.

export interface ThreadData {
  parent: Message;
  replies: Message[];
}

// A reply that is already there is replaced (edit, reaction), a new one is
// appended. The server sends replies oldest first, so appending keeps order.
export function upsertReply(replies: Message[], msg: Message): Message[] {
  const i = replies.findIndex(r => r.id === msg.id);
  if (i === -1) return [...replies, msg];
  const next = replies.slice();
  next[i] = msg;
  return next;
}

export function removeReply(replies: Message[], id: string): Message[] {
  return replies.some(r => r.id === id) ? replies.filter(r => r.id !== id) : replies;
}

// A live event is only for the thread on screen; anything else is ignored.
export function applyReplyEvent(
  thread: ThreadData | null,
  threadId: string,
  msg: Message
): ThreadData | null {
  if (!thread || thread.parent.id !== threadId) return thread;
  return { ...thread, replies: upsertReply(thread.replies, msg) };
}

export function applyReplyDeleted(
  thread: ThreadData | null,
  threadId: string,
  messageId: string
): ThreadData | null {
  if (!thread || thread.parent.id !== threadId) return thread;
  const replies = removeReply(thread.replies, messageId);
  return replies === thread.replies ? thread : { ...thread, replies };
}

// The parent changed in the channel (reaction, edit, new reply count).
export function applyParentUpdate(thread: ThreadData | null, msg: Message): ThreadData | null {
  if (!thread || thread.parent.id !== msg.id) return thread;
  return { ...thread, parent: msg };
}

export function replyLabel(count: number): string {
  return count === 1 ? '1 reply' : `${count} replies`;
}

// "just now", "5 min ago", "3 h ago", "yesterday", otherwise the date.
export function formatLastReply(iso: string | undefined, now: number = Date.now()): string {
  if (!iso) return '';
  const t = new Date(iso).getTime();
  if (Number.isNaN(t)) return '';
  const diff = Math.max(0, now - t);
  const min = Math.floor(diff / 60000);
  if (min < 1) return 'just now';
  if (min < 60) return `${min} min ago`;
  const h = Math.floor(min / 60);
  if (h < 24) return `${h} h ago`;
  const d = Math.floor(h / 24);
  if (d === 1) return 'yesterday';
  if (d < 7) return `${d} days ago`;
  return new Date(t).toLocaleDateString();
}

// Same toggle the server does: a second click removes your reaction, and an
// emoji nobody uses any more disappears.
export function toggleReaction(msg: Message, emoji: string, user: string): Message {
  const reactions = { ...(msg.reactions ?? {}) };
  const users = reactions[emoji] ?? [];
  if (users.includes(user)) {
    const rest = users.filter(u => u !== user);
    if (rest.length === 0) delete reactions[emoji];
    else reactions[emoji] = rest;
  } else {
    reactions[emoji] = [...users, user];
  }
  return { ...msg, reactions };
}
