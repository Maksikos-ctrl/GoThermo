import { EMOJI_GROUPS, EmojiItem } from './emojiData';

const RECENT_KEY = 'gothermo.recentEmoji';
export const MAX_RECENT = 16;

export function loadRecent(): string[] {
  try {
    const raw = JSON.parse(localStorage.getItem(RECENT_KEY) || '[]');
    return Array.isArray(raw) ? raw.filter((e): e is string => typeof e === 'string').slice(0, MAX_RECENT) : [];
  } catch {
    return [];
  }
}

// The newest pick goes first; picking it again moves it up instead of repeating it.
export function pushRecent(list: string[], emoji: string): string[] {
  return [emoji, ...list.filter(e => e !== emoji)].slice(0, MAX_RECENT);
}

export function saveRecent(list: string[]): void {
  try {
    localStorage.setItem(RECENT_KEY, JSON.stringify(list));
  } catch {
    /* blocked storage: recents last until the app closes */
  }
}

const ALL: EmojiItem[] = EMOJI_GROUPS.flatMap(g => g.emojis);

// Every word of the query has to appear in the emoji's name.
export function searchEmoji(query: string): EmojiItem[] {
  const words = query.toLowerCase().split(/\s+/).filter(Boolean);
  if (words.length === 0) return [];
  return ALL.filter(item => words.every(w => item.n.includes(w)));
}
