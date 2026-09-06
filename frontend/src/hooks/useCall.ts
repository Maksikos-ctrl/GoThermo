import { useState, useRef, useCallback, useEffect } from 'react';

export type CallStatus = 'idle' | 'calling' | 'ringing' | 'connected';

interface IncomingCallInfo {
  from: string;
  hasVideo: boolean;
}

interface UseCallParams {
  currentUser: string;
  sendSignal: (type: string, payload: any) => void;
  onCallEnded?: (info: {
    remoteUser: string;
    status: 'completed' | 'declined' | 'cancelled';
    durationSeconds: number;
  }) => void;
}


const ICE_SERVERS = [{ urls: 'stun:stun.l.google.com:19302' }];

export function useCall({ currentUser, sendSignal, onCallEnded }: UseCallParams) {
  const [status, setStatus] = useState<CallStatus>('idle');
  const [remoteUser, setRemoteUser] = useState<string | null>(null);
  const [incomingCall, setIncomingCall] = useState<IncomingCallInfo | null>(null);
  const [isMuted, setIsMuted] = useState(false);
  const [isVideoEnabled, setIsVideoEnabled] = useState(false);
  const [remoteHasVideo, setRemoteHasVideo] = useState(false);

  const pcRef = useRef<RTCPeerConnection | null>(null);
  const localStreamRef = useRef<MediaStream | null>(null);
  const remoteAudioRef = useRef<HTMLAudioElement | null>(null);
  const localVideoRef = useRef<HTMLVideoElement | null>(null);
  const remoteVideoRef = useRef<HTMLVideoElement | null>(null);
  const localVideoStreamRef = useRef<MediaStream | null>(null);
  const remoteVideoStreamRef = useRef<MediaStream | null>(null);
  const pendingOfferRef = useRef<RTCSessionDescriptionInit | null>(null);
  const iceQueueRef = useRef<RTCIceCandidateInit[]>([]);


  const initialNegotiationDoneRef = useRef(false);

  
  const isInitiatorRef = useRef(false);
  const callStartTimeRef = useRef<number | null>(null);
  const remoteUserRef = useRef<string | null>(null);

 
  const wantsVideoRef = useRef(false);

  
  const audioCtxRef = useRef<AudioContext | null>(null);
  const ringTimeoutRef = useRef<number | null>(null);

  const getAudioCtx = useCallback(() => {
    if (!audioCtxRef.current) {
      const Ctx = window.AudioContext || (window as any).webkitAudioContext;
      audioCtxRef.current = new Ctx();
    }
    return audioCtxRef.current;
  }, []);

  const playTone = useCallback((freqs: number[], duration: number, gainValue = 0.15) => {
    const ctx = getAudioCtx();
    if (ctx.state === 'suspended') {
      ctx.resume().catch(() => {});
    }
    const now = ctx.currentTime;
    const gain = ctx.createGain();
    gain.gain.setValueAtTime(gainValue, now);
    gain.gain.setValueAtTime(gainValue, now + duration - 0.03);
    gain.gain.linearRampToValueAtTime(0, now + duration);
    gain.connect(ctx.destination);

    freqs.forEach((freq) => {
      const osc = ctx.createOscillator();
      osc.type = 'sine';
      osc.frequency.value = freq;
      osc.connect(gain);
      osc.start(now);
      osc.stop(now + duration);
    });
  }, [getAudioCtx]);

  const stopRingtone = useCallback(() => {
    if (ringTimeoutRef.current !== null) {
      clearTimeout(ringTimeoutRef.current);
      ringTimeoutRef.current = null;
    }
  }, []);

  
  const startOutgoingRingback = useCallback(() => {
    stopRingtone();
    const cycle = () => {
      playTone([425], 1);
      ringTimeoutRef.current = window.setTimeout(cycle, 4000);
    };
    cycle();
  }, [playTone, stopRingtone]);


  const startIncomingRing = useCallback(() => {
    stopRingtone();
    const cycle = () => {
      playTone([800, 1000], 0.25);
      ringTimeoutRef.current = window.setTimeout(() => {
        playTone([800, 1000], 0.25);
        ringTimeoutRef.current = window.setTimeout(cycle, 1800);
      }, 400);
    };
    cycle();
  }, [playTone, stopRingtone]);

  useEffect(() => {
    if (status === 'calling') {
      startOutgoingRingback();
    } else if (status === 'ringing') {
      startIncomingRing();
    } else {
      stopRingtone();
    }
    return stopRingtone;
  }, [status, startOutgoingRingback, startIncomingRing, stopRingtone]);
  

  useEffect(() => {
    if (status === 'connected') {
      initialNegotiationDoneRef.current = true;
    } else if (status === 'idle') {
      initialNegotiationDoneRef.current = false;
    }
  }, [status]);

  useEffect(() => {
    if (isVideoEnabled && localVideoRef.current && localVideoStreamRef.current) {
      localVideoRef.current.srcObject = localVideoStreamRef.current;
      localVideoRef.current.play().catch(() => {});
    }
  }, [isVideoEnabled]);

  useEffect(() => {
    if (remoteHasVideo && remoteVideoRef.current && remoteVideoStreamRef.current) {
      remoteVideoRef.current.srcObject = remoteVideoStreamRef.current;
      remoteVideoRef.current.play().catch(() => {});
    }
  }, [remoteHasVideo]);

  const cleanup = useCallback(
    (explicitStatus?: 'completed' | 'declined' | 'cancelled') => {
      stopRingtone();

      if (isInitiatorRef.current && onCallEnded && remoteUserRef.current) {
        const wasConnected = callStartTimeRef.current !== null;
        const finalStatus = explicitStatus || (wasConnected ? 'completed' : 'cancelled');
        const durationSeconds = wasConnected
          ? Math.max(0, Math.round((Date.now() - callStartTimeRef.current!) / 1000))
          : 0;
        onCallEnded({ remoteUser: remoteUserRef.current, status: finalStatus, durationSeconds });
      }

      isInitiatorRef.current = false;
      callStartTimeRef.current = null;
      remoteUserRef.current = null;
      wantsVideoRef.current = false;

      pcRef.current?.close();
      pcRef.current = null;
      localStreamRef.current?.getTracks().forEach((t) => t.stop());
      localStreamRef.current = null;
      localVideoStreamRef.current?.getTracks().forEach((t) => t.stop());
      localVideoStreamRef.current = null;
      remoteVideoStreamRef.current = null;
      if (remoteAudioRef.current) remoteAudioRef.current.srcObject = null;
      if (localVideoRef.current) localVideoRef.current.srcObject = null;
      if (remoteVideoRef.current) remoteVideoRef.current.srcObject = null;
      iceQueueRef.current = [];
      pendingOfferRef.current = null;
      setStatus('idle');
      setRemoteUser(null);
      setIncomingCall(null);
      setIsMuted(false);
      setIsVideoEnabled(false);
      setRemoteHasVideo(false);
    },
    [onCallEnded, stopRingtone]
  );

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
        if (e.track.kind === 'audio') {
          if (remoteAudioRef.current) {
            remoteAudioRef.current.srcObject = e.streams[0] || new MediaStream([e.track]);
          }
          return;
        }


        if (!remoteVideoStreamRef.current) {
          remoteVideoStreamRef.current = new MediaStream();
        }
        remoteVideoStreamRef.current.addTrack(e.track);
        if (remoteVideoRef.current) {
          remoteVideoRef.current.srcObject = remoteVideoStreamRef.current;
        }
        setRemoteHasVideo(true);

        e.track.onended = () => {
          setRemoteHasVideo(false);
        };
      };

   
      pc.onnegotiationneeded = async () => {
        if (!initialNegotiationDoneRef.current) return;
        try {
          const offer = await pc.createOffer();
          await pc.setLocalDescription(offer);
          sendSignal('call_renegotiate_offer', { to: targetUser, from: currentUser, sdp: offer });
        } catch (err) {
          console.error('Renegotiation offer failed:', err);
        }
      };

      pc.onconnectionstatechange = () => {
        if (pc.connectionState === 'connected') {
          if (callStartTimeRef.current === null) callStartTimeRef.current = Date.now();
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
    async (targetUser: string, withVideo: boolean = false) => {
      if (status !== 'idle') {
        alert('You are already in a call');
        return;
      }
      try {
        const stream = await navigator.mediaDevices.getUserMedia({ audio: true });
        localStreamRef.current = stream;

        const pc = createPeerConnection(targetUser);
        stream.getTracks().forEach((track) => pc.addTrack(track, stream));

        wantsVideoRef.current = withVideo;

        const offer = await pc.createOffer();
        await pc.setLocalDescription(offer);

        setRemoteUser(targetUser);
        remoteUserRef.current = targetUser;
        isInitiatorRef.current = true;
        setStatus('calling');

        sendSignal('call_offer', { to: targetUser, from: currentUser, sdp: offer, wantsVideo: withVideo });
      } catch (err) {
        console.error('Failed to start call:', err);
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

      wantsVideoRef.current = incomingCall.hasVideo;

      await pc.setRemoteDescription(new RTCSessionDescription(pendingOfferRef.current));

    
      for (const c of iceQueueRef.current) {
        await pc.addIceCandidate(new RTCIceCandidate(c));
      }
      iceQueueRef.current = [];

      const answer = await pc.createAnswer();
      await pc.setLocalDescription(answer);

      setRemoteUser(caller);
      remoteUserRef.current = caller;
      isInitiatorRef.current = false;
      setIncomingCall(null);

      sendSignal('call_answer', { to: caller, from: currentUser, sdp: answer });
    } catch (err) {
      console.error('Failed to accept call:', err);
      sendSignal('call_reject', { to: caller, from: currentUser });
      cleanup();
    }
  }, [incomingCall, createPeerConnection, currentUser, sendSignal, cleanup]);


  const declineCall = useCallback(() => {
    stopRingtone();
    if (incomingCall) {
      sendSignal('call_reject', { to: incomingCall.from, from: currentUser });
    }
    setIncomingCall(null);
    pendingOfferRef.current = null;
    iceQueueRef.current = [];
    setStatus('idle');
  }, [incomingCall, currentUser, sendSignal, stopRingtone]);

  
  const endCall = useCallback(() => {
    if (remoteUser) {
      sendSignal('call_end', { to: remoteUser, from: currentUser });
    }
    cleanup();
  }, [remoteUser, currentUser, sendSignal, cleanup]);

  
  const toggleVideo = useCallback(async () => {
    const pc = pcRef.current;
    if (!pc) return;

    if (!localVideoStreamRef.current) {
      try {
        const videoStream = await navigator.mediaDevices.getUserMedia({ video: true });
        localVideoStreamRef.current = videoStream;
        const videoTrack = videoStream.getVideoTracks()[0];
        pc.addTrack(videoTrack, videoStream);
        if (localVideoRef.current) {
          localVideoRef.current.srcObject = videoStream;
        }
        setIsVideoEnabled(true);
      } catch (err) {
        console.error('Failed to enable camera:', err);
      }
      return;
    }

    const newEnabled = !isVideoEnabled;
    localVideoStreamRef.current.getVideoTracks().forEach((t) => (t.enabled = newEnabled));
    setIsVideoEnabled(newEnabled);
  }, [isVideoEnabled]);

  const toggleMute = useCallback(() => {
    if (!localStreamRef.current) return;
    const newMuted = !isMuted;
    localStreamRef.current.getAudioTracks().forEach((t) => (t.enabled = !newMuted));
    setIsMuted(newMuted);
  }, [isMuted]);

  // Once the (audio-only) handshake connects, if this was meant to be a
  // video call, kick off the camera through the same reliable renegotiation
  // path used by the manual toggle button - this fires once per call.
  useEffect(() => {
    if (status === 'connected' && wantsVideoRef.current) {
      wantsVideoRef.current = false;
      toggleVideo();
    }
  }, [status, toggleVideo]);


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
          setIncomingCall({ from: payload.from, hasVideo: !!payload.wantsVideo });
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

        case 'call_renegotiate_offer': {
          if (payload.to !== currentUser || !pcRef.current) return;
          const pc = pcRef.current;
          pc.setRemoteDescription(new RTCSessionDescription(payload.sdp))
            .then(() => pc.createAnswer())
            .then((answer) => pc.setLocalDescription(answer).then(() => answer))
            .then((answer) => {
              sendSignal('call_renegotiate_answer', { to: payload.from, from: currentUser, sdp: answer });
            })
            .catch((err) => console.error('Renegotiation offer handling failed:', err));
          break;
        }

        case 'call_renegotiate_answer': {
          if (payload.to !== currentUser || !pcRef.current) return;
          pcRef.current
            .setRemoteDescription(new RTCSessionDescription(payload.sdp))
            .catch((err) => console.error('Renegotiation answer handling failed:', err));
          break;
        }

        case 'call_reject': {
          if (payload.to !== currentUser) return;
          cleanup('declined');
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
    isVideoEnabled,
    remoteHasVideo,
    remoteAudioRef,
    localVideoRef,
    remoteVideoRef,
    startCall,
    acceptCall,
    declineCall,
    endCall,
    toggleMute,
    toggleVideo,
    handleSignal,
  };
}