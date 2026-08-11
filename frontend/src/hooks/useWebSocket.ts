import { useState, useEffect, useCallback } from 'react';
import { Message } from '../types';

interface WSMessage {
  type: string;
  payload: any;
}

export const useWebSocket = (
  username: string,
  onStatusUpdate: (username: string, status: string) => void,
  onNewMessage: (channel: string, message: Message) => void
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
    const wsUrl = `${protocol}//${window.location.host}/ws?username=${username}`;
    
    const socket = new WebSocket(wsUrl);
    
    socket.onopen = () => {
      console.log('✓ WebSocket is connected');
      setIsConnected(true);
    };
    
    socket.onclose = () => {
      console.log('✗ WebSocket is disconnected');
      setIsConnected(false);
      setTimeout(() => connect(), 3000);
    };
    
    socket.onerror = (error) => {
      console.error('WebSocket error:', error);
    };
    
    socket.onmessage = (event) => {
      try {
        const data: WSMessage = JSON.parse(event.data);
        handleMessage(data);
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
        console.log(`🔄 Status updated: ${username} -> ${status}`);
        onStatusUpdate(username, status);
        break;
        
      case 'channel_message':
        const { channel, message } = data.payload;
        console.log(`📨 Message in #${channel} from ${message.user}`);
        onNewMessage(channel, message);
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