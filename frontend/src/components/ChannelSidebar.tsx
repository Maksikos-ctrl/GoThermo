import React, { useState } from 'react';
import { Channel } from '../types';

interface ChannelSidebarProps {
  channels: Channel[];
  currentChannel: string;
  isLoading: boolean;
  currentUser: string;
  onChannelChange: (channel: string) => void;
  onCreateChannel: () => void;
  onDeleteChannel: (channelName: string) => void;
  isDragging: string | null;
  dragOver: string | null;
  onDragStart: (e: React.DragEvent, channelId: string) => void;
  onDragOver: (e: React.DragEvent, channelId: string) => void;
  onDragLeave: () => void;
  onDrop: (e: React.DragEvent, dropTargetId: string) => void;
}

export const ChannelSidebar: React.FC<ChannelSidebarProps> = ({
  channels,
  currentChannel,
  isLoading,
  currentUser,
  onChannelChange,
  onCreateChannel,
  onDeleteChannel,
  isDragging,
  dragOver,
  onDragStart,
  onDragOver,
  onDragLeave,
  onDrop,
}) => {
  return (
    <div className="sidebar">
      <div className="sidebar-header">
        <h2>Channels</h2>
        <button 
          className="create-channel-btn"
          onClick={onCreateChannel}
          title="Create channel"
        >
          +
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
                <span className="drag-handle">⋮⋮</span>
                <span className="channel-hashtag">#</span>
                <span className="channel-name">{channel.name}</span>
              </div>
              
              {channel.createdBy === currentUser && channel.createdBy !== 'system' && (
                <button 
                  className="delete-channel-btn"
                  onClick={(e) => {
                    e.stopPropagation();
                    onDeleteChannel(channel.name);
                  }}
                  title="Delete channel"
                >
                  ×
                </button>
              )}
            </div>
          ))
        )}
      </div>
    </div>
  );
};