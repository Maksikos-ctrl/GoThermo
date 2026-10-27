import React from 'react';
import { PinOff, X } from 'lucide-react';
import { Message } from '../types';
import { MessageItem } from './MessageItem';
import { PinnedMessage, pinCountLabel, pinnedByLabel } from './pinState';

interface PinnedPanelProps {
  pins: PinnedMessage[];
  loading: boolean;
  error: string | null;
  // "#general" or "@bob"
  channelTag: string;
  currentUser: string;
  knownUsernames?: string[];
  onClose: () => void;
  onUnpin: (message: Message) => void;
  onAddReaction: (messageId: string, emoji: string) => void;
  onEditMessage: (messageId: string, newText: string) => void;
  onDeleteMessage: (messageId: string) => void;
}

// The list of pinned messages of the open channel, shown on the right.
export const PinnedPanel: React.FC<PinnedPanelProps> = ({
  pins,
  loading,
  error,
  channelTag,
  currentUser,
  knownUsernames = [],
  onClose,
  onUnpin,
  onAddReaction,
  onEditMessage,
  onDeleteMessage,
}) => (
  <aside className="thread-panel pins-panel" aria-label="Pinned messages">
    <header className="thread-header">
      <div className="thread-title">
        <strong>Pinned</strong>
        <span className="thread-channel">{channelTag}</span>
      </div>
      <button type="button" className="thread-close" onClick={onClose} aria-label="Close pinned messages" title="Close">
        <X size={16} />
      </button>
    </header>

    <div className="thread-body">
      {error && <div className="thread-error" role="alert">{error}</div>}
      {!error && pins.length === 0 && (
        <div className="pins-empty">
          {loading ? (
            'Loading…'
          ) : (
            <>
              <strong>Nothing pinned yet</strong>
              <span>Hover a message and choose the pin to keep it here for everyone.</span>
            </>
          )}
        </div>
      )}
      {pins.length > 0 && <div className="pins-count">{pinCountLabel(pins.length)}</div>}
      {pins.map(p => (
        <section className="pin-item" key={p.message.id}>
          <div className="pin-meta">
            <span>{pinnedByLabel(p.pinnedBy, p.pinnedAt)}</span>
            <button
              type="button"
              className="pin-unpin"
              onClick={() => onUnpin(p.message)}
              title="Unpin"
              aria-label={`Unpin message from ${p.message.user}`}
            >
              <PinOff size={13} /> Unpin
            </button>
          </div>
          <MessageItem
            message={p.message}
            currentUser={currentUser}
            onAddReaction={onAddReaction}
            onEditMessage={onEditMessage}
            onDeleteMessage={onDeleteMessage}
            knownUsernames={knownUsernames}
          />
        </section>
      ))}
    </div>
  </aside>
);
