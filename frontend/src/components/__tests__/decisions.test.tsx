import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import {
  DecisionEntry,
  canRemoveDecision,
  decisionMeta,
  filterDecisions,
  normalizeDecisions,
  openTaskCount,
} from '../decisionState';
import { MessageItem } from '../MessageItem';

const get = vi.fn();
const setDone = vi.fn();
const remove = vi.fn();
vi.mock('../../services/api', () => ({
  api: {
    decisions: {
      get: (...a: unknown[]) => get(...a),
      setDone: (...a: unknown[]) => setDone(...a),
      remove: (...a: unknown[]) => remove(...a),
    },
  },
}));

import { DecisionsPanel } from '../DecisionsPanel';

const entry = (over: Partial<DecisionEntry>): DecisionEntry => ({
  messageId: 'm1',
  channel: 'general',
  kind: 'decision',
  text: 'We ship on Friday',
  author: 'alice',
  markedBy: 'alice',
  done: false,
  at: '2026-11-06T08:00:00Z',
  ...over,
});

const list = [
  entry({ messageId: 'd1', text: 'Use Postgres' }),
  entry({ messageId: 't1', kind: 'task', text: 'Deploy', assignee: 'bob', markedBy: 'carol' }),
  entry({ messageId: 't2', kind: 'task', text: 'Write docs', done: true, doneBy: 'bob' }),
];

describe('decision helpers', () => {
  it('filters and counts', () => {
    expect(filterDecisions(list, 'all')).toHaveLength(3);
    expect(filterDecisions(list, 'decisions').map(d => d.messageId)).toEqual(['d1']);
    expect(filterDecisions(list, 'open').map(d => d.messageId)).toEqual(['t1']);
    expect(filterDecisions(list, 'done').map(d => d.messageId)).toEqual(['t2']);
    expect(openTaskCount(list)).toBe(1);
  });

  it('ignores junk from the server', () => {
    expect(normalizeDecisions(null)).toEqual([]);
    expect(normalizeDecisions([{ kind: 'task' }, { messageId: 'x', kind: 'wish' }, list[0]])).toEqual([list[0]]);
  });

  it('describes who did what, and who may remove', () => {
    expect(decisionMeta(list[1])).toBe('by alice · marked by carol · for @bob');
    expect(decisionMeta(list[2])).toBe('by alice · done by bob');
    expect(canRemoveDecision(list[1], 'carol')).toBe(true);
    expect(canRemoveDecision(list[1], 'alice')).toBe(true);
    expect(canRemoveDecision(list[1], 'bob')).toBe(false);
  });
});

describe('DecisionsPanel', () => {
  beforeEach(() => {
    get.mockReset().mockResolvedValue(list);
    setDone.mockReset().mockResolvedValue(null);
    remove.mockReset().mockResolvedValue(null);
  });

  const show = (user = 'bob') =>
    render(
      <DecisionsPanel channel="general" channelTag="#general" currentUser={user} refreshKey={0} onClose={() => {}} />
    );

  it('lists entries and filters them', async () => {
    show();
    expect(await screen.findByText('Use Postgres')).toBeTruthy();
    fireEvent.click(screen.getByRole('tab', { name: 'Open tasks' }));
    expect(screen.queryByText('Use Postgres')).toBeNull();
    expect(screen.getByText('Deploy')).toBeTruthy();
  });

  it('ticks a task off and reloads', async () => {
    show();
    await screen.findByText('Deploy');
    fireEvent.click(screen.getByLabelText('Mark task as done'));
    await waitFor(() => expect(setDone).toHaveBeenCalledWith('general', 't1', 'bob', true));
    await waitFor(() => expect(get).toHaveBeenCalledTimes(2));
  });

  it('offers removal only to the one who marked it or the author', async () => {
    show('bob');
    await screen.findByText('Deploy');
    expect(screen.queryByLabelText(/^Remove/)).toBeNull();
  });

  it('removes an entry', async () => {
    show('carol');
    await screen.findByText('Deploy');
    fireEvent.click(screen.getByLabelText('Remove task by alice'));
    await waitFor(() => expect(remove).toHaveBeenCalledWith('general', 't1', 'carol'));
  });

  it('shows an empty state and a load error', async () => {
    get.mockResolvedValueOnce([]);
    const { unmount } = show();
    expect(await screen.findByText('Nothing here yet')).toBeTruthy();
    unmount();
    get.mockRejectedValueOnce('boom');
    show();
    expect((await screen.findByRole('alert')).textContent).toMatch(/boom/);
  });
});

describe('marking from a message', () => {
  const noop = () => {};
  const msg = { id: 'm1', channel: 'general', reactions: {}, user: 'bob', text: 'hello', timestamp: '2026-01-01T12:00:00Z' };

  it('offers decision or task and passes the choice on', async () => {
    const onMark = vi.fn();
    const { container } = render(
      <MessageItem
        message={msg as any}
        currentUser="alice"
        onAddReaction={noop}
        onEditMessage={noop}
        onDeleteMessage={noop}
        onMarkDecision={onMark}
      />
    );
    fireEvent.mouseEnter(container.querySelector('.message') as Element);
    fireEvent.click(await screen.findByLabelText('Add to decisions'));
    fireEvent.click(screen.getByText('Mark as task'));
    expect(onMark).toHaveBeenCalledWith(expect.objectContaining({ id: 'm1' }), 'task');
    expect(screen.queryByRole('menu')).toBeNull();
  });
});
