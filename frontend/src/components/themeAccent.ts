// Accent colour themes. The base light/dark choice is untouched; this layer
// only replaces --accent and --accent-hover (buttons, links, badges, marks).
// They are set as inline variables on <html>, which beats both :root rules.

export interface AccentPreset {
  id: string;
  name: string;
  color: string;
}

// The first one is the app's own colour: choosing it removes the override.
export const DEFAULT_ACCENT = '#5865f2';

export const ACCENT_PRESETS: AccentPreset[] = [
  { id: 'blurple', name: 'Blurple', color: DEFAULT_ACCENT },
  { id: 'ocean', name: 'Ocean', color: '#0b7fc4' },
  { id: 'teal', name: 'Teal', color: '#0c8599' },
  { id: 'forest', name: 'Forest', color: '#2b8a3e' },
  { id: 'sunset', name: 'Sunset', color: '#d9480f' },
  { id: 'rose', name: 'Rose', color: '#d6336c' },
  { id: 'violet', name: 'Violet', color: '#7048e8' },
  { id: 'graphite', name: 'Graphite', color: '#5c677d' },
];

export const STORAGE_KEY = 'gothermo.accent';

// "#abc", "abc", "#AABBCC" -> "#aabbcc"; anything else -> null.
export function normalizeHex(input: string): string | null {
  const s = input.trim().replace(/^#/, '').toLowerCase();
  if (/^[0-9a-f]{3}$/.test(s)) return '#' + s.split('').map(c => c + c).join('');
  if (/^[0-9a-f]{6}$/.test(s)) return '#' + s;
  return null;
}

function rgb(hex: string): [number, number, number] {
  const n = parseInt(hex.slice(1), 16);
  return [(n >> 16) & 255, (n >> 8) & 255, n & 255];
}

const toHex = (r: number, g: number, b: number) =>
  '#' + [r, g, b].map(v => Math.max(0, Math.min(255, Math.round(v))).toString(16).padStart(2, '0')).join('');

// Mix with black: amount 0 = same colour, 1 = black.
export function darken(hex: string, amount: number): string {
  const [r, g, b] = rgb(hex);
  const k = 1 - amount;
  return toHex(r * k, g * k, b * k);
}

function luminance(hex: string): number {
  const lin = (v: number) => {
    const c = v / 255;
    return c <= 0.03928 ? c / 12.92 : Math.pow((c + 0.055) / 1.055, 2.4);
  };
  const [r, g, b] = rgb(hex);
  return 0.2126 * lin(r) + 0.7152 * lin(g) + 0.0722 * lin(b);
}

// WCAG contrast of white text on this colour (1 = unreadable, 21 = best).
export function contrastWithWhite(hex: string): number {
  return 1.05 / (luminance(hex) + 0.05);
}

// The app writes white text on the accent, so it has to stay dark enough.
export const MIN_ACCENT_CONTRAST = 3;

export function isReadableAccent(hex: string): boolean {
  return contrastWithWhite(hex) >= MIN_ACCENT_CONTRAST;
}

export function accentVars(hex: string): Record<string, string> {
  return { '--accent': hex, '--accent-hover': darken(hex, 0.18) };
}

// null (or the default colour) removes the override.
export function applyAccent(hex: string | null, el: HTMLElement = document.documentElement): void {
  const clean = hex && normalizeHex(hex);
  if (!clean || clean === DEFAULT_ACCENT) {
    el.style.removeProperty('--accent');
    el.style.removeProperty('--accent-hover');
    return;
  }
  for (const [k, v] of Object.entries(accentVars(clean))) el.style.setProperty(k, v);
}

export function loadAccent(): string | null {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    const hex = raw ? normalizeHex(raw) : null;
    return hex && isReadableAccent(hex) ? hex : null;
  } catch {
    return null;
  }
}

export function saveAccent(hex: string | null): void {
  try {
    if (!hex || hex === DEFAULT_ACCENT) localStorage.removeItem(STORAGE_KEY);
    else localStorage.setItem(STORAGE_KEY, hex);
  } catch {
    // private mode or blocked storage: the colour just lasts until the app closes
  }
}

export function applySavedAccent(): void {
  applyAccent(loadAccent());
}
