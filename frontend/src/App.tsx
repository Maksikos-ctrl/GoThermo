import React, { useState, useEffect, useCallback } from 'react';
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
import { ChannelMembers } from './components/ChannelMembers';

function App() {
  
  const [isLoggedIn, setIsLoggedIn] = useState(false);
  const [currentUser, setCurrentUser] = useState('');
  const [currentUserStatus, setCurrentUserStatus] = useState<StatusType>('online');

  
  const [messages, setMessages] = useState<Message[]>([]);
  const [channels, setChannels] = useState<Channel[]>([]);
  const [users, setUsers] = useState<User[]>([]);
  const [currentChannel, setCurrentChannel] = useState('general');

 
  const [showUserPanel, setShowUserPanel] = useState(true);
  const [showCreateChannelModal, setShowCreateChannelModal] = useState(false);
  const [newChannelName, setNewChannelName] = useState('');
  const [newChannelDescription, setNewChannelDescription] = useState('');
  const [isLoadingChannels, setIsLoadingChannels] = useState(false);
  const [isPostMode, setIsPostMode] = useState(false);
  const [newMessage, setNewMessage] = useState('');
  const [showMembersPanel, setShowMembersPanel] = useState(false);


  const [isDragging, setIsDragging] = useState<string | null>(null);
  const [dragOver, setDragOver] = useState<string | null>(null);


  const { isConnected, subscribeToChannel, changeStatus } = useWebSocket(
    currentUser,
    handleStatusUpdate,
    handleNewMessage
  );

  const [dmChannels, setDMChannels] = useState<Channel[]>([]);

 
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

  
  const loadMessages = async () => {
    try {
      const msgs = await api.messages.getByChannel(currentChannel);
      const uniqueMessages = msgs.filter((msg, index, self) =>
        index === self.findIndex((m) => m.id === msg.id)
      );
      setMessages(uniqueMessages || []);
    } catch (error) {
      console.error('Error loading messages:', error);
    }
  };

  const loadChannels = async () => {
    setIsLoadingChannels(true);
    try {
      const channelsList = await api.channels.getAll();
      setChannels(channelsList || []);
    } catch (error) {
      console.error('Error loading channels:', error);
      setChannels([]);
    } finally {
      setIsLoadingChannels(false);
    }
  };

  const loadDMChannels = async () => {
    try {
      const dmChannelsList = await api.dm.getAll(currentUser);
      setDMChannels(dmChannelsList || []);
    } catch (error) {
      console.error('Error loading DM channels:', error);
      setDMChannels([]);
    }
  }

  const loadUsers = async () => {
    try {
      const usersData = await api.users.getAll();
      
      const uniqueUsersMap = new Map<string, User>();
      
      (usersData || []).forEach((user: any) => {
        const typedUser = {
          ...user,
          status: (['online', 'away', 'offline'].includes(user.status) 
            ? user.status 
            : 'offline') as StatusType,
          isOnline: user.status === 'online' || user.status === 'away'
        };
       
        uniqueUsersMap.set(user.username, typedUser);
      });
      
      setUsers(Array.from(uniqueUsersMap.values()));
    } catch (error) {
      console.error('Error loading users:', error);
      setUsers([]);
    }
  };


  const getChannelMembers = useCallback(() => {
    const messageUsers = new Map<string, User>();
    
    messages.forEach(msg => {
      const user = users.find(u => u.username === msg.user);
      if (user) {
        messageUsers.set(user.username, user);
      }
    });
    
    return Array.from(messageUsers.values());
  }, [messages, users]);

 
  useEffect(() => {
    if (isLoggedIn) {
      loadMessages();
      loadChannels();
      loadDMChannels();
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
        console.error('Error sending message:', error);
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
      console.error('Error adding reaction:', error);
    }
  };

  const handleCreateChannel = async () => {
    if (!newChannelName.trim()) {
      alert('Please enter a channel name');
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
      alert(`Error creating channel: ${error}`);
    }
  };

  const handleDeleteChannel = async (channelName: string) => {
    if (!confirm(`Delete channel "${channelName}"?`)) {
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
      alert(`Error deleting channel: ${error}`);
    }
  };

  
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

  
  const startDirectMessage = async (username: string) => {
    try {
      const dmChannel = await api.dm.getOrCreate(currentUser, username);
      await loadDMChannels();
      setCurrentChannel(dmChannel.name);
    } catch (error: any) {
      alert(`DM with ${username} failed: ${error}`);
    }
  };

  const startVideoCall = () => {
    alert('Video call (soon will be implemented)');
  };

  const startAudioCall = () => {
    alert('Audio call (soon will be implemented)');
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
        onChangeStatusViaWS={changeStatus}
      />

      <ChannelSidebar
        channels={channels}
        dmChannels={dmChannels}
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
          onShowMembers={() => setShowMembersPanel(true)} 
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

      
      {showMembersPanel && (
        <ChannelMembers
          channel={channels.find(ch => ch.name === currentChannel) || null}
          users={getChannelMembers()}
          currentChannel={currentChannel}
          onClose={() => setShowMembersPanel(false)}
        />
      )}
    </div>
  );
}

export default App;