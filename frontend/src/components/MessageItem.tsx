import React, { useState } from 'react';
import { Message, EMOJI_LIST } from '../types';

interface MessageItemProps {
  message: Message;
  currentUser: string;
  onAddReaction: (messageId: string, emoji: string) => void;
}

export const MessageItem: React.FC<MessageItemProps> = ({
  message,
  currentUser,
  onAddReaction,
}) => {
  const [isHovered, setIsHovered] = useState(false);

  const handleMouseEnter = () => {
    setTimeout(() => setIsHovered(true), 50);
  };

  const handleMouseLeave = () => {
    setTimeout(() => setIsHovered(false), 100);
  };

  const hasUserReacted = (emoji: string) => {
    return message.reactions?.[emoji]?.includes(currentUser) || false;
  };

  const formatTime = (timestamp: string) => {
    return new Date(timestamp).toLocaleTimeString([], { 
      hour: '2-digit', 
      minute: '2-digit' 
    });
  };

  return (
    <div 
      className={`message ${message.isPost ? 'post-message' : ''} ${message.user === currentUser ? 'my-message' : ''}`}
      onMouseEnter={handleMouseEnter}
      onMouseLeave={handleMouseLeave}
    >
      <div className="message-header">
        <span className="message-user">{message.user}</span>
        <span className="message-time">{formatTime(message.timestamp)}</span>
        {message.isPost && <span className="post-badge">📌 Post</span>}
      </div>
      <div className="message-text">{message.text}</div>
      
      <div className="message-footer">
        {message.reactions && Object.entries(message.reactions).some(([_, users]) => users.length > 0) && (
          <div className="reactions">
            {Object.entries(message.reactions).map(([emoji, users]) => (
              users.length > 0 && (
                <button
                  key={emoji}
                  className={`reaction ${hasUserReacted(emoji) ? 'reacted' : ''}`}
                  onClick={() => onAddReaction(message.id, emoji)}
                  title={users.join(', ')}
                >
                  {emoji} <span className="reaction-count">{users.length}</span>
                </button>
              )
            ))}
          </div>
        )}

        {isHovered && (
          <div className="quick-reactions">
            {EMOJI_LIST.slice(0, 5).map(emoji => (
              <button
                key={emoji}
                className="quick-reaction-btn"
                onClick={() => onAddReaction(message.id, emoji)}
                title={`React with ${emoji}`}
              >
                {emoji}
              </button>
            ))}
          </div>
        )}
      </div>
    </div>
  );
};