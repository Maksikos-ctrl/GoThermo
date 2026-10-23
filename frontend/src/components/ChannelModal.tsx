import React, { useRef, useEffect } from 'react';

interface ChannelModalProps {
  isOpen: boolean;
  channelName: string;
  channelDescription: string;
  isPrivate: boolean;
  // Everyone who can be invited (the current user is not in this list).
  availableUsers: string[];
  selectedMembers: string[];
  onClose: () => void;
  onCreate: () => void;
  onNameChange: (name: string) => void;
  onDescriptionChange: (description: string) => void;
  onPrivateChange: (isPrivate: boolean) => void;
  onToggleMember: (username: string) => void;
}

export const ChannelModal: React.FC<ChannelModalProps> = ({
  isOpen,
  channelName,
  channelDescription,
  isPrivate,
  availableUsers,
  selectedMembers,
  onClose,
  onCreate,
  onNameChange,
  onDescriptionChange,
  onPrivateChange,
  onToggleMember,
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
          <button className="close-modal-btn" onClick={onClose}>
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
                <input
                  type="radio"
                  name="privacy"
                  checked={!isPrivate}
                  onChange={() => onPrivateChange(false)}
                />
                <span className="privacy-icon">🌐</span>
                <span className="privacy-text">
                  <strong>Public</strong>
                  <small>Anyone can join</small>
                </span>
              </label>
              <label className="privacy-option">
                <input
                  type="radio"
                  name="privacy"
                  checked={isPrivate}
                  onChange={() => onPrivateChange(true)}
                />
                <span className="privacy-icon">🔒</span>
                <span className="privacy-text">
                  <strong>Private</strong>
                  <small>Only invited members</small>
                </span>
              </label>
            </div>
          </div>

          {isPrivate && (
            <div className="form-group">
              <label>Invite members</label>
              {availableUsers.length === 0 ? (
                <div className="input-hint">No other users yet. You can add people later.</div>
              ) : (
                <div
                  style={{
                    maxHeight: '160px',
                    overflowY: 'auto',
                    border: '1px solid #3f4147',
                    borderRadius: '8px',
                    padding: '4px',
                  }}
                >
                  {availableUsers.map((username) => (
                    <label
                      key={username}
                      style={{
                        display: 'flex',
                        alignItems: 'center',
                        gap: '10px',
                        padding: '6px 8px',
                        borderRadius: '6px',
                        cursor: 'pointer',
                        background: selectedMembers.includes(username)
                          ? 'rgba(88, 101, 242, 0.25)'
                          : 'transparent',
                      }}
                    >
                      <input
                        type="checkbox"
                        checked={selectedMembers.includes(username)}
                        onChange={() => onToggleMember(username)}
                      />
                      <span>{username}</span>
                    </label>
                  ))}
                </div>
              )}
              <div className="input-hint">
                {selectedMembers.length} selected. You are added automatically.
              </div>
            </div>
          )}
        </div>
        <div className="modal-footer">
          <button className="modal-cancel-btn" onClick={onClose}>
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