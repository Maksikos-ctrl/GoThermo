import { describe, it, expect } from 'vitest';
import type { Message } from '../../types';
import {
  applyParentUpdate,
  applyReplyDeleted,
  applyReplyEvent,
  formatLastReply,
  removeReply,
  replyLabel,
  toggleReaction,
  upsertReply,
  type ThreadData,
} from '../threadState';

const m = (id: string, extra: Partial<Message> = {}): Message => ({
  id,
  channel: 'general',
  reactions: {},
  user: 'bob',
  text: `text ${id}`,
  timestamp: '2026-01-01T12:00:00Z',
  ...extra,
});

const thread = (replies: Message[] = []): ThreadData => ({ parent: m('p'), replies });

describe('upsertReply / removeReply', () => {
  it('appends a new reply and keeps the order', () => {
    expect(upsertReply([m('1'), m('2')], m('3')).map(x => x.id)).toEqual(['1', '2', '3']);
  });

  it('replaces a reply with the same id instead of duplicating it', () => {
    const out = upsertReply([m('1'), m('2')], m('2', { text: 'edited' }));
    expect(out.map(x => x.id)).toEqual(['1', '2']);
    expect(out[1].text).toBe('edited');
  });

  it('does not change the input array', () => {
    const input = [m('1')];
    upsertReply(input, m('2'));
    expect(input).toHaveLength(1);
  });

  it('removes by id, and returns the same array when nothing matches', () => {
    const list = [m('1'), m('2')];
    expect(removeReply(list, '1').map(x => x.id)).toEqual(['2']);
    expect(removeReply(list, 'nope')).toBe(list);
  });
});

describe('live events only touch the open thread', () => {
  it('adds a reply of the open thread', () => {
    const out = applyReplyEvent(thread(), 'p', m('r1', { threadId: 'p' }));
    expect(out?.replies.map(r => r.id)).toEqual(['r1']);
  });

  it('ignores a reply of another thread (same object back)', () => {
    const t = thread();
    expect(applyReplyEvent(t, 'other', m('r1'))).toBe(t);
  });

  it('does nothing when no thread is open', () => {
    expect(applyReplyEvent(null, 'p', m('r1'))).toBeNull();
    expect(applyReplyDeleted(null, 'p', 'r1')).toBeNull();
    expect(applyParentUpdate(null, m('p'))).toBeNull();
  });

  it('removes a deleted reply, ignores unknown ids and other threads', () => {
    const t = thread([m('r1'), m('r2')]);
    expect(applyReplyDeleted(t, 'p', 'r1')?.replies.map(r => r.id)).toEqual(['r2']);
    expect(applyReplyDeleted(t, 'p', 'zzz')).toBe(t);
    expect(applyReplyDeleted(t, 'other', 'r1')).toBe(t);
  });

  it('updates the parent only when the id matches', () => {
    const t = thread([m('r1')]);
    const updated = applyParentUpdate(t, m('p', { text: 'new', replyCount: 4 }));
    expect(updated?.parent.text).toBe('new');
    expect(updated?.parent.replyCount).toBe(4);
    expect(updated?.replies).toHaveLength(1);
    expect(applyParentUpdate(t, m('someone-else'))).toBe(t);
  });
});

describe('replyLabel', () => {
  it('uses the singular only for one', () => {
    expect(replyLabel(1)).toBe('1 reply');
    expect(replyLabel(2)).toBe('2 replies');
    expect(replyLabel(0)).toBe('0 replies');
  });
});

describe('formatLastReply', () => {
  const now = new Date('2026-03-10T12:00:00Z').getTime();
  const ago = (ms: number) => new Date(now - ms).toISOString();
  const MIN = 60_000;
  const H = 60 * MIN;
  const D = 24 * H;

  it('reads naturally at every scale', () => {
    expect(formatLastReply(ago(20_000), now)).toBe('just now');
    expect(formatLastReply(ago(5 * MIN), now)).toBe('5 min ago');
    expect(formatLastReply(ago(59 * MIN), now)).toBe('59 min ago');
    expect(formatLastReply(ago(60 * MIN), now)).toBe('1 h ago');
    expect(formatLastReply(ago(3 * H), now)).toBe('3 h ago');
    expect(formatLastReply(ago(30 * H), now)).toBe('yesterday');
    expect(formatLastReply(ago(3 * D), now)).toBe('3 days ago');
  });

  it('falls back to a date after a week', () => {
    const out = formatLastReply(ago(20 * D), now);
    expect(out).not.toMatch(/ago|yesterday|just now/);
    expect(out.length).toBeGreaterThan(3);
  });

  it('is empty for missing or broken input, and never negative', () => {
    expect(formatLastReply(undefined, now)).toBe('');
    expect(formatLastReply('not a date', now)).toBe('');
    expect(formatLastReply(new Date(now + 5 * MIN).toISOString(), now)).toBe('just now');
  });
});

describe('toggleReaction', () => {
  it('adds, then removes, and drops an emoji nobody uses', () => {
    const a = toggleReaction(m('1'), '👍', 'alice');
    expect(a.reactions?.['👍']).toEqual(['alice']);
    const b = toggleReaction(a, '👍', 'carol');
    expect(b.reactions?.['👍']).toEqual(['alice', 'carol']);
    const c = toggleReaction(b, '👍', 'alice');
    expect(c.reactions?.['👍']).toEqual(['carol']);
    const d = toggleReaction(c, '👍', 'carol');
    expect(d.reactions && '👍' in d.reactions).toBe(false);
  });

  it('works when reactions are missing and does not mutate the original', () => {
    const base = { ...m('1'), reactions: undefined };
    const out = toggleReaction(base, '🚀', 'alice');
    expect(out.reactions?.['🚀']).toEqual(['alice']);
    expect(base.reactions).toBeUndefined();
  });
});
