import React, { useRef, useEffect } from 'react';

interface ChannelModalProps {
  isOpen: boolean;
  channelName: string;
  channelDescription: string;
  onClose: () => void;
  onCreate: () => void;
  onNameChange: (name: string) => void;
  onDescriptionChange: (description: string) => void;
}

export const ChannelModal: React.FC<ChannelModalProps> = ({
  isOpen,
  channelName,
  channelDescription,
  onClose,
  onCreate,
  onNameChange,
  onDescriptionChange,
}) => {
  const inputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    if (isOpen && inputRef.current) {
      setTimeout(() => {
        inputRef.current?.focus();
      }, 100);
    }
  }, [isOpen]);

  const handleKeyPress = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter') {
      onCreate();
    }
  };

  if (!isOpen) return null;

  return (
    <div className="modal-overlay">
      <div className="modal">
        <div className="modal-header">
          <h3>Create New Channel</h3>
          <button 
            className="close-modal-btn"
            onClick={onClose}
          >
            ✕
          </button>
        </div>
        
        <div className="modal-content">
          <div className="form-group">
            <label>Channel Name</label>
            <input
              ref={inputRef}
              type="text"
              placeholder="announcements, help-desk, fun-stuff"
              value={channelName}
              onChange={(e) => onNameChange(e.target.value)}
              className="modal-input"
              onKeyPress={handleKeyPress}
            />
            <div className="input-hint">
              Lowercase letters, numbers, and hyphens only
            </div>
          </div>
          
          <div className="form-group">
            <label>Description (Optional)</label>
            <textarea
              placeholder="What is this channel about?"
              value={channelDescription}
              onChange={(e) => onDescriptionChange(e.target.value)}
              className="modal-textarea"
              rows={3}
            />
          </div>
          
          <div className="form-group">
            <label>Privacy</label>
            <div className="privacy-options">
              <label className="privacy-option">
                <input type="radio" name="privacy" defaultChecked />
                <span className="privacy-icon">🌐</span>
                <span className="privacy-text">
                  <strong>Public</strong>
                  <small>Anyone can join</small>
                </span>
              </label>
              
              <label className="privacy-option">
                <input type="radio" name="privacy" disabled />
                <span className="privacy-icon">🔒</span>
                <span className="privacy-text">
                  <strong>Private</strong>
                  <small>Only invited members</small>
                  <em className="coming-soon">Coming soon</em>
                </span>
              </label>
            </div>
          </div>
        </div>
        
        <div className="modal-footer">
          <button 
            className="modal-cancel-btn"
            onClick={onClose}
          >
            Cancel
          </button>
          <button 
            className="modal-create-btn"
            onClick={onCreate}
            disabled={!channelName.trim()}
          >
            Create Channel
          </button>
        </div>
      </div>
    </div>
  );
};