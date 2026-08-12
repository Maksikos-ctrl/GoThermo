import React, { useState, useRef, useEffect } from 'react';
import { Message, EMOJI_LIST } from '../types';

interface MessageItemProps {
  message: Message;
  currentUser: string;
  onAddReaction: (messageId: string, emoji: string) => void;
  onEditMessage: (messageId: string, newText: string) => void;
  onDeleteMessage: (messageId: string) => void;
  knownUsernames?: string[]; 
}

export const MessageItem: React.FC<MessageItemProps> = ({
  message,
  currentUser,
  onAddReaction,
  onEditMessage,
  onDeleteMessage,
  knownUsernames = [], 
}) => {
  const [isHovered, setIsHovered] = useState(false);
  const [isEditing, setIsEditing] = useState(false); 
  const [editText, setEditText] = useState(message.text); 
  const editInputRef = useRef<HTMLTextAreaElement>(null);

  const isOwnMessage = message.user === currentUser; 

  
  const mentionsCurrentUser = new RegExp(`(?:^|\\s)@${escapeRegex(currentUser)}(?:\\s|$|[.,!?])`).test(message.text);

  useEffect(() => {
    if (isEditing) {
      editInputRef.current?.focus();
      editInputRef.current?.setSelectionRange(editText.length, editText.length);
    }
  }, [isEditing]);

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


  const saveEdit = () => {
    const trimmed = editText.trim();
    if (trimmed && trimmed !== message.text) {
      onEditMessage(message.id, trimmed);
    }
    setIsEditing(false);
  };

  
  const cancelEdit = () => {
    setEditText(message.text);
    setIsEditing(false);
  };

 
  const handleDeleteClick = () => {
    onDeleteMessage(message.id);
  };

  return (
    <div 
      className={`message ${message.isPost ? 'post-message' : ''} ${isOwnMessage ? 'my-message' : ''} ${mentionsCurrentUser ? 'mentions-me' : ''}`}
      onMouseEnter={handleMouseEnter}
      onMouseLeave={handleMouseLeave}
    >
      <div className="message-header">
        <span className="message-user">{message.user}</span>
        <span className="message-time">{formatTime(message.timestamp)}</span>
        {message.isPost && <span className="post-badge">📌 Post</span>}
        {message.isEdited && <span className="edited-badge" style={{ fontSize: '11px', color: '#8a8f98', marginLeft: '6px' }}>(edited)</span>}
      </div>

      {isEditing ? (
      
        <div className="edit-message-form">
          <textarea
            ref={editInputRef}
            className="edit-message-textarea"
            value={editText}
            onChange={(e) => setEditText(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === 'Enter' && !e.shiftKey) {
                e.preventDefault();
                saveEdit();
              } else if (e.key === 'Escape') {
                cancelEdit();
              }
            }}
            rows={2}
            style={{ width: '100%', resize: 'vertical' }}
          />
          <div className="edit-message-actions" style={{ marginTop: '4px', display: 'flex', gap: '8px' }}>
            <button onClick={saveEdit} className="edit-save-btn">Save</button>
            <button onClick={cancelEdit} className="edit-cancel-btn">Cancel</button>
            <span style={{ fontSize: '11px', color: '#8a8f98', alignSelf: 'center' }}>
              Enter to save · Esc to cancel
            </span>
          </div>
        </div>
      ) : (
        
        <div className="message-text">{renderTextWithMentions(message.text, knownUsernames, currentUser)}</div>
      )}
      
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

        {isHovered && !isEditing && (
          <div className="message-hover-actions" style={{ display: 'flex', gap: '4px', alignItems: 'center' }}>
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

            
            {isOwnMessage && (
              <>
                <button
                  className="quick-reaction-btn"
                  onClick={() => setIsEditing(true)}
                  title="Edit message"
                >
                  ✏️
                </button>
                <button
                  className="quick-reaction-btn"
                  onClick={handleDeleteClick}
                  title="Delete message"
                >
                  🗑️
                </button>
              </>
            )}
          </div>
        )}
      </div>
    </div>
  );
};


function escapeRegex(str: string): string {
  return str.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
}


function renderTextWithMentions(text: string, knownUsernames: string[], currentUser: string): React.ReactNode {
  if (knownUsernames.length === 0) return text;

  const knownSet = new Set(knownUsernames.map(u => u.toLowerCase()));
  const parts: React.ReactNode[] = [];
  const regex = /@([a-zA-Z0-9_]+)/g;

  let lastIndex = 0;
  let match: RegExpExecArray | null;
  let key = 0;

  while ((match = regex.exec(text)) !== null) {
    const username = match[1];
    const isKnown = knownSet.has(username.toLowerCase());

    if (!isKnown) continue; 

    if (match.index > lastIndex) {
      parts.push(text.substring(lastIndex, match.index));
    }

    const isSelf = username.toLowerCase() === currentUser.toLowerCase();

    parts.push(
      <span
        key={`mention-${key++}`}
        style={{
          background: isSelf ? '#5865f2' : 'rgba(88, 101, 242, 0.15)',
          color: isSelf ? 'white' : '#a9b3ff',
          borderRadius: '4px',
          padding: '0 4px',
          fontWeight: 600,
        }}
      >
        @{username}
      </span>
    );

    lastIndex = match.index + match[0].length;
  }

  if (lastIndex < text.length) {
    parts.push(text.substring(lastIndex));
  }

  return parts.length > 0 ? parts : text;
}