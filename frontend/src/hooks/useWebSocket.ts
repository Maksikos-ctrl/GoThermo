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
  onCallSignal?: (type: string, payload: any) => void // ✅ ДОБАВЛЕНО
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
    // ✅ ИСПРАВЛЕНО - в нативном окне Wails window.location.hostname возвращает
    // служебный "wails.localhost", а не "localhost", и подключение к нашему
    // отдельному серверу на 8081 по этому адресу не проходит. Наш WS-сервер
    // всегда слушает на loopback-интерфейсе, поэтому просто фиксируем 127.0.0.1
    // напрямую - работает одинаково что в браузере, что в десктопном окне.
    const wsUrl = `${protocol}//127.0.0.1:8081/ws?username=${username}`;

    // 🔍 ВРЕМЕННАЯ ДИАГНОСТИКА - красная плашка внизу экрана с точным адресом
    // подключения. Работает через прямой DOM, не зависит от React/title синка.
    // Убери этот блок после того, как разберёмся с багом подключения десктопа.
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
    const debugEl = debugElRaw as HTMLDivElement; // ✅ ФИКС - после этой строки TS точно знает, что не null
    debugEl.textContent = `WS attempt: ${wsUrl}`;
    
    const socket = new WebSocket(wsUrl);
    
    socket.onopen = () => {
      console.log('✓ WebSocket подключен');
      setIsConnected(true);
      debugEl.textContent = `WS OPEN: ${wsUrl}`;
      debugEl.style.background = '#23a55a';
    };
    
    socket.onclose = () => {
      console.log('✗ WebSocket отключен');
      setIsConnected(false);
      debugEl.textContent = `WS CLOSED: ${wsUrl}`;
      debugEl.style.background = '#da373c';
      setTimeout(() => connect(), 3000);
    };
    
    socket.onerror = (error) => {
      console.error('WebSocket ошибка:', error);
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

      // ✅ ДОБАВЛЕНО - удаление сообщения
      case 'message_deleted':
        if (onMessageDeleted) {
          onMessageDeleted(data.payload.channel, data.payload.messageId);
        }
        break;

      // ✅ ДОБАВЛЕНО - все сигналы звонка просто прокидываем наверх в App.tsx
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
        console.log(`✅ Подписан на канал: ${data.payload.channel}`);
        break;
        
      case 'connected':
        console.log('WebSocket: ' + data.payload.message);
        break;

      case 'users_list':
        console.log(`👥 Получен список пользователей: ${data.payload.length}`);
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
    console.log(`🔄 Отправка изменения статуса: ${status}`);
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