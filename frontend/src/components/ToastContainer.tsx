import React from 'react';
import { createPortal } from 'react-dom';
import { Toast } from '../hooks/useToast';

interface ToastContainerProps {
  toasts: Toast[];
  onDismiss: (id: string) => void;
}

export const ToastContainer: React.FC<ToastContainerProps> = ({ toasts, onDismiss }) => {
  if (toasts.length === 0) return null;

  return createPortal(
    <div
      style={{
        position: 'fixed',
        bottom: '20px',
        right: '20px',
        zIndex: 10002,
        display: 'flex',
        flexDirection: 'column-reverse',
        gap: '8px',
        maxWidth: '340px',
      }}
    >
      {toasts.map((toast) => (
        <div
          key={toast.id}
          onClick={() => onDismiss(toast.id)}
          style={{
            display: 'flex',
            alignItems: 'flex-start',
            gap: '10px',
            background: '#2b2d31',
            border: `1px solid ${toast.type === 'error' ? 'rgba(218,55,60,0.5)' : 'rgba(35,165,90,0.5)'}`,
            borderLeft: `3px solid ${toast.type === 'error' ? '#da373c' : '#23a55a'}`,
            borderRadius: '8px',
            padding: '12px 14px',
            boxShadow: '0 8px 24px rgba(0,0,0,0.4)',
            cursor: 'pointer',
            animation: 'toast-in 0.2s ease-out',
          }}
        >
          <span style={{ fontSize: '16px', lineHeight: '18px', flexShrink: 0 }}>
            {toast.type === 'error' ? '⚠️' : '✅'}
          </span>
          <span style={{ color: '#f2f3f5', fontSize: '13px', lineHeight: '18px' }}>{toast.message}</span>
        </div>
      ))}

      <style>{`
        @keyframes toast-in {
          from { opacity: 0; transform: translateY(8px); }
          to { opacity: 1; transform: translateY(0); }
        }
      `}</style>
    </div>,
    document.body
  );
};