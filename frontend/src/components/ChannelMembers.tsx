import React, { useState, useEffect } from 'react';
import { User, Channel } from '../types';

interface ChannelMembersProps {
  channel: Channel | null;
  users: User[];
  currentChannel: string;
  onClose: () => void;
}

export const ChannelMembers: React.FC<ChannelMembersProps> = ({
  channel,
  users,
  currentChannel,
  onClose
}) => {
  const [members, setMembers] = useState<User[]>([]);

  // Обновляем список участников при изменении users
  useEffect(() => {
    // Сортируем: сначала онлайн, потом по алфавиту
    const sorted = [...users].sort((a, b) => {
      if (a.status === 'online' && b.status !== 'online') return -1;
      if (a.status !== 'online' && b.status === 'online') return 1;
      return a.username.localeCompare(b.username);
    });
    setMembers(sorted);
  }, [users]);

  const getStatusIcon = (status: string = 'offline') => {
    switch(status) {
      case 'online': return '🟢';
      case 'away': return '🟡';
      case 'offline': return '⚪';
      default: return '⚪';
    }
  };

  const getStatusClass = (status: string = 'offline') => {
    switch(status) {
      case 'online': return 'online';
      case 'away': return 'away';
      case 'offline': return 'offline';
      default: return 'offline';
    }
  };

  const onlineCount = members.filter(m => m.status === 'online').length;
  const totalCount = members.length;

  if (!channel) return null;

  return (
    <div className="channel-members-panel">
      <div className="channel-members-header">
        <div className="channel-info">
          <h3># {currentChannel}</h3>
          {channel.description && (
            <span className="channel-description">{channel.description}</span>
          )}
        </div>
        <div className="members-stats">
          <span className="online-count">🟢 {onlineCount}</span>
          <span className="total-count">👥 {totalCount}</span>
        </div>
        <button className="close-btn" onClick={onClose}>×</button>
      </div>
      
      <div className="members-list">
        {members.length === 0 ? (
          <div className="no-members">No members yet</div>
        ) : (
          <>
            {/* Онлайн участники */}
            {members.filter(m => m.status === 'online').length > 0 && (
              <div className="members-section">
                <div className="section-label">ONLINE — {members.filter(m => m.status === 'online').length}</div>
                {members.filter(m => m.status === 'online').map(member => (
                  <div key={member.username} className="member-item">
                    <div className="member-avatar" style={{ background: getAvatarColor(member.username) }}>
                      {member.username.charAt(0).toUpperCase()}
                    </div>
                    <div className="member-info">
                      <span className="member-name">{member.username}</span>
                      <span className={`member-status ${getStatusClass(member.status)}`}>
                        {getStatusIcon(member.status)}
                        <span className="status-text">{member.status}</span>
                      </span>
                    </div>
                  </div>
                ))}
              </div>
            )}

            {/* Офлайн участники */}
            {members.filter(m => m.status === 'offline').length > 0 && (
              <div className="members-section">
                <div className="section-label">OFFLINE — {members.filter(m => m.status === 'offline').length}</div>
                {members.filter(m => m.status === 'offline').map(member => (
                  <div key={member.username} className="member-item offline">
                    <div className="member-avatar" style={{ background: getAvatarColor(member.username) }}>
                      {member.username.charAt(0).toUpperCase()}
                    </div>
                    <div className="member-info">
                      <span className="member-name">{member.username}</span>
                      <span className="member-status offline">
                        {getStatusIcon(member.status)}
                        <span className="status-text">offline</span>
                      </span>
                    </div>
                  </div>
                ))}
              </div>
            )}
          </>
        )}
      </div>

      <div className="channel-members-footer">
        <div className="channel-created">
          Created by {channel.createdBy}
          <span className="created-date">
            {new Date(channel.createdAt).toLocaleDateString()}
          </span>
        </div>
      </div>
    </div>
  );
};

// Функция для генерации цвета аватара на основе имени
function getAvatarColor(username: string): string {
  const colors = [
    '#5865f2', '#ed4245', '#57f287', '#fee75c', 
    '#eb459e', '#f47b67', '#839bfe', '#b473b5'
  ];
  let hash = 0;
  for (let i = 0; i < username.length; i++) {
    hash = username.charCodeAt(i) + ((hash << 5) - hash);
  }
  return colors[Math.abs(hash) % colors.length];
}