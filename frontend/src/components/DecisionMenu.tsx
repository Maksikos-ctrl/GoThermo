import React, { useEffect } from 'react';

interface DecisionMenuProps {
  onPick: (kind: 'decision' | 'task') => void;
  onClose: () => void;
}

export const DecisionMenu: React.FC<DecisionMenuProps> = ({ onPick, onClose }) => {
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose();
    };
    document.addEventListener('keydown', onKey);
    return () => document.removeEventListener('keydown', onKey);
  }, [onClose]);

  return (
    <div className="report-menu" role="menu" aria-label="Mark this message">
      <div className="report-menu-title">Add to the Decisions list</div>
      <button role="menuitem" className="report-menu-item" onClick={() => onPick('decision')}>
        Mark as decision
      </button>
      <button role="menuitem" className="report-menu-item" onClick={() => onPick('task')}>
        Mark as task
      </button>
      <button className="report-menu-cancel" onClick={onClose}>
        Cancel
      </button>
    </div>
  );
};
