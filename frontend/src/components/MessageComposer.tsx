import React, { useRef, useEffect, useState, useMemo } from 'react';
import { EMOJI_LIST } from '../types';

interface MessageComposerProps {
  currentChannel: string;
  isPostMode: boolean;
  message: string;
  onSend: () => void;
  onMessageChange: (message: string) => void;
  onTogglePostMode: () => void;
  mentionableUsers?: string[]; 
}

export const MessageComposer: React.FC<MessageComposerProps> = ({
  currentChannel,
  isPostMode,
  message,
  onSend,
  onMessageChange,
  onTogglePostMode,
  mentionableUsers = [],
}) => {
  const [showEmojiPopup, setShowEmojiPopup] = useState(false);
  const textareaRef = useRef<HTMLTextAreaElement>(null);
  const emojiPopupRef = useRef<HTMLDivElement>(null);


  const [mentionQuery, setMentionQuery] = useState<string | null>(null); 
  const [mentionStartIndex, setMentionStartIndex] = useState<number>(-1);
  const [activeSuggestionIndex, setActiveSuggestionIndex] = useState(0);
  const mentionPopupRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (textareaRef.current) {
      textareaRef.current.style.height = 'auto';
      textareaRef.current.style.height = Math.min(textareaRef.current.scrollHeight, 150) + 'px';
    }
  }, [message]);

  useEffect(() => {
    const handleClickOutside = (event: MouseEvent) => {
      if (emojiPopupRef.current && !emojiPopupRef.current.contains(event.target as Node)) {
        setShowEmojiPopup(false);
      }
      if (mentionPopupRef.current && !mentionPopupRef.current.contains(event.target as Node)) {
        setMentionQuery(null);
      }
    };

    document.addEventListener('mousedown', handleClickOutside);
    return () => document.removeEventListener('mousedown', handleClickOutside);
  }, []);


  const filteredSuggestions = useMemo(() => {
    if (mentionQuery === null) return [];
    const q = mentionQuery.toLowerCase();
    return mentionableUsers
      .filter(u => u.toLowerCase().startsWith(q))
      .slice(0, 6);
  }, [mentionQuery, mentionableUsers]);

  const insertEmoji = (emoji: string) => {
    const cursorPos = textareaRef.current?.selectionStart || 0;
    const textBefore = message.substring(0, cursorPos);
    const textAfter = message.substring(cursorPos);
    
    onMessageChange(textBefore + emoji + textAfter);
    setShowEmojiPopup(false);
    
    setTimeout(() => {
      textareaRef.current?.focus();
      const newCursorPos = cursorPos + emoji.length;
      textareaRef.current?.setSelectionRange(newCursorPos, newCursorPos);
    }, 10);
  };

  
  const handleChange = (e: React.ChangeEvent<HTMLTextAreaElement>) => {
    const value = e.target.value;
    onMessageChange(value);

    const cursorPos = e.target.selectionStart;
    const textBeforeCursor = value.substring(0, cursorPos);

    
    const match = textBeforeCursor.match(/(?:^|\s)@([a-zA-Z0-9_]*)$/);

    if (match) {
      setMentionQuery(match[1]);
      setMentionStartIndex(cursorPos - match[1].length - 1); 
      setActiveSuggestionIndex(0);
    } else {
      setMentionQuery(null);
    }
  };

  
  const selectMention = (username: string) => {
    if (mentionStartIndex === -1) return;

    const cursorPos = textareaRef.current?.selectionStart || 0;
    const before = message.substring(0, mentionStartIndex);
    const after = message.substring(cursorPos);
    const newValue = `${before}@${username} ${after}`;

    onMessageChange(newValue);
    setMentionQuery(null);

    setTimeout(() => {
      textareaRef.current?.focus();
      const newCursorPos = mentionStartIndex + username.length + 2;
      textareaRef.current?.setSelectionRange(newCursorPos, newCursorPos);
    }, 10);
  };

  const handleKeyPress = (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
  
    if (mentionQuery !== null && filteredSuggestions.length > 0) {
      if (e.key === 'ArrowDown') {
        e.preventDefault();
        setActiveSuggestionIndex(prev => (prev + 1) % filteredSuggestions.length);
        return;
      }
      if (e.key === 'ArrowUp') {
        e.preventDefault();
        setActiveSuggestionIndex(prev => (prev - 1 + filteredSuggestions.length) % filteredSuggestions.length);
        return;
      }
      if (e.key === 'Enter' || e.key === 'Tab') {
        e.preventDefault();
        selectMention(filteredSuggestions[activeSuggestionIndex]);
        return;
      }
      if (e.key === 'Escape') {
        e.preventDefault();
        setMentionQuery(null);
        return;
      }
    }

    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      onSend();
    }
  };

  return (
    <div className="composer">
      {isPostMode && (
        <div className="composer-mode-indicator">
          <span>📌 Creating a post</span>
          <button 
            onClick={onTogglePostMode}
            className="close-mode-btn"
          >
            ✕
          </button>
        </div>
      )}
      
      <div className="composer-input-wrapper" style={{ position: 'relative' }}>
        <textarea
          ref={textareaRef}
          placeholder={`Message #${currentChannel}`}
          value={message}
          onChange={handleChange}
          onKeyDown={handleKeyPress}
          className="composer-textarea"
          rows={1}
        />

        {/* ✅ ДОБАВЛЕНО - выпадающий список упоминаний */}
        {mentionQuery !== null && filteredSuggestions.length > 0 && (
          <div
            ref={mentionPopupRef}
            className="mention-popup"
            style={{
              position: 'absolute',
              bottom: '100%',
              left: 0,
              marginBottom: '4px',
              background: '#2b2d31',
              border: '1px solid #3f4147',
              borderRadius: '8px',
              boxShadow: '0 4px 12px rgba(0,0,0,0.3)',
              minWidth: '200px',
              overflow: 'hidden',
              zIndex: 100,
            }}
          >
            {filteredSuggestions.map((username, idx) => (
              <div
                key={username}
                onClick={() => selectMention(username)}
                onMouseEnter={() => setActiveSuggestionIndex(idx)}
                style={{
                  padding: '8px 12px',
                  cursor: 'pointer',
                  background: idx === activeSuggestionIndex ? '#5865f2' : 'transparent',
                  color: idx === activeSuggestionIndex ? 'white' : '#dbdee1',
                  display: 'flex',
                  alignItems: 'center',
                  gap: '8px',
                  fontSize: '14px',
                }}
              >
                <span style={{
                  width: '20px', height: '20px', borderRadius: '50%',
                  background: '#5865f2', display: 'flex', alignItems: 'center',
                  justifyContent: 'center', fontSize: '11px', fontWeight: 700,
                  flexShrink: 0,
                }}>
                  {username.charAt(0).toUpperCase()}
                </span>
                @{username}
              </div>
            ))}
          </div>
        )}
      </div>
      
      <div className="composer-toolbar">
        <div className="toolbar-left">
          <button 
            className={`toolbar-icon-btn ${isPostMode ? 'active-icon' : ''}`}
            onClick={onTogglePostMode}
            title={isPostMode ? "Switch to message" : "Create a post"}
          >
            📝
          </button>
          
          <div className="emoji-trigger" ref={emojiPopupRef}>
            <button 
              className="toolbar-icon-btn" 
              onClick={() => setShowEmojiPopup(!showEmojiPopup)}
              title="Emoji"
            >
              😊
            </button>
            
            {showEmojiPopup && (
              <div className="emoji-popup show">
                {EMOJI_LIST.map(emoji => (
                  <button
                    key={emoji}
                    className="emoji-btn"
                    onClick={(e) => {
                      e.stopPropagation();
                      insertEmoji(emoji);
                    }}
                    title={emoji}
                  >
                    {emoji}
                  </button>
                ))}
              </div>
            )}
          </div>
        </div>
        
        <button 
          className="send-button"
          onClick={onSend}
          disabled={!message.trim()}
          title="Send message (Enter)"
        >
          <svg width="16" height="16" viewBox="0 0 20 20" fill="currentColor">
            <path d="M2.5 3l15 7-15 7V3zm2 2.5v9L14.5 10 4.5 5.5z"/>
          </svg>
          Send
        </button>
      </div>
    </div>
  );
};