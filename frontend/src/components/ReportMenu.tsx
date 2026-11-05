import React, { useEffect } from 'react';

export const REPORT_REASONS: { id: string; label: string }[] = [
  { id: 'spam', label: 'Spam' },
  { id: 'nsfw', label: 'Inappropriate / NSFW' },
  { id: 'harassment', label: 'Harassment' },
  { id: 'malicious', label: 'Malicious link or file' },
  { id: 'other', label: 'Something else' },
];

interface ReportMenuProps {
  onPick: (reason: string) => void;
  onClose: () => void;
}

export const ReportMenu: React.FC<ReportMenuProps> = ({ onPick, onClose }) => {
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose();
    };
    document.addEventListener('keydown', onKey);
    return () => document.removeEventListener('keydown', onKey);
  }, [onClose]);

  return (
    <div className="report-menu" role="menu" aria-label="Report this message">
      <div className="report-menu-title">Report this message</div>
      {REPORT_REASONS.map(r => (
        <button key={r.id} role="menuitem" className="report-menu-item" onClick={() => onPick(r.id)}>
          {r.label}
        </button>
      ))}
      <button className="report-menu-cancel" onClick={onClose}>
        Cancel
      </button>
    </div>
  );
};
