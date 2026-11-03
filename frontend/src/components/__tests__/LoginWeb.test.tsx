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

  it('has no invite code field in the browser: the invitation link carries the code', () => {
    vi.stubEnv('VITE_WEB', '1');
    serverSays('');
    render(<Login onLogin={() => {}} />);
    fireEvent.click(screen.getAllByText('Sign Up')[0]);
    expect(screen.queryByPlaceholderText('Invite code')).toBeNull();
  });

  it('points to the invitation link when a code is missing', async () => {
    vi.stubEnv('VITE_WEB', '1');
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => ({ error: 'This server needs an invite code to register. Add it under Server, Change.' }),
    }));
    render(<Login onLogin={() => {}} />);
    fireEvent.click(screen.getAllByText('Sign Up')[0]);
    fireEvent.change(screen.getByPlaceholderText('Work Email'), { target: { value: 'a@example.com' } });
    fireEvent.change(screen.getByPlaceholderText('Password'), { target: { value: 'secret1' } });
    fireEvent.change(screen.getByPlaceholderText('Confirm Password'), { target: { value: 'secret1' } });
    fireEvent.click(document.querySelector('button.login-button') as HTMLButtonElement);
    await waitFor(() => expect(screen.getByText(/invitation link/)).toBeTruthy());
    expect(screen.queryByText(/Server, Change/)).toBeNull();
  });

  it('opens on sign-up and keeps the code from the link', () => {
    vi.stubEnv('VITE_WEB', '1');
    serverSays('');
    window.history.replaceState(null, '', '/app/?invite=fromlink');
    render(<Login onLogin={() => {}} />);
    expect(getInviteCode()).toBe('fromlink');
    expect(screen.queryByPlaceholderText('Invite code')).toBeNull();
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
