import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { Login } from '../Login';
import { getInviteCode, setInviteCode } from '../../services/server';

const store: Record<string, string> = {};
vi.stubGlobal('localStorage', {
  getItem: (k: string) => (k in store ? store[k] : null),
  setItem: (k: string, v: string) => { store[k] = String(v); },
  removeItem: (k: string) => { delete store[k]; },
});

beforeEach(() => {
  setInviteCode('');
  window.history.replaceState(null, '', '/app/');
});

afterEach(() => {
  vi.unstubAllEnvs();
  vi.unstubAllGlobals();
  vi.stubGlobal('localStorage', { getItem: (k: string) => (k in store ? store[k] : null), setItem: (k: string, v: string) => { store[k] = String(v); }, removeItem: (k: string) => { delete store[k]; } });
});

describe('sign-in screen in the browser', () => {
  const serverSays = (clientId: string) =>
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: true, status: 200, json: async () => ({ result: clientId }) }));

  it('shows the Google button only when the server has a browser client', async () => {
    vi.stubEnv('VITE_WEB', '1');
    serverSays('web-client');
    render(<Login onLogin={() => {}} />);
    await waitFor(() => expect(screen.getByText(/Sign in with Google/)).toBeTruthy());
  });

  it('hides the Google button when the server has none', async () => {
    vi.stubEnv('VITE_WEB', '1');
    serverSays('');
    render(<Login onLogin={() => {}} />);
    await new Promise((r) => setTimeout(r, 20));
    expect(screen.queryByText(/Sign in with Google/)).toBeNull();
  });

  it('has an invite code field on sign-up', () => {
    vi.stubEnv('VITE_WEB', '1');
    serverSays('');
    render(<Login onLogin={() => {}} />);
    fireEvent.click(screen.getAllByText('Sign Up')[0]);
    const field = screen.getByPlaceholderText('Invite code') as HTMLInputElement;
    fireEvent.change(field, { target: { value: 'secret' } });
    expect(getInviteCode()).toBe('secret');
  });

  it('opens on sign-up with the code filled in when the link carries one', () => {
    vi.stubEnv('VITE_WEB', '1');
    serverSays('');
    window.history.replaceState(null, '', '/app/?invite=fromlink');
    render(<Login onLogin={() => {}} />);
    expect((screen.getByPlaceholderText('Invite code') as HTMLInputElement).value).toBe('fromlink');
    expect(screen.getByPlaceholderText('Confirm Password')).toBeTruthy();
    expect(window.location.search).toBe('');
  });

  it('a desktop app connected to a server also asks for the invite code', async () => {
    vi.stubEnv('VITE_DEFAULT_SERVER', 'https://team.example.org');
    vi.resetModules();
    const { Login: FreshLogin } = await import('../Login');
    render(<FreshLogin onLogin={() => {}} />);
    fireEvent.click(screen.getAllByText('Sign Up')[0]);
    expect(screen.getByPlaceholderText('Invite code')).toBeTruthy();
  });

  it('the desktop app keeps the Google button and has no invite field', () => {
    render(<Login onLogin={() => {}} />);
    expect(screen.getByText(/Sign in with Google/)).toBeTruthy();
    fireEvent.click(screen.getAllByText('Sign Up')[0]);
    expect(screen.queryByPlaceholderText('Invite code')).toBeNull();
  });
});
