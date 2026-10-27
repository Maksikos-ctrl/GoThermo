import { describe, it, expect, beforeEach } from 'vitest';
import {
  ACCENT_PRESETS,
  DEFAULT_ACCENT,
  STORAGE_KEY,
  accentVars,
  applyAccent,
  applySavedAccent,
  contrastWithWhite,
  darken,
  isReadableAccent,
  loadAccent,
  normalizeHex,
  saveAccent,
} from '../themeAccent';

// The test environment may not have a usable localStorage.
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

describe('normalizeHex', () => {
  it('accepts 3 and 6 digits, with or without #, any case', () => {
    expect(normalizeHex('#AABBCC')).toBe('#aabbcc');
    expect(normalizeHex('abc')).toBe('#aabbcc');
    expect(normalizeHex('  #2B8A3E ')).toBe('#2b8a3e');
  });
  it('rejects everything else', () => {
    for (const bad of ['', '#12', '#12345', '#1234567', 'red', '#ggg', 'rgb(1,2,3)']) {
      expect(normalizeHex(bad)).toBeNull();
    }
  });
});

describe('colour maths', () => {
  it('darken mixes with black', () => {
    expect(darken('#ffffff', 0)).toBe('#ffffff');
    expect(darken('#ffffff', 1)).toBe('#000000');
    expect(darken('#646464', 0.5)).toBe('#323232');
  });
  it('white on black is 21, white on white is 1', () => {
    expect(contrastWithWhite('#000000')).toBeCloseTo(21, 0);
    expect(contrastWithWhite('#ffffff')).toBeCloseTo(1, 1);
  });
  it('matches known WCAG values', () => {
    expect(contrastWithWhite('#777777')).toBeCloseTo(4.48, 1);
    expect(contrastWithWhite('#0a0a0a')).toBeGreaterThan(19);
  });
  it('every preset is readable with white text', () => {
    for (const p of ACCENT_PRESETS) expect(isReadableAccent(p.color), p.name).toBe(true);
  });
  it('a pale colour is not readable', () => {
    expect(isReadableAccent('#ffee88')).toBe(false);
    expect(isReadableAccent('#ffffff')).toBe(false);
  });
  it('presets are unique and the first is the default', () => {
    expect(new Set(ACCENT_PRESETS.map(p => p.color)).size).toBe(ACCENT_PRESETS.length);
    expect(ACCENT_PRESETS[0]!.color).toBe(DEFAULT_ACCENT);
  });
  it('the hover colour is darker', () => {
    const v = accentVars('#2b8a3e');
    expect(v['--accent']).toBe('#2b8a3e');
    expect(contrastWithWhite(v['--accent-hover']!)).toBeGreaterThan(contrastWithWhite('#2b8a3e'));
  });
});

describe('applyAccent', () => {
  const css = (n: string) => document.documentElement.style.getPropertyValue(n);

  it('sets both variables on <html>', () => {
    applyAccent('#2b8a3e');
    expect(css('--accent')).toBe('#2b8a3e');
    expect(css('--accent-hover')).not.toBe('');
  });
  it('null and the default colour remove the override', () => {
    applyAccent('#2b8a3e');
    applyAccent(null);
    expect(css('--accent')).toBe('');
    applyAccent('#2b8a3e');
    applyAccent(DEFAULT_ACCENT);
    expect(css('--accent')).toBe('');
    expect(css('--accent-hover')).toBe('');
  });
  it('ignores junk', () => {
    applyAccent('not a colour');
    expect(css('--accent')).toBe('');
  });
});

describe('saving', () => {
  it('round-trips and applies at start-up', () => {
    saveAccent('#d6336c');
    expect(loadAccent()).toBe('#d6336c');
    applySavedAccent();
    expect(document.documentElement.style.getPropertyValue('--accent')).toBe('#d6336c');
  });
  it('saving the default clears the entry', () => {
    saveAccent('#d6336c');
    saveAccent(DEFAULT_ACCENT);
    expect(store[STORAGE_KEY]).toBeUndefined();
    expect(loadAccent()).toBeNull();
  });
  it('a damaged or unreadable stored value is ignored', () => {
    store[STORAGE_KEY] = 'banana';
    expect(loadAccent()).toBeNull();
    store[STORAGE_KEY] = '#ffffff';
    expect(loadAccent()).toBeNull();
  });
  it('works without usable storage', () => {
    Object.defineProperty(globalThis, 'localStorage', {
      value: { getItem() { throw new Error('blocked'); }, setItem() { throw new Error('blocked'); }, removeItem() { throw new Error('blocked'); } },
      configurable: true,
    });
    expect(() => saveAccent('#d6336c')).not.toThrow();
    expect(loadAccent()).toBeNull();
  });
});
