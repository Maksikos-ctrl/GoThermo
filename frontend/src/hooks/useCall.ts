import { useState, useRef, useCallback } from 'react';


function showCallDebugBanner(message: string, color: string = '#da373c') {
  let el = document.getElementById('__call_debug__');
  if (!el) {
    el = document.createElement('div');
    el.id = '__call_debug__';
    el.style.cssText =
      'position:fixed;top:0;left:0;right:0;color:white;' +
      'font-size:13px;font-family:monospace;padding:8px 12px;z-index:999999;' +
      'word-break:break-all;text-align:center;';
    document.body.appendChild(el);
  }
  el.style.background = color;
  el.textContent = message;
}

export type CallStatus = 'idle' | 'calling' | 'ringing' | 'connected';

interface IncomingCallInfo {
  from: string;
}

interface UseCallParams {
  currentUser: string;
  sendSignal: (type: string, payload: any) => void;
}


const ICE_SERVERS = [{ urls: 'stun:stun.l.google.com:19302' }];

export function useCall({ currentUser, sendSignal }: UseCallParams) {
  const [status, setStatus] = useState<CallStatus>('idle');
  const [remoteUser, setRemoteUser] = useState<string | null>(null);
  const [incomingCall, setIncomingCall] = useState<IncomingCallInfo | null>(null);
  const [isMuted, setIsMuted] = useState(false);

  const pcRef = useRef<RTCPeerConnection | null>(null);
  const localStreamRef = useRef<MediaStream | null>(null);
  const remoteAudioRef = useRef<HTMLAudioElement | null>(null);
  const pendingOfferRef = useRef<RTCSessionDescriptionInit | null>(null);
  const iceQueueRef = useRef<RTCIceCandidateInit[]>([]);

  const cleanup = useCallback(() => {
    pcRef.current?.close();
    pcRef.current = null;
    localStreamRef.current?.getTracks().forEach((t) => t.stop());
    localStreamRef.current = null;
    if (remoteAudioRef.current) remoteAudioRef.current.srcObject = null;
    iceQueueRef.current = [];
    pendingOfferRef.current = null;
    setStatus('idle');
    setRemoteUser(null);
    setIncomingCall(null);
    setIsMuted(false);
  }, []);

  const createPeerConnection = useCallback(
    (targetUser: string) => {
      const pc = new RTCPeerConnection({ iceServers: ICE_SERVERS });

      pc.onicecandidate = (e) => {
        
        if (e.candidate && e.candidate.candidate) {
          sendSignal('call_ice_candidate', {
            to: targetUser,
            from: currentUser,
            candidate: {
              candidate: e.candidate.candidate,
              sdpMid: e.candidate.sdpMid,
              sdpMLineIndex: e.candidate.sdpMLineIndex,
              usernameFragment: e.candidate.usernameFragment,
            },
          });
        }
      };

      pc.ontrack = (e) => {
        if (remoteAudioRef.current) {
          remoteAudioRef.current.srcObject = e.streams[0];
        }
      };

      pc.onconnectionstatechange = () => {
        if (pc.connectionState === 'connected') {
          setStatus('connected');
        }
        if (pc.connectionState === 'failed' || pc.connectionState === 'closed') {
          
          cleanup();
        }
      };

      pcRef.current = pc;
      return pc;
    },
    [currentUser, sendSignal, cleanup]
  );

 
  const startCall = useCallback(
    async (targetUser: string) => {
      if (status !== 'idle') {
        alert('You are already in a call');
        return;
      }
      try {
        const stream = await navigator.mediaDevices.getUserMedia({ audio: true });
        localStreamRef.current = stream;

        const pc = createPeerConnection(targetUser);
        stream.getTracks().forEach((track) => pc.addTrack(track, stream));

        const offer = await pc.createOffer();
        await pc.setLocalDescription(offer);

        setRemoteUser(targetUser);
        setStatus('calling');

        sendSignal('call_offer', { to: targetUser, from: currentUser, sdp: offer });
      } catch (err) {
        console.error('Failed to start call:', err);
        const errorMessage = err instanceof Error ? err.message : String(err);
        showCallDebugBanner(`Start call failed: ${errorMessage}`); 
        cleanup();
      }
    },
    [status, createPeerConnection, currentUser, sendSignal, cleanup]
  );

 
  const acceptCall = useCallback(async () => {
    if (!incomingCall || !pendingOfferRef.current) return;
    const caller = incomingCall.from;

    try {
      const stream = await navigator.mediaDevices.getUserMedia({ audio: true });
      localStreamRef.current = stream;

      const pc = createPeerConnection(caller);
      stream.getTracks().forEach((track) => pc.addTrack(track, stream));

      await pc.setRemoteDescription(new RTCSessionDescription(pendingOfferRef.current));

    
      for (const c of iceQueueRef.current) {
        await pc.addIceCandidate(new RTCIceCandidate(c));
      }
      iceQueueRef.current = [];

      const answer = await pc.createAnswer();
      await pc.setLocalDescription(answer);

      setRemoteUser(caller);
      setIncomingCall(null);

      sendSignal('call_answer', { to: caller, from: currentUser, sdp: answer });
    } catch (err) {
      console.error('Failed to accept call:', err);
      const errorMessage = err instanceof Error ? err.message : String(err);
      showCallDebugBanner(`Accept call failed: ${errorMessage}`); 
      sendSignal('call_reject', { to: caller, from: currentUser });
      cleanup();
    }
  }, [incomingCall, createPeerConnection, currentUser, sendSignal, cleanup]);


  const declineCall = useCallback(() => {
    if (incomingCall) {
      sendSignal('call_reject', { to: incomingCall.from, from: currentUser });
    }
    setIncomingCall(null);
    pendingOfferRef.current = null;
    iceQueueRef.current = [];
    setStatus('idle');
  }, [incomingCall, currentUser, sendSignal]);

  
  const endCall = useCallback(() => {
    if (remoteUser) {
      sendSignal('call_end', { to: remoteUser, from: currentUser });
    }
    cleanup();
  }, [remoteUser, currentUser, sendSignal, cleanup]);

  
  const toggleMute = useCallback(() => {
    if (!localStreamRef.current) return;
    const newMuted = !isMuted;
    localStreamRef.current.getAudioTracks().forEach((t) => (t.enabled = !newMuted));
    setIsMuted(newMuted);
  }, [isMuted]);


  const handleSignal = useCallback(
    (type: string, payload: any) => {
      switch (type) {
        case 'call_offer': {
          if (payload.to !== currentUser) return;

          
          if (status !== 'idle') {
            sendSignal('call_reject', { to: payload.from, from: currentUser });
            return;
          }

          pendingOfferRef.current = payload.sdp;
          setIncomingCall({ from: payload.from });
          setStatus('ringing');
          break;
        }

        case 'call_answer': {
          if (payload.to !== currentUser || !pcRef.current) return;
          pcRef.current
            .setRemoteDescription(new RTCSessionDescription(payload.sdp))
            .then(async () => {
              for (const c of iceQueueRef.current) {
                await pcRef.current?.addIceCandidate(new RTCIceCandidate(c));
              }
              iceQueueRef.current = [];
              setStatus('connected');
            })
            .catch((err) => console.error('setRemoteDescription failed:', err));
          break;
        }

        case 'call_ice_candidate': {
          if (payload.to !== currentUser) return;
          
          if (!payload.candidate || !payload.candidate.candidate) return;

          if (pcRef.current && pcRef.current.remoteDescription) {
            pcRef.current.addIceCandidate(new RTCIceCandidate(payload.candidate)).catch(console.error);
          } else {
           
            iceQueueRef.current.push(payload.candidate);
          }
          break;
        }

        case 'call_reject': {
          if (payload.to !== currentUser) return;
          showCallDebugBanner(`${payload.from} declined the call`, '#f0b232'); 
          cleanup();
          break;
        }

        case 'call_end': {
          if (payload.to !== currentUser) return;
          cleanup();
          break;
        }
      }
    },
    [currentUser, status, sendSignal, cleanup]
  );

  return {
    status,
    remoteUser,
    incomingCall,
    isMuted,
    remoteAudioRef,
    startCall,
    acceptCall,
    declineCall,
    endCall,
    toggleMute,
    handleSignal,
  };
}