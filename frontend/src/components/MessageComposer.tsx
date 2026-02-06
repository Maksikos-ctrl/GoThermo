import React, { useRef, useEffect, useState } from 'react';
import { EMOJI_LIST } from '../types';

interface MessageComposerProps {
  currentChannel: string;
  isPostMode: boolean;
  message: string;
  onSend: () => void;
  onMessageChange: (message: string) => void;
  onTogglePostMode: () => void;
}

export const MessageComposer: React.FC<MessageComposerProps> = ({
  currentChannel,
  isPostMode,
  message,
  onSend,
  onMessageChange,
  onTogglePostMode,
}) => {
  const [showEmojiPopup, setShowEmojiPopup] = useState(false);
  const textareaRef = useRef<HTMLTextAreaElement>(null);
  const emojiPopupRef = useRef<HTMLDivElement>(null);

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
    };

    document.addEventListener('mousedown', handleClickOutside);
    return () => document.removeEventListener('mousedown', handleClickOutside);
  }, []);

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

  const handleKeyPress = (e: React.KeyboardEvent) => {
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
      
      <div className="composer-input-wrapper">
        <textarea
          ref={textareaRef}
          placeholder={`Message #${currentChannel}`}
          value={message}
          onChange={(e) => onMessageChange(e.target.value)}
          onKeyPress={handleKeyPress}
          className="composer-textarea"
          rows={1}
        />
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