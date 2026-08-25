import React, { useEffect, useState } from 'react';
import { CallStatus } from '../hooks/useCall';

interface CallBarProps {
  status: CallStatus;
  remoteUser: string | null;
  isMuted: boolean;
  remoteAudioRef: React.RefObject<HTMLAudioElement>;
  onToggleMute: () => void;
  onEndCall: () => void;
}

export const CallBar: React.FC<CallBarProps> = ({
  status,
  remoteUser,
  isMuted,
  remoteAudioRef,
  onToggleMute,
  onEndCall,
}) => {
  const [seconds, setSeconds] = useState(0);

  
  useEffect(() => {
    if (status !== 'connected') {
      setSeconds(0);
      return;
    }
    const interval = setInterval(() => setSeconds((s) => s + 1), 1000);
    return () => clearInterval(interval);
  }, [status]);

  const formatDuration = (s: number) => {
    const m = Math.floor(s / 60);
    const sec = s % 60;
    return `${m}:${sec.toString().padStart(2, '0')}`;
  };


  const audioEl = <audio ref={remoteAudioRef} autoPlay style={{ display: 'none' }} />;

  if (status === 'idle' || status === 'ringing') {

    return status === 'idle' ? null : audioEl;
  }

  return (
    <>
      {audioEl}
      <div
        style={{
          position: 'fixed',
          top: '16px',
          left: '50%',
          transform: 'translateX(-50%)',
          background: '#1e1f22',
          border: '1px solid #3f4147',
          borderRadius: '12px',
          padding: '10px 16px',
          display: 'flex',
          alignItems: 'center',
          gap: '14px',
          boxShadow: '0 6px 20px rgba(0,0,0,0.4)',
          zIndex: 9999,
        }}
      >
        <div
          style={{
            width: '10px',
            height: '10px',
            borderRadius: '50%',
            background: status === 'connected' ? '#23a55a' : '#f0b232',
            flexShrink: 0,
            animation: status === 'calling' ? 'blinkDot 1s infinite' : 'none',
          }}
        />

        <div>
          <div style={{ color: '#f2f3f5', fontSize: '13px', fontWeight: 700 }}>
            {remoteUser}
          </div>
          <div style={{ color: '#8a8f98', fontSize: '11px' }}>
            {status === 'calling' ? 'Calling...' : formatDuration(seconds)}
          </div>
        </div>

        <button
          onClick={onToggleMute}
          title={isMuted ? 'Unmute' : 'Mute'}
          style={{
            width: '34px',
            height: '34px',
            borderRadius: '50%',
            background: isMuted ? '#3f4147' : 'transparent',
            border: '1px solid #3f4147',
            color: 'white',
            cursor: 'pointer',
            fontSize: '15px',
          }}
        >
          {isMuted ? '🔇' : '🎤'}
        </button>

        <button
          onClick={onEndCall}
          title="End call"
          style={{
            width: '34px',
            height: '34px',
            borderRadius: '50%',
            background: '#da373c',
            border: 'none',
            color: 'white',
            cursor: 'pointer',
            fontSize: '15px',
          }}
        >
          ✕
        </button>
      </div>

      <style>{`
        @keyframes blinkDot {
          0%, 100% { opacity: 1; }
          50% { opacity: 0.3; }
        }
      `}</style>
    </>
  );
};