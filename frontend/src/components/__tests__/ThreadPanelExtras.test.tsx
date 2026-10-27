import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, act } from '@testing-library/react';
import { ThreadPanel } from '../ThreadPanel';
import { MessageItem } from '../MessageItem';
import { threadUnread } from '../threadExtras';
import type { Message } from '../../types';

const msg = (id: string, user: string, text: string, extra: Partial<Message> = {}): Message => ({
  id,
  channel: 'general',
  reactions: {},
  user,
  text,
  timestamp: '2026-01-01T12:00:00Z',
  ...extra,
});

const parent = msg('p', 'bob', 'the parent', { replyCount: 3 });
const replies = [msg('r1', 'alice', 'deploy finished'), msg('r2', 'carol', 'thanks'), msg('r3', 'alice', 'Deploy log attached')];

function setup(over: Partial<React.ComponentProps<typeof ThreadPanel>> = {}) {
  const props = {
    thread: { parent, replies },
    loading: false,
    error: null,
    channelTag: '#general',
    currentUser: 'alice',
    knownUsernames: ['alice', 'bob', 'carol'],
    onClose: vi.fn(),
    onSend: vi.fn().mockResolvedValue(undefined),
    onAddReaction: vi.fn(),
    onEditMessage: vi.fn(),
    onDeleteMessage: vi.fn(),
    ...over,
  };
  return { props, ...render(<ThreadPanel {...props} />) };
}

describe('search in a thread', () => {
  it('is closed at first and shows every reply', () => {
    setup();
    expect(screen.queryByPlaceholderText(/search in this thread/i)).toBeNull();
    expect(screen.getByText('3 replies')).toBeTruthy();
    expect(screen.getByText('thanks')).toBeTruthy();
  });

  it('filters replies, keeps the parent, and counts the matches', () => {
    setup();
    fireEvent.click(screen.getByLabelText('Search in thread', { selector: 'button' }));
    fireEvent.change(screen.getByPlaceholderText(/search in this thread/i), { target: { value: 'deploy' } });
    expect(screen.getByText('2 of 3 match')).toBeTruthy();
    expect(screen.queryByText('thanks')).toBeNull();
    expect(screen.getByText('the parent')).toBeTruthy();
  });

  it('says so when nothing matches', () => {
    setup();
    fireEvent.click(screen.getByLabelText('Search in thread', { selector: 'button' }));
    fireEvent.change(screen.getByPlaceholderText(/search in this thread/i), { target: { value: 'zzz' } });
    expect(screen.getByText('No replies match')).toBeTruthy();
  });

  it('Escape closes the search and brings every reply back', () => {
    setup();
    fireEvent.click(screen.getByLabelText('Search in thread', { selector: 'button' }));
    const box = screen.getByPlaceholderText(/search in this thread/i);
    fireEvent.change(box, { target: { value: 'deploy' } });
    fireEvent.keyDown(box, { key: 'Escape' });
    expect(screen.queryByPlaceholderText(/search in this thread/i)).toBeNull();
    expect(screen.getByText('thanks')).toBeTruthy();
    expect(screen.getByText('3 replies')).toBeTruthy();
  });

  it('does not close the panel while searching', () => {
    const { props } = setup();
    fireEvent.click(screen.getByLabelText('Search in thread', { selector: 'button' }));
    fireEvent.keyDown(screen.getByPlaceholderText(/search in this thread/i), { key: 'Escape' });
    expect(props.onClose).not.toHaveBeenCalled();
  });
});

describe('export', () => {
  it('has no export button when the caller gives no handler', () => {
    setup();
    expect(screen.queryByLabelText('Export thread')).toBeNull();
  });

  it('offers the three formats and calls the handler once', () => {
    const onExport = vi.fn();
    setup({ onExport });
    fireEvent.click(screen.getByLabelText('Export thread'));
    expect(screen.getByText('Text (.txt)')).toBeTruthy();
    expect(screen.getByText('JSON (.json)')).toBeTruthy();
    fireEvent.click(screen.getByText('CSV (.csv)'));
    expect(onExport).toHaveBeenCalledTimes(1);
    expect(onExport).toHaveBeenCalledWith('csv');
    expect(screen.queryByText('CSV (.csv)')).toBeNull(); // menu closed
  });

  it('closes the menu on a click elsewhere', () => {
    setup({ onExport: vi.fn() });
    fireEvent.click(screen.getByLabelText('Export thread'));
    fireEvent.mouseDown(document.body);
    expect(screen.queryByText('Text (.txt)')).toBeNull();
  });
});

describe('unread badge on the replies row', () => {
  beforeEach(() => act(() => threadUnread.setAll({})));

  const row = () =>
    render(
      <MessageItem
        message={parent}
        currentUser="alice"
        onAddReaction={vi.fn()}
        onEditMessage={vi.fn()}
        onDeleteMessage={vi.fn()}
        onOpenThread={vi.fn()}
      />
    );

  it('shows nothing when everything is read', () => {
    row();
    expect(screen.queryByText(/ new$/)).toBeNull();
  });

  it('shows "N new" and follows live changes', () => {
    row();
    act(() => threadUnread.setAll({ p: 2 }));
    expect(screen.getByText('2 new')).toBeTruthy();
    act(() => threadUnread.setAll({ p: 3 }));
    expect(screen.getByText('3 new')).toBeTruthy();
    act(() => threadUnread.clear('p'));
    expect(screen.queryByText(/ new$/)).toBeNull();
  });

  it('only the thread with unread replies gets a badge', () => {
    row();
    act(() => threadUnread.setAll({ other: 4 }));
    expect(screen.queryByText(/ new$/)).toBeNull();
  });
});
