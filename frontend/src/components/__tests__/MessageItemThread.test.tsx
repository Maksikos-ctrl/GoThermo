import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { MessageItem } from '../MessageItem';
import { MessagesList } from '../MessagesList';
import type { Message } from '../../types';

const base: Message = {
  id: 'm1',
  channel: 'general',
  reactions: {},
  user: 'bob',
  text: 'hello',
  timestamp: '2026-01-01T12:00:00Z',
};

const noop = () => {};

function item(message: Message, onOpenThread?: (m: Message) => void) {
  return render(
    <MessageItem
      message={message}
      currentUser="alice"
      onAddReaction={noop}
      onEditMessage={noop}
      onDeleteMessage={noop}
      knownUsernames={['alice', 'bob']}
      onOpenThread={onOpenThread}
    />
  ).container;
}

describe('thread summary under a message', () => {
  const withReplies: Message = {
    ...base,
    replyCount: 3,
    lastReplyAt: new Date(Date.now() - 5 * 60_000).toISOString(),
    replyUsers: ['alice', 'carol'],
  };

  it('shows the count, the repliers and the time, and opens the thread on click', () => {
    const open = vi.fn();
    const c = item(withReplies, open);
    const row = c.querySelector('.thread-summary') as HTMLElement;
    expect(row.querySelector('.thread-summary-count')?.textContent).toBe('3 replies');
    expect(row.querySelector('.thread-summary-time')?.textContent).toBe('5 min ago');
    expect(row.querySelectorAll('.thread-summary-avatar')).toHaveLength(2);
    fireEvent.click(row);
    expect(open).toHaveBeenCalledWith(withReplies);
  });

  it('says "1 reply" for a single reply', () => {
    const c = item({ ...withReplies, replyCount: 1 }, vi.fn());
    expect(c.querySelector('.thread-summary-count')?.textContent).toBe('1 reply');
  });

  it('is not shown without replies, without a handler, or on call logs', () => {
    expect(item({ ...base, replyCount: 0 }, vi.fn()).querySelector('.thread-summary')).toBeNull();
    expect(item(base, vi.fn()).querySelector('.thread-summary')).toBeNull();
    expect(item(withReplies).querySelector('.thread-summary')).toBeNull();
    expect(
      item({ ...withReplies, isCall: true, callStatus: 'completed' }, vi.fn()).querySelector('.thread-summary')
    ).toBeNull();
  });
});

describe('Reply button in the hover toolbar', () => {
  it('appears on hover and opens the thread', async () => {
    const open = vi.fn();
    const c = item(base, open);
    fireEvent.mouseEnter(c.querySelector('.message') as Element);
    const btn = await screen.findByLabelText('Reply in thread');
    fireEvent.click(btn);
    expect(open).toHaveBeenCalledWith(base);
  });

  it('is there for other people and for your own messages', async () => {
    const c = item({ ...base, user: 'alice' }, vi.fn());
    fireEvent.mouseEnter(c.querySelector('.message') as Element);
    expect(await screen.findByLabelText('Reply in thread')).toBeTruthy();
  });

  it('is missing when no handler is given', async () => {
    const c = item(base);
    fireEvent.mouseEnter(c.querySelector('.message') as Element);
    await new Promise(r => setTimeout(r, 150)); // longer than the toolbar's own hover delay
    expect(screen.queryByLabelText('Reply in thread')).toBeNull();
  });
});

describe('MessagesList passes the thread handler on', () => {
  it('opens the thread of the message whose summary was clicked', () => {
    const open = vi.fn();
    const messages: Message[] = [
      { ...base, id: 'a' },
      { ...base, id: 'b', replyCount: 2, replyUsers: ['carol'], lastReplyAt: new Date().toISOString() },
    ];
    const { container } = render(
      <MessagesList
        messages={messages}
        currentChannel="general"
        currentUser="alice"
        onAddReaction={noop}
        onEditMessage={noop}
        onDeleteMessage={noop}
        onOpenThread={open}
      />
    );
    const rows = container.querySelectorAll('.thread-summary');
    expect(rows).toHaveLength(1);
    fireEvent.click(rows[0]);
    expect(open.mock.calls[0][0].id).toBe('b');
  });
});
