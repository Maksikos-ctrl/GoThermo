import { useCallback, useRef, useState } from 'react';

export type ToastType = 'error' | 'success';

export interface Toast {
  id: string;
  message: string;
  type: ToastType;
}

export function useToast() {
  const [toasts, setToasts] = useState<Toast[]>([]);
  const nextId = useRef(0);

  const removeToast = useCallback((id: string) => {
    setToasts((prev) => prev.filter((t) => t.id !== id));
  }, []);

  const showToast = useCallback(
    (message: string, type: ToastType = 'error') => {
      const id = `toast-${nextId.current++}`;
      setToasts((prev) => [...prev, { id, message, type }]);
      setTimeout(() => removeToast(id), type === 'error' ? 5000 : 3000);
    },
    [removeToast]
  );

  return { toasts, showToast, removeToast };
}