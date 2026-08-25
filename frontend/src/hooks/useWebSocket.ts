import { useState, useEffect, useCallback } from 'react';
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
  onCallSignal?: (type: string, payload: any) => void 
) => {
  const [ws, setWs] = useState<WebSocket | null>(null);
  const [isConnected, setIsConnected] = useState(false);

  const sendMessage = useCallback((type: string, payload: any) => {
    if (ws && isConnected) {
      const message = { type, payload };
      ws.send(JSON.stringify(message));
    }
  }, [ws, isConnected]);

  const connect = useCallback(() => {
    if (!username) return;

    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
   
    const wsUrl = `${protocol}//${window.location.hostname}:8081/ws?username=${username}`;

  
    let debugElRaw = document.getElementById('__ws_debug__');
    if (!debugElRaw) {
      debugElRaw = document.createElement('div');
      debugElRaw.id = '__ws_debug__';
      debugElRaw.style.cssText =
        'position:fixed;bottom:0;left:0;right:0;background:#da373c;color:white;' +
        'font-size:13px;font-family:monospace;padding:6px 10px;z-index:999999;' +
        'word-break:break-all;';
      document.body.appendChild(debugElRaw);
    }
    const debugEl = debugElRaw as HTMLDivElement; 
    debugEl.textContent = `WS attempt: ${wsUrl}`;
    
    const socket = new WebSocket(wsUrl);
    
    socket.onopen = () => {
      console.log('✓ WebSocket is connected');
      setIsConnected(true);
      debugEl.textContent = `WS OPEN: ${wsUrl}`;
      debugEl.style.background = '#23a55a';
    };
    
    socket.onclose = () => {
      console.log('✗ WebSocket is disconnected');
      setIsConnected(false);
      debugEl.textContent = `WS CLOSED: ${wsUrl}`;
      debugEl.style.background = '#da373c';
      setTimeout(() => connect(), 3000);
    };
    
    socket.onerror = (error) => {
      console.error('WebSocket error:', error);
      debugEl.textContent = `WS ERROR: ${wsUrl}`;
      debugEl.style.background = '#f0b232';
    };
    
    socket.onmessage = (event) => {
      try {
        const messages = event.data.split('\n').filter(Boolean);
        messages.forEach((raw: string) => {
          const data: WSMessage = JSON.parse(raw);
          handleMessage(data);
        });
      } catch (error) {
        console.error('WebSocket message parsing error:', error);
      }
    };
    
    setWs(socket);
  }, [username]);

  const handleMessage = (data: WSMessage) => {
    switch (data.type) {
      case 'status_update':
        const { username, status } = data.payload;
        console.log(`🔄 Status updated: ${username} -> ${status}`);
        onStatusUpdate(username, status);
        break;
        
      case 'channel_message':
        const { channel, message } = data.payload;
        console.log(`📨 Message in #${channel} from ${message.user}`);
        onNewMessage(channel, message);
        break;

   
      case 'message_deleted':
        if (onMessageDeleted) {
          onMessageDeleted(data.payload.channel, data.payload.messageId);
        }
        break;

    
      case 'call_offer':
      case 'call_answer':
      case 'call_ice_candidate':
      case 'call_reject':
      case 'call_end':
        if (onCallSignal) {
          onCallSignal(data.type, data.payload);
        }
        break;
        
      case 'subscribed':
        console.log(`✅ Subscribed to channel: ${data.payload.channel}`);
        break;
        
      case 'connected':
        console.log('WebSocket: ' + data.payload.message);
        break;

      case 'users_list':
        console.log(`👥 Received users list: ${data.payload.length}`);
        data.payload.forEach((user: any) => {
          onStatusUpdate(user.username, user.status || 'offline');
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
    console.log(`🔄 Sending status change: ${status}`);
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