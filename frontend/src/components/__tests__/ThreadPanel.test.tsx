import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { ThreadPanel } from '../ThreadPanel';
import type { Message } from '../../types';

const T0 = new Date('2026-01-01T12:00:00Z').getTime();
const at = (min: number) => new Date(T0 + min * 60_000).toISOString();
const msg = (id: string, user: string, min: number, extra: Partial<Message> = {}): Message => ({
  id,
  channel: 'general',
  reactions: {},
  user,
  text: `text ${id}`,
  timestamp: at(min),
  ...extra,
});

const parent = msg('p', 'bob', 0, { text: 'the parent', replyCount: 2 });

function setup(over: Partial<React.ComponentProps<typeof ThreadPanel>> = {}) {
  const props = {
    thread: { parent, replies: [msg('r1', 'alice', 1), msg('r2', 'carol', 2)] },
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
  const view = render(<ThreadPanel {...props} />);
  const input = screen.getByLabelText('Reply') as HTMLTextAreaElement;
  return { ...view, props, input };
}

describe('ThreadPanel content', () => {
  it('shows the parent, the reply count and the replies in order', () => {
    const { container } = setup();
    const texts = Array.from(container.querySelectorAll('.message-text')).map(e => e.textContent);
    expect(texts).toEqual(['the parent', 'text r1', 'text r2']);
    expect(container.querySelector('.thread-divider')?.textContent).toBe('2 replies');
    expect(container.querySelector('.thread-channel')?.textContent).toBe('#general');
  });

  it('says so when there are no replies yet', () => {
    const { container } = setup({ thread: { parent, replies: [] } });
    expect(container.querySelector('.thread-divider')?.textContent).toBe('No replies yet');
  });

  it('says loading instead of "No replies yet" while the replies load', () => {
    const { container } = setup({ thread: { parent, replies: [] }, loading: true });
    expect(container.querySelector('.thread-divider')?.textContent).toBe('Loading replies…');
  });

  it('shows an error', () => {
    setup({ error: 'Could not load the thread: boom' });
    expect(screen.getByRole('alert').textContent).toContain('boom');
  });

  it('has no thread summary or reply button inside the panel itself', () => {
    const { container } = setup();
    expect(container.querySelector('.thread-summary')).toBeNull();
    fireEvent.mouseEnter(container.querySelector('.message') as Element);
    expect(screen.queryByLabelText('Reply in thread')).toBeNull();
  });

  it('groups consecutive replies of one person, not of different people', () => {
    const { container } = setup({
      thread: {
        parent,
        replies: [msg('a', 'carol', 1), msg('b', 'carol', 2), msg('c', 'alice', 3), msg('d', 'carol', 4)],
      },
    });
    const rows = Array.from(container.querySelectorAll('.thread-body .message')).slice(1); // skip the parent
    expect(rows.map(r => r.classList.contains('grouped'))).toEqual([false, true, false, false]);
  });

  it('starts a new group when the same person pauses for 5 minutes or more', () => {
    const { container } = setup({
      thread: { parent, replies: [msg('a', 'carol', 1), msg('b', 'carol', 4), msg('c', 'carol', 9)] },
    });
    const rows = Array.from(container.querySelectorAll('.thread-body .message')).slice(1);
    expect(rows.map(r => r.classList.contains('grouped'))).toEqual([false, true, false]);
  });

  it('routes a reaction click to the handler with the message id', () => {
    const { container, props } = setup({
      thread: { parent, replies: [msg('r1', 'carol', 1, { reactions: { '🚀': ['bob'] } })] },
    });
    fireEvent.click(container.querySelector('.reaction') as Element);
    expect(props.onAddReaction).toHaveBeenCalledWith('r1', '🚀');
  });
});

describe('ThreadPanel composer', () => {
  it('sends the trimmed text on Enter and clears the field', async () => {
    const { input, props } = setup();
    fireEvent.change(input, { target: { value: '  hello there  ' } });
    fireEvent.keyDown(input, { key: 'Enter' });
    await waitFor(() => expect(props.onSend).toHaveBeenCalledWith('hello there'));
    await waitFor(() => expect(input.value).toBe(''));
  });

  it('Shift+Enter does not send', () => {
    const { input, props } = setup();
    fireEvent.change(input, { target: { value: 'line one' } });
    fireEvent.keyDown(input, { key: 'Enter', shiftKey: true });
    expect(props.onSend).not.toHaveBeenCalled();
    expect(input.value).toBe('line one');
  });

  it('does not send blank text, and the button is disabled for it', () => {
    const { input, props } = setup();
    const btn = screen.getByLabelText('Send reply') as HTMLButtonElement;
    expect(btn.disabled).toBe(true);
    fireEvent.change(input, { target: { value: '   ' } });
    fireEvent.keyDown(input, { key: 'Enter' });
    expect(props.onSend).not.toHaveBeenCalled();
    expect(btn.disabled).toBe(true);
    fireEvent.change(input, { target: { value: 'x' } });
    expect(btn.disabled).toBe(false);
  });

  it('keeps the draft when sending fails', async () => {
    const onSend = vi.fn().mockRejectedValue(new Error('nope'));
    const { input } = setup({ onSend });
    fireEvent.change(input, { target: { value: 'important text' } });
    fireEvent.keyDown(input, { key: 'Enter' });
    await waitFor(() => expect(onSend).toHaveBeenCalled());
    await waitFor(() => expect((screen.getByLabelText('Send reply') as HTMLButtonElement).disabled).toBe(false));
    expect(input.value).toBe('important text');
  });

  it('does not send twice while the first send is still running', async () => {
    let finish: () => void = () => {};
    const onSend = vi.fn().mockImplementation(() => new Promise<void>(res => (finish = res)));
    const { input } = setup({ onSend });
    fireEvent.change(input, { target: { value: 'once' } });
    fireEvent.keyDown(input, { key: 'Enter' });
    fireEvent.keyDown(input, { key: 'Enter' });
    fireEvent.click(screen.getByLabelText('Send reply'));
    expect(onSend).toHaveBeenCalledTimes(1);
    finish();
    await waitFor(() => expect(input.value).toBe(''));
  });

  it('is disabled until the thread is there', () => {
    setup({ thread: null, loading: true });
    expect((screen.getByLabelText('Reply') as HTMLTextAreaElement).disabled).toBe(true);
    expect((screen.getByLabelText('Send reply') as HTMLButtonElement).disabled).toBe(true);
  });
});

describe('ThreadPanel closing', () => {
  it('closes with the X button', () => {
    const { props } = setup();
    fireEvent.click(screen.getByLabelText('Close thread'));
    expect(props.onClose).toHaveBeenCalledTimes(1);
  });

  it('Escape closes only when the draft is empty', () => {
    const { input, props } = setup();
    fireEvent.change(input, { target: { value: 'draft' } });
    fireEvent.keyDown(input, { key: 'Escape' });
    expect(props.onClose).not.toHaveBeenCalled();
    fireEvent.change(input, { target: { value: '' } });
    fireEvent.keyDown(input, { key: 'Escape' });
    expect(props.onClose).toHaveBeenCalledTimes(1);
  });
});
