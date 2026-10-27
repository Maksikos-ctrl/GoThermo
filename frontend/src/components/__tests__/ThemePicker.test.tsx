import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { ThemePicker } from '../ThemePicker';
import { STORAGE_KEY } from '../themeAccent';

const store: Record<string, string> = {};
const fake = {
  getItem: (k: string) => (k in store ? store[k] : null),
  setItem: (k: string, v: string) => { store[k] = String(v); },
  removeItem: (k: string) => { delete store[k]; },
};

beforeEach(() => {
  for (const k of Object.keys(store)) delete store[k];
  Object.defineProperty(globalThis, 'localStorage', { value: fake, configurable: true });
  document.documentElement.removeAttribute('style');
});

const accent = () => document.documentElement.style.getPropertyValue('--accent');

describe('ThemePicker', () => {
  it('renders nothing when closed', () => {
    const { container } = render(<ThemePicker isOpen={false} onClose={vi.fn()} />);
    expect(container.firstChild).toBeNull();
  });

  it('shows the presets with the default selected', () => {
    render(<ThemePicker isOpen onClose={vi.fn()} />);
    expect(screen.getAllByRole('radio').length).toBeGreaterThanOrEqual(6);
    expect(screen.getByRole('radio', { name: 'Blurple' }).getAttribute('aria-checked')).toBe('true');
  });

  it('choosing a preset applies it at once and remembers it', () => {
    render(<ThemePicker isOpen onClose={vi.fn()} />);
    fireEvent.click(screen.getByRole('radio', { name: 'Forest' }));
    expect(accent()).toBe('#2b8a3e');
    expect(store[STORAGE_KEY]).toBe('#2b8a3e');
    expect(screen.getByRole('radio', { name: 'Forest' }).getAttribute('aria-checked')).toBe('true');
    expect(screen.getByRole('radio', { name: 'Blurple' }).getAttribute('aria-checked')).toBe('false');
  });

  it('opens on the saved colour', () => {
    store[STORAGE_KEY] = '#d6336c';
    render(<ThemePicker isOpen onClose={vi.fn()} />);
    expect(screen.getByRole('radio', { name: 'Rose' }).getAttribute('aria-checked')).toBe('true');
  });

  it('a custom colour is applied after Apply', () => {
    render(<ThemePicker isOpen onClose={vi.fn()} />);
    fireEvent.change(screen.getByLabelText('Your own colour'), { target: { value: '8e44ad' } });
    fireEvent.click(screen.getByText('Apply'));
    expect(accent()).toBe('#8e44ad');
    expect(store[STORAGE_KEY]).toBe('#8e44ad');
  });

  it('a malformed or too light colour cannot be applied and says why', () => {
    render(<ThemePicker isOpen onClose={vi.fn()} />);
    const input = screen.getByLabelText('Your own colour');
    const apply = screen.getByText('Apply') as HTMLButtonElement;

    fireEvent.change(input, { target: { value: 'blue' } });
    expect(apply.disabled).toBe(true);
    expect(screen.getByRole('alert').textContent).toMatch(/colour like/i);

    fireEvent.change(input, { target: { value: '#ffee88' } });
    expect(apply.disabled).toBe(true);
    expect(screen.getByRole('alert').textContent).toMatch(/too light/i);
    fireEvent.click(apply);
    expect(accent()).toBe('');
  });

  it('Enter applies a valid custom colour', () => {
    render(<ThemePicker isOpen onClose={vi.fn()} />);
    const input = screen.getByLabelText('Your own colour');
    fireEvent.change(input, { target: { value: '#0b7fc4' } });
    fireEvent.keyDown(input, { key: 'Enter' });
    expect(accent()).toBe('#0b7fc4');
  });

  it('reset goes back to the default and clears the saved colour', () => {
    render(<ThemePicker isOpen onClose={vi.fn()} />);
    fireEvent.click(screen.getByRole('radio', { name: 'Rose' }));
    fireEvent.click(screen.getByText('Reset to default'));
    expect(accent()).toBe('');
    expect(store[STORAGE_KEY]).toBeUndefined();
  });

  it('closes with Escape, the X button and a click outside', () => {
    const onClose = vi.fn();
    const { container } = render(<ThemePicker isOpen onClose={onClose} />);
    fireEvent.keyDown(document, { key: 'Escape' });
    fireEvent.click(screen.getByLabelText('Close'));
    fireEvent.mouseDown(container.querySelector('.theme-overlay')!);
    expect(onClose).toHaveBeenCalledTimes(3);
  });

  it('a click inside the dialog does not close it', () => {
    const onClose = vi.fn();
    render(<ThemePicker isOpen onClose={onClose} />);
    fireEvent.mouseDown(screen.getByRole('dialog'));
    expect(onClose).not.toHaveBeenCalled();
  });
});
