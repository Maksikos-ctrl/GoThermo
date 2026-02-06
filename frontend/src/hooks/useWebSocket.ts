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

  const connect = useCallback(() => {
    if (!username) return;

    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const wsUrl = `${protocol}//${window.location.host}/ws?username=${username}`;
    
    const socket = new WebSocket(wsUrl);
    
    socket.onopen = () => {
      console.log('✓ WebSocket подключен');
      setIsConnected(true);
    };
    
    socket.onclose = () => {
      console.log('✗ WebSocket отключен');
      setIsConnected(false);
      setTimeout(() => connect(), 3000);
    };
    
    socket.onerror = (error) => {
      console.error('WebSocket ошибка:', error);
    };
    
    socket.onmessage = (event) => {
      try {
        const data: WSMessage = JSON.parse(event.data);
        handleMessage(data);
      } catch (error) {
        console.error('Ошибка парсинга WebSocket сообщения:', error);
      }
    };
    
    setWs(socket);
  }, [username]);

  const handleMessage = (data: WSMessage) => {
    switch (data.type) {
      case 'status_update':
        const { username, status } = data.payload;
        onStatusUpdate(username, status);
        break;
        
      case 'channel_message':
        const { channel, message } = data.payload;
        onNewMessage(channel, message);
        break;
        
      case 'subscribed':
        console.log(`Подписан на канал: ${data.payload.channel}`);
        break;
        
      case 'connected':
        console.log('WebSocket: ' + data.payload.message);
        break;
    }
  };

  const subscribeToChannel = useCallback((channel: string) => {
    if (ws && isConnected) {
      const subscribeMsg = {
        type: 'subscribe_channel',
        payload: channel
      };
      ws.send(JSON.stringify(subscribeMsg));
    }
  }, [ws, isConnected]);

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

  return { ws, isConnected, subscribeToChannel };
};