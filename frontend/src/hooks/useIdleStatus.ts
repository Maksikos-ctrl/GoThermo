import { useEffect, useRef } from 'react';
import { StatusType } from '../types';

interface UseIdleStatusOptions {
  enabled: boolean;           
  currentStatus: StatusType;   
  inCall: boolean;             
  idleMs?: number;           
  onGoAway: () => void;        
  onReturn: () => void;        
}

const ACTIVITY_EVENTS = ['mousemove', 'mousedown', 'keydown', 'wheel', 'touchstart', 'focus'];

export function useIdleStatus({
  enabled,
  currentStatus,
  inCall,
  idleMs = 15 * 1000,
  onGoAway,
  onReturn,
}: UseIdleStatusOptions) {
  const lastActivityRef = useRef(Date.now());
  const autoAwayRef = useRef(false); 
  const statusRef = useRef(currentStatus);
  const inCallRef = useRef(inCall);
  const onGoAwayRef = useRef(onGoAway);
  const onReturnRef = useRef(onReturn);

  statusRef.current = currentStatus;
  inCallRef.current = inCall;
  onGoAwayRef.current = onGoAway;
  onReturnRef.current = onReturn;

  
  useEffect(() => {
    if (currentStatus !== 'away') {
      autoAwayRef.current = false;
    }
  }, [currentStatus]);

  useEffect(() => {
    if (!enabled) return;

    lastActivityRef.current = Date.now();

    const handleActivity = () => {
      lastActivityRef.current = Date.now();
      if (autoAwayRef.current && statusRef.current === 'away') {
        autoAwayRef.current = false;
        onReturnRef.current();
      }
    };

    ACTIVITY_EVENTS.forEach((e) => window.addEventListener(e, handleActivity, { passive: true }));

    const timer = setInterval(() => {
     
      if (inCallRef.current) {
        lastActivityRef.current = Date.now();
        return;
      }
   
      if (statusRef.current !== 'online' || autoAwayRef.current) return;

      if (Date.now() - lastActivityRef.current >= idleMs) {
        autoAwayRef.current = true;
        onGoAwayRef.current();
      }
    }, 10_000);

    return () => {
      ACTIVITY_EVENTS.forEach((e) => window.removeEventListener(e, handleActivity));
      clearInterval(timer);
    };
  }, [enabled, idleMs]);
}