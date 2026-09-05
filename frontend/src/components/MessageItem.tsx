import React, { useState, useRef, useEffect } from 'react';
import { Message, EMOJI_LIST } from '../types';
import { api } from '../services/api'; 

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
      {!message.isCall && (
        <div className="message-header">
          <span className="message-user">{message.user}</span>
          <span className="message-time">{formatTime(message.timestamp)}</span>
          {message.isPost && <span className="post-badge">📌 Post</span>}
          {message.isEdited && <span className="edited-badge" style={{ fontSize: '11px', color: '#8a8f98', marginLeft: '6px' }}>(edited)</span>}
        </div>
      )}

      {message.isCall ? (
        <CallLogBubble message={message} isOwnMessage={isOwnMessage} />
      ) : isEditing ? (
      
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
      ) : message.isFile ? (
       
        <FileAttachment message={message} />
      ) : (
        <div className="message-text">{renderTextWithMentions(message.text, knownUsernames, currentUser)}</div>
      )}
      
      {!message.isCall && (
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
      )}
    </div>
  );
};

function escapeRegex(str: string): string {
  return str.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
}

function formatCallDuration(totalSeconds?: number): string {
  const s = Math.max(0, totalSeconds || 0);
  const mins = Math.floor(s / 60);
  const secs = s % 60;
  return `${mins}:${secs.toString().padStart(2, '0')}`;
}

function CallLogBubble({ message, isOwnMessage }: { message: Message; isOwnMessage: boolean }) {
  const status = message.callStatus;
  const isDeclinedOrCancelled = status === 'declined' || status === 'cancelled';

  let label: string;
  if (status === 'completed') {
    label = `Audio call · ${formatCallDuration(message.callDuration)}`;
  } else if (status === 'declined') {
    label = isOwnMessage ? 'Call declined' : 'You declined the call';
  } else if (status === 'cancelled') {
    label = isOwnMessage ? 'Call cancelled' : 'Missed call';
  } else {
    label = 'Call ended';
  }

  return (
    <div
      className="call-log-bubble"
      style={{
        display: 'inline-flex',
        alignItems: 'center',
        gap: '10px',
        background: '#2b2d31',
        border: `1px solid ${isDeclinedOrCancelled ? 'rgba(218,55,60,0.35)' : '#3f4147'}`,
        borderRadius: '10px',
        padding: '10px 16px',
        marginTop: '2px',
      }}
    >
      <span
        style={{
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          width: '30px',
          height: '30px',
          borderRadius: '50%',
          background: isDeclinedOrCancelled ? '#da373c' : '#23a55a',
          fontSize: '15px',
          flexShrink: 0,
        }}
      >
        {isDeclinedOrCancelled ? '✕' : '📞'}
      </span>
      <span
        style={{
          color: isDeclinedOrCancelled ? '#da373c' : '#f2f3f5',
          fontSize: '14px',
          fontWeight: 600,
        }}
      >
        {label}
      </span>
      <span style={{ color: '#8a8f98', fontSize: '12px' }}>
        {new Date(message.timestamp).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}
      </span>
    </div>
  );
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

    if (!isKnown) continue; // не подсвечиваем "@" перед случайным словом

    if (match.index > lastIndex) {
      parts.push(text.substring(lastIndex, match.index));
    }

    const isSelf = username.toLowerCase() === currentUser.toLowerCase();

    parts.push(
      <span
        key={`mention-${key++}`}
        style={{
          background: isSelf ? '#5865f2' : 'rgba(255, 255, 255, 0.18)',
          color: isSelf ? 'white' : '#ffffff',
          borderRadius: '4px',
          padding: '1px 5px',
          fontWeight: 700,
          boxShadow: isSelf ? 'none' : 'inset 0 0 0 1px rgba(255,255,255,0.25)',
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


function FileAttachment({ message }: { message: Message }) {
  const [dataUrl, setDataUrl] = useState<string | null>(null);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState(false);

  const isImage = (message.mimeType || '').startsWith('image/');

  const formatSize = (bytes?: number) => {
    if (!bytes) return '';
    if (bytes < 1024) return `${bytes} B`;
    if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
    return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
  };

  
  useEffect(() => {
    if (isImage && message.fileId && !dataUrl) {
      setIsLoading(true);
      api.files.getData(message.fileId)
        .then((url: string) => setDataUrl(url))
        .catch(() => setError(true))
        .finally(() => setIsLoading(false));
    }
  }, [isImage, message.fileId]);

  const handleDownload = async () => {
    if (!message.fileId) return;

    
    const isNativeWails =
      !!(window as any).chrome?.webview || !!(window as any).webkit?.messageHandlers;

    if (isNativeWails) {
      try {
        await api.files.saveToDisk(message.fileId);
        return;
      } catch (nativeErr) {
        console.warn('Native save failed, falling back to blob download:', nativeErr);
       
      }
    }

    try {
      let url = dataUrl;
      if (!url) {
        setIsLoading(true);
        url = await api.files.getData(message.fileId);
        setDataUrl(url);
        setIsLoading(false);
      }

     
      const response = await fetch(url as string);
      const blob = await response.blob();
      const blobUrl = URL.createObjectURL(blob);

      const link = document.createElement('a');
      link.href = blobUrl;
      link.download = message.fileName || 'file';
      document.body.appendChild(link);
      link.click();
      document.body.removeChild(link);

      setTimeout(() => URL.revokeObjectURL(blobUrl), 5000);
    } catch (e) {
      console.error('Download failed:', e);
      setError(true);
      setIsLoading(false);
    }
  };

  if (isImage) {
    return (
      <div className="message-file-attachment">
        {isLoading && !dataUrl && (
          <div style={{ color: '#8a8f98', fontSize: '13px', padding: '8px 0' }}>Loading image...</div>
        )}
        {error && (
          <div style={{ color: '#da373c', fontSize: '13px' }}>Failed to load image</div>
        )}
        {dataUrl && (
          <img
            src={dataUrl}
            alt={message.fileName}
            onClick={handleDownload}
            style={{
              maxWidth: '320px',
              maxHeight: '320px',
              borderRadius: '8px',
              cursor: 'pointer',
              display: 'block',
            }}
            title={`${message.fileName} (${formatSize(message.fileSize)}) — click to download`}
          />
        )}
      </div>
    );
  }


  return (
    <div
      onClick={handleDownload}
      style={{
        display: 'inline-flex',
        alignItems: 'center',
        gap: '10px',
        background: '#2b2d31',
        border: '1px solid #3f4147',
        borderRadius: '8px',
        padding: '10px 14px',
        cursor: 'pointer',
        maxWidth: '320px',
      }}
    >
      <span style={{ fontSize: '22px' }}>📄</span>
      <div style={{ overflow: 'hidden' }}>
        <div style={{ color: '#f2f3f5', fontSize: '14px', fontWeight: 600, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>
          {message.fileName}
        </div>
        <div style={{ color: '#8a8f98', fontSize: '12px' }}>
          {isLoading ? 'Loading...' : `${formatSize(message.fileSize)} · Click to download`}
        </div>
      </div>
    </div>
  );
}