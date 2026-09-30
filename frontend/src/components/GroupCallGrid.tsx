import React, { useEffect, useState } from 'react';
import { createPortal } from 'react-dom';

interface GroupCallGridProps {
  inCall: boolean;
  participants: string[];
  joinedParticipants: string[];
  currentUser: string;
  isMuted: boolean;
  isVideoEnabled: boolean;
  isScreenSharing: boolean;
  remotePeers: Record<string, { hasVideo: boolean; isScreenSharing: boolean }>;
  localVideoRef: React.RefObject<HTMLVideoElement>;
  registerRemoteAudioRef: (username: string) => (el: HTMLAudioElement | null) => void;
  registerRemoteVideoRef: (username: string) => (el: HTMLVideoElement | null) => void;
  registerRemoteScreenVideoRef: (username: string) => (el: HTMLVideoElement | null) => void;
  onToggleMute: () => void;
  onToggleVideo: () => void;
  onToggleScreenShare: () => void;
  onLeave: () => void;
}

export const GroupCallGrid: React.FC<GroupCallGridProps> = ({
  inCall,
  participants,
  joinedParticipants,
  currentUser,
  isMuted,
  isVideoEnabled,
  isScreenSharing,
  remotePeers,
  localVideoRef,
  registerRemoteAudioRef,
  registerRemoteVideoRef,
  registerRemoteScreenVideoRef,
  onToggleMute,
  onToggleVideo,
  onToggleScreenShare,
  onLeave,
}) => {
  const [seconds, setSeconds] = useState(0);

  useEffect(() => {
    if (!inCall) {
      setSeconds(0);
      return;
    }
    const interval = setInterval(() => setSeconds((s) => s + 1), 1000);
    return () => clearInterval(interval);
  }, [inCall]);

  if (!inCall) return null;

  const formatDuration = (s: number) => {
    const m = Math.floor(s / 60);
    const sec = s % 60;
    return `${m}:${sec.toString().padStart(2, '0')}`;
  };

  const others = joinedParticipants.filter((p) => p !== currentUser);
  const waitingFor = participants.filter((p) => p !== currentUser && !joinedParticipants.includes(p));

  return createPortal(
    <>
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
        <div style={{ width: '10px', height: '10px', borderRadius: '50%', background: '#23a55a', flexShrink: 0 }} />

        <div>
          <div style={{ color: '#f2f3f5', fontSize: '13px', fontWeight: 700 }}>
            Group call · {joinedParticipants.length} joined
          </div>
          <div style={{ color: '#8a8f98', fontSize: '11px' }}>
            {formatDuration(seconds)}
            {waitingFor.length > 0 ? ` · waiting for ${waitingFor.join(', ')}` : ''}
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

        <button
          onClick={onToggleScreenShare}
          title={isScreenSharing ? 'Stop sharing screen' : 'Share your screen'}
          style={{
            width: '34px',
            height: '34px',
            borderRadius: '50%',
            background: isScreenSharing ? '#5865f2' : 'transparent',
            border: '1px solid #3f4147',
            color: 'white',
            cursor: 'pointer',
            fontSize: '15px',
          }}
        >
          🖥️
        </button>

        <button
          onClick={onLeave}
          title="Leave call"
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

      <div
        style={{
          position: 'fixed',
          bottom: '20px',
          right: '20px',
          width: '360px',
          display: 'grid',
          gridTemplateColumns: others.length > 1 ? '1fr 1fr' : '1fr',
          gap: '6px',
          zIndex: 9999,
        }}
      >
        {others.map((username) => {
          const peerState = remotePeers[username];
          const hasVideo = peerState?.hasVideo;
          const isSharing = peerState?.isScreenSharing;
          return (
            <div
              key={username}
              style={{
                position: 'relative',
                aspectRatio: '4 / 3',
                background: '#000',
                borderRadius: '12px',
                overflow: 'hidden',
                border: '1px solid #3f4147',
                boxShadow: '0 8px 24px rgba(0,0,0,0.4)',
              }}
            >
              <audio ref={registerRemoteAudioRef(username)} autoPlay style={{ display: 'none' }} />

              {isSharing ? (
                <video
                  ref={registerRemoteScreenVideoRef(username)}
                  autoPlay
                  muted
                  playsInline
                  style={{ width: '100%', height: '100%', objectFit: 'contain', background: '#000' }}
                />
              ) : hasVideo ? (
                <video
                  ref={registerRemoteVideoRef(username)}
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
                    color: 'white',
                    fontSize: '22px',
                    fontWeight: 700,
                    background: '#5865f2',
                  }}
                >
                  {username.charAt(0).toUpperCase()}
                </div>
              )}

              {isSharing && hasVideo && (
                <video
                  ref={registerRemoteVideoRef(username)}
                  autoPlay
                  muted
                  playsInline
                  style={{
                    position: 'absolute',
                    top: '6px',
                    left: '6px',
                    width: '34px',
                    height: '34px',
                    objectFit: 'cover',
                    borderRadius: '50%',
                    border: '2px solid #1e1f22',
                    background: '#111',
                  }}
                />
              )}

              <div
                style={{
                  position: 'absolute',
                  bottom: '4px',
                  left: '6px',
                  color: 'white',
                  fontSize: '11px',
                  fontWeight: 600,
                  textShadow: '0 1px 3px rgba(0,0,0,0.8)',
                }}
              >
                {username}
                {isSharing ? ' · sharing screen' : ''}
              </div>
            </div>
          );
        })}

        {isVideoEnabled && (
          <div
            style={{
              position: 'relative',
              aspectRatio: '4 / 3',
              background: '#000',
              borderRadius: '12px',
              overflow: 'hidden',
              border: '1px solid #3f4147',
              boxShadow: '0 8px 24px rgba(0,0,0,0.4)',
            }}
          >
            <video
              ref={localVideoRef}
              autoPlay
              muted
              playsInline
              style={{ width: '100%', height: '100%', objectFit: 'cover', transform: 'scaleX(-1)' }}
            />
            <div
              style={{
                position: 'absolute',
                bottom: '4px',
                left: '6px',
                color: 'white',
                fontSize: '11px',
                fontWeight: 600,
                textShadow: '0 1px 3px rgba(0,0,0,0.8)',
              }}
            >
              You
            </div>
          </div>
        )}
      </div>
    </>,
    document.body
  );
};