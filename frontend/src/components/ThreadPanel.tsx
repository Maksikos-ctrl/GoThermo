import React, { useEffect, useLayoutEffect, useRef, useState } from 'react';
import { Download, Search, Send, X } from 'lucide-react';
import { Message } from '../types';
import { MessageItem } from './MessageItem';
import { ThreadData, replyLabel } from './threadState';
import { filterReplies } from './threadExtras';
import '../styles/thread-extras.css';

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
  // When given, the header gets an export button (txt, json or csv).
  onExport?: (format: 'txt' | 'json' | 'csv') => Promise<void> | void;
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
  onExport,
}) => {
  const [searching, setSearching] = useState(false);
  const [query, setQuery] = useState('');
  const [menuOpen, setMenuOpen] = useState(false);
  const menuRef = useRef<HTMLDivElement>(null);
  const [draft, setDraft] = useState('');
  const [sending, setSending] = useState(false);
  const bodyRef = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLTextAreaElement>(null);
  const stickRef = useRef(true); // follow new replies only while at the bottom
  const lastCountRef = useRef(0);

  const replies = thread?.replies ?? [];
  // Search only changes what is shown; the scroll logic below keeps using all replies.
  const shown = searching ? filterReplies(replies, query) : replies;

  useEffect(() => {
    if (!menuOpen) return;
    const close = (e: MouseEvent) => {
      if (menuRef.current && !menuRef.current.contains(e.target as Node)) setMenuOpen(false);
    };
    document.addEventListener('mousedown', close);
    return () => document.removeEventListener('mousedown', close);
  }, [menuOpen]);

  const toggleSearch = () => {
    setSearching(s => !s);
    setQuery('');
  };

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
        <div className="thread-actions" ref={menuRef}>
          <button
            type="button"
            className={'thread-close' + (searching ? ' is-on' : '')}
            onClick={toggleSearch}
            aria-label="Search in thread"
            aria-pressed={searching}
            title="Search in thread"
          >
            <Search size={16} />
          </button>
          {onExport && (
            <button
              type="button"
              className={'thread-close' + (menuOpen ? ' is-on' : '')}
              onClick={() => setMenuOpen(o => !o)}
              aria-label="Export thread"
              aria-expanded={menuOpen}
              title="Export thread"
            >
              <Download size={16} />
            </button>
          )}
          {menuOpen && onExport && (
            <div className="thread-export-menu" role="menu">
              {(['txt', 'json', 'csv'] as const).map(f => (
                <button
                  key={f}
                  type="button"
                  role="menuitem"
                  onClick={() => {
                    setMenuOpen(false);
                    void onExport(f);
                  }}
                >
                  {f === 'txt' ? 'Text (.txt)' : f === 'json' ? 'JSON (.json)' : 'CSV (.csv)'}
                </button>
              ))}
            </div>
          )}
          <button type="button" className="thread-close" onClick={onClose} aria-label="Close thread" title="Close">
            <X size={16} />
          </button>
        </div>
      </header>

      {searching && (
        <div className="thread-search">
          <input
            autoFocus
            type="search"
            value={query}
            placeholder="Search in this thread…"
            aria-label="Search in thread"
            onChange={e => setQuery(e.target.value)}
            onKeyDown={e => {
              if (e.key === 'Escape') toggleSearch();
            }}
          />
        </div>
      )}

      <div className="thread-body" ref={bodyRef} onScroll={onBodyScroll}>
        {error && <div className="thread-error" role="alert">{error}</div>}
        {thread && (
          <>
            <div className="thread-parent">{item(thread.parent, false)}</div>
            <div className="thread-divider">
              <span>
                {searching && query.trim() !== ''
                  ? shown.length > 0
                    ? `${shown.length} of ${replies.length} match`
                    : 'No replies match'
                  : replies.length > 0
                    ? replyLabel(replies.length)
                    : loading
                      ? 'Loading replies…'
                      : 'No replies yet'}
              </span>
            </div>
            {shown.map((r, i) => item(r, sameRun(shown[i - 1], r)))}
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
