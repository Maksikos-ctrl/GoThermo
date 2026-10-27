import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { TypingIndicator } from '../TypingIndicator';
import { ChannelSidebar } from '../ChannelSidebar';
import { MessageComposer } from '../MessageComposer';
import type { Channel } from '../../types';

describe('TypingIndicator', () => {
  it('is empty when nobody types, but still takes its place', () => {
    const { container } = render(<TypingIndicator users={[]} />);
    const el = container.querySelector('.typing-indicator')!;
    expect(el).toBeTruthy();
    expect(el.textContent).toBe('');
  });

  it('says who is typing', () => {
    render(<TypingIndicator users={['alice', 'bob']} />);
    expect(screen.getByText('alice and bob are typing…')).toBeTruthy();
  });
});

const ch = (name: string): Channel => ({
  id: name,
  name,
  description: '',
  members: [],
  createdBy: 'system',
  createdAt: '2026-01-01T00:00:00Z',
  isPrivate: false,
});

function sidebar(unread: Record<string, number>, mentions?: Record<string, number>) {
  return render(
    <ChannelSidebar
      channels={[ch('general'), ch('random')]}
      dmChannels={[]}
      currentChannel="general"
      isLoading={false}
      currentUser="alice"
      unreadCounts={unread}
      mentionCounts={mentions}
      onChannelChange={vi.fn()}
      onCreateChannel={vi.fn()}
      onDeleteChannel={vi.fn()}
      onDeleteDM={vi.fn()}
      isDragging={null}
      dragOver={null}
      onDragStart={vi.fn()}
      onDragOver={vi.fn()}
      onDragLeave={vi.fn()}
      onDrop={vi.fn()}
    />
  );
}

describe('channel list badges', () => {
  it('shows only the unread count when nobody mentioned you', () => {
    const { container } = sidebar({ random: 3 });
    expect(container.querySelectorAll('.unread-badge').length).toBe(1);
    expect(container.querySelector('.mention-badge')).toBeNull();
  });

  it('shows @N next to the unread count', () => {
    const { container } = sidebar({ random: 5 }, { random: 2 });
    expect(container.querySelector('.mention-badge')!.textContent).toBe('@2');
    expect(container.querySelector('.unread-badge')!.textContent).toBe('5');
  });

  it('still shows a mention when the unread count is zero', () => {
    const { container } = sidebar({}, { random: 1 });
    expect(container.querySelector('.mention-badge')!.textContent).toBe('@1');
    expect(container.querySelector('.unread-badge')).toBeNull();
  });

  it('caps huge numbers', () => {
    const { container } = sidebar({ random: 500 }, { random: 250 });
    expect(container.querySelector('.mention-badge')!.textContent).toBe('@99+');
    expect(container.querySelector('.unread-badge')!.textContent).toBe('99+');
  });

  it('keeps working without the new prop', () => {
    const { container } = sidebar({ random: 1 });
    expect(container.querySelector('.unread-badge')).toBeTruthy();
  });
});

describe('composer typing callback', () => {
  function composer(onTyping?: () => void) {
    const onMessageChange = vi.fn();
    render(
      <MessageComposer
        currentChannel="general"
        isPostMode={false}
        message=""
        onSend={vi.fn()}
        onMessageChange={onMessageChange}
        onTogglePostMode={vi.fn()}
        onTyping={onTyping}
      />
    );
    return { box: screen.getByRole('textbox') as HTMLTextAreaElement, onMessageChange };
  }

  it('is called when text is typed', () => {
    const onTyping = vi.fn();
    const { box, onMessageChange } = composer(onTyping);
    fireEvent.change(box, { target: { value: 'hel' } });
    expect(onMessageChange).toHaveBeenCalledWith('hel');
    expect(onTyping).toHaveBeenCalledTimes(1);
  });

  it('is not called when the box is emptied or holds only spaces', () => {
    const onTyping = vi.fn();
    const { box } = composer(onTyping);
    fireEvent.change(box, { target: { value: '   ' } });
    fireEvent.change(box, { target: { value: '' } });
    expect(onTyping).not.toHaveBeenCalled();
  });

  it('works without the callback', () => {
    const { box, onMessageChange } = composer();
    expect(() => fireEvent.change(box, { target: { value: 'x' } })).not.toThrow();
    expect(onMessageChange).toHaveBeenCalledWith('x');
  });
});
