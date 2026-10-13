import React, { useEffect, useRef, useState } from 'react';
import { api } from '../services/api';

interface ExportMenuProps {
  channel: string;
  username: string;
}

const FORMATS: { id: 'json' | 'csv' | 'txt'; label: string; hint: string }[] = [
  { id: 'txt', label: 'Text', hint: 'readable transcript' },
  { id: 'csv', label: 'CSV', hint: 'opens in Excel / Numbers' },
  { id: 'json', label: 'JSON', hint: 'everything, for developers' },
];

export const ExportMenu: React.FC<ExportMenuProps> = ({ channel, username }) => {
  const [open, setOpen] = useState(false);
  const [busy, setBusy] = useState(false);
  const [note, setNote] = useState<{ text: string; error: boolean } | null>(null);
  const rootRef = useRef<HTMLDivElement>(null);

  // close when clicking elsewhere
  useEffect(() => {
    if (!open) return;
    const onDown = (e: MouseEvent) => {
      if (rootRef.current && !rootRef.current.contains(e.target as Node)) setOpen(false);
    };
    document.addEventListener('mousedown', onDown);
    return () => document.removeEventListener('mousedown', onDown);
  }, [open]);

  // hide the result message after a moment
  useEffect(() => {
    if (!note) return;
    const t = setTimeout(() => setNote(null), note.error ? 5000 : 3000);
    return () => clearTimeout(t);
  }, [note]);

  const run = async (format: 'json' | 'csv' | 'txt') => {
    setOpen(false);
    setBusy(true);
    try {
      const path = await api.files.exportChannel(channel, username, format);
      if (path) setNote({ text: 'Saved ✓', error: false }); // empty path = dialog closed
    } catch (err: any) {
      setNote({ text: String(err?.message || err), error: true });
    } finally {
      setBusy(false);
    }
  };

  return (
    <div ref={rootRef} style={{ position: 'relative', display: 'inline-block' }}>
      <button
        className="search-btn"
        onClick={() => setOpen(!open)}
        disabled={busy}
        title="Export chat history"
        style={{ cursor: busy ? 'wait' : undefined, opacity: busy ? 0.5 : undefined }}
      >
        ⬇️
      </button>

      {open && (
        <div
          style={{
            position: 'absolute',
            right: 0,
            top: '100%',
            zIndex: 50,
            minWidth: '210px',
            padding: '6px',
            borderRadius: '8px',
            background: '#2b3445',
            color: '#e6e9ef',
            boxShadow: '0 6px 20px rgba(0,0,0,0.35)',
          }}
        >
          <div style={{ fontSize: '11px', opacity: 0.6, padding: '4px 8px' }}>Export #{channel}</div>
          {FORMATS.map((f) => (
            <button
              key={f.id}
              onClick={() => run(f.id)}
              style={{
                display: 'block',
                width: '100%',
                textAlign: 'left',
                background: 'transparent',
                border: 'none',
                color: 'inherit',
                cursor: 'pointer',
                padding: '7px 8px',
                borderRadius: '6px',
                fontSize: '13px',
              }}
              onMouseEnter={(e) => (e.currentTarget.style.background = 'rgba(255,255,255,0.08)')}
              onMouseLeave={(e) => (e.currentTarget.style.background = 'transparent')}
            >
              <strong>{f.label}</strong>
              <span style={{ opacity: 0.6, marginLeft: '8px', fontSize: '11px' }}>{f.hint}</span>
            </button>
          ))}
        </div>
      )}

      {note && (
        <div
          style={{
            position: 'absolute',
            right: 0,
            top: '100%',
            zIndex: 50,
            marginTop: '4px',
            padding: '6px 10px',
            borderRadius: '6px',
            fontSize: '12px',
            whiteSpace: 'nowrap',
            maxWidth: '320px',
            overflow: 'hidden',
            textOverflow: 'ellipsis',
            background: note.error ? '#b3261e' : '#2e7d32',
            color: '#fff',
          }}
        >
          {note.text}
        </div>
      )}
    </div>
  );
};