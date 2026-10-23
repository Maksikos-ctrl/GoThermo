import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, fireEvent } from '@testing-library/react';
import { MessagesList } from '../MessagesList';
import { MessageItem } from '../MessageItem';
import type { Message } from '../../types';

const T0 = new Date('2026-01-01T12:00:00Z').getTime();
const at = (min: number) => new Date(T0 + min * 60_000).toISOString();
const msg = (id: string, user: string, min: number, extra: Partial<Message> = {}): Message => ({
  id,
  channel: "general",
  reactions: {},
  user,
  text: `text ${id}`,
  timestamp: at(min),
  ...extra,
});

const noop = () => {};
const base = {
  currentChannel: 'general',
  currentUser: 'alice',
  onAddReaction: noop,
  onEditMessage: noop,
  onDeleteMessage: noop,
};

const rows = (c: HTMLElement) => Array.from(c.querySelectorAll('.message'));

describe('MessagesList grouping', () => {
  it('groups same-user messages less than 5 minutes apart', () => {
    const { container } = render(
      <MessagesList {...base} messages={[msg('1', 'bob', 0), msg('2', 'bob', 2), msg('3', 'bob', 4)]} />
    );
    const r = rows(container);
    expect(r.map(x => x.classList.contains('grouped'))).toEqual([false, true, true]);
  });

  it('starts a new group after 5 minutes, or for another user', () => {
    const { container } = render(
      <MessagesList
        {...base}
        messages={[msg('1', 'bob', 0), msg('2', 'bob', 6), msg('3', 'carol', 7), msg('4', 'bob', 8)]}
      />
    );
    expect(rows(container).map(x => x.classList.contains('grouped'))).toEqual([false, false, false, false]);
  });

  it('never groups calls or posts', () => {
    const { container } = render(
      <MessagesList
        {...base}
        messages={[
          msg('1', 'bob', 0),
          msg('2', 'bob', 1, { isPost: true }),
          msg('3', 'bob', 2),
          msg('4', 'bob', 3, { isCall: true, callStatus: 'ended' }),
        ]}
      />
    );
    const g = rows(container).map(x => x.classList.contains('grouped'));
    expect(g[1]).toBe(false);
    expect(g[3]).toBe(false);
  });
});

describe('MessagesList history paging', () => {
  const scrollEl = (c: HTMLElement) => c.querySelector('.messages') as HTMLElement;
  const m = [msg('1', 'bob', 0), msg('2', 'carol', 1)];

  afterEach(() => {
    delete (HTMLElement.prototype as any).scrollHeight;
    delete (HTMLElement.prototype as any).clientHeight;
  });

  // Content taller than the viewport, so the "fill the screen" effect stays quiet.
  const tall = () => {
    Object.defineProperty(HTMLElement.prototype, 'scrollHeight', { configurable: true, get: () => 2000 });
    Object.defineProperty(HTMLElement.prototype, 'clientHeight', { configurable: true, get: () => 500 });
  };

  it('asks for older messages when scrolled near the top', () => {
    tall();
    const onLoadOlder = vi.fn();
    const { container } = render(<MessagesList {...base} messages={m} hasMoreOlder onLoadOlder={onLoadOlder} />);
    const el = scrollEl(container);
    el.scrollTop = 50;
    fireEvent.scroll(el);
    expect(onLoadOlder).toHaveBeenCalledTimes(1);
  });

  it('does not ask when far from the top', () => {
    tall();
    const onLoadOlder = vi.fn();
    const { container } = render(<MessagesList {...base} messages={m} hasMoreOlder onLoadOlder={onLoadOlder} />);
    const el = scrollEl(container);
    el.scrollTop = 800;
    fireEvent.scroll(el);
    expect(onLoadOlder).not.toHaveBeenCalled();
  });

  it('does not ask while loading or when there is nothing older', () => {
    tall();
    const a = vi.fn();
    const b = vi.fn();
    const r1 = render(<MessagesList {...base} messages={m} hasMoreOlder loadingOlder onLoadOlder={a} />);
    fireEvent.scroll(scrollEl(r1.container));
    const r2 = render(<MessagesList {...base} messages={m} hasMoreOlder={false} onLoadOlder={b} />);
    fireEvent.scroll(scrollEl(r2.container));
    expect(a).not.toHaveBeenCalled();
    expect(b).not.toHaveBeenCalled();
  });

  it('loads more by itself when the messages do not fill the screen', () => {
    Object.defineProperty(HTMLElement.prototype, 'scrollHeight', { configurable: true, get: () => 300 });
    Object.defineProperty(HTMLElement.prototype, 'clientHeight', { configurable: true, get: () => 500 });
    const onLoadOlder = vi.fn();
    render(<MessagesList {...base} messages={m} hasMoreOlder onLoadOlder={onLoadOlder} />);
    expect(onLoadOlder).toHaveBeenCalled();
  });

  it('keeps the same message in view after older ones are prepended', () => {
    let height = 1000;
    Object.defineProperty(HTMLElement.prototype, 'scrollHeight', { configurable: true, get: () => height });
    Object.defineProperty(HTMLElement.prototype, 'clientHeight', { configurable: true, get: () => 500 });
    const onLoadOlder = vi.fn();
    const { container, rerender } = render(
      <MessagesList {...base} messages={m} hasMoreOlder onLoadOlder={onLoadOlder} />
    );
    const el = scrollEl(container);
    el.scrollTop = 20;
    fireEvent.scroll(el);
    expect(onLoadOlder).toHaveBeenCalled();

    height = 1600; // 600px of older messages appeared above
    rerender(
      <MessagesList
        {...base}
        messages={[msg('0a', 'bob', -2), msg('0b', 'bob', -1), ...m]}
        hasMoreOlder
        onLoadOlder={onLoadOlder}
      />
    );
    expect(el.scrollTop).toBe(620);
  });

  it('shows the beginning marker only when nothing older exists', () => {
    const a = render(<MessagesList {...base} messages={m} hasMoreOlder={false} />);
    expect(a.container.textContent).toContain('This is the beginning of #general');
    const b = render(<MessagesList {...base} messages={m} hasMoreOlder />);
    expect(b.container.textContent).not.toContain('This is the beginning of');
  });
});

describe('MessageItem', () => {
  const item = (message: Message, extra: Record<string, any> = {}) =>
    render(
      <MessageItem
        message={message}
        currentUser="alice"
        onAddReaction={noop}
        onEditMessage={noop}
        onDeleteMessage={noop}
        knownUsernames={['alice', 'bob']}
        {...extra}
      />
    ).container.querySelector('.message') as HTMLElement;

  it('marks own messages and only own messages', () => {
    expect(item(msg('1', 'alice', 0)).classList.contains('my-message')).toBe(true);
    expect(item(msg('2', 'bob', 0)).classList.contains('my-message')).toBe(false);
  });

  it('marks messages that mention me', () => {
    expect(item({ ...msg('1', 'bob', 0), text: 'hey @alice' }).classList.contains('mentions-me')).toBe(true);
    expect(item({ ...msg('2', 'bob', 0), text: 'hey all' }).classList.contains('mentions-me')).toBe(false);
  });

  it('animates only fresh messages', () => {
    const fresh = item({ ...msg('1', 'bob', 0), timestamp: new Date().toISOString() });
    const old = item(msg('2', 'bob', 0));
    const minuteAgo = item({ ...msg('3', 'bob', 0), timestamp: new Date(Date.now() - 60_000).toISOString() });
    expect(fresh.classList.contains('msg-enter')).toBe(true);
    expect(old.classList.contains('msg-enter')).toBe(false);
    expect(minuteAgo.classList.contains('msg-enter')).toBe(false);
  });

  it('a grouped message shows a time gutter instead of avatar and name', () => {
    const el = item(msg('1', 'bob', 0), { isGrouped: true });
    expect(el.querySelector('.msg-avatar')).toBeNull();
    expect(el.querySelector('.message-user')).toBeNull();
    expect(el.querySelector('.msg-gutter')).not.toBeNull();
  });

  it('a normal message shows avatar initial and name', () => {
    const el = item(msg('1', 'bob', 0));
    expect(el.querySelector('.msg-avatar')?.textContent).toBe('B');
    expect(el.querySelector('.message-user')?.textContent).toBe('bob');
  });
});
