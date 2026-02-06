import React, { useState, useEffect } from 'react';
import './App.css';
import { 
  Message, 
  Channel, 
  User, 
  StatusType 
} from './types';
import { api } from './services/api';
import { useWebSocket } from './hooks/useWebSocket';
import { Login } from './components/Login';
import { UserPanel } from './components/UserPanel';
import { ChannelSidebar } from './components/ChannelSidebar';
import { ChannelModal } from './components/ChannelModal';
import { ChatHeader } from './components/ChatHeader';
import { MessagesList } from './components/MessagesList';
import { MessageComposer } from './components/MessageComposer';

function App() {
  // Состояние авторизации
  const [isLoggedIn, setIsLoggedIn] = useState(false);
  const [currentUser, setCurrentUser] = useState('');
  const [currentUserStatus, setCurrentUserStatus] = useState<StatusType>('online');

  // Состояние данных
  const [messages, setMessages] = useState<Message[]>([]);
  const [channels, setChannels] = useState<Channel[]>([]);
  const [users, setUsers] = useState<User[]>([]);
  const [currentChannel, setCurrentChannel] = useState('general');

  // Состояние UI
  const [showUserPanel, setShowUserPanel] = useState(true);
  const [showCreateChannelModal, setShowCreateChannelModal] = useState(false);
  const [newChannelName, setNewChannelName] = useState('');
  const [newChannelDescription, setNewChannelDescription] = useState('');
  const [isLoadingChannels, setIsLoadingChannels] = useState(false);
  const [isPostMode, setIsPostMode] = useState(false);
  const [newMessage, setNewMessage] = useState('');

  // Состояние drag & drop
  const [isDragging, setIsDragging] = useState<string | null>(null);
  const [dragOver, setDragOver] = useState<string | null>(null);

  // WebSocket
  const { isConnected, subscribeToChannel } = useWebSocket(
    currentUser,
    handleStatusUpdate,
    handleNewMessage
  );

  // Обработчики WebSocket
  function handleStatusUpdate(username: string, status: string) {
    setUsers(prev => prev.map(user => 
      user.username === username 
        ? { 
            ...user, 
            status: status as StatusType,
            isOnline: status !== 'offline'
          } 
        : user
    ));
  }

  function handleNewMessage(channel: string, message: Message) {
    if (channel === currentChannel) {
      setMessages(prev => {
        if (!prev.some(m => m.id === message.id)) {
          return [...prev, message];
        }
        return prev;
      });
    }
  }

  // Загрузка данных
  const loadMessages = async () => {
    try {
      const msgs = await api.messages.getByChannel(currentChannel);
      const uniqueMessages = msgs.filter((msg, index, self) =>
        index === self.findIndex((m) => m.id === msg.id)
      );
      setMessages(uniqueMessages || []);
    } catch (error) {
      console.error('Ошибка загрузки сообщений:', error);
    }
  };

  const loadChannels = async () => {
    setIsLoadingChannels(true);
    try {
      const channelsList = await api.channels.getAll();
      setChannels(channelsList || []);
    } catch (error) {
      console.error('Ошибка загрузки каналов:', error);
      setChannels([]);
    } finally {
      setIsLoadingChannels(false);
    }
  };

  const loadUsers = async () => {
    try {
      const usersData = await api.users.getAll();
      const typedUsers = (usersData || []).map((user: any) => ({
        ...user,
        status: (['online', 'away', 'offline'].includes(user.status) 
          ? user.status 
          : 'offline') as StatusType,
        isOnline: user.status === 'online' || user.status === 'away'
      }));
      setUsers(typedUsers);
    } catch (error) {
      console.error('Ошибка загрузки пользователей:', error);
      setUsers([]);
    }
  };

  // Эффекты
  useEffect(() => {
    if (isLoggedIn) {
      loadMessages();
      loadChannels();
      loadUsers();
    }
  }, [currentChannel, isLoggedIn]);

  useEffect(() => {
    if (!isLoggedIn) return;
    
    const interval = setInterval(() => {
      loadChannels();
      loadMessages();
      loadUsers();
    }, 2000);
    
    return () => clearInterval(interval);
  }, [isLoggedIn, currentChannel]);

  useEffect(() => {
    if (currentChannel && isConnected) {
      subscribeToChannel(currentChannel);
    }
  }, [currentChannel, isConnected, subscribeToChannel]);

  // Обработчики действий
  const handleLogin = (username: string) => {
    setCurrentUser(username);
    setIsLoggedIn(true);
  };

  const handleSendMessage = async () => {
    if (newMessage.trim()) {
      try {
        if (isPostMode) {
          await api.messages.sendPost(currentUser, newMessage, currentChannel);
        } else {
          await api.messages.send(currentUser, newMessage, currentChannel);
        }
        setNewMessage('');
        setIsPostMode(false);
        setTimeout(() => loadMessages(), 100);
      } catch (error) {
        console.error('Ошибка отправки:', error);
      }
    }
  };

  const handleAddReaction = async (messageId: string, emoji: string) => {
    try {
      await api.messages.addReaction(messageId, emoji, currentUser, currentChannel);
      setMessages(prev => prev.map(msg => {
        if (msg.id === messageId) {
          const reactions = { ...msg.reactions };
          const users = reactions?.[emoji] || [];
          const userIndex = users.indexOf(currentUser);
          
          if (userIndex > -1) {
            const newUsers = [...users];
            newUsers.splice(userIndex, 1);
            if (newUsers.length === 0) {
              delete reactions[emoji];
            } else {
              reactions[emoji] = newUsers;
            }
          } else {
            reactions[emoji] = [...users, currentUser];
          }
          
          return { ...msg, reactions };
        }
        return msg;
      }));
      
      setTimeout(() => loadMessages(), 100);
    } catch (error) {
      console.error('Ошибка добавления реакции:', error);
    }
  };

  const handleCreateChannel = async () => {
    if (!newChannelName.trim()) {
      alert('Введите название канала');
      return;
    }

    try {
      const channel = await api.channels.create(
        newChannelName.trim(),
        newChannelDescription.trim(),
        currentUser
      );
      
      setChannels(prev => [...prev, { ...channel, order: prev.length }]);
      setShowCreateChannelModal(false);
      setNewChannelName('');
      setNewChannelDescription('');
      setCurrentChannel(channel.name);
      
      await loadChannels();
    } catch (error: any) {
      alert(`Ошибка создания канала: ${error}`);
    }
  };

  const handleDeleteChannel = async (channelName: string) => {
    if (!confirm(`Удалить канал "${channelName}"?`)) {
      return;
    }

    try {
      await api.channels.delete(channelName, currentUser);
      setChannels(prev => prev.filter(ch => ch.name !== channelName));
      
      if (channelName === currentChannel && channels.length > 0) {
        const remainingChannels = channels.filter(ch => ch.name !== channelName);
        if (remainingChannels.length > 0) {
          setCurrentChannel(remainingChannels[0].name);
        }
      }
      
      await loadChannels();
    } catch (error: any) {
      alert(`Ошибка удаления канала: ${error}`);
    }
  };

  // Drag & Drop обработчики
  const handleDragStart = (e: React.DragEvent, channelId: string) => {
    setIsDragging(channelId);
    e.dataTransfer.setData('text/plain', channelId);
  };

  const handleDragOver = (e: React.DragEvent, channelId: string) => {
    e.preventDefault();
    setDragOver(channelId);
  };

  const handleDragLeave = () => {
    setDragOver(null);
  };

  const handleDrop = (e: React.DragEvent, dropTargetId: string) => {
    e.preventDefault();
    const draggedId = isDragging;
    
    if (draggedId && draggedId !== dropTargetId) {
      const draggedIndex = channels.findIndex(ch => ch.id === draggedId);
      const dropIndex = channels.findIndex(ch => ch.id === dropTargetId);
      
      if (draggedIndex !== -1 && dropIndex !== -1) {
        const newChannels = [...channels];
        const [draggedItem] = newChannels.splice(draggedIndex, 1);
        newChannels.splice(dropIndex, 0, draggedItem);
        
        const updatedChannels = newChannels.map((ch, index) => ({
          ...ch,
          order: index
        }));
        
        setChannels(updatedChannels);
      }
    }
    
    setIsDragging(null);
    setDragOver(null);
  };

  // Вспомогательные функции
  const startDirectMessage = (username: string) => {
    alert(`DM с ${username} (скоро будет)`);
  };

  const startVideoCall = () => {
    alert('Видеозвонок (скоро будет)');
  };

  const startAudioCall = () => {
    alert('Аудиозвонок (скоро будет)');
  };

  if (!isLoggedIn) {
    return <Login onLogin={handleLogin} />;
  }

  return (
    <div className="chat-container">
      <div className={`ws-status ${isConnected ? 'online' : 'offline'}`}>
        {isConnected ? '🟢' : '🔴'}
      </div>

      <UserPanel
        currentUser={currentUser}
        currentUserStatus={currentUserStatus}
        users={users}
        showUserPanel={showUserPanel}
        onTogglePanel={() => setShowUserPanel(!showUserPanel)}
        onStatusChange={setCurrentUserStatus}
        onStartDirectMessage={startDirectMessage}
        onStartVideoCall={startVideoCall}
        onStartAudioCall={startAudioCall}
      />

      <ChannelSidebar
        channels={channels}
        currentChannel={currentChannel}
        isLoading={isLoadingChannels}
        currentUser={currentUser}
        onChannelChange={setCurrentChannel}
        onCreateChannel={() => setShowCreateChannelModal(true)}
        onDeleteChannel={handleDeleteChannel}
        isDragging={isDragging}
        dragOver={dragOver}
        onDragStart={handleDragStart}
        onDragOver={handleDragOver}
        onDragLeave={handleDragLeave}
        onDrop={handleDrop}
      />

      <ChannelModal
        isOpen={showCreateChannelModal}
        channelName={newChannelName}
        channelDescription={newChannelDescription}
        onClose={() => setShowCreateChannelModal(false)}
        onCreate={handleCreateChannel}
        onNameChange={setNewChannelName}
        onDescriptionChange={setNewChannelDescription}
      />

      <div className="main-content">
        <ChatHeader
          currentChannel={currentChannel}
          channels={channels}
          messagesCount={messages.length}
          onStartVideoCall={startVideoCall}
          onStartAudioCall={startAudioCall}
        />

        <MessagesList
          messages={messages}
          currentChannel={currentChannel}
          currentUser={currentUser}
          onAddReaction={handleAddReaction}
        />

        <MessageComposer
          currentChannel={currentChannel}
          isPostMode={isPostMode}
          message={newMessage}
          onSend={handleSendMessage}
          onMessageChange={setNewMessage}
          onTogglePostMode={() => setIsPostMode(!isPostMode)}
        />
      </div>
    </div>
  );
}

export default App;