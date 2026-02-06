import React from 'react';
import { Channel } from '../types';

interface ChatHeaderProps {
  currentChannel: string;
  channels: Channel[];
  messagesCount: number;
  onStartVideoCall: () => void;
  onStartAudioCall: () => void;
}

export const ChatHeader: React.FC<ChatHeaderProps> = ({
  currentChannel,
  channels,
  messagesCount,
  onStartVideoCall,
  onStartAudioCall,
}) => {
  const currentChannelData = channels.find(ch => ch.name === currentChannel);
  const membersCount = currentChannelData?.members.length || 0;

  return (
    <div className="chat-header">
      <div className="channel-header">
        <div className="channel-title">
          <span className="channel-hashtag">#</span>
          <h3>{currentChannel}</h3>
        </div>
        <div className="channel-meta">
          <span className="meta-item">
            {membersCount} members
          </span>
          <span className="meta-divider">•</span>
          <span className="meta-item">
            {messagesCount} {messagesCount === 1 ? 'message' : 'messages'}
          </span>
        </div>
      </div>

      <div className="header-actions">
        <button className="action-btn" onClick={onStartVideoCall} title="Video call">
          <span className="icon">📹</span>
        </button>
        <button className="action-btn" onClick={onStartAudioCall} title="Audio call">
          <span className="icon">📞</span>
        </button>
        <button className="action-btn" title="Add people">
          <span className="icon">👥</span>
        </button>
      </div>
    </div>
  );
};