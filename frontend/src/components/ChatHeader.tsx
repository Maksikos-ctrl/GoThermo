import React from 'react';
import { Channel } from '../types';

interface ChatHeaderProps {
  currentChannel: string;
  channels: Channel[];
  messagesCount: number;
  onStartVideoCall: () => void;
  onStartAudioCall: () => void;
  onShowMembers?: () => void;
}

export const ChatHeader: React.FC<ChatHeaderProps> = ({
  currentChannel,
  channels,
  messagesCount,
  onStartVideoCall,
  onStartAudioCall,
  onShowMembers,
}) => {
  const channel = channels.find(ch => ch.name === currentChannel);

  return (
    <div className="chat-header">
      <div className="channel-info">
        <div className="channel-title">
          <h2># {currentChannel}</h2>
          {channel?.description && (
            <span className="channel-description-badge">{channel.description}</span>
          )}
        </div>
        <div className="channel-stats">
          <span className="messages-count">{messagesCount} messages</span>
        </div>
      </div>
      
      <div className="header-actions">
        <button 
          className="members-btn" 
          onClick={onShowMembers}
          title="Show channel members"
        >
          👥
        </button>
        <button 
          className="call-btn audio-call-btn" 
          onClick={onStartAudioCall} 
          title="Voice call"
        >
          📞
        </button>
        <button 
          className="call-btn video-call-btn" 
          onClick={onStartVideoCall} 
          title="Video call"
        >
          📹
        </button>
      </div>
    </div>
  );
};