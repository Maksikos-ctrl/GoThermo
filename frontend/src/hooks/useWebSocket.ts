import { useState, useEffect, useCallback, useRef } from 'react';
import { Message } from '../types';

interface WSMessage {
  type: string;
  payload: any;
}

export const useWebSocket = (
  username: string,
  onStatusUpdate: (username: string, status: string) => void,
  onNewMessage: (channel: string, message: Message) => void,
  onMessageDeleted?: (channel: string, messageId: string) => void,
  onCallSignal?: (type: string, payload: any) => void,
  onGroupCallSignal?: (type: string, payload: any) => void,
  onChannelCreated?: (channel: any) => void,
  onChannelDeleted?: (channelName: string) => void
) => {
  const [ws, setWs] = useState<WebSocket | null>(null);
  const [isConnected, setIsConnected] = useState(false);

 
  const onStatusUpdateRef = useRef(onStatusUpdate);
  const onNewMessageRef = useRef(onNewMessage);
  const onMessageDeletedRef = useRef(onMessageDeleted);
  const onCallSignalRef = useRef(onCallSignal);
  const onGroupCallSignalRef = useRef(onGroupCallSignal);
  const onChannelCreatedRef = useRef(onChannelCreated);
  const onChannelDeletedRef = useRef(onChannelDeleted);

  onStatusUpdateRef.current = onStatusUpdate;
  onNewMessageRef.current = onNewMessage;
  onMessageDeletedRef.current = onMessageDeleted;
  onCallSignalRef.current = onCallSignal;
  onGroupCallSignalRef.current = onGroupCallSignal;
  onChannelCreatedRef.current = onChannelCreated;
  onChannelDeletedRef.current = onChannelDeleted;

  const sendMessage = useCallback((type: string, payload: any) => {
    if (ws && isConnected) {
      const message = { type, payload };
      ws.send(JSON.stringify(message));
    }
  }, [ws, isConnected]);

  const connect = useCallback(() => {
    if (!username) return;

    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
   
    const wsUrl = `${protocol}//127.0.0.1:8081/ws?username=${username}`;

    const socket = new WebSocket(wsUrl);

    socket.onopen = () => {
      setIsConnected(true);
    };

    socket.onclose = () => {
      setIsConnected(false);
      setTimeout(() => connect(), 3000);
    };

    socket.onerror = (error) => {
      console.error('WebSocket error:', error);
    };

    socket.onmessage = (event) => {
      try {
        const messages = event.data.split('\n').filter(Boolean);
        messages.forEach((raw: string) => {
          const data: WSMessage = JSON.parse(raw);
          handleMessage(data);
        });
      } catch (error) {
        console.error('Error parsing WebSocket message:', error);
      }
    };

    setWs(socket);
  }, [username]);

  const handleMessage = (data: WSMessage) => {
    switch (data.type) {
      case 'status_update':
        const { username, status } = data.payload;
        onStatusUpdateRef.current(username, status);
        break;

      case 'channel_message':
        const { channel, message } = data.payload;
        onNewMessageRef.current(channel, message);
        break;

      case 'message_deleted':
        if (onMessageDeletedRef.current) {
          onMessageDeletedRef.current(data.payload.channel, data.payload.messageId);
        }
        break;

      case 'channel_created':
        if (onChannelCreatedRef.current) {
          onChannelCreatedRef.current(data.payload);
        }
        break;

      case 'channel_deleted':
        if (onChannelDeletedRef.current) {
          onChannelDeletedRef.current(data.payload.channel);
        }
        break;

      case 'call_offer':
      case 'call_answer':
      case 'call_ice_candidate':
      case 'call_reject':
      case 'call_end':
      case 'call_renegotiate_offer':
      case 'call_renegotiate_answer':
      case 'call_screen_share_status':
        if (onCallSignalRef.current) {
          onCallSignalRef.current(data.type, data.payload);
        }
        break;

      case 'group_call_invite':
      case 'group_call_accept':
      case 'group_call_decline':
      case 'group_call_join':
      case 'group_call_roster':
      case 'group_call_offer':
      case 'group_call_answer':
      case 'group_call_ice_candidate':
      case 'group_call_leave':
      case 'group_call_screen_share_status':
        if (onGroupCallSignalRef.current) {
          onGroupCallSignalRef.current(data.type, data.payload);
        }
        break;

      case 'subscribed':
        break;

      case 'connected':
        break;

      case 'users_list':
        data.payload.forEach((user: any) => {
          onStatusUpdateRef.current(user.username, user.status || 'offline');
        });
        break;

      case 'pong':
        break;
    }
  };

  const subscribeToChannel = useCallback((channel: string) => {
    sendMessage('subscribe_channel', channel);
  }, [sendMessage]);

  const changeStatus = useCallback((status: 'online' | 'away' | 'offline') => {
    sendMessage('status_change', { status });
  }, [sendMessage]);

  useEffect(() => {
    if (!isConnected) return;

    const pingInterval = setInterval(() => {
      sendMessage('ping', null);
    }, 25000);

    return () => clearInterval(pingInterval);
  }, [isConnected, sendMessage]);

  useEffect(() => {
    if (username) {
      connect();
    }

    return () => {
      if (ws) {
        ws.close();
      }
    };
  }, [username]);

  return {
    ws,
    isConnected,
    subscribeToChannel,
    changeStatus,
    sendMessage
  };
};