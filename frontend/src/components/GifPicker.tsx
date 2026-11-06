import React, { useEffect, useRef, useState } from 'react';
import { api } from '../services/api';
import '../styles/gifs.css';

interface Gif {
  id: string;
  title: string;
  url: string;
  preview: string;
}

interface GifPickerProps {
  onPick: (url: string) => void;
  onClose: () => void;
}

// Search Giphy through our server (the key stays there); empty box = trending.
export const GifPicker: React.FC<GifPickerProps> = ({ onPick, onClose }) => {
  const [query, setQuery] = useState('');
  const [gifs, setGifs] = useState<Gif[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose();
    };
    const onDown = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) onClose();
    };
    document.addEventListener('keydown', onKey);
    document.addEventListener('mousedown', onDown);
    return () => {
      document.removeEventListener('keydown', onKey);
      document.removeEventListener('mousedown', onDown);
    };
  }, [onClose]);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    setError('');
    const timer = setTimeout(async () => {
      try {
        const found = (await api.gifs.search(query)) as Gif[];
        if (!cancelled) setGifs(found || []);
      } catch (e: any) {
        if (!cancelled) {
          setGifs([]);
          setError(String(e?.message ?? e ?? 'GIF search failed').replace(/^Error:\s*/, ''));
        }
      } finally {
        if (!cancelled) setLoading(false);
      }
    }, query ? 350 : 0);
    return () => {
      cancelled = true;
      clearTimeout(timer);
    };
  }, [query]);

  return (
    <div className="gif-picker" ref={ref} role="dialog" aria-label="GIF picker">
      <input
        autoFocus
        className="gif-search"
        placeholder="Search GIFs"
        aria-label="Search GIFs"
        value={query}
        onChange={e => setQuery(e.target.value)}
      />
      <div className="gif-grid">
        {loading && <div className="gif-note">Loading…</div>}
        {!loading && error && <div className="gif-note">{error}</div>}
        {!loading && !error && gifs.length === 0 && <div className="gif-note">Nothing found</div>}
        {!loading &&
          gifs.map(g => (
            <button key={g.id} type="button" className="gif-cell" title={g.title} onClick={() => onPick(g.url)}>
              <img src={g.preview} alt={g.title || 'GIF'} loading="lazy" />
            </button>
          ))}
      </div>
      <div className="gif-credit">Powered by GIPHY</div>
    </div>
  );
};
