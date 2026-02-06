import React, { useRef, useState } from 'react';
import { User, STATUS_OPTIONS, StatusType } from '../types';
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
}) => {
  const [showStatusMenu, setShowStatusMenu] = useState(false);
  const statusMenuRef = useRef<HTMLDivElement>(null);

  const otherUsers = users.filter(user => user.username !== currentUser);
  const onlineUsers = otherUsers.filter(user => user.status === 'online');
  const awayUsers = otherUsers.filter(user => user.status === 'away');
  const offlineUsers = otherUsers.filter(user => user.status === 'offline');

  const handleStatusChange = async (status: StatusType) => {
    try {
      const success = await api.users.updateStatus(currentUser, status);
      if (success) {
        onStatusChange(status);
      }
    } catch (error) {
      console.error('Ошибка обновления статуса:', error);
    }
    setShowStatusMenu(false);
  };

  if (!showUserPanel) {
    return (
      <button 
        className="show-panel-btn"
        onClick={onTogglePanel}
      >
        →
      </button>
    );
  }

  return (
    <div className="left-panel">
      <div className="panel-header">
        <div className="workspace-info">
          <div className="workspace-logo">GT</div>
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
        >
          ←
        </button>
      </div>

      <div className="current-user-card" ref={statusMenuRef}>
        <div className="user-avatar large">{currentUser.charAt(0).toUpperCase()}</div>
        <div className="user-details">
          <div className="user-name">{currentUser}</div>
          <div className="user-status" onClick={() => setShowStatusMenu(!showStatusMenu)}>
            <span className={`status-dot ${currentUserStatus}`}></span>
            <span>{STATUS_OPTIONS.find(s => s.value === currentUserStatus)?.label}</span>
            <span className="status-arrow">▼</span>
          </div>
        </div>
        
        {showStatusMenu && (
          <div className="status-menu">
            {STATUS_OPTIONS.map(status => (
              <div 
                key={status.value}
                className={`status-option ${currentUserStatus === status.value ? 'active' : ''}`}
                onClick={() => handleStatusChange(status.value as StatusType)}
              >
                <span className="status-dot" style={{ backgroundColor: status.color }}></span>
                <span>{status.label}</span>
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
                key={user.id} 
                className="user-item"
                onClick={() => onStartDirectMessage(user.username)}
              >
                <div className="user-avatar small">{user.username.charAt(0).toUpperCase()}</div>
                <div className="user-name">{user.username}</div>
                <span className={`status-dot ${user.status}`}></span>
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
                key={user.id} 
                className="user-item"
                onClick={() => onStartDirectMessage(user.username)}
              >
                <div className="user-avatar small">{user.username.charAt(0).toUpperCase()}</div>
                <div className="user-name">{user.username}</div>
                <span className={`status-dot ${user.status}`}></span>
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
                key={user.id} 
                className="user-item offline"
                onClick={() => onStartDirectMessage(user.username)}
              >
                <div className="user-avatar small">{user.username.charAt(0).toUpperCase()}</div>
                <div className="user-name">{user.username}</div>
                <span className={`status-dot ${user.status}`}></span>
              </div>
            ))}
          </div>
        </div>
      )}

      <div className="panel-footer">
        <button className="footer-btn" onClick={onStartVideoCall} title="Video call">
          <span className="icon">📹</span>
          <span>Video</span>
        </button>
        <button className="footer-btn" onClick={onStartAudioCall} title="Audio call">
          <span className="icon">📞</span>
          <span>Audio</span>
        </button>
        <button className="footer-btn" title="Settings">
          <span className="icon">⚙️</span>
        </button>
      </div>
    </div>
  );
};