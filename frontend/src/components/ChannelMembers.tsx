import React, { useState, useEffect } from 'react';
import { User, Channel } from '../types';

interface ChannelMembersProps {
  channel: Channel | null;
  users: User[];
  currentChannel: string;
  onClose: () => void;
  // Private channels only: who is looking, everyone who exists, and how to invite.
  currentUser?: string;
  allUsers?: User[];
  onAddMember?: (username: string) => void;
  onLeave?: () => void;
}

export const ChannelMembers: React.FC<ChannelMembersProps> = ({
  channel,
  users,
  currentChannel,
  onClose,
  currentUser,
  allUsers = [],
  onAddMember,
  onLeave,
}) => {
  const [members, setMembers] = useState<User[]>([]);
  const [inviteName, setInviteName] = useState('');

  const isPrivateChannel = !!channel?.isPrivate && !channel.name.startsWith('dm_');
  const memberNames = channel?.members ?? [];

  
  useEffect(() => {
    
    // In a private channel the list is its real member list, not message authors.
    const source: User[] = isPrivateChannel
      ? memberNames.map(
          name =>
            allUsers.find(u => u.username === name) ||
            ({ id: name, username: name, email: '', isOnline: false, status: 'offline' } as User)
        )
      : users;
    const sorted = [...source].sort((a, b) => {
      if (a.status === 'online' && b.status !== 'online') return -1;
      if (a.status !== 'online' && b.status === 'online') return 1;
      return a.username.localeCompare(b.username);
    });
    setMembers(sorted);
  }, [users, allUsers, channel, isPrivateChannel]);

  const candidates = allUsers.filter(u => !memberNames.includes(u.username));
  const canInvite =
    isPrivateChannel && !!onAddMember && !!currentUser && memberNames.includes(currentUser);

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
          <h3>{isPrivateChannel ? '🔒' : '#'} {currentChannel}</h3>
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

      {canInvite && (
        <div style={{ padding: '10px 12px', borderTop: '1px solid #3f4147' }}>
          <div className="section-label">ADD MEMBER</div>
          {candidates.length === 0 ? (
            <div className="no-members">Everyone is already here</div>
          ) : (
            <div style={{ display: 'flex', gap: '6px', marginTop: '6px' }}>
              <select
                value={inviteName}
                onChange={e => setInviteName(e.target.value)}
                className="modal-input"
                style={{ flex: 1, minWidth: 0 }}
              >
                <option value="">Choose a user…</option>
                {candidates.map(u => (
                  <option key={u.username} value={u.username}>
                    {u.username}
                  </option>
                ))}
              </select>
              <button
                className="modal-create-btn"
                disabled={!inviteName}
                onClick={() => {
                  onAddMember!(inviteName);
                  setInviteName('');
                }}
              >
                Add
              </button>
            </div>
          )}
        </div>
      )}

      {isPrivateChannel && !!onLeave && !!currentUser && memberNames.includes(currentUser) && (
        <div style={{ padding: '10px 12px', borderTop: '1px solid #3f4147' }}>
          <button
            onClick={onLeave}
            style={{
              width: '100%',
              padding: '8px 12px',
              background: 'transparent',
              color: '#da373c',
              border: '1px solid #da373c',
              borderRadius: '6px',
              cursor: 'pointer',
              fontWeight: 600,
            }}
          >
            Leave channel
          </button>
        </div>
      )}

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