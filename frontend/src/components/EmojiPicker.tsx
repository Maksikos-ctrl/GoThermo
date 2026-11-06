import React, { useEffect, useMemo, useRef, useState } from 'react';
import { EMOJI_GROUPS } from './emojiData';
import { loadRecent, pushRecent, saveRecent, searchEmoji } from './emojiState';
import '../styles/emoji.css';

interface EmojiPickerProps {
  onPick: (emoji: string) => void;
  onClose: () => void;
  // The host closes it itself when someone clicks outside (the composer does).
  closeOnOutsideClick?: boolean;
}

// Pick one emoji: recent ones first, then groups, with a search box.
export const EmojiPicker: React.FC<EmojiPickerProps> = ({ onPick, onClose, closeOnOutsideClick = true }) => {
  const [query, setQuery] = useState('');
  const [group, setGroup] = useState<string>(() => (loadRecent().length ? 'recent' : EMOJI_GROUPS[0].id));
  const [recent, setRecent] = useState<string[]>(() => loadRecent());
  const ref = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    inputRef.current?.focus();
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose();
    };
    const onDown = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) onClose();
    };
    document.addEventListener('keydown', onKey);
    if (closeOnOutsideClick) document.addEventListener('mousedown', onDown);
    return () => {
      document.removeEventListener('keydown', onKey);
      document.removeEventListener('mousedown', onDown);
    };
  }, [onClose, closeOnOutsideClick]);

  const shown = useMemo(() => {
    if (query.trim()) return searchEmoji(query).map(i => ({ e: i.e, n: i.n }));
    if (group === 'recent') return recent.map(e => ({ e, n: e }));
    return EMOJI_GROUPS.find(g => g.id === group)?.emojis ?? [];
  }, [query, group, recent]);

  const pick = (emoji: string) => {
    const next = pushRecent(recent, emoji);
    setRecent(next);
    saveRecent(next);
    onPick(emoji);
  };

  return (
    <div className="emoji-picker" ref={ref} role="dialog" aria-label="Emoji picker">
      <input
        ref={inputRef}
        className="emoji-search"
        placeholder="Search emoji"
        value={query}
        onChange={e => setQuery(e.target.value)}
        aria-label="Search emoji"
      />
      {!query.trim() && (
        <div className="emoji-tabs" role="tablist">
          {recent.length > 0 && (
            <button
              type="button"
              role="tab"
              aria-selected={group === 'recent'}
              aria-label="Recent"
              title="Recent"
              className={`emoji-tab${group === 'recent' ? ' active' : ''}`}
              onClick={() => setGroup('recent')}
            >
              🕘
            </button>
          )}
          {EMOJI_GROUPS.map(g => (
            <button
              key={g.id}
              type="button"
              role="tab"
              aria-selected={group === g.id}
              aria-label={g.name}
              title={g.name}
              className={`emoji-tab${group === g.id ? ' active' : ''}`}
              onClick={() => setGroup(g.id)}
            >
              {g.icon}
            </button>
          ))}
        </div>
      )}
      <div className="emoji-grid">
        {shown.length === 0 && <div className="emoji-empty">{query.trim() ? 'No emoji found' : 'Nothing yet'}</div>}
        {shown.map(item => (
          <button key={item.e} type="button" className="emoji-cell" title={item.n} onClick={() => pick(item.e)}>
            {item.e}
          </button>
        ))}
      </div>
    </div>
  );
};
