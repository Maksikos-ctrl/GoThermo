import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { PinnedPanel } from '../PinnedPanel';
import { MessageItem } from '../MessageItem';
import { ChatHeader } from '../ChatHeader';
import { MessagesList } from '../MessagesList';
import type { Message } from '../../types';
import type { PinnedMessage } from '../pinState';

const msg = (id: string, text: string, user = 'bob'): Message => ({
  id,
  channel: 'general',
  reactions: {},
  user,
  text,
  timestamp: '2026-01-01T12:00:00Z',
});

const pins: PinnedMessage[] = [
  { message: msg('a', 'Release notes live in the wiki'), pinnedBy: 'alice', pinnedAt: new Date(Date.now() - 3 * 3600_000).toISOString() },
  { message: msg('b', 'Standup is at 10:00'), pinnedBy: 'carol', pinnedAt: new Date(Date.now() - 60_000).toISOString() },
];

const noop = () => {};

function panel(over: Partial<React.ComponentProps<typeof PinnedPanel>> = {}) {
  const onClose = vi.fn();
  const onUnpin = vi.fn();
  const props = {
    pins,
    loading: false,
    error: null,
    channelTag: '#general',
    currentUser: 'alice',
    knownUsernames: ['alice', 'bob'],
    onClose,
    onUnpin,
    onAddReaction: noop,
    onEditMessage: noop,
    onDeleteMessage: noop,
    ...over,
  };
  render(<PinnedPanel {...props} />);
  return { onClose, onUnpin };
}

describe('PinnedPanel', () => {
  it('lists every pin with who pinned it and a count', () => {
    panel();
    expect(screen.getByText('2 pinned messages')).toBeTruthy();
    expect(screen.getByText('Release notes live in the wiki')).toBeTruthy();
    expect(screen.getByText('Standup is at 10:00')).toBeTruthy();
    expect(screen.getByText(/Pinned by alice · 3 h ago/)).toBeTruthy();
    expect(screen.getByText(/Pinned by carol · 1 min ago/)).toBeTruthy();
    expect(screen.getByText('#general')).toBeTruthy();
  });

  it('unpins the right message', () => {
    const p = panel();
    fireEvent.click(screen.getAllByLabelText('Unpin message from bob')[0]!);
    expect(p.onUnpin).toHaveBeenCalledTimes(1);
    expect(p.onUnpin.mock.calls[0][0].id).toBe('a');
  });

  it('closes', () => {
    const p = panel();
    fireEvent.click(screen.getByLabelText('Close pinned messages'));
    expect(p.onClose).toHaveBeenCalled();
  });

  it('explains an empty list, and shows loading and errors instead of it', () => {
    const { unmount } = render(
      <PinnedPanel pins={[]} loading={false} error={null} channelTag="#x" currentUser="a" onClose={noop} onUnpin={noop}
        onAddReaction={noop} onEditMessage={noop} onDeleteMessage={noop} />
    );
    expect(screen.getByText('Nothing pinned yet')).toBeTruthy();
    unmount();
    const l = render(
      <PinnedPanel pins={[]} loading error={null} channelTag="#x" currentUser="a" onClose={noop} onUnpin={noop}
        onAddReaction={noop} onEditMessage={noop} onDeleteMessage={noop} />
    );
    expect(screen.queryByText('Nothing pinned yet')).toBeNull();
    expect(screen.getByText('Loading…')).toBeTruthy();
    l.unmount();
    render(
      <PinnedPanel pins={[]} loading={false} error="Could not load" channelTag="#x" currentUser="a" onClose={noop}
        onUnpin={noop} onAddReaction={noop} onEditMessage={noop} onDeleteMessage={noop} />
    );
    expect(screen.getByRole('alert').textContent).toBe('Could not load');
    expect(screen.queryByText('Nothing pinned yet')).toBeNull();
  });
});

describe('pin button on a message', () => {
  const item = (isPinned: boolean, onTogglePin?: (m: Message) => void, message = msg('m1', 'hi')) =>
    render(
      <MessageItem
        message={message}
        currentUser="alice"
        onAddReaction={noop}
        onEditMessage={noop}
        onDeleteMessage={noop}
        isPinned={isPinned}
        onTogglePin={onTogglePin}
      />
    ).container;

  const hover = (c: HTMLElement) => {
    fireEvent.mouseEnter(c.querySelector('.message') as HTMLElement);
  };

  it('offers Pin on hover and passes the message', async () => {
    const onTogglePin = vi.fn();
    const c = item(false, onTogglePin);
    expect(screen.queryByLabelText('Pin message')).toBeNull();
    hover(c);
    fireEvent.click(await screen.findByLabelText('Pin message'));
    expect(onTogglePin.mock.calls[0][0].id).toBe('m1');
  });

  it('offers Unpin for a pinned message and marks it', async () => {
    const c = item(true, vi.fn());
    expect(screen.getByLabelText('Pinned')).toBeTruthy();
    hover(c);
    expect(await screen.findByLabelText('Unpin message')).toBeTruthy();
  });

  it('shows no pin controls without a handler, and none on call logs', async () => {
    const c = item(false, undefined);
    hover(c);
    await new Promise(r => setTimeout(r, 80));
    expect(screen.queryByLabelText('Pin message')).toBeNull();
  });

  it('does not offer to pin a call log', async () => {
    const c = item(false, vi.fn(), { ...msg('c1', ''), isCall: true, callStatus: 'completed', callDuration: 5 });
    hover(c);
    await new Promise(r => setTimeout(r, 80));
    expect(screen.queryByLabelText('Pin message')).toBeNull();
  });
});

describe('MessagesList marks pinned messages', () => {
  it('passes the pin state to each message', () => {
    const list = [msg('a', 'one'), msg('b', 'two')];
    render(
      <MessagesList
        messages={list}
        currentChannel="general"
        currentUser="alice"
        onAddReaction={noop}
        onEditMessage={noop}
        onDeleteMessage={noop}
        pinnedIds={new Set(['b'])}
        onTogglePin={noop}
      />
    );
    expect(screen.getAllByLabelText('Pinned')).toHaveLength(1);
  });
});

describe('ChatHeader pin button', () => {
  const header = (over: Partial<React.ComponentProps<typeof ChatHeader>> = {}) =>
    render(
      <ChatHeader
        currentChannel="general"
        channels={[]}
        messagesCount={3}
        onStartVideoCall={noop}
        onStartAudioCall={noop}
        {...over}
      />
    );

  it('shows the count and toggles the panel', () => {
    const onTogglePins = vi.fn();
    header({ pinnedCount: 4, onTogglePins });
    expect(screen.getByText('4')).toBeTruthy();
    fireEvent.click(screen.getByLabelText('Pinned messages'));
    expect(onTogglePins).toHaveBeenCalled();
  });

  it('has no badge for zero pins and no button when pins are not wired', () => {
    const { unmount } = header({ pinnedCount: 0, onTogglePins: noop });
    expect(document.querySelector('.pins-btn-badge')).toBeNull();
    unmount();
    header();
    expect(screen.queryByLabelText('Pinned messages')).toBeNull();
  });

  it('reflects that the panel is open', () => {
    header({ pinnedCount: 1, onTogglePins: noop, pinsOpen: true });
    expect(screen.getByLabelText('Pinned messages').getAttribute('aria-pressed')).toBe('true');
  });
});
