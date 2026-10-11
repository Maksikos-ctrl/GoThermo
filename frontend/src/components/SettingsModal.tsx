import React from 'react';
import { Settings, Theme, DefaultStatus } from '../hooks/useSettings';
import { playRingtonePreview } from '../hooks/notificationSound';

interface SettingsModalProps {
  isOpen: boolean;
  settings: Settings;
  onUpdate: (partial: Partial<Settings>) => void;
  onClose: () => void;
  onLogout: () => void;
}

export const SettingsModal: React.FC<SettingsModalProps> = ({
  isOpen,
  settings,
  onUpdate,
  onClose,
  onLogout,
}) => {
  if (!isOpen) return null;

  const rowStyle: React.CSSProperties = {
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'space-between',
    padding: '14px 0',
    borderBottom: '1px solid var(--border-color, #3f4147)',
  };

  const labelStyle: React.CSSProperties = {
    color: 'var(--text-primary, #f2f3f5)',
    fontSize: '14px',
    fontWeight: 600,
  };

  const subLabelStyle: React.CSSProperties = {
    color: 'var(--text-secondary, #8a8f98)',
    fontSize: '12px',
    marginTop: '2px',
  };

  const segmentButtonStyle = (active: boolean): React.CSSProperties => ({
    padding: '6px 14px',
    borderRadius: '6px',
    border: `1px solid ${active ? 'var(--accent, #5865f2)' : 'var(--border-color, #3f4147)'}`,
    background: active ? 'var(--accent, #5865f2)' : 'transparent',
    color: active ? 'white' : 'var(--text-primary, #f2f3f5)',
    fontSize: '13px',
    fontWeight: 600,
    cursor: 'pointer',
  });

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
          background: 'var(--bg-secondary, #2b2d31)',
          borderRadius: '16px',
          padding: '28px',
          width: '420px',
          maxHeight: '80vh',
          overflowY: 'auto',
          boxShadow: '0 12px 40px rgba(0,0,0,0.5)',
        }}
        onClick={(e) => e.stopPropagation()}
      >
        <div
          style={{
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
            marginBottom: '8px',
          }}
        >
          <div style={{ color: 'var(--text-primary, #f2f3f5)', fontSize: '19px', fontWeight: 700 }}>
            Settings
          </div>
          <button
            onClick={onClose}
            style={{
              background: 'transparent',
              border: 'none',
              color: 'var(--text-secondary, #8a8f98)',
              fontSize: '20px',
              cursor: 'pointer',
              lineHeight: 1,
            }}
          >
            ✕
          </button>
        </div>

        {/* Theme */}
        <div style={rowStyle}>
          <div>
            <div style={labelStyle}>Theme</div>
            <div style={subLabelStyle}>
              Only parts of the app follow this so far - more is on the way.
            </div>
          </div>
          <div style={{ display: 'flex', gap: '8px' }}>
            {(['dark', 'light'] as Theme[]).map((t) => (
              <button
                key={t}
                style={segmentButtonStyle(settings.theme === t)}
                onClick={() => onUpdate({ theme: t })}
              >
                {t === 'dark' ? '🌙 Dark' : '☀️ Light'}
              </button>
            ))}
          </div>
        </div>

        {/* Message sound */}
        <div style={rowStyle}>
          <div>
            <div style={labelStyle}>Message sound</div>
            <div style={subLabelStyle}>Play a sound for new messages</div>
          </div>
          <button
            onClick={() => onUpdate({ messageSoundEnabled: !settings.messageSoundEnabled })}
            style={{
              width: '44px',
              height: '24px',
              borderRadius: '12px',
              border: 'none',
              cursor: 'pointer',
              background: settings.messageSoundEnabled ? 'var(--success, #23a55a)' : 'var(--border-color, #3f4147)',
              position: 'relative',
              transition: 'background 0.15s',
            }}
          >
            <span
              style={{
                position: 'absolute',
                top: '2px',
                left: settings.messageSoundEnabled ? '22px' : '2px',
                width: '20px',
                height: '20px',
                borderRadius: '50%',
                background: 'white',
                transition: 'left 0.15s',
              }}
            />
          </button>
        </div>

        {/* Ringtone volume */}
        <div style={rowStyle}>
          <div style={{ flex: 1 }}>
            <div style={labelStyle}>Call ringtone volume</div>
            <div style={subLabelStyle}>Applies to incoming and outgoing call tones</div>
            <input
              type="range"
              min={0}
              max={1}
              step={0.05}
              value={settings.ringtoneVolume}
              onChange={(e) => onUpdate({ ringtoneVolume: parseFloat(e.target.value) })}
              onMouseUp={(e) => playRingtonePreview(parseFloat((e.currentTarget as HTMLInputElement).value))}
              onTouchEnd={(e) => playRingtonePreview(parseFloat((e.currentTarget as HTMLInputElement).value))}
              onKeyUp={(e) => playRingtonePreview(parseFloat((e.currentTarget as HTMLInputElement).value))}
              style={{ width: '100%', marginTop: '10px' }}
            />
          </div>
        </div>

        {/* Default status */}
        <div style={rowStyle}>
          <div>
            <div style={labelStyle}>Status on login</div>
            <div style={subLabelStyle}>What you show as when you sign in</div>
          </div>
          <div style={{ display: 'flex', gap: '8px' }}>
            {(['online', 'away'] as DefaultStatus[]).map((s) => (
              <button
                key={s}
                style={segmentButtonStyle(settings.defaultStatus === s)}
                onClick={() => onUpdate({ defaultStatus: s })}
              >
                {s === 'online' ? '🟢 Online' : '🟡 Away'}
              </button>
            ))}
          </div>
        </div>

        {/* Logout */}
        <div style={{ paddingTop: '20px' }}>
          <button
            onClick={onLogout}
            style={{
              width: '100%',
              padding: '10px',
              borderRadius: '8px',
              border: '1px solid var(--danger, #da373c)',
              background: 'transparent',
              color: 'var(--danger, #da373c)',
              fontSize: '14px',
              fontWeight: 600,
              cursor: 'pointer',
            }}
          >
            Log out
          </button>
        </div>
      </div>
    </div>
  );
};