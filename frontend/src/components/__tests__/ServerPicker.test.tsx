import { describe, it, expect, beforeEach, vi } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { ServerPicker } from '../ServerPicker';
import { getInviteCode, getServerUrl, setInviteCode, setServerUrl } from '../../services/server';

beforeEach(() => {
  setServerUrl('');
  setInviteCode('');
});

describe('ServerPicker', () => {
  it('shows the local backend by default', () => {
    render(<ServerPicker />);
    expect(screen.getByText(/Server: this computer/)).toBeTruthy();
  });

  it('connects to a reachable GoThermo server and remembers it', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ json: async () => ({ ok: true, app: 'gothermo' }) }));
    render(<ServerPicker />);
    fireEvent.click(screen.getByLabelText('Change server'));
    fireEvent.change(screen.getByLabelText('Server address'), { target: { value: 'chat.example.com' } });
    fireEvent.click(screen.getByText('Connect'));
    await waitFor(() => expect(getServerUrl()).toBe('https://chat.example.com'));
    expect(screen.getByText(/Server: chat.example.com/)).toBeTruthy();
  });

  it('keeps the old choice and shows an error when the server cannot be reached', async () => {
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new Error('down')));
    render(<ServerPicker />);
    fireEvent.click(screen.getByLabelText('Change server'));
    fireEvent.change(screen.getByLabelText('Server address'), { target: { value: 'chat.example.com' } });
    fireEvent.click(screen.getByText('Connect'));
    await screen.findByRole('alert');
    expect(getServerUrl()).toBe('');
  });

  it('switches back to this computer', () => {
    setServerUrl('https://chat.example.com');
    render(<ServerPicker />);
    fireEvent.click(screen.getByLabelText('Change server'));
    fireEvent.click(screen.getByText('Use this computer'));
    expect(getServerUrl()).toBe('');
    expect(screen.getByText(/Server: this computer/)).toBeTruthy();
  });

  it('saves the invite code together with the server', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ json: async () => ({ ok: true, app: 'gothermo' }) }));
    render(<ServerPicker />);
    fireEvent.click(screen.getByLabelText('Change server'));
    fireEvent.change(screen.getByLabelText('Server address'), { target: { value: 'chat.example.com' } });
    fireEvent.change(screen.getByLabelText(/Invite code/), { target: { value: '  s3cret ' } });
    fireEvent.click(screen.getByText('Connect'));
    await waitFor(() => expect(getServerUrl()).toBe('https://chat.example.com'));
    expect(getInviteCode()).toBe('s3cret');
  });
});
