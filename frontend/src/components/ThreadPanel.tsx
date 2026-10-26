import React, { useEffect, useLayoutEffect, useRef, useState } from 'react';
import { Send, X } from 'lucide-react';
import { Message } from '../types';
import { MessageItem } from './MessageItem';
import { ThreadData, replyLabel } from './threadState';

interface ThreadPanelProps {
  thread: ThreadData | null;
  loading: boolean;
  error: string | null;
  // "#general" or "@bob" - where the thread lives
  channelTag: string;
  currentUser: string;
  knownUsernames?: string[];
  onClose: () => void;
  // One set of handlers for the parent and the replies; the caller knows which is which.
  onSend: (text: string) => Promise<void> | void;
  onAddReaction: (messageId: string, emoji: string) => void;
  onEditMessage: (messageId: string, newText: string) => void;
  onDeleteMessage: (messageId: string) => void;
}

const GROUP_WINDOW_MS = 5 * 60 * 1000;

const sameRun = (prev: Message | undefined, cur: Message): boolean => {
  if (!prev || prev.user !== cur.user) return false;
  const gap = new Date(cur.timestamp).getTime() - new Date(prev.timestamp).getTime();
  return gap >= 0 && gap < GROUP_WINDOW_MS;
};

export const ThreadPanel: React.FC<ThreadPanelProps> = ({
  thread,
  loading,
  error,
  channelTag,
  currentUser,
  knownUsernames = [],
  onClose,
  onSend,
  onAddReaction,
  onEditMessage,
  onDeleteMessage,
}) => {
  const [draft, setDraft] = useState('');
  const [sending, setSending] = useState(false);
  const bodyRef = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLTextAreaElement>(null);
  const stickRef = useRef(true); // follow new replies only while at the bottom
  const lastCountRef = useRef(0);

  const replies = thread?.replies ?? [];

  useEffect(() => {
    inputRef.current?.focus();
  }, [thread?.parent.id]);

  const onBodyScroll = () => {
    const el = bodyRef.current;
    if (el) stickRef.current = el.scrollHeight - el.scrollTop - el.clientHeight < 80;
  };

  // New replies keep the view at the bottom, unless the reader scrolled up.
  // The reader's own reply always brings it down.
  useLayoutEffect(() => {
    const el = bodyRef.current;
    if (!el) return;
    const grew = replies.length > lastCountRef.current;
    const mine = grew && replies[replies.length - 1]?.user === currentUser;
    if (lastCountRef.current === 0 || (grew && (stickRef.current || mine))) {
      el.scrollTop = el.scrollHeight;
    }
    lastCountRef.current = replies.length;
  }, [replies.length]);

  const submit = async () => {
    const text = draft.trim();
    if (!text || sending) return;
    setSending(true);
    try {
      await onSend(text);
      setDraft('');
      stickRef.current = true;
    } catch {
      // the caller shows the error; the draft stays so nothing is lost
    } finally {
      setSending(false);
      inputRef.current?.focus();
    }
  };

  const onKeyDown = (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.key === 'Enter' && !e.shiftKey && !e.nativeEvent.isComposing) {
      e.preventDefault();
      void submit();
    } else if (e.key === 'Escape' && draft === '') {
      onClose();
    }
  };

  const item = (m: Message, grouped: boolean) => (
    <MessageItem
      key={m.id}
      message={m}
      isGrouped={grouped}
      currentUser={currentUser}
      onAddReaction={onAddReaction}
      onEditMessage={onEditMessage}
      onDeleteMessage={onDeleteMessage}
      knownUsernames={knownUsernames}
    />
  );

  return (
    <aside className="thread-panel" aria-label="Thread">
      <header className="thread-header">
        <div className="thread-title">
          <strong>Thread</strong>
          <span className="thread-channel">{channelTag}</span>
        </div>
        <button type="button" className="thread-close" onClick={onClose} aria-label="Close thread" title="Close">
          <X size={16} />
        </button>
      </header>

      <div className="thread-body" ref={bodyRef} onScroll={onBodyScroll}>
        {error && <div className="thread-error" role="alert">{error}</div>}
        {thread && (
          <>
            <div className="thread-parent">{item(thread.parent, false)}</div>
            <div className="thread-divider">
              <span>
                {replies.length > 0 ? replyLabel(replies.length) : loading ? 'Loading replies…' : 'No replies yet'}
              </span>
            </div>
            {replies.map((r, i) => item(r, sameRun(replies[i - 1], r)))}
          </>
        )}
      </div>

      <div className="thread-composer">
        <textarea
          ref={inputRef}
          value={draft}
          rows={1}
          placeholder="Reply…"
          aria-label="Reply"
          disabled={!thread}
          onChange={e => setDraft(e.target.value)}
          onKeyDown={onKeyDown}
        />
        <button
          type="button"
          className="thread-send"
          onClick={() => void submit()}
          disabled={!thread || sending || draft.trim() === ''}
          aria-label="Send reply"
          title="Send (Enter)"
        >
          <Send size={16} />
        </button>
      </div>
    </aside>
  );
};
