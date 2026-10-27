import { describe, it, expect, beforeEach, vi } from 'vitest';
import { filterReplies, threadUnread } from '../threadExtras';
import type { Message } from '../../types';

const m = (id: string, user: string, text: string): Message => ({
  id,
  channel: 'general',
  user,
  text,
  timestamp: '2026-01-01T12:00:00Z',
});

describe('filterReplies', () => {
  const all = [m('1', 'alice', 'Deploy is done'), m('2', 'bob', 'thanks!'), m('3', 'Carol', 'see the DEPLOY log')];

  it('keeps everything for an empty or blank query', () => {
    expect(filterReplies(all, '')).toBe(all);
    expect(filterReplies(all, '   ')).toBe(all);
  });

  it('matches text without caring about case', () => {
    expect(filterReplies(all, 'deploy').map(r => r.id)).toEqual(['1', '3']);
  });

  it('matches the author too', () => {
    expect(filterReplies(all, 'CAROL').map(r => r.id)).toEqual(['3']);
  });

  it('trims the query and returns nothing for no match', () => {
    expect(filterReplies(all, '  thanks ').map(r => r.id)).toEqual(['2']);
    expect(filterReplies(all, 'zzz')).toEqual([]);
  });

  it('does not change the input', () => {
    const copy = [...all];
    filterReplies(all, 'deploy');
    expect(all).toEqual(copy);
  });
});

describe('threadUnread store', () => {
  beforeEach(() => threadUnread.setAll({}));

  it('replaces everything on setAll and defaults to 0', () => {
    threadUnread.setAll({ a: 2, b: 1 });
    expect(threadUnread.get('a')).toBe(2);
    threadUnread.setAll({ c: 5 });
    expect(threadUnread.get('a')).toBe(0);
    expect(threadUnread.get('c')).toBe(5);
    threadUnread.setAll(null);
    expect(threadUnread.get('c')).toBe(0);
  });

  it('clear removes one thread only', () => {
    threadUnread.setAll({ a: 2, b: 1 });
    threadUnread.clear('a');
    expect(threadUnread.get('a')).toBe(0);
    expect(threadUnread.get('b')).toBe(1);
  });

  it('tells subscribers, and stops after unsubscribe', () => {
    const fn = vi.fn();
    const off = threadUnread.subscribe(fn);
    threadUnread.setAll({ a: 1 });
    threadUnread.clear('a');
    expect(fn).toHaveBeenCalledTimes(2);
    off();
    threadUnread.setAll({ a: 3 });
    expect(fn).toHaveBeenCalledTimes(2);
  });

  it('does not notify when clearing a thread that has nothing', () => {
    const fn = vi.fn();
    const off = threadUnread.subscribe(fn);
    threadUnread.clear('nothing');
    expect(fn).not.toHaveBeenCalled();
    off();
  });
});
