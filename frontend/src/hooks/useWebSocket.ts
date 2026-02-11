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

  // ✅ Функция для отправки сообщений
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
        console.log(`🔄 Статус обновлен: ${username} -> ${status}`);
        onStatusUpdate(username, status);
        break;
        
      case 'channel_message':
        const { channel, message } = data.payload;
        console.log(`📨 Сообщение в #${channel} от ${message.user}`);
        onNewMessage(channel, message);
        break;
        
      case 'subscribed':
        console.log(`✅ Подписан на канал: ${data.payload.channel}`);
        break;
        
      case 'connected':
        console.log('WebSocket: ' + data.payload.message);
        break;

      // ✅ НОВОЕ: получаем список всех пользователей
      case 'users_list':
        console.log(`👥 Получен список пользователей: ${data.payload.length}`);
        data.payload.forEach((user: any) => {
          onStatusUpdate(user.username, user.status || 'offline');
        });
        break;
        
      case 'pong':
        // Ответ на ping, ничего не делаем
        break;
    }
  };

  const subscribeToChannel = useCallback((channel: string) => {
    sendMessage('subscribe_channel', channel);
  }, [sendMessage]);

  // ✅ НОВОЕ: функция для изменения статуса
  const changeStatus = useCallback((status: 'online' | 'away' | 'offline') => {
    console.log(`🔄 Отправка изменения статуса: ${status}`);
    sendMessage('status_change', { status });
  }, [sendMessage]);

  // ✅ НОВОЕ: ping каждые 25 секунд для поддержания соединения
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
    changeStatus, // ✅ Экспортируем
    sendMessage 
  };
};