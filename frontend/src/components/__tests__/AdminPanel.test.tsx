import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';

const api = vi.hoisted(() => ({
  admin: {
    overview: vi.fn(),
    users: vi.fn(),
    reports: vi.fn(),
    ban: vi.fn(),
    unban: vi.fn(),
    dismissReport: vi.fn(),
    deleteReported: vi.fn(),
  },
}));
vi.mock('../../services/api', () => ({ api }));

import { AdminPanel } from '../AdminPanel';

const users = [
  { username: 'ann', email: 'ann@x.io', status: 'online', hasAvatar: false, banned: false },
  { username: 'bob', email: 'bob@x.io', status: 'offline', hasAvatar: false, banned: true },
];
const reports = [
  { id: 'r1', reporter: 'ann', author: 'bob', channel: 'general', messageId: 'm1', reason: 'spam', snippet: 'buy now', at: new Date().toISOString() },
];

beforeEach(() => {
  Object.values(api.admin).forEach(f => f.mockReset());
  api.admin.overview.mockResolvedValue({ users: 2, online: 1, banned: 1, reports: 1 });
  api.admin.users.mockResolvedValue(users);
  api.admin.reports.mockResolvedValue(reports);
  api.admin.ban.mockResolvedValue(undefined);
  api.admin.unban.mockResolvedValue(undefined);
  api.admin.deleteReported.mockResolvedValue(undefined);
});

describe('AdminPanel', () => {
  it('shows the numbers', async () => {
    render(<AdminPanel onLogout={() => {}} />);
    await waitFor(() => expect(screen.getByText('Online now').previousSibling?.textContent).toBe('1'));
    expect(screen.getByText('Open reports').previousSibling?.textContent).toBe('1');
  });

  it('lists users, filters, and bans / unbans', async () => {
    vi.spyOn(window, 'confirm').mockReturnValue(true);
    render(<AdminPanel onLogout={() => {}} />);
    fireEvent.click(await screen.findByRole('tab', { name: /Users/ }));
    expect(await screen.findByText('bob')).toBeTruthy();
    fireEvent.change(screen.getByLabelText('Search users'), { target: { value: 'ann' } });
    expect(screen.queryByText('bob')).toBeNull();
    fireEvent.click(screen.getByText('Ban'));
    await waitFor(() => expect(api.admin.ban).toHaveBeenCalledWith('ann'));
    fireEvent.change(screen.getByLabelText('Search users'), { target: { value: 'bob' } });
    fireEvent.click(screen.getByText('Unban'));
    await waitFor(() => expect(api.admin.unban).toHaveBeenCalledWith('bob'));
  });

  it('handles a report', async () => {
    render(<AdminPanel onLogout={() => {}} />);
    fireEvent.click(await screen.findByRole('tab', { name: /Reports/ }));
    expect(await screen.findByText('buy now')).toBeTruthy();
    fireEvent.click(screen.getByText('Delete message'));
    await waitFor(() => expect(api.admin.deleteReported).toHaveBeenCalledWith('r1'));
  });

  it('signs out', async () => {
    const out = vi.fn();
    render(<AdminPanel onLogout={out} />);
    fireEvent.click(await screen.findByText('Sign out'));
    expect(out).toHaveBeenCalled();
  });
});
