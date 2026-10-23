import React from 'react';
import { Plus, Lock, Hash, AtSign, X, GripVertical } from 'lucide-react';
import { Channel } from '../types';

interface ChannelSidebarProps {
  channels: Channel[];
  dmChannels: Channel[];
  currentChannel: string;
  isLoading: boolean;
  currentUser: string;
  unreadCounts: Record<string, number>; 
  onChannelChange: (channel: string) => void;
  onCreateChannel: () => void;
  onDeleteChannel: (channelName: string) => void;
  onDeleteDM: (channelName: string) => void; 
  isDragging: string | null;
  dragOver: string | null;
  onDragStart: (e: React.DragEvent, channelId: string) => void;
  onDragOver: (e: React.DragEvent, channelId: string) => void;
  onDragLeave: () => void;
  onDrop: (e: React.DragEvent, dropTargetId: string) => void;
}

export const ChannelSidebar: React.FC<ChannelSidebarProps> = ({
  channels,
  dmChannels,
  currentChannel,
  isLoading,
  currentUser,
  unreadCounts,
  onChannelChange,
  onCreateChannel,
  onDeleteChannel,
  onDeleteDM,
  isDragging,
  dragOver,
  onDragStart,
  onDragOver,
  onDragLeave,
  onDrop,
}) => {
  const getDMPartnerName = (dmChannelName: string): string => {
    const names = dmChannelName.replace('dm_', '').split('_');
    return names.find(n => n !== currentUser) || dmChannelName;
  };

  const renderUnreadBadge = (channelName: string) => {
    const count = unreadCounts[channelName] || 0;
    if (count === 0) return null;
    return (
      <span className="unread-badge">
        {count > 99 ? '99+' : count}
      </span>
    );
  };

  return (
    <div className="sidebar">
      <div className="sidebar-header">
        <h2>Channels</h2>
        <button 
          className="create-channel-btn"
          onClick={onCreateChannel}
          title="Create channel"
          aria-label="Create channel"
        >
          <Plus size={14} strokeWidth={2.5} />
        </button>
      </div>

      <div className="channel-list">
        {isLoading ? (
          <div className="loading-channels">Loading channels...</div>
        ) : channels.length === 0 ? (
          <div className="no-channels">No channels yet</div>
        ) : (
          channels.map((channel) => (
            <div
              key={channel.id}
              className={`channel ${currentChannel === channel.name ? 'active' : ''} ${
                (unreadCounts[channel.name] || 0) > 0 ? 'has-unread' : ''
              } ${
                isDragging === channel.id ? 'dragging' : ''
              } ${dragOver === channel.id ? 'drag-over' : ''}`}
              onClick={() => onChannelChange(channel.name)}
              title={channel.description || channel.name}
              draggable
              onDragStart={(e) => onDragStart(e, channel.id)}
              onDragOver={(e) => onDragOver(e, channel.id)}
              onDragLeave={onDragLeave}
              onDrop={(e) => onDrop(e, channel.id)}
            >
              <div className="channel-content">
                <span className="drag-handle"><GripVertical size={14} /></span>
                <span className="channel-hashtag">
                  {channel.isPrivate ? <Lock size={15} /> : <Hash size={15} />}
                </span>
                <span className="channel-name">{channel.name}</span>
              </div>

             
              {renderUnreadBadge(channel.name)}
              
              {channel.createdBy === currentUser && channel.createdBy !== 'system' && (
                <button 
                  className="delete-channel-btn"
                  onClick={(e) => {
                    e.stopPropagation();
                    onDeleteChannel(channel.name);
                  }}
                  title="Delete channel"
                  aria-label="Delete channel"
                >
                  <X size={14} />
                </button>
              )}
            </div>
          ))
        )}
      </div>

      {dmChannels.length > 0 && (
        <>
          <div className="sidebar-header" style={{ marginTop: '16px' }}>
            <h2>Direct Messages</h2>
          </div>
          <div className="channel-list">
            {dmChannels.map((channel) => (
              <div
                key={channel.id}
                className={`channel ${currentChannel === channel.name ? 'active' : ''} ${
                  (unreadCounts[channel.name] || 0) > 0 ? 'has-unread' : ''
                }`}
                onClick={() => onChannelChange(channel.name)}
                title={getDMPartnerName(channel.name)}
              >
                <div className="channel-content">
                  <span className="channel-hashtag"><AtSign size={15} /></span>
                  <span className="channel-name">{getDMPartnerName(channel.name)}</span>
                </div>

             
                {renderUnreadBadge(channel.name)}

                
                <button
                  className="delete-channel-btn"
                  onClick={(e) => {
                    e.stopPropagation();
                    onDeleteDM(channel.name);
                  }}
                  title="Delete chat"
                  aria-label="Delete chat"
                >
                  <X size={14} />
                </button>
              </div>
            ))}
          </div>
        </>
      )}
    </div>
  );
};