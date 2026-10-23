import React, { useRef, useEffect, useState, useMemo } from 'react';
import { EMOJI_LIST } from '../types';

// Same limit as the backend (uploads.go). Checked here too so a huge file is
// rejected before it is read into memory.
const MAX_UPLOAD_BYTES = 25 * 1024 * 1024;

const readAsDataURL = (file: File): Promise<string> =>
  new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.onload = () => resolve(reader.result as string);
    reader.onerror = () => reject(reader.error ?? new Error('read failed'));
    reader.readAsDataURL(file);
  });

// Pasted screenshots / copied photos usually have a generic name ("image.png").
const nameForFile = (file: File): string => {
  const generic = !file.name || /^image\.(png|jpe?g|gif|webp|tiff?)$/i.test(file.name);
  if (!generic) return file.name;
  const ext = (file.type.split('/')[1] || 'png').replace('jpeg', 'jpg').replace('tiff', 'png');
  const stamp = new Date().toISOString().replace(/[:T]/g, '-').slice(0, 19);
  return `pasted-${stamp}.${ext}`;
};

const hasFiles = (dt: DataTransfer | null): boolean =>
  !!dt && Array.from(dt.types || []).includes('Files');

interface MessageComposerProps {
  currentChannel: string;
  isPostMode: boolean;
  message: string;
  onSend: () => void;
  onMessageChange: (message: string) => void;
  onTogglePostMode: () => void;
  mentionableUsers?: string[];
  onSendFile?: (fileName: string, mimeType: string, base64Data: string) => void | Promise<void>;
}

export const MessageComposer: React.FC<MessageComposerProps> = ({
  currentChannel,
  isPostMode,
  message,
  onSend,
  onMessageChange,
  onTogglePostMode,
  mentionableUsers = [],
  onSendFile, 
}) => {
  const [showEmojiPopup, setShowEmojiPopup] = useState(false);
  const [isUploading, setIsUploading] = useState(false); 
  const [isDragging, setIsDragging] = useState(false);
  const [uploadError, setUploadError] = useState<string | null>(null);
  const textareaRef = useRef<HTMLTextAreaElement>(null);
  const emojiPopupRef = useRef<HTMLDivElement>(null);
  const fileInputRef = useRef<HTMLInputElement>(null); 

  
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

 
  // One path for all three ways of adding files: the 📎 button, paste and
  // drag & drop. Files are sent one after another.
  const sendFiles = async (files: File[]) => {
    if (!onSendFile || files.length === 0) return;
    setUploadError(null);
    setIsUploading(true);
    try {
      for (const file of files) {
        if (file.size > MAX_UPLOAD_BYTES) {
          setUploadError(`"${file.name}" is too large (max 25 MB)`);
          continue;
        }
        try {
          const dataUrl = await readAsDataURL(file);
          await onSendFile(nameForFile(file), file.type || 'application/octet-stream', dataUrl);
        } catch (err) {
          console.error('Error sending file', err);
          setUploadError(`Could not send "${file.name}"`);
        }
      }
    } finally {
      setIsUploading(false);
    }
  };

  // Document-level listeners always call the newest sendFiles.
  const sendFilesRef = useRef(sendFiles);
  sendFilesRef.current = sendFiles;

  useEffect(() => {
    if (!uploadError) return;
    const timer = setTimeout(() => setUploadError(null), 5000);
    return () => clearTimeout(timer);
  }, [uploadError]);

  // Drag & drop anywhere on the window. The counter handles dragenter/leave
  // firing for every child element.
  useEffect(() => {
    if (!onSendFile) return;
    let depth = 0;

    const onDragEnter = (e: DragEvent) => {
      if (!hasFiles(e.dataTransfer)) return;
      e.preventDefault();
      depth++;
      setIsDragging(true);
    };
    const onDragOver = (e: DragEvent) => {
      if (!hasFiles(e.dataTransfer)) return;
      e.preventDefault(); // required, otherwise drop never fires
      if (e.dataTransfer) e.dataTransfer.dropEffect = 'copy';
    };
    const onDragLeave = (e: DragEvent) => {
      if (!hasFiles(e.dataTransfer)) return;
      depth = Math.max(0, depth - 1);
      if (depth === 0) setIsDragging(false);
    };
    const onDrop = (e: DragEvent) => {
      if (!hasFiles(e.dataTransfer)) return;
      e.preventDefault(); // stops the webview from navigating to the file
      depth = 0;
      setIsDragging(false);
      const files = Array.from(e.dataTransfer?.files ?? []);
      void sendFilesRef.current(files);
    };

    window.addEventListener('dragenter', onDragEnter);
    window.addEventListener('dragover', onDragOver);
    window.addEventListener('dragleave', onDragLeave);
    window.addEventListener('drop', onDrop);
    return () => {
      window.removeEventListener('dragenter', onDragEnter);
      window.removeEventListener('dragover', onDragOver);
      window.removeEventListener('dragleave', onDragLeave);
      window.removeEventListener('drop', onDrop);
    };
  }, [onSendFile]);

  const handleFileSelect = (e: React.ChangeEvent<HTMLInputElement>) => {
    const files = Array.from(e.target.files ?? []);
    e.target.value = '';
    void sendFiles(files);
  };

  // Cmd+V with an image / file in the clipboard (screenshot, photo copied from
  // Telegram, file copied in Finder). Plain text pastes as usual.
  const handlePaste = (e: React.ClipboardEvent<HTMLTextAreaElement>) => {
    if (!onSendFile) return;
    const cd = e.clipboardData;
    let files = Array.from(cd.files ?? []);
    if (files.length === 0) {
      files = Array.from(cd.items ?? [])
        .filter(item => item.kind === 'file')
        .map(item => item.getAsFile())
        .filter((f): f is File => f !== null);
    }
    if (files.length === 0) return;
    e.preventDefault(); // otherwise the file path lands in the text box
    void sendFiles(files);
  };

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
      {isDragging && (
        <div
          style={{
            position: 'fixed', inset: 0, zIndex: 2000,
            background: 'rgba(88, 101, 242, 0.25)',
            border: '3px dashed #5865f2',
            display: 'flex', alignItems: 'center', justifyContent: 'center',
            pointerEvents: 'none',
          }}
        >
          <div style={{
            background: '#2b2d31', color: '#fff', padding: '18px 28px',
            borderRadius: '12px', fontSize: '18px', fontWeight: 600,
            boxShadow: '0 8px 24px rgba(0,0,0,0.4)',
          }}>
            📎 Drop files to send to #{currentChannel}
          </div>
        </div>
      )}
      {uploadError && (
        <div style={{ color: '#f23f42', fontSize: '13px', padding: '4px 8px' }}>
          ⚠️ {uploadError}
        </div>
      )}
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
          onPaste={handlePaste}
          className="composer-textarea"
          rows={1}
        />

       
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

          
          {onSendFile && (
            <>
              <input
                ref={fileInputRef}
                type="file"
                multiple
                style={{ display: 'none' }}
                onChange={handleFileSelect}
              />
              <button
                className="toolbar-icon-btn"
                onClick={() => fileInputRef.current?.click()}
                title="Attach file"
                disabled={isUploading}
              >
                {isUploading ? '⏳' : '📎'}
              </button>
            </>
          )}
          
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