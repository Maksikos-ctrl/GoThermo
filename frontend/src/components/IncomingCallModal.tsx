import React from 'react';

interface IncomingCallModalProps {
  callerName: string | null;
  onAccept: () => void;
  onDecline: () => void;
}

export const IncomingCallModal: React.FC<IncomingCallModalProps> = ({
  callerName,
  onAccept,
  onDecline,
}) => {
  if (!callerName) return null;

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
    >
      <div
        style={{
          background: '#2b2d31',
          borderRadius: '16px',
          padding: '32px',
          width: '320px',
          textAlign: 'center',
          boxShadow: '0 12px 40px rgba(0,0,0,0.5)',
        }}
      >
        <div
          style={{
            width: '72px',
            height: '72px',
            borderRadius: '50%',
            background: '#5865f2',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            fontSize: '28px',
            fontWeight: 700,
            color: 'white',
            margin: '0 auto 16px',
            animation: 'pulseCall 1.4s infinite',
          }}
        >
          {callerName.charAt(0).toUpperCase()}
        </div>

        <div style={{ color: '#f2f3f5', fontSize: '18px', fontWeight: 700, marginBottom: '4px' }}>
          {callerName}
        </div>
        <div style={{ color: '#8a8f98', fontSize: '13px', marginBottom: '28px' }}>
          Incoming audio call...
        </div>

        <div style={{ display: 'flex', gap: '16px', justifyContent: 'center' }}>
          <button
            onClick={onDecline}
            title="Decline"
            style={{
              width: '52px',
              height: '52px',
              borderRadius: '50%',
              background: '#da373c',
              border: 'none',
              color: 'white',
              fontSize: '22px',
              cursor: 'pointer',
            }}
          >
            ✕
          </button>
          <button
            onClick={onAccept}
            title="Accept"
            style={{
              width: '52px',
              height: '52px',
              borderRadius: '50%',
              background: '#23a55a',
              border: 'none',
              color: 'white',
              fontSize: '22px',
              cursor: 'pointer',
            }}
          >
            📞
          </button>
        </div>
      </div>

      <style>{`
        @keyframes pulseCall {
          0% { box-shadow: 0 0 0 0 rgba(88,101,242,0.5); }
          70% { box-shadow: 0 0 0 14px rgba(88,101,242,0); }
          100% { box-shadow: 0 0 0 0 rgba(88,101,242,0); }
        }
      `}</style>
    </div>
  );
};