import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
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
});

describe('sign-in screen in the browser', () => {
  it('has an invite code field on sign-up and no Google button', () => {
    vi.stubEnv('VITE_WEB', '1');
    render(<Login onLogin={() => {}} />);
    expect(screen.queryByText(/Sign in with Google/)).toBeNull();
    fireEvent.click(screen.getAllByText('Sign Up')[0]);
    const field = screen.getByPlaceholderText('Invite code') as HTMLInputElement;
    fireEvent.change(field, { target: { value: 'secret' } });
    expect(getInviteCode()).toBe('secret');
  });

  it('opens on sign-up with the code filled in when the link carries one', () => {
    vi.stubEnv('VITE_WEB', '1');
    window.history.replaceState(null, '', '/app/?invite=fromlink');
    render(<Login onLogin={() => {}} />);
    expect((screen.getByPlaceholderText('Invite code') as HTMLInputElement).value).toBe('fromlink');
    expect(screen.getByPlaceholderText('Confirm Password')).toBeTruthy();
    expect(window.location.search).toBe('');
  });

  it('the desktop app keeps the Google button and has no invite field', () => {
    render(<Login onLogin={() => {}} />);
    expect(screen.getByText(/Sign in with Google/)).toBeTruthy();
    fireEvent.click(screen.getAllByText('Sign Up')[0]);
    expect(screen.queryByPlaceholderText('Invite code')).toBeNull();
  });
});
