import { describe, it, expect } from 'vitest';
import type { Message } from '../../types';
import {
  PinnedMessage,
  applyMessageToPins,
  isPinned,
  normalizePins,
  pinCountLabel,
  pinnedByLabel,
  pinnedIdSet,
  removeMessageFromPins,
} from '../pinState';

const msg = (id: string, text = 't'): Message => ({
  id,
  channel: 'general',
  reactions: {},
  user: 'bob',
  text,
  timestamp: '2026-01-01T12:00:00Z',
});

const pin = (id: string, at = '2026-01-02T10:00:00Z', by = 'alice'): PinnedMessage => ({
  message: msg(id),
  pinnedBy: by,
  pinnedAt: at,
});

describe('pin lookups', () => {
  it('knows which messages are pinned', () => {
    const pins = [pin('a'), pin('b')];
    expect(isPinned(pins, 'a')).toBe(true);
    expect(isPinned(pins, 'z')).toBe(false);
    expect([...pinnedIdSet(pins)].sort()).toEqual(['a', 'b']);
  });
});

describe('applyMessageToPins', () => {
  it('replaces the pinned copy when the message changes', () => {
    const pins = [pin('a'), pin('b')];
    const next = applyMessageToPins(pins, { ...msg('b'), text: 'edited', reactions: { '👍': ['x'] } });
    expect(next[1]?.message.text).toBe('edited');
    expect(next[1]?.message.reactions?.['👍']).toEqual(['x']);
    expect(next[1]?.pinnedBy).toBe('alice'); // pin details stay
    expect(next[0]).toBe(pins[0]);
  });
  it('returns the same array for a message that is not pinned', () => {
    const pins = [pin('a')];
    expect(applyMessageToPins(pins, msg('z'))).toBe(pins);
  });
});

describe('removeMessageFromPins', () => {
  it('drops the pin of a deleted message', () => {
    const next = removeMessageFromPins([pin('a'), pin('b')], 'a');
    expect(next.map(p => p.message.id)).toEqual(['b']);
  });
  it('returns the same array when nothing was pinned', () => {
    const pins = [pin('a')];
    expect(removeMessageFromPins(pins, 'z')).toBe(pins);
  });
});

describe('normalizePins', () => {
  it('puts the newest pin first and skips broken entries', () => {
    const raw = [pin('old', '2026-01-01T00:00:00Z'), null, { foo: 1 }, pin('new', '2026-03-01T00:00:00Z')];
    expect(normalizePins(raw).map(p => p.message.id)).toEqual(['new', 'old']);
  });
  it('survives a missing list', () => {
    expect(normalizePins(null)).toEqual([]);
    expect(normalizePins(undefined)).toEqual([]);
  });
  it('does not change the list it was given', () => {
    const raw = [pin('a', '2026-01-01T00:00:00Z'), pin('b', '2026-02-01T00:00:00Z')];
    normalizePins(raw);
    expect(raw[0]?.message.id).toBe('a');
  });
});

describe('labels', () => {
  it('counts', () => {
    expect(pinCountLabel(1)).toBe('1 pinned message');
    expect(pinCountLabel(3)).toBe('3 pinned messages');
  });
  it('says who pinned and how long ago', () => {
    const now = new Date('2026-01-02T12:00:00Z').getTime();
    expect(pinnedByLabel('alice', '2026-01-02T11:59:40Z', now)).toBe('Pinned by alice · just now');
    expect(pinnedByLabel('alice', '2026-01-02T11:30:00Z', now)).toBe('Pinned by alice · 30 min ago');
    expect(pinnedByLabel('alice', '2026-01-02T09:00:00Z', now)).toBe('Pinned by alice · 3 h ago');
    expect(pinnedByLabel('alice', 'nonsense', now)).toBe('Pinned by alice');
  });
  it('switches from minutes to hours at exactly one hour', () => {
    const now = new Date('2026-01-02T12:00:00Z').getTime();
    expect(pinnedByLabel('a', '2026-01-02T11:00:01Z', now)).toBe('Pinned by a · 59 min ago');
    expect(pinnedByLabel('a', '2026-01-02T11:00:00Z', now)).toBe('Pinned by a · 1 h ago');
  });
});
