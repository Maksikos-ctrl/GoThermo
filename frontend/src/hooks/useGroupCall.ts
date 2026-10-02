import { useState, useRef, useCallback, useEffect } from 'react';

export const MAX_GROUP_PARTICIPANTS = 4;

interface IncomingGroupInvite {
  callId: string;
  from: string;
  participants: string[]; 
}

interface RemotePeerState {
  hasVideo: boolean;
  isScreenSharing: boolean;
}

interface PeerEntry {
  pc: RTCPeerConnection;
  initialNegotiationDone: boolean;
  iceQueue: RTCIceCandidateInit[];
  
  expectingScreenTrack: boolean;
}

interface UseGroupCallParams {
  currentUser: string;
  sendSignal: (type: string, payload: any) => void;
  onGroupCallEnded?: (info: {
    participants: string[]; 
    durationSeconds: number;
    status: 'completed' | 'cancelled';
  }) => void;
  onError?: (message: string) => void;
}

const ICE_SERVERS = [{ urls: 'stun:stun.l.google.com:19302' }];

export function useGroupCall({ currentUser, sendSignal, onGroupCallEnded, onError }: UseGroupCallParams) {
  const [inCall, setInCall] = useState(false);
  const [participants, setParticipants] = useState<string[]>([]);
  const [joinedParticipants, setJoinedParticipants] = useState<string[]>([]);
  const [incomingInvite, setIncomingInvite] = useState<IncomingGroupInvite | null>(null);
  const [isMuted, setIsMuted] = useState(false);
  const [isVideoEnabled, setIsVideoEnabled] = useState(false);
  const [isScreenSharing, setIsScreenSharing] = useState(false);
  const [remotePeers, setRemotePeers] = useState<Record<string, RemotePeerState>>({});

  const callIdRef = useRef<string | null>(null);
  const isInitiatorRef = useRef(false);
  const participantsRef = useRef<Set<string>>(new Set());
  const joinedRef = useRef<Set<string>>(new Set());
  const callStartTimeRef = useRef<number | null>(null);

  const peersRef = useRef<Map<string, PeerEntry>>(new Map());
  const localStreamRef = useRef<MediaStream | null>(null);
  const localVideoStreamRef = useRef<MediaStream | null>(null);
  const localScreenStreamRef = useRef<MediaStream | null>(null);
  const screenSendersRef = useRef<Map<string, RTCRtpSender>>(new Map());

  const localVideoRef = useRef<HTMLVideoElement | null>(null);
  const remoteAudioRefs = useRef<Map<string, HTMLAudioElement>>(new Map());
  const remoteVideoRefs = useRef<Map<string, HTMLVideoElement>>(new Map());
  const remoteScreenVideoRefs = useRef<Map<string, HTMLVideoElement>>(new Map());
  const remoteStreams = useRef<
    Map<string, { audio: MediaStream; video: MediaStream | null; screen: MediaStream | null }>
  >(new Map());

 
  const reattachPeerMedia = useCallback((username: string) => {
    const streams = remoteStreams.current.get(username);
    if (!streams) return;
    const audioEl = remoteAudioRefs.current.get(username);
    if (audioEl && audioEl.srcObject !== streams.audio) {
      audioEl.srcObject = streams.audio;
      audioEl.play().catch(() => {});
    }
    const videoEl = remoteVideoRefs.current.get(username);
    if (videoEl && streams.video && videoEl.srcObject !== streams.video) {
      videoEl.srcObject = streams.video;
      videoEl.play().catch(() => {});
    }
    const screenEl = remoteScreenVideoRefs.current.get(username);
    if (screenEl && streams.screen && screenEl.srcObject !== streams.screen) {
      screenEl.srcObject = streams.screen;
      screenEl.play().catch(() => {});
    }
  }, []);

  const registerRemoteAudioRef = useCallback(
    (username: string) => (el: HTMLAudioElement | null) => {
      if (el) {
        remoteAudioRefs.current.set(username, el);
        reattachPeerMedia(username);
      } else {
        remoteAudioRefs.current.delete(username);
      }
    },
    [reattachPeerMedia]
  );

  const registerRemoteVideoRef = useCallback(
    (username: string) => (el: HTMLVideoElement | null) => {
      if (el) {
        remoteVideoRefs.current.set(username, el);
        reattachPeerMedia(username);
      } else {
        remoteVideoRefs.current.delete(username);
      }
    },
    [reattachPeerMedia]
  );

  const registerRemoteScreenVideoRef = useCallback(
    (username: string) => (el: HTMLVideoElement | null) => {
      if (el) {
        remoteScreenVideoRefs.current.set(username, el);
        reattachPeerMedia(username);
      } else {
        remoteScreenVideoRefs.current.delete(username);
      }
    },
    [reattachPeerMedia]
  );

  useEffect(() => {
    if (isVideoEnabled && localVideoRef.current && localVideoStreamRef.current) {
      localVideoRef.current.srcObject = localVideoStreamRef.current;
      localVideoRef.current.play().catch(() => {});
    }
  }, [isVideoEnabled]);

  
  useEffect(() => {
    Object.keys(remotePeers).forEach((username) => reattachPeerMedia(username));
  }, [remotePeers, reattachPeerMedia]);

  const removePeer = useCallback((username: string) => {
    const entry = peersRef.current.get(username);
    if (entry) {
      entry.pc.close();
      peersRef.current.delete(username);
    }
    remoteStreams.current.delete(username);
    remoteAudioRefs.current.delete(username);
    remoteVideoRefs.current.delete(username);
    remoteScreenVideoRefs.current.delete(username);
    screenSendersRef.current.delete(username);
    setRemotePeers((prev) => {
      const next = { ...prev };
      delete next[username];
      return next;
    });
  }, []);

  const cleanupGroupCall = useCallback(() => {
    peersRef.current.forEach((entry) => entry.pc.close());
    peersRef.current.clear();
    remoteStreams.current.clear();
    remoteAudioRefs.current.clear();
    remoteVideoRefs.current.clear();
    remoteScreenVideoRefs.current.clear();
    screenSendersRef.current.clear();

    localStreamRef.current?.getTracks().forEach((t) => t.stop());
    localStreamRef.current = null;
    localVideoStreamRef.current?.getTracks().forEach((t) => t.stop());
    localVideoStreamRef.current = null;
    localScreenStreamRef.current?.getTracks().forEach((t) => t.stop());
    localScreenStreamRef.current = null;
    if (localVideoRef.current) localVideoRef.current.srcObject = null;

    callIdRef.current = null;
    isInitiatorRef.current = false;
    participantsRef.current = new Set();
    joinedRef.current = new Set();
    callStartTimeRef.current = null;

    setInCall(false);
    setParticipants([]);
    setJoinedParticipants([]);
    setIsMuted(false);
    setIsVideoEnabled(false);
    setIsScreenSharing(false);
    setRemotePeers({});
  }, []);

  const announceScreenShareTo = useCallback(
    (peerUsername: string, sharing: boolean) => {
      sendSignal('group_call_screen_share_status', {
        to: peerUsername,
        from: currentUser,
        callId: callIdRef.current,
        sharing,
      });
    },
    [currentUser, sendSignal]
  );

  const createPeerConnectionTo = useCallback(
    (peerUsername: string): PeerEntry => {
      const pc = new RTCPeerConnection({ iceServers: ICE_SERVERS });
      const entry: PeerEntry = { pc, initialNegotiationDone: false, iceQueue: [], expectingScreenTrack: false };

      pc.onicecandidate = (e) => {
        if (e.candidate && e.candidate.candidate) {
          sendSignal('group_call_ice_candidate', {
            to: peerUsername,
            from: currentUser,
            callId: callIdRef.current,
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
        const existing =
          remoteStreams.current.get(peerUsername) || { audio: new MediaStream(), video: null, screen: null };

        if (e.track.kind === 'audio') {
          existing.audio.addTrack(e.track);
        } else if (entry.expectingScreenTrack) {
          entry.expectingScreenTrack = false;
          if (!existing.screen) existing.screen = new MediaStream();
          existing.screen.addTrack(e.track);
          setRemotePeers((prev) => ({
            ...prev,
            [peerUsername]: { hasVideo: prev[peerUsername]?.hasVideo || false, isScreenSharing: true },
          }));
          e.track.onended = () => {
            setRemotePeers((prev) => ({
              ...prev,
              [peerUsername]: { hasVideo: prev[peerUsername]?.hasVideo || false, isScreenSharing: false },
            }));
          };
        } else {
          if (!existing.video) existing.video = new MediaStream();
          existing.video.addTrack(e.track);
          setRemotePeers((prev) => ({
            ...prev,
            [peerUsername]: { hasVideo: true, isScreenSharing: prev[peerUsername]?.isScreenSharing || false },
          }));
          e.track.onended = () => {
            setRemotePeers((prev) => ({
              ...prev,
              [peerUsername]: { hasVideo: false, isScreenSharing: prev[peerUsername]?.isScreenSharing || false },
            }));
          };
        }

        remoteStreams.current.set(peerUsername, existing);
        reattachPeerMedia(peerUsername);
      };

      pc.onconnectionstatechange = () => {
        if (pc.connectionState === 'connected') {
          entry.initialNegotiationDone = true;
        }
        if (pc.connectionState === 'failed' || pc.connectionState === 'closed') {
          removePeer(peerUsername);
        }
      };

      pc.onnegotiationneeded = async () => {
        if (!entry.initialNegotiationDone) return;
        try {
          const offer = await pc.createOffer();
          await pc.setLocalDescription(offer);
          sendSignal('group_call_offer', {
            to: peerUsername,
            from: currentUser,
            callId: callIdRef.current,
            sdp: offer,
          });
        } catch (err) {
          console.error('Group call renegotiation failed:', err);
        }
      };

      localStreamRef.current?.getTracks().forEach((track) => {
        pc.addTrack(track, localStreamRef.current!);
      });
      if (isVideoEnabled && localVideoStreamRef.current) {
        localVideoStreamRef.current.getTracks().forEach((track) => {
          pc.addTrack(track, localVideoStreamRef.current!);
        });
      }
      if (isScreenSharing && localScreenStreamRef.current) {
        localScreenStreamRef.current.getTracks().forEach((track) => {
          const sender = pc.addTrack(track, localScreenStreamRef.current!);
          screenSendersRef.current.set(peerUsername, sender);
        });
        // This peer wasn't around for our original "I'm sharing" broadcast,
        // so let them know before the track/offer arrives.
        announceScreenShareTo(peerUsername, true);
      }

      peersRef.current.set(peerUsername, entry);
      return entry;
    },
    [currentUser, sendSignal, removePeer, reattachPeerMedia, isVideoEnabled, isScreenSharing, announceScreenShareTo]
  );

  const initiateOfferTo = useCallback(
    async (peerUsername: string) => {
      if (peersRef.current.has(peerUsername)) return;
      const entry = createPeerConnectionTo(peerUsername);
      try {
        const offer = await entry.pc.createOffer();
        await entry.pc.setLocalDescription(offer);
        entry.initialNegotiationDone = true;
        sendSignal('group_call_offer', {
          to: peerUsername,
          from: currentUser,
          callId: callIdRef.current,
          sdp: offer,
        });
      } catch (err) {
        console.error('Failed to create group call offer:', err);
      }
    },
    [createPeerConnectionTo, currentUser, sendSignal]
  );

  const broadcastRoster = useCallback(
    (newJoiner: string) => {
      const joinedList = Array.from(joinedRef.current);
      joinedList.forEach((member) => {
        if (member === currentUser) return;
        sendSignal('group_call_roster', {
          to: member,
          from: currentUser,
          callId: callIdRef.current,
          joined: joinedList,
          newJoiner,
        });
      });
    },
    [currentUser, sendSignal]
  );

  const startGroupCall = useCallback(
    async (usernames: string[]) => {
      if (inCall) {
        onError ? onError('You are already in a call') : alert('You are already in a call');
        return;
      }
      const uniqueOthers = Array.from(new Set(usernames.filter((u) => u !== currentUser)));
      if (uniqueOthers.length === 0) {
        onError ? onError('Pick at least one person to call') : alert('Pick at least one person to call');
        return;
      }
      if (uniqueOthers.length + 1 > MAX_GROUP_PARTICIPANTS) {
        const msg = `Group calls support up to ${MAX_GROUP_PARTICIPANTS} people for now`;
        onError ? onError(msg) : alert(msg);
        return;
      }

      try {
        const stream = await navigator.mediaDevices.getUserMedia({ audio: true });
        localStreamRef.current = stream;

        const callId = `${currentUser}-${Date.now()}`;
        const allParticipants = [currentUser, ...uniqueOthers];

        callIdRef.current = callId;
        isInitiatorRef.current = true;
        participantsRef.current = new Set(allParticipants);
        joinedRef.current = new Set([currentUser]);
        callStartTimeRef.current = Date.now();

        setParticipants(allParticipants);
        setJoinedParticipants([currentUser]);
        setInCall(true);

        uniqueOthers.forEach((u) => {
          sendSignal('group_call_invite', {
            to: u,
            from: currentUser,
            callId,
            participants: allParticipants,
          });
        });
      } catch (err) {
        console.error('Failed to start group call:', err);
        cleanupGroupCall();
      }
    },
    [inCall, currentUser, sendSignal, cleanupGroupCall, onError]
  );

  const acceptGroupInvite = useCallback(async () => {
    if (!incomingInvite) return;
    const invite = incomingInvite;

    try {
      const stream = await navigator.mediaDevices.getUserMedia({ audio: true });
      localStreamRef.current = stream;

      callIdRef.current = invite.callId;
      isInitiatorRef.current = false;
      participantsRef.current = new Set(invite.participants);
      joinedRef.current = new Set([currentUser]);

      setParticipants(invite.participants);
      setJoinedParticipants([currentUser]);
      setInCall(true);
      setIncomingInvite(null);

    
      sendSignal('group_call_join', { to: invite.from, from: currentUser, callId: invite.callId });
    } catch (err) {
      console.error('Failed to accept group call:', err);
      setIncomingInvite(null);
    }
  }, [incomingInvite, currentUser, sendSignal]);

  const declineGroupInvite = useCallback(() => {
    if (incomingInvite) {
      sendSignal('group_call_decline', { to: incomingInvite.from, from: currentUser, callId: incomingInvite.callId });
    }
    setIncomingInvite(null);
  }, [incomingInvite, currentUser, sendSignal]);

  const leaveGroupCall = useCallback(() => {
    joinedRef.current.forEach((member) => {
      if (member === currentUser) return;
      sendSignal('group_call_leave', { to: member, from: currentUser, callId: callIdRef.current });
    });

   
    if (isInitiatorRef.current && onGroupCallEnded) {
      const joinedList = Array.from(joinedRef.current);
      const wasConnected = joinedList.length > 1;
      const durationSeconds = wasConnected && callStartTimeRef.current
        ? Math.max(0, Math.round((Date.now() - callStartTimeRef.current) / 1000))
        : 0;
      onGroupCallEnded({
        participants: joinedList,
        durationSeconds,
        status: wasConnected ? 'completed' : 'cancelled',
      });
    }

    cleanupGroupCall();
  }, [currentUser, sendSignal, cleanupGroupCall, onGroupCallEnded]);

  const toggleMute = useCallback(() => {
    if (!localStreamRef.current) return;
    const newMuted = !isMuted;
    localStreamRef.current.getAudioTracks().forEach((t) => (t.enabled = !newMuted));
    setIsMuted(newMuted);
  }, [isMuted]);

  const toggleVideo = useCallback(async () => {
    if (!localVideoStreamRef.current) {
      try {
        const videoStream = await navigator.mediaDevices.getUserMedia({ video: true });
        localVideoStreamRef.current = videoStream;
        const videoTrack = videoStream.getVideoTracks()[0];
        peersRef.current.forEach((entry) => {
          entry.pc.addTrack(videoTrack, videoStream);
        });
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

  const toggleScreenShare = useCallback(async () => {
    if (isScreenSharing) {
      screenSendersRef.current.forEach((sender, peerUsername) => {
        const entry = peersRef.current.get(peerUsername);
        entry?.pc.removeTrack(sender);
      });
      screenSendersRef.current.clear();
      localScreenStreamRef.current?.getTracks().forEach((t) => t.stop());
      localScreenStreamRef.current = null;
      setIsScreenSharing(false);

      joinedRef.current.forEach((member) => {
        if (member === currentUser) return;
        announceScreenShareTo(member, false);
      });
      return;
    }

    try {
      const screenStream = await navigator.mediaDevices.getDisplayMedia({
        video: { cursor: 'always' } as MediaTrackConstraints,
        audio: false,
      });
      const screenTrack = screenStream.getVideoTracks()[0];
      localScreenStreamRef.current = screenStream;

      peersRef.current.forEach((entry, peerUsername) => {
      
        announceScreenShareTo(peerUsername, true);
        const sender = entry.pc.addTrack(screenTrack, screenStream);
        screenSendersRef.current.set(peerUsername, sender);
      });

      setIsScreenSharing(true);

      screenTrack.onended = () => {
        toggleScreenShareRef.current();
      };
    } catch (err) {
      console.error('Failed to start screen share:', err);
    }
  }, [isScreenSharing, currentUser, announceScreenShareTo]);

  const toggleScreenShareRef = useRef<() => void>(() => {});
  useEffect(() => {
    toggleScreenShareRef.current = toggleScreenShare;
  }, [toggleScreenShare]);

  const handleGroupSignal = useCallback(
    (type: string, payload: any) => {
      switch (type) {
        case 'group_call_invite': {
          if (payload.to !== currentUser) return;
          if (inCall) {
            sendSignal('group_call_decline', { to: payload.from, from: currentUser, callId: payload.callId });
            return;
          }
          setIncomingInvite({ callId: payload.callId, from: payload.from, participants: payload.participants || [] });
          break;
        }

        case 'group_call_join': {
          if (payload.to !== currentUser || payload.callId !== callIdRef.current) return;
          if (!isInitiatorRef.current) return;
          joinedRef.current.add(payload.from);
          setJoinedParticipants(Array.from(joinedRef.current));
          broadcastRoster(payload.from);
          
          initiateOfferTo(payload.from);
          break;
        }

        case 'group_call_roster': {
          if (payload.to !== currentUser || payload.callId !== callIdRef.current) return;
          const joinedList: string[] = payload.joined || [];
          joinedRef.current = new Set(joinedList);
          setJoinedParticipants(joinedList);
          if (payload.newJoiner && payload.newJoiner !== currentUser) {
            initiateOfferTo(payload.newJoiner);
          }
          break;
        }

        case 'group_call_offer': {
          if (payload.to !== currentUser || payload.callId !== callIdRef.current) return;
          let entry = peersRef.current.get(payload.from);
          if (!entry) {
            entry = createPeerConnectionTo(payload.from);
          }
          entry.pc
            .setRemoteDescription(new RTCSessionDescription(payload.sdp))
            .then(async () => {
              for (const c of entry!.iceQueue) {
                await entry!.pc.addIceCandidate(new RTCIceCandidate(c));
              }
              entry!.iceQueue = [];
              return entry!.pc.createAnswer();
            })
            .then((answer) => entry!.pc.setLocalDescription(answer).then(() => answer))
            .then((answer) => {
              entry!.initialNegotiationDone = true;
              sendSignal('group_call_answer', {
                to: payload.from,
                from: currentUser,
                callId: callIdRef.current,
                sdp: answer,
              });
            })
            .catch((err) => console.error('Group call offer handling failed:', err));
          break;
        }

        case 'group_call_answer': {
          if (payload.to !== currentUser || payload.callId !== callIdRef.current) return;
          const entry = peersRef.current.get(payload.from);
          if (!entry) return;
          entry.pc
            .setRemoteDescription(new RTCSessionDescription(payload.sdp))
            .then(async () => {
              for (const c of entry.iceQueue) {
                await entry.pc.addIceCandidate(new RTCIceCandidate(c));
              }
              entry.iceQueue = [];
            })
            .catch((err) => console.error('Group call answer handling failed:', err));
          break;
        }

        case 'group_call_ice_candidate': {
          if (payload.to !== currentUser || payload.callId !== callIdRef.current) return;
          if (!payload.candidate || !payload.candidate.candidate) return;
          const entry = peersRef.current.get(payload.from);
          if (entry && entry.pc.remoteDescription) {
            entry.pc.addIceCandidate(new RTCIceCandidate(payload.candidate)).catch(console.error);
          } else if (entry) {
            entry.iceQueue.push(payload.candidate);
          }
          break;
        }

        case 'group_call_decline': {
        
          break;
        }

        case 'group_call_leave': {
          if (payload.callId !== callIdRef.current) return;
          removePeer(payload.from);
          joinedRef.current.delete(payload.from);
          setJoinedParticipants(Array.from(joinedRef.current));
          break;
        }

        case 'group_call_screen_share_status': {
          if (payload.to !== currentUser || payload.callId !== callIdRef.current) return;
          const entry = peersRef.current.get(payload.from);
          if (payload.sharing) {
            if (entry) entry.expectingScreenTrack = true;
          } else {
            if (entry) entry.expectingScreenTrack = false;
            const streams = remoteStreams.current.get(payload.from);
            if (streams) streams.screen = null;
            const screenEl = remoteScreenVideoRefs.current.get(payload.from);
            if (screenEl) screenEl.srcObject = null;
            setRemotePeers((prev) => ({
              ...prev,
              [payload.from]: { hasVideo: prev[payload.from]?.hasVideo || false, isScreenSharing: false },
            }));
          }
          break;
        }
      }
    },
    [currentUser, inCall, sendSignal, broadcastRoster, initiateOfferTo, createPeerConnectionTo, removePeer]
  );

  return {
    inCall,
    participants,
    joinedParticipants,
    incomingInvite,
    isMuted,
    isVideoEnabled,
    isScreenSharing,
    remotePeers,
    localVideoRef,
    registerRemoteAudioRef,
    registerRemoteVideoRef,
    registerRemoteScreenVideoRef,
    startGroupCall,
    acceptGroupInvite,
    declineGroupInvite,
    leaveGroupCall,
    toggleMute,
    toggleVideo,
    toggleScreenShare,
    handleGroupSignal,
  };
}