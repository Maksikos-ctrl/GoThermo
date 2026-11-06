import React, { useRef, useState } from 'react';
import { Video, Phone, Settings, Palette, ChevronDown, ChevronsLeft, ChevronsRight, Check } from 'lucide-react';
import { User, STATUS_OPTIONS, StatusType } from '../types';
import { Avatar } from './Avatar';
import { useAvatar, setAvatar } from './avatarStore';
import logoUrl from '../assets/logo.svg';
import { api } from '../services/api';

interface UserPanelProps {
  currentUser: string;
  currentUserStatus: StatusType;
  users: User[];
  showUserPanel: boolean;
  onTogglePanel: () => void;
  onStatusChange: (status: StatusType) => void;
  onStartDirectMessage: (username: string) => void;
  onStartVideoCall: () => void;
  onStartAudioCall: () => void;
  onOpenSettings: () => void;
  onOpenThemes?: () => void;
  
  onChangeStatusViaWS?: (status: StatusType) => void;
  onAvatarError?: (message: string) => void;
}

export const UserPanel: React.FC<UserPanelProps> = ({
  currentUser,
  currentUserStatus,
  users,
  showUserPanel,
  onTogglePanel,
  onStatusChange,
  onStartDirectMessage,
  onStartVideoCall,
  onStartAudioCall,
  onOpenSettings,
  onOpenThemes,
  onChangeStatusViaWS, 
  onAvatarError,
}) => {
  const [showStatusMenu, setShowStatusMenu] = useState(false);
  const statusMenuRef = useRef<HTMLDivElement>(null);
  const fileRef = useRef<HTMLInputElement>(null);
  const myAvatar = useAvatar(currentUser);

  const pickAvatar = async (file: File | undefined) => {
    if (!file) return;
    if (!file.type.startsWith('image/')) {
      onAvatarError?.('Please choose an image file');
      return;
    }
    if (file.size > 2 * 1024 * 1024) {
      onAvatarError?.('Picture is too big (2 MB max)');
      return;
    }
    try {
      const dataUrl = await new Promise<string>((resolve, reject) => {
        const r = new FileReader();
        r.onload = () => resolve(String(r.result));
        r.onerror = () => reject(r.error);
        r.readAsDataURL(file);
      });
      await api.users.setAvatar(currentUser, dataUrl);
    } catch (e: any) {
      onAvatarError?.(String(e?.message ?? e ?? 'Could not set picture'));
    }
  };

  const dropAvatar = async () => {
    try {
      await api.users.removeAvatar(currentUser);
      setAvatar(currentUser, undefined);
    } catch (e: any) {
      onAvatarError?.(String(e?.message ?? e ?? 'Could not remove picture'));
    }
  };

 
  const uniqueUsersMap = new Map<string, User>();
  users.forEach(user => {
    uniqueUsersMap.set(user.username, user);
  });
  const uniqueUsers = Array.from(uniqueUsersMap.values());

  const otherUsers = uniqueUsers.filter(user => user.username !== currentUser);
  const onlineUsers = otherUsers.filter(user => user.status === 'online');
  const awayUsers = otherUsers.filter(user => user.status === 'away');
  const offlineUsers = otherUsers.filter(user => user.status === 'offline');

  const handleStatusChange = async (status: StatusType) => {
    try {
      
      const success = await api.users.updateStatus(currentUser, status);
      if (success) {
        onStatusChange(status);
        
       
        if (onChangeStatusViaWS) {
          onChangeStatusViaWS(status);
        }
      }
    } catch (error) {
      console.error('Ошибка обновления статуса:', error);
    }
    setShowStatusMenu(false);
  };


  const getStatusIcon = (status: string) => (
    <span className={`presence-dot ${['online', 'away', 'offline'].includes(status) ? status : 'offline'}`} />
  );

  if (!showUserPanel) {
    return (
      <button 
        className="show-panel-btn"
        onClick={onTogglePanel}
        aria-label="Show panel"
      >
        <ChevronsRight size={18} />
      </button>
    );
  }

  return (
    <div className="left-panel">
      <div className="panel-header">
        <div className="workspace-info">
          <img className="workspace-logo" src={logoUrl} alt="GoThermo" />
          <div>
            <h3>GoThermo</h3>
            <div className="workspace-status">
              <span className="status-dot online"></span>
              <span>Active workspace</span>
            </div>
          </div>
        </div>
        <button 
          className="panel-toggle"
          onClick={onTogglePanel}
          title="Hide panel"
          aria-label="Hide panel"
        >
          <ChevronsLeft size={18} />
        </button>
      </div>

      <div className="current-user-card" ref={statusMenuRef}>
        <button
          type="button"
          className="avatar-upload"
          title="Change picture"
          aria-label="Change picture"
          onClick={() => fileRef.current?.click()}
        >
          <Avatar username={currentUser} className="user-avatar large" />
        </button>
        <input
          ref={fileRef}
          type="file"
          accept="image/*"
          hidden
          data-testid="avatar-input"
          onChange={e => {
            void pickAvatar(e.target.files?.[0]);
            e.target.value = '';
          }}
        />
        <div className="user-details">
          <div className="user-name">{currentUser}</div>
          <div 
            className="user-status" 
            onClick={() => setShowStatusMenu(!showStatusMenu)}
            style={{ cursor: 'pointer' }}
          >
            <span className={`status-dot ${currentUserStatus}`}></span>
            <span>{STATUS_OPTIONS.find(s => s.value === currentUserStatus)?.label}</span>
            <ChevronDown size={14} className="status-arrow" />
          </div>
        </div>
        
        {showStatusMenu && (
          <div className="status-menu">
            {myAvatar && (
              <div className="status-option" onClick={() => { setShowStatusMenu(false); void dropAvatar(); }}>
                <span>Remove picture</span>
              </div>
            )}
            {STATUS_OPTIONS.map(status => (
              <div 
                key={status.value}
                className={`status-option ${currentUserStatus === status.value ? 'active' : ''}`}
                onClick={() => handleStatusChange(status.value as StatusType)}
              >
                <span className="status-dot" style={{ backgroundColor: status.color }}></span>
                <span>{status.label}</span>
                {currentUserStatus === status.value && <Check size={14} style={{ marginLeft: '8px' }} />}
              </div>
            ))}
          </div>
        )}
      </div>

      {onlineUsers.length > 0 && (
        <div className="panel-section">
          <div className="section-header">
            <span className="section-title">Online</span>
            <span className="section-count">{onlineUsers.length}</span>
          </div>
          <div className="users-list">
            {onlineUsers.map(user => (
              <div 
                key={user.id || user.username} 
                className="user-item"
                onClick={() => onStartDirectMessage(user.username)}
              >
                <Avatar username={user.username} className="user-avatar small" />
                <div className="user-name">{user.username}</div>
                <span className="status-icon">{getStatusIcon(user.status || 'online')}</span>
              </div>
            ))}
          </div>
        </div>
      )}

      {awayUsers.length > 0 && (
        <div className="panel-section">
          <div className="section-header">
            <span className="section-title">Away</span>
            <span className="section-count">{awayUsers.length}</span>
          </div>
          <div className="users-list">
            {awayUsers.map(user => (
              <div 
                key={user.id || user.username} 
                className="user-item"
                onClick={() => onStartDirectMessage(user.username)}
              >
                <Avatar username={user.username} className="user-avatar small" />
                <div className="user-name">{user.username}</div>
                <span className="status-icon">{getStatusIcon(user.status || 'away')}</span>
              </div>
            ))}
          </div>
        </div>
      )}

      {offlineUsers.length > 0 && (
        <div className="panel-section">
          <div className="section-header">
            <span className="section-title">Offline</span>
            <span className="section-count">{offlineUsers.length}</span>
          </div>
          <div className="users-list">
            {offlineUsers.map(user => (
              <div 
                key={user.id || user.username} 
                className="user-item offline"
                onClick={() => onStartDirectMessage(user.username)}
              >
                <Avatar username={user.username} className="user-avatar small" />
                <div className="user-name">{user.username}</div>
                <span className="status-icon">{getStatusIcon(user.status || 'offline')}</span>
              </div>
            ))}
          </div>
        </div>
      )}

      <div className="panel-footer">
        <button className="footer-btn" onClick={onStartVideoCall} title="Video call">
          <Video size={16} className="icon" />
          <span>Video</span>
        </button>
        <button className="footer-btn" onClick={onStartAudioCall} title="Audio call">
          <Phone size={16} className="icon" />
          <span>Audio</span>
        </button>
        {onOpenThemes && (
          <button className="footer-btn" title="Appearance" aria-label="Appearance" onClick={onOpenThemes}>
            <Palette size={16} className="icon" />
          </button>
        )}
        <button className="footer-btn" title="Settings" aria-label="Settings" onClick={onOpenSettings}>
          <Settings size={16} className="icon" />
        </button>
      </div>
    </div>
  );
};