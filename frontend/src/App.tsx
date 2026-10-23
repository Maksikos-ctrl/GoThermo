import React, { useState, useEffect, useCallback, useRef } from 'react';
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
import { ConfirmModal } from './components/ConfirmModal';

import { SearchModal } from './components/SearchModal';
import { useCall } from './hooks/useCall';
import { IncomingCallModal } from './components/IncomingCallModal';
import { CallBar } from './components/CallBar';
import { useGroupCall } from './hooks/useGroupCall';
import { GroupCallInviteModal } from './components/GroupCallInviteModal';
import { StartGroupCallModal } from './components/StartGroupCallModal';
import { GroupCallGrid } from './components/GroupCallGrid';
import { useToast } from './hooks/useToast';
import { ToastContainer } from './components/ToastContainer';
import { useSettings } from './hooks/useSettings';
import { ThemeVariables } from './components/ThemeVariables';
import { SettingsModal } from './components/SettingsModal';
import { playNotificationSound } from './hooks/notificationSound';
import { useIdleStatus } from './hooks/useIdleStatus';
import { clearAuthToken, getAuthToken } from './services/session';



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
  const [newChannelPrivate, setNewChannelPrivate] = useState(false);
  const [newChannelMembers, setNewChannelMembers] = useState<string[]>([]);
  const [isLoadingChannels, setIsLoadingChannels] = useState(false);
  const [isPostMode, setIsPostMode] = useState(false);
  const [newMessage, setNewMessage] = useState('');
  const [showMembersPanel, setShowMembersPanel] = useState(false);


  const [isDragging, setIsDragging] = useState<string | null>(null);
  const [dragOver, setDragOver] = useState<string | null>(null);


  const [dmChannels, setDMChannels] = useState<Channel[]>([]);
  
  const [unreadCounts, setUnreadCounts] = useState<Record<string, number>>({});
  // Always points at the channel on screen, so async callbacks never see a stale one.
  const currentChannelRef = useRef(currentChannel);
  currentChannelRef.current = currentChannel;

  const { toasts, showToast, removeToast } = useToast();
  const { settings, updateSettings } = useSettings(currentUser);
  const [showSettingsModal, setShowSettingsModal] = useState(false);
  const appliedDefaultStatusRef = useRef(false);

  const [confirmDialog, setConfirmDialog] = useState<{
    message: string;
    onConfirm: () => void;
  } | null>(null);

  const [showSearchModal, setShowSearchModal] = useState(false);

  const callSignalHandlerRef = useRef<((type: string, payload: any) => void) | null>(null);
  const groupCallSignalHandlerRef = useRef<((type: string, payload: any) => void) | null>(null);
  const { isConnected, subscribeToChannel, changeStatus, sendMessage } = useWebSocket(
    currentUser,
    handleStatusUpdate,
    handleNewMessage,
    handleMessageDeleted,
    (type, payload) => callSignalHandlerRef.current?.(type, payload),
    (type, payload) => groupCallSignalHandlerRef.current?.(type, payload),
    handleChannelCreated,
    handleChannelDeleted
  );

  const call = useCall({
    currentUser,
    sendSignal: sendMessage,
    onCallEnded: handleCallEnded,
    onError: (msg) => showToast(msg, 'error'),
    ringtoneVolume: settings.ringtoneVolume,
  });
  const groupCall = useGroupCall({
    currentUser,
    sendSignal: sendMessage,
    onGroupCallEnded: handleGroupCallEnded,
    onError: (msg) => showToast(msg, 'error'),
  });

  async function setStatusAndBroadcast(status: StatusType) {
    try {
      await api.users.updateStatus(currentUser, status);
      setCurrentUserStatus(status);
      changeStatus(status);
    } catch (error) {
      console.error('Error updating status:', error);
    }
  }

  useIdleStatus({
    enabled: isLoggedIn,
    currentStatus: currentUserStatus,
    inCall: call.status !== 'idle' || groupCall.inCall,
    onGoAway: () => setStatusAndBroadcast('away'),
    onReturn: () => setStatusAndBroadcast('online'),
  });

  callSignalHandlerRef.current = call.handleSignal;
  groupCallSignalHandlerRef.current = groupCall.handleGroupSignal;

  const [showStartGroupCallModal, setShowStartGroupCallModal] = useState(false);



 
  function handleStatusUpdate(username: string, status: string) {
    setUsers(prev => {
      const exists = prev.some(u => u.username === username);
      if (exists) {
        return prev.map(user =>
          user.username === username
            ? { ...user, status: status as StatusType, isOnline: status !== 'offline' }
            : user
        );
      }
      
      return [
        ...prev,
        {
          id: username,
          username,
          email: '',
          isOnline: status !== 'offline',
          status: status as StatusType,
        },
      ];
    });
  }

  function handleNewMessage(channel: string, message: Message) {
    if (message.user !== currentUser && settings.messageSoundEnabled) {
      playNotificationSound(0.5);
    }
    if (channel === currentChannel) {
      // The user is looking at this channel, so the message is read at once.
      // Without this the backend counts it as unread after lastRead (set when
      // the channel was opened) and the badge comes back on the next refresh.
      if (message.user !== currentUser) {
        void markChannelReadNow(channel);
      }
      setMessages(prev => {
        const index = prev.findIndex(m => m.id === message.id);
        if (index === -1) {
          return [...prev, message];
        }
       
        const next = [...prev];
        next[index] = message;
        return next;
      });
    } else if (message.user !== currentUser) {
      setUnreadCounts(prev => ({ ...prev, [channel]: (prev[channel] || 0) + 1 }));
    }
  }

  async function markChannelReadNow(channel: string) {
    if (!currentUser) return;
    try {
      await api.unread.markRead(currentUser, channel);
    } catch (error) {
      console.error('Error marking channel as read:', error);
    }
  }

  function handleChannelCreated(channel: Channel) {
    // DMs are managed elsewhere. A private channel is only ever sent to its
    // members; an event for a channel we already have is an update (new member).
    if (channel.isPrivate && channel.name.startsWith('dm_')) return;
    setChannels(prev =>
      prev.some(ch => ch.name === channel.name)
        ? prev.map(ch => (ch.name === channel.name ? { ...ch, ...channel, order: ch.order } : ch))
        : [...prev, channel]
    );
  }

  function handleChannelDeleted(channelName: string) {
    setChannels(prev => prev.filter(ch => ch.name !== channelName));
    if (channelName === currentChannel) {
      setCurrentChannel('general');
    }
  }

  function handleMessageDeleted(channel: string, messageId: string) {
    if (channel !== currentChannel) return;
    setMessages(prev => prev.filter(m => m.id !== messageId));
  }

  async function handleCallEnded(info: {
    remoteUser: string;
    status: 'completed' | 'declined' | 'cancelled';
    durationSeconds: number;
  }) {
    try {
      const dmChannel = await api.dm.getOrCreate(currentUser, info.remoteUser);
      await api.calls.log(dmChannel.name, currentUser, info.status, info.durationSeconds);
    } catch (error) {
      console.error('Error logging call:', error);
    }
  }

  async function handleGroupCallEnded(info: {
    participants: string[];
    durationSeconds: number;
    status: 'completed' | 'cancelled';
  }) {
    const originChannel = groupCallChannelRef.current;
    groupCallChannelRef.current = null;
    if (originChannel) {
      try {
        await api.calls.log(originChannel, currentUser, info.status, info.durationSeconds);
      } catch (error) {
        console.error('Error logging channel call:', error);
      }
      return;
    }
    try {
      const groupChannel = await api.dm.getOrCreateGroup(info.participants, currentUser);
      await api.calls.log(groupChannel.name, currentUser, info.status, info.durationSeconds);
    } catch (error) {
      console.error('Error logging group call:', error);
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

  const isGroupChannel = (channelName: string) => channelName.startsWith('dm_group_');

  const getGroupMembers = (): string[] => {
    const channel = dmChannels.find(ch => ch.name === currentChannel);
    return channel ? channel.members.filter(m => m !== currentUser) : [];
  };

  // Members of the open channel when it is a private channel (calls there
  // are group calls with everyone in it).
  const getPrivateChannelMembers = (): string[] | null => {
    const channel = channels.find(ch => ch.name === currentChannel);
    if (!channel || !channel.isPrivate) return null;
    return (channel.members || []).filter(m => m !== currentUser);
  };

  // Which channel the current group call was started from, so the call log
  // goes there. null = a regular group call (logged to its own group chat).
  const groupCallChannelRef = useRef<string | null>(null);

  const startGroupCallFromChannel = (members: string[]) => {
    if (members.length === 0) {
      showToast('No one else in this channel yet', 'error');
      return;
    }
    groupCallChannelRef.current = currentChannel;
    groupCall.startGroupCall(members);
  };

  const getDMPartner = (): string | null => {
    if (!currentChannel.startsWith('dm_') || isGroupChannel(currentChannel)) return null;
    const names = currentChannel.replace('dm_', '').split('_');
    return names.find(n => n !== currentUser) || null;
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
      // Order matters: save "read" first, then ask for counts. Run together,
      // the counts could be computed before lastRead is saved and bring the
      // badge of the open channel back.
      markCurrentChannelAsRead().then(loadUnreadCounts);
    }
  }, [currentChannel, isLoggedIn]);

 
  useEffect(() => {
    if (!isLoggedIn) return;
    
    const interval = setInterval(() => {
      loadChannels();
      loadMessages();
      loadUsers();
      loadUnreadCounts();
    }, 45000);
    
    return () => clearInterval(interval);
  }, [isLoggedIn, currentChannel]);

  useEffect(() => {
    if (currentChannel && isConnected) {
      subscribeToChannel(currentChannel);
    }
  }, [currentChannel, isConnected, subscribeToChannel]);

  
  useEffect(() => {
    if (!isLoggedIn || !isConnected || appliedDefaultStatusRef.current) return;
    appliedDefaultStatusRef.current = true;
    if (settings.defaultStatus !== 'online') {
      api.users.updateStatus(currentUser, settings.defaultStatus)
        .then(() => {
          setCurrentUserStatus(settings.defaultStatus);
          changeStatus(settings.defaultStatus);
        })
        .catch((error) => console.error('Error applying default status:', error));
    }
  }, [isLoggedIn, isConnected, currentUser, settings.defaultStatus, changeStatus]);


  const handleLogin = (username: string) => {
    setCurrentUser(username);
    setIsLoggedIn(true);
    appliedDefaultStatusRef.current = false;
  };

  const handleLogout = async () => {
    try {
      await api.users.updateStatus(currentUser, 'offline');
    } catch (error) {
      console.error('Error setting offline status on logout:', error);
    }
    try {
      await api.auth.logout(getAuthToken());
    } catch (error) {
      console.error('Error logging out:', error);
    }
    clearAuthToken();
    setShowSettingsModal(false);
    setIsLoggedIn(false);
    setCurrentUser('');
    setCurrentUserStatus('online');
    setMessages([]);
    setChannels([]);
    setUsers([]);
    setDMChannels([]);
    setUnreadCounts({});
    setCurrentChannel('general');
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
       
      } catch (error) {
        console.error('Error sending message:', error);
      }
    }
  };

   
  const handleSendFile = async (fileName: string, mimeType: string, base64Data: string) => {
    try {
      await api.files.send(currentUser, currentChannel, fileName, mimeType, base64Data);
      
    } catch (error: any) {
      showToast(`Failed to send file: ${error}`, 'error');
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
     
    } catch (error) {
      console.error('Error adding reaction:', error);
    }
  };

  const handleAddChannelMember = async (channelName: string, username: string) => {
    try {
      const updated = await api.channels.addMember(channelName, currentUser, username);
      setChannels(prev =>
        prev.map(ch => (ch.name === channelName ? { ...ch, ...updated, order: ch.order } : ch))
      );
      showToast(`${username} added to #${channelName}`, 'success');
    } catch (error: any) {
      showToast(`Could not add member: ${error}`, 'error');
    }
  };

  const handleCreateChannel = async () => {
    if (!newChannelName.trim()) {
      showToast('Please enter a channel name', 'error');
      return;
    }

    try {
      const channel = await api.channels.create(
        newChannelName.trim(),
        newChannelDescription.trim(),
        currentUser,
        newChannelPrivate,
        newChannelMembers
      );
      
      setChannels(prev => [...prev, { ...channel, order: prev.length }]);
      setShowCreateChannelModal(false);
      setNewChannelName('');
      setNewChannelDescription('');
      setNewChannelPrivate(false);
      setNewChannelMembers([]);
      setCurrentChannel(channel.name);
      showToast(`Channel #${channel.name} created`, 'success');
      // Other clients pick this up via the channel_created WS broadcast;
      // we already have it locally, no reload needed.
    } catch (error: any) {
      showToast(`Error creating channel: ${error}`, 'error');
    }
  };

  const handleDeleteChannel = (channelName: string) => {
    setConfirmDialog({
      message: `Delete channel "${channelName}"? This cannot be undone.`,
      onConfirm: async () => {
        setConfirmDialog(null);
        try {
          await api.channels.delete(channelName, currentUser);
          setChannels(prev => prev.filter(ch => ch.name !== channelName));
  
          if (channelName === currentChannel && channels.length > 0) {
            const remainingChannels = channels.filter(ch => ch.name !== channelName);
            if (remainingChannels.length > 0) {
              setCurrentChannel(remainingChannels[0].name);
            }
          }
          showToast(`Channel #${channelName} deleted`, 'success');
         
        } catch (error: any) {
          showToast(`Error deleting channel: ${error}`, 'error');
        }
      },
    });
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
      showToast(`DM with ${username} failed: ${error}`, 'error');
    }
  };


  const startVideoCall = () => {
    if (isGroupChannel(currentChannel)) {
      const members = getGroupMembers();
      if (members.length === 0) {
        showToast('No one else in this group chat yet', 'error');
        return;
      }
      groupCallChannelRef.current = null;
      groupCall.startGroupCall(members);
      return;
    }
    const privateMembers = getPrivateChannelMembers();
    if (privateMembers) {
      startGroupCallFromChannel(privateMembers);
      return;
    }
    const partner = getDMPartner();
    if (!partner) {
      showToast('Calls are available in direct messages and private channels', 'error');
      return;
    }
    call.startCall(partner, true);
  };

  const startAudioCall = () => {
    if (isGroupChannel(currentChannel)) {
      const members = getGroupMembers();
      if (members.length === 0) {
        showToast('No one else in this group chat yet', 'error');
        return;
      }
      groupCallChannelRef.current = null;
      groupCall.startGroupCall(members);
      return;
    }
    const privateMembers = getPrivateChannelMembers();
    if (privateMembers) {
      startGroupCallFromChannel(privateMembers);
      return;
    }
    const partner = getDMPartner();
    if (!partner) {
      showToast('Calls are available in direct messages and private channels', 'error');
      return;
    }
    call.startCall(partner);
  };
 


  if (!isLoggedIn) {
    return <Login onLogin={handleLogin} />;
  }

  const loadUnreadCounts = async() => {
    try {
      const counts = await api.unread.getCounts(currentUser);
      // The open channel is never unread, whatever the server says.
      setUnreadCounts({ ...(counts || {}), [currentChannelRef.current]: 0 });
    }
    catch (error) {
      console.error('Error loading unread counts:', error);
    }
  };

  const markCurrentChannelAsRead = async () => {
    if (!currentUser || !currentChannel) return;
    try {
      await api.unread.markRead(currentUser, currentChannel);
      
      setUnreadCounts(prev => ({ ...prev, [currentChannel]: 0 }));
    } catch (error) {
      console.error('Error marking channel as read:', error);
    }
  };

  const handleDeleteDM = (channelName: string) => {
    setConfirmDialog({
      message: 'Delete this conversation? This cannot be undone.',
      onConfirm: async () => {
        setConfirmDialog(null);
        try {
          await api.dm.delete(channelName, currentUser);
          setDMChannels(prev => prev.filter(ch => ch.name !== channelName));
  
          if (channelName === currentChannel) {
            setCurrentChannel('general');
          }
        } catch (error: any) {
          showToast(`Failed to delete chat: ${error}`, 'error');
        }
      },
    });
  };



  const handleEditMessage = async (messageId: string, newText: string) => {
    try {
      await api.messages.edit(messageId, currentChannel, currentUser, newText);
      setMessages(prev => prev.map(msg =>
        msg.id === messageId ? { ...msg, text: newText, isEdited: true } : msg
      ));
    } catch (error) {
      console.error('Error editing message:', error);
    }
  };


  const handleDeleteMessage = (messageId: string) => {
    setConfirmDialog({
      message: 'Delete this message? This cannot be undone.',
      onConfirm: async () => {
        setConfirmDialog(null);
        try {
          await api.messages.delete(messageId, currentChannel, currentUser);
          setMessages(prev => prev.filter(msg => msg.id !== messageId));
        } catch (error) {
          console.error('Error deleting message:', error);
        }
      },
    });
  };

  const allUsernames = users.map(u => u.username);


  


  return (
    <div className="chat-container">
      <ThemeVariables />
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
        onOpenSettings={() => setShowSettingsModal(true)}
        onChangeStatusViaWS={changeStatus}
      />

      <ChannelSidebar
        channels={channels}
        dmChannels={dmChannels}
        currentChannel={currentChannel}
        unreadCounts={unreadCounts} 
        isLoading={isLoadingChannels}
        currentUser={currentUser}
        onChannelChange={setCurrentChannel}
        onCreateChannel={() => {
          setNewChannelPrivate(false);
          setNewChannelMembers([]);
          setShowCreateChannelModal(true);
        }}
        onDeleteChannel={handleDeleteChannel}
        onDeleteDM={handleDeleteDM}  
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
        isPrivate={newChannelPrivate}
        availableUsers={users.map(u => u.username).filter(n => n !== currentUser)}
        selectedMembers={newChannelMembers}
        onClose={() => setShowCreateChannelModal(false)}
        onCreate={handleCreateChannel}
        onNameChange={setNewChannelName}
        onDescriptionChange={setNewChannelDescription}
        onPrivateChange={setNewChannelPrivate}
        onToggleMember={(name) =>
          setNewChannelMembers(prev =>
            prev.includes(name) ? prev.filter(n => n !== name) : [...prev, name]
          )
        }
      />

      <div className="main-content">
        <ChatHeader
          currentChannel={currentChannel}
          channels={channels}
          messagesCount={messages.length}
          onStartVideoCall={startVideoCall}
          onStartAudioCall={startAudioCall}
          onShowMembers={() => setShowMembersPanel(true)}
          onOpenSearch={() => setShowSearchModal(true)} 
        />


        <MessagesList
          messages={messages}
          currentChannel={currentChannel}
          currentUser={currentUser}
          onAddReaction={handleAddReaction}
          onEditMessage={handleEditMessage}
          onDeleteMessage={handleDeleteMessage}
          knownUsernames={allUsernames}
        />

        <MessageComposer
          currentChannel={currentChannel}
          isPostMode={isPostMode}
          message={newMessage}
          onSend={handleSendMessage}
          onMessageChange={setNewMessage}
          onTogglePostMode={() => setIsPostMode(!isPostMode)}
          mentionableUsers={allUsernames}
          onSendFile={handleSendFile}
        />
      </div>

      
      {showMembersPanel && (
        <ChannelMembers
          channel={[...channels, ...dmChannels].find(ch => ch.name === currentChannel) || null}
          users={getChannelMembers()}
          currentChannel={currentChannel}
          onClose={() => setShowMembersPanel(false)}
          currentUser={currentUser}
          allUsers={users}
          onAddMember={(username) => handleAddChannelMember(currentChannel, username)}
        />
      )}
      <ConfirmModal
        isOpen={confirmDialog !== null}
        message={confirmDialog?.message || ''}
        onConfirm={() => confirmDialog?.onConfirm()}
        onCancel={() => setConfirmDialog(null)}
      />
      <SearchModal
        isOpen={showSearchModal}
        currentUser={currentUser}
        onClose={() => setShowSearchModal(false)}
        onSelectChannel={(channelName) => {
          setCurrentChannel(channelName);
        }}
      />  
      <IncomingCallModal
        callerName={call.incomingCall?.from || null}
        isVideoCall={call.incomingCall?.hasVideo || false}
        onAccept={call.acceptCall}
        onDecline={call.declineCall}
      />

      <CallBar
        status={call.status}
        remoteUser={call.remoteUser}
        isMuted={call.isMuted}
        isVideoEnabled={call.isVideoEnabled}
        remoteHasVideo={call.remoteHasVideo}
        isScreenSharing={call.isScreenSharing}
        remoteIsScreenSharing={call.remoteIsScreenSharing}
        remoteAudioRef={call.remoteAudioRef}
        localVideoRef={call.localVideoRef}
        remoteVideoRef={call.remoteVideoRef}
        remoteScreenVideoRef={call.remoteScreenVideoRef}
        onToggleMute={call.toggleMute}
        onToggleVideo={call.toggleVideo}
        onToggleScreenShare={call.toggleScreenShare}
        onEndCall={call.endCall}
      />

      <button
        onClick={() => setShowStartGroupCallModal(true)}
        title="Start a group call"
        style={{
          position: 'fixed',
          bottom: '20px',
          left: showUserPanel ? '260px' : '20px',
          zIndex: 9998,
          width: '44px',
          height: '44px',
          borderRadius: '50%',
          background: '#5865f2',
          border: 'none',
          color: 'white',
          fontSize: '18px',
          cursor: 'pointer',
          boxShadow: '0 4px 12px rgba(0,0,0,0.35)',
        }}
      >
        👥
      </button>

      <StartGroupCallModal
        isOpen={showStartGroupCallModal}
        users={users}
        currentUser={currentUser}
        onClose={() => setShowStartGroupCallModal(false)}
        onStart={(usernames) => {
          groupCallChannelRef.current = null;
          groupCall.startGroupCall(usernames);
        }}
      />

      <GroupCallInviteModal
        from={groupCall.incomingInvite?.from || null}
        participants={groupCall.incomingInvite?.participants || []}
        onAccept={groupCall.acceptGroupInvite}
        onDecline={groupCall.declineGroupInvite}
      />

      <GroupCallGrid
        inCall={groupCall.inCall}
        participants={groupCall.participants}
        joinedParticipants={groupCall.joinedParticipants}
        currentUser={currentUser}
        isMuted={groupCall.isMuted}
        isVideoEnabled={groupCall.isVideoEnabled}
        isScreenSharing={groupCall.isScreenSharing}
        remotePeers={groupCall.remotePeers}
        localVideoRef={groupCall.localVideoRef}
        registerRemoteAudioRef={groupCall.registerRemoteAudioRef}
        registerRemoteVideoRef={groupCall.registerRemoteVideoRef}
        registerRemoteScreenVideoRef={groupCall.registerRemoteScreenVideoRef}
        onToggleMute={groupCall.toggleMute}
        onToggleVideo={groupCall.toggleVideo}
        onToggleScreenShare={groupCall.toggleScreenShare}
        onLeave={groupCall.leaveGroupCall}
      />

      <ToastContainer toasts={toasts} onDismiss={removeToast} />

      <SettingsModal
        isOpen={showSettingsModal}
        settings={settings}
        onUpdate={updateSettings}
        onClose={() => setShowSettingsModal(false)}
        onLogout={handleLogout}
      />

    </div>
  );
}

export default App;