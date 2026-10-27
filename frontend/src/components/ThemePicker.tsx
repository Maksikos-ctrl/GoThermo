import React, { useEffect, useState } from 'react';
import { Check, X } from 'lucide-react';
import {
  ACCENT_PRESETS,
  DEFAULT_ACCENT,
  applyAccent,
  isReadableAccent,
  loadAccent,
  normalizeHex,
  saveAccent,
} from './themeAccent';
import '../styles/theme-picker.css';

interface ThemePickerProps {
  isOpen: boolean;
  onClose: () => void;
}

export const ThemePicker: React.FC<ThemePickerProps> = ({ isOpen, onClose }) => {
  const [current, setCurrent] = useState<string>(() => loadAccent() ?? DEFAULT_ACCENT);
  const [custom, setCustom] = useState('');

  useEffect(() => {
    if (!isOpen) return;
    setCurrent(loadAccent() ?? DEFAULT_ACCENT);
    setCustom('');
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose();
    };
    document.addEventListener('keydown', onKey);
    return () => document.removeEventListener('keydown', onKey);
  }, [isOpen, onClose]);

  if (!isOpen) return null;

  const choose = (hex: string) => {
    setCurrent(hex);
    applyAccent(hex);
    saveAccent(hex);
  };

  const customHex = normalizeHex(custom);
  const customError =
    custom.trim() === '' ? null : !customHex ? 'Use a colour like #2b8a3e' : !isReadableAccent(customHex) ? 'Too light: white text would be hard to read' : null;

  return (
    <div className="theme-overlay" onMouseDown={e => e.target === e.currentTarget && onClose()}>
      <div className="theme-modal" role="dialog" aria-label="Accent colour">
        <header className="theme-header">
          <strong>Accent colour</strong>
          <button type="button" className="theme-x" onClick={onClose} aria-label="Close">
            <X size={16} />
          </button>
        </header>

        <div className="theme-grid" role="radiogroup" aria-label="Presets">
          {ACCENT_PRESETS.map(p => {
            const on = p.color === current;
            return (
              <button
                key={p.id}
                type="button"
                role="radio"
                aria-checked={on}
                aria-label={p.name}
                className={'theme-swatch' + (on ? ' is-on' : '')}
                onClick={() => choose(p.color)}
              >
                <span className="theme-dot" style={{ background: p.color }}>
                  {on && <Check size={14} />}
                </span>
                <span className="theme-name">{p.name}</span>
              </button>
            );
          })}
        </div>

        <div className="theme-custom">
          <label htmlFor="theme-custom-input">Your own colour</label>
          <div className="theme-custom-row">
            <input
              id="theme-custom-input"
              value={custom}
              placeholder="#2b8a3e"
              spellCheck={false}
              onChange={e => setCustom(e.target.value)}
              onKeyDown={e => {
                if (e.key === 'Enter' && customHex && !customError) choose(customHex);
              }}
            />
            <button type="button" disabled={!customHex || !!customError} onClick={() => customHex && choose(customHex)}>
              Apply
            </button>
          </div>
          {customError && <div className="theme-error" role="alert">{customError}</div>}
        </div>

        <div className="theme-preview" aria-hidden="true">
          <span className="theme-preview-btn">Send</span>
          <span className="theme-preview-badge">3 new</span>
          <span className="theme-preview-link">a link</span>
        </div>

        <footer className="theme-footer">
          <button type="button" className="theme-reset" onClick={() => choose(DEFAULT_ACCENT)} disabled={current === DEFAULT_ACCENT}>
            Reset to default
          </button>
          <span className="theme-note">Saved on this computer. Light and dark mode work as before.</span>
        </footer>
      </div>
    </div>
  );
};
