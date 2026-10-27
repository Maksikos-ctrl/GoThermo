import { describe, it, expect } from 'vitest';
import {
  TYPING_TTL_MS,
  clearTyping,
  mentionNotice,
  mentionsUser,
  noteTyping,
  pruneTyping,
  shouldSendTyping,
  typingLabel,
  typingUsers,
} from '../typingState';

const T = 1_000_000;

describe('typing map', () => {
  it('notes a user and lists them until the hint expires', () => {
    const m = noteTyping({}, 'general', 'alice', T);
    expect(typingUsers(m, 'general', T + 1)).toEqual(['alice']);
    expect(typingUsers(m, 'general', T + TYPING_TTL_MS - 1)).toEqual(['alice']);
    expect(typingUsers(m, 'general', T + TYPING_TTL_MS)).toEqual([]);
  });

  it('a second event refreshes the hint', () => {
    let m = noteTyping({}, 'general', 'alice', T);
    m = noteTyping(m, 'general', 'alice', T + 4000);
    expect(typingUsers(m, 'general', T + 6000)).toEqual(['alice']);
  });

  it('keeps channels apart and never lists yourself', () => {
    let m = noteTyping({}, 'general', 'alice', T);
    m = noteTyping(m, 'random', 'bob', T);
    m = noteTyping(m, 'general', 'me', T);
    expect(typingUsers(m, 'general', T + 1, 'me')).toEqual(['alice']);
    expect(typingUsers(m, 'random', T + 1, 'me')).toEqual(['bob']);
    expect(typingUsers(m, 'nowhere', T + 1)).toEqual([]);
  });

  it('lists users in a stable order', () => {
    let m = noteTyping({}, 'c', 'carol', T);
    m = noteTyping(m, 'c', 'alice', T);
    expect(typingUsers(m, 'c', T + 1)).toEqual(['alice', 'carol']);
  });

  it('does not change the map it was given', () => {
    const m = {};
    noteTyping(m, 'c', 'a', T);
    expect(m).toEqual({});
  });

  it('clearTyping removes one user, and the channel when it was the last', () => {
    let m = noteTyping(noteTyping({}, 'c', 'a', T), 'c', 'b', T);
    m = clearTyping(m, 'c', 'a');
    expect(typingUsers(m, 'c', T + 1)).toEqual(['b']);
    m = clearTyping(m, 'c', 'b');
    expect(m).toEqual({});
  });

  it('clearTyping returns the same object when there is nothing to clear', () => {
    const m = noteTyping({}, 'c', 'a', T);
    expect(clearTyping(m, 'c', 'zed')).toBe(m);
    expect(clearTyping(m, 'other', 'a')).toBe(m);
  });

  it('pruneTyping drops expired hints and empty channels', () => {
    let m = noteTyping({}, 'c', 'a', T);
    m = noteTyping(m, 'c', 'b', T + 3000);
    const later = pruneTyping(m, T + TYPING_TTL_MS + 1);
    expect(later).toEqual({ c: { b: T + 3000 + TYPING_TTL_MS } });
    expect(pruneTyping(later, T + TYPING_TTL_MS + 1)).toBe(later);
    expect(pruneTyping(later, T + 99_999)).toEqual({});
  });
});

describe('typingLabel', () => {
  it('reads naturally for 0 to many people', () => {
    expect(typingLabel([])).toBe('');
    expect(typingLabel(['alice'])).toBe('alice is typing…');
    expect(typingLabel(['alice', 'bob'])).toBe('alice and bob are typing…');
    expect(typingLabel(['alice', 'bob', 'carol'])).toBe('alice, bob and carol are typing…');
    expect(typingLabel(['a', 'b', 'c', 'd'])).toBe('Several people are typing…');
  });
});

describe('shouldSendTyping', () => {
  it('announces at most once per gap', () => {
    expect(shouldSendTyping(0, 10_000)).toBe(true);
    expect(shouldSendTyping(10_000, 11_000)).toBe(false);
    expect(shouldSendTyping(10_000, 12_999)).toBe(false);
    expect(shouldSendTyping(10_000, 13_000)).toBe(true);
  });
});

describe('mentionsUser (the same rule as the server)', () => {
  const cases: [string, boolean][] = [
    ['@alice', true],
    ['hey @alice', true],
    ['hey @alice, look', true],
    ['@alice!', true],
    ['you @alice?', true],
    ['@alice\nnext', true],
    ['@alicebob', false],
    ['mail@alice', false],
    ['(@alice)', false],
    ['@Alice', false],
    ['nothing', false],
    ['', false],
  ];
  for (const [text, want] of cases) {
    it(JSON.stringify(text), () => expect(mentionsUser(text, 'alice')).toBe(want));
  }
  it('matches a name with dots literally', () => {
    expect(mentionsUser('hi @a.b', 'a.b')).toBe(true);
    expect(mentionsUser('hi @axb', 'a.b')).toBe(false);
  });
  it('never matches an empty name', () => {
    expect(mentionsUser('@ hi', '')).toBe(false);
  });
});

describe('mentionNotice', () => {
  it('names the channel, or says direct message', () => {
    expect(mentionNotice('alice', '#general')).toBe('alice mentioned you in #general');
    expect(mentionNotice('alice', '@alice')).toBe('alice mentioned you in a direct message');
  });
});
