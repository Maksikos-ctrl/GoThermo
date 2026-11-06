import { describe, it, expect, beforeEach } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { PALETTES, applyPalette, applySavedPalette, loadPalette, savePalette, PALETTE_KEY } from '../palette';
import { ThemePicker } from '../ThemePicker';

describe('palette', () => {
  beforeEach(() => {
    localStorage.clear();
    document.documentElement.removeAttribute('data-palette');
  });

  it('has the default and the generated ones', () => {
    expect(PALETTES.map(p => p.id)).toEqual(['default', 'midnight', 'forest', 'plum', 'mocha', 'amoled']);
  });

  it('applies a palette to <html> and removes it for the default', () => {
    applyPalette('forest');
    expect(document.documentElement.getAttribute('data-palette')).toBe('forest');
    applyPalette('default');
    expect(document.documentElement.hasAttribute('data-palette')).toBe(false);
  });

  it('ignores unknown names, also from storage', () => {
    applyPalette('hotpink');
    expect(document.documentElement.hasAttribute('data-palette')).toBe(false);
    localStorage.setItem(PALETTE_KEY, 'hotpink');
    expect(loadPalette()).toBe('default');
  });

  it('remembers the choice', () => {
    savePalette('plum');
    expect(loadPalette()).toBe('plum');
    applySavedPalette();
    expect(document.documentElement.getAttribute('data-palette')).toBe('plum');
    savePalette('default');
    expect(localStorage.getItem(PALETTE_KEY)).toBeNull();
  });

  it('is chosen in the theme picker', () => {
    render(<ThemePicker isOpen onClose={() => {}} />);
    fireEvent.click(screen.getByRole('radio', { name: 'Midnight background' }));
    expect(document.documentElement.getAttribute('data-palette')).toBe('midnight');
    expect(loadPalette()).toBe('midnight');
    fireEvent.click(screen.getByRole('radio', { name: 'Default background' }));
    expect(document.documentElement.hasAttribute('data-palette')).toBe(false);
  });
});
