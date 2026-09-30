import React, { useState } from 'react';
import { User } from '../types';
import { MAX_GROUP_PARTICIPANTS } from '../hooks/useGroupCall';

interface StartGroupCallModalProps {
  isOpen: boolean;
  users: User[];
  currentUser: string;
  onClose: () => void;
  onStart: (usernames: string[]) => void;
}

export const StartGroupCallModal: React.FC<StartGroupCallModalProps> = ({
  isOpen,
  users,
  currentUser,
  onClose,
  onStart,
}) => {
  const [selected, setSelected] = useState<string[]>([]);

  if (!isOpen) return null;

  const maxOthers = MAX_GROUP_PARTICIPANTS - 1;
  const candidates = users.filter((u) => u.username !== currentUser);

  const toggle = (username: string) => {
    setSelected((prev) => {
      if (prev.includes(username)) return prev.filter((u) => u !== username);
      if (prev.length >= maxOthers) return prev;
      return [...prev, username];
    });
  };

  const handleStart = () => {
    if (selected.length === 0) return;
    onStart(selected);
    setSelected([]);
    onClose();
  };

  return (
    <div
      style={{
        position: 'fixed',
        inset: 0,
        background: 'rgba(0,0,0,0.6)',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        zIndex: 10001,
      }}
      onClick={onClose}
    >
      <div
        style={{
          background: '#2b2d31',
          borderRadius: '16px',
          padding: '24px',
          width: '360px',
          boxShadow: '0 12px 40px rgba(0,0,0,0.5)',
        }}
        onClick={(e) => e.stopPropagation()}
      >
        <div style={{ color: '#f2f3f5', fontSize: '17px', fontWeight: 700, marginBottom: '4px' }}>
          Start a group call
        </div>
        <div style={{ color: '#8a8f98', fontSize: '12px', marginBottom: '16px' }}>
          Pick up to {maxOthers} people ({selected.length}/{maxOthers} selected)
        </div>

        <div style={{ maxHeight: '280px', overflowY: 'auto', display: 'flex', flexDirection: 'column', gap: '4px' }}>
          {candidates.length === 0 && (
            <div style={{ color: '#8a8f98', fontSize: '13px', padding: '12px 0' }}>No other users yet.</div>
          )}
          {candidates.map((u) => {
            const isChecked = selected.includes(u.username);
            const disabled = !isChecked && selected.length >= maxOthers;
            return (
              <label
                key={u.username}
                style={{
                  display: 'flex',
                  alignItems: 'center',
                  gap: '10px',
                  padding: '8px 10px',
                  borderRadius: '8px',
                  cursor: disabled ? 'not-allowed' : 'pointer',
                  opacity: disabled ? 0.4 : 1,
                  background: isChecked ? 'rgba(88,101,242,0.15)' : 'transparent',
                }}
              >
                <input
                  type="checkbox"
                  checked={isChecked}
                  disabled={disabled}
                  onChange={() => toggle(u.username)}
                />
                <span
                  style={{
                    width: '10px',
                    height: '10px',
                    borderRadius: '50%',
                    background: u.status === 'online' ? '#23a55a' : u.status === 'away' ? '#faa81a' : '#747f8d',
                    flexShrink: 0,
                  }}
                />
                <span style={{ color: '#f2f3f5', fontSize: '14px' }}>{u.username}</span>
              </label>
            );
          })}
        </div>

        <div style={{ display: 'flex', gap: '10px', marginTop: '20px', justifyContent: 'flex-end' }}>
          <button
            onClick={onClose}
            style={{
              padding: '8px 16px',
              borderRadius: '8px',
              border: '1px solid #3f4147',
              background: 'transparent',
              color: '#f2f3f5',
              cursor: 'pointer',
              fontSize: '13px',
            }}
          >
            Cancel
          </button>
          <button
            onClick={handleStart}
            disabled={selected.length === 0}
            style={{
              padding: '8px 16px',
              borderRadius: '8px',
              border: 'none',
              background: selected.length === 0 ? '#3f4147' : '#5865f2',
              color: 'white',
              cursor: selected.length === 0 ? 'not-allowed' : 'pointer',
              fontSize: '13px',
              fontWeight: 600,
            }}
          >
            Start call
          </button>
        </div>
      </div>
    </div>
  );
};