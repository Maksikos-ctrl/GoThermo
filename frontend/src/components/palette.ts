import { GENERATED_PALETTES, PaletteInfo } from './palettes.generated';

// Background palettes for the dark theme (Midnight, Forest, ...). The rules
// live in styles/palettes.generated.css and switch on with <html data-palette>.
// The light theme ignores them. See frontend/scripts/gen-palettes.mjs.

export const DEFAULT_PALETTE: PaletteInfo = { id: 'default', name: 'Default', bg: '#1e1f22', panel: '#2b2d31', line: '#3f4147' };

export const PALETTES: PaletteInfo[] = [DEFAULT_PALETTE, ...GENERATED_PALETTES];

export const PALETTE_KEY = 'gothermo.palette';

export const isPaletteId = (id: unknown): id is string =>
  typeof id === 'string' && PALETTES.some(p => p.id === id);

export function applyPalette(id: string | null, el: HTMLElement = document.documentElement): void {
  if (!id || id === DEFAULT_PALETTE.id || !isPaletteId(id)) {
    el.removeAttribute('data-palette');
    return;
  }
  el.setAttribute('data-palette', id);
}

export function loadPalette(): string {
  try {
    const raw = localStorage.getItem(PALETTE_KEY);
    return isPaletteId(raw) ? raw : DEFAULT_PALETTE.id;
  } catch {
    return DEFAULT_PALETTE.id;
  }
}

export function savePalette(id: string): void {
  try {
    if (id === DEFAULT_PALETTE.id) localStorage.removeItem(PALETTE_KEY);
    else localStorage.setItem(PALETTE_KEY, id);
  } catch {
    // blocked storage: the palette lasts until the app closes
  }
}

export function applySavedPalette(): void {
  applyPalette(loadPalette());
}
