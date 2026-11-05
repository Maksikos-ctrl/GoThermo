import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { MessageItem } from '../MessageItem';
import type { Message } from '../../types';

const msg = (user: string): Message => ({
  id: 'm1',
  channel: 'general',
  reactions: {},
  user,
  text: 'hello',
  timestamp: '2026-01-01T12:00:00Z',
});

const noop = () => {};

function show(message: Message, onReport?: (m: Message, reason: string) => void) {
  const { container } = render(
    <MessageItem
      message={message}
      currentUser="alice"
      onAddReaction={noop}
      onEditMessage={noop}
      onDeleteMessage={noop}
      knownUsernames={['alice', 'bob']}
      onReport={onReport}
    />
  );
  fireEvent.mouseEnter(container.querySelector('.message') as Element);
}

describe('report a message', () => {
  it('offers reasons and sends the chosen one', async () => {
    const onReport = vi.fn();
    show(msg('bob'), onReport);
    fireEvent.click(await screen.findByLabelText('Report message'));
    fireEvent.click(screen.getByText('Inappropriate / NSFW'));
    expect(onReport).toHaveBeenCalledWith(expect.objectContaining({ id: 'm1' }), 'nsfw');
    expect(screen.queryByRole('menu')).toBeNull();
  });

  it('can be cancelled with the button and with Escape', async () => {
    const onReport = vi.fn();
    show(msg('bob'), onReport);
    fireEvent.click(await screen.findByLabelText('Report message'));
    fireEvent.click(screen.getByText('Cancel'));
    expect(screen.queryByRole('menu')).toBeNull();
    fireEvent.click(screen.getByLabelText('Report message'));
    fireEvent.keyDown(document, { key: 'Escape' });
    expect(screen.queryByRole('menu')).toBeNull();
    expect(onReport).not.toHaveBeenCalled();
  });

  it('is not offered for your own message or without a handler', async () => {
    show(msg('alice'), vi.fn());
    await screen.findByLabelText('Edit message');
    expect(screen.queryByLabelText('Report message')).toBeNull();
  });

  it('is hidden where reporting is not supported', async () => {
    show(msg('bob'));
    await screen.findAllByRole('button');
    expect(screen.queryByLabelText('Report message')).toBeNull();
  });
});
