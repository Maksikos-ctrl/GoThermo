import React, { useEffect, useState } from 'react';
import { createPortal } from 'react-dom';
import { CallStatus } from '../hooks/useCall';

interface CallBarProps {
  status: CallStatus;
  remoteUser: string | null;
  isMuted: boolean;
  isVideoEnabled: boolean;
  remoteHasVideo: boolean;
  remoteAudioRef: React.RefObject<HTMLAudioElement>;
  localVideoRef: React.RefObject<HTMLVideoElement>;
  remoteVideoRef: React.RefObject<HTMLVideoElement>;
  onToggleMute: () => void;
  onToggleVideo: () => void;
  onEndCall: () => void;
}

export const CallBar: React.FC<CallBarProps> = ({
  status,
  remoteUser,
  isMuted,
  isVideoEnabled,
  remoteHasVideo,
  remoteAudioRef,
  localVideoRef,
  remoteVideoRef,
  onToggleMute,
  onToggleVideo,
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

        {status === 'connected' && (
          <button
            onClick={onToggleVideo}
            title={isVideoEnabled ? 'Turn camera off' : 'Turn camera on'}
            style={{
              width: '34px',
              height: '34px',
              borderRadius: '50%',
              background: isVideoEnabled ? '#23a55a' : 'transparent',
              border: '1px solid #3f4147',
              color: 'white',
              cursor: 'pointer',
              fontSize: '15px',
            }}
          >
            {isVideoEnabled ? '📹' : '📷'}
          </button>
        )}

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

      {(isVideoEnabled || remoteHasVideo) &&
        createPortal(
          <div
            style={{
              position: 'fixed',
              bottom: '20px',
              right: '20px',
              width: '260px',
              aspectRatio: '4 / 3',
              background: '#000',
              borderRadius: '14px',
              overflow: 'hidden',
              boxShadow: '0 10px 30px rgba(0,0,0,0.5)',
              border: '1px solid #3f4147',
              zIndex: 9999,
            }}
          >
            {remoteHasVideo ? (
              <video
                ref={remoteVideoRef}
                autoPlay
                muted
                playsInline
                style={{ width: '100%', height: '100%', objectFit: 'cover' }}
              />
            ) : (
              <div
                style={{
                  width: '100%',
                  height: '100%',
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'center',
                  color: '#8a8f98',
                  fontSize: '13px',
                }}
              >
                {remoteUser}'s camera is off
              </div>
            )}

            {isVideoEnabled && (
              <video
                ref={localVideoRef}
                autoPlay
                muted
                playsInline
                style={{
                  position: 'absolute',
                  bottom: '8px',
                  right: '8px',
                  width: '78px',
                  height: '58px',
                  objectFit: 'cover',
                  borderRadius: '8px',
                  border: '2px solid #1e1f22',
                  transform: 'scaleX(-1)',
                  background: '#111',
                }}
              />
            )}
          </div>,
          document.body
        )}
    </>
  );
};