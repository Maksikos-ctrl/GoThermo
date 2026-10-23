import React, { useRef, useEffect, useLayoutEffect } from 'react';
import { Message } from '../types';
import { MessageItem } from './MessageItem';
import { channelTag } from './channelName';

interface MessagesListProps {
  messages: Message[];
  currentChannel: string;
  currentUser: string;
  onAddReaction: (messageId: string, emoji: string) => void;
  onEditMessage: (messageId: string, newText: string) => void;
  onDeleteMessage: (messageId: string) => void;
  knownUsernames?: string[];
  // History paging: older messages are loaded when the user scrolls to the top.
  hasMoreOlder?: boolean;
  loadingOlder?: boolean;
  onLoadOlder?: () => void;
}

// How close to the top (px) the user has to scroll to trigger loading.
const TOP_THRESHOLD = 100;

// Messages from the same person this close together are shown as one block.
const GROUP_WINDOW_MS = 5 * 60 * 1000;

const startsGroup = (prev: Message | undefined, cur: Message): boolean => {
  if (!prev) return false;
  const a = prev as any;
  const b = cur as any;
  if (a.isCall || b.isCall || a.isPost || b.isPost) return false;
  if (prev.user !== cur.user) return false;
  const gap = new Date(cur.timestamp).getTime() - new Date(prev.timestamp).getTime();
  return gap >= 0 && gap < GROUP_WINDOW_MS;
};

export const MessagesList: React.FC<MessagesListProps> = ({
  messages,
  currentChannel,
  currentUser,
  onAddReaction,
  onEditMessage,
  onDeleteMessage,
  knownUsernames = [],
  hasMoreOlder = false,
  loadingOlder = false,
  onLoadOlder,
}) => {
  const messagesEndRef = useRef<HTMLDivElement>(null);
  const containerRef = useRef<HTMLDivElement>(null);
  const lastMessageIdRef = useRef<string | null>(null);
  const lastChannelRef = useRef<string>(currentChannel);

  // Remembers where the user was when older messages were requested, so the
  // view can stay on the same message after they are added above it.
  const anchorRef = useRef<{ height: number; top: number } | null>(null);
  const firstIdRef = useRef<string | null>(null);

  const isNearBottom = (): boolean => {
    const el = containerRef.current;
    if (!el) return true;
    const threshold = 120;
    return el.scrollHeight - el.scrollTop - el.clientHeight < threshold;
  };

  const scrollToBottom = () => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  };

  const requestOlder = () => {
    const el = containerRef.current;
    if (!el || !onLoadOlder || !hasMoreOlder || loadingOlder) return;
    anchorRef.current = { height: el.scrollHeight, top: el.scrollTop };
    onLoadOlder();
  };

  const handleScroll = () => {
    const el = containerRef.current;
    if (el && el.scrollTop < TOP_THRESHOLD) requestOlder();
  };

  // Older messages were added above: keep the same message under the cursor.
  // Runs before the browser paints, so there is no visible jump.
  useLayoutEffect(() => {
    const el = containerRef.current;
    const firstId = messages.length ? messages[0].id : null;
    const prevFirst = firstIdRef.current;

    if (el && anchorRef.current && prevFirst && firstId !== prevFirst) {
      const wasPrepended = messages.findIndex(m => m.id === prevFirst) > 0;
      if (wasPrepended) {
        // add the height that appeared above; keeps any scrolling done meanwhile
        el.scrollTop += el.scrollHeight - anchorRef.current.height;
        anchorRef.current = null;
      }
    }
    firstIdRef.current = firstId;
  }, [messages]);

  // A request that brought nothing must not leave a stale anchor behind.
  useEffect(() => {
    if (!loadingOlder) anchorRef.current = null;
  }, [loadingOlder]);

  // If the loaded messages do not fill the screen there is nothing to scroll,
  // so load older ones until there is.
  useEffect(() => {
    const el = containerRef.current;
    if (!el || !hasMoreOlder || loadingOlder || messages.length === 0) return;
    if (el.scrollHeight <= el.clientHeight + 40) requestOlder();
    // not re-run on loadingOlder: a failed request must not retry in a loop
  }, [messages, hasMoreOlder]);

  useEffect(() => {
    const lastMsg = messages[messages.length - 1];
    const lastId = lastMsg ? lastMsg.id : null;

    const channelChanged = lastChannelRef.current !== currentChannel;
    const isNewMessage = lastId !== null && lastId !== lastMessageIdRef.current;

    if (channelChanged) {
      requestAnimationFrame(() => {
        messagesEndRef.current?.scrollIntoView({ behavior: 'auto' });
      });
    } else if (isNewMessage && isNearBottom()) {
      scrollToBottom();
    }

    lastMessageIdRef.current = lastId;
    lastChannelRef.current = currentChannel;
  }, [messages, currentChannel]);

  if (messages.length === 0) {
    return (
      <div className="messages" ref={containerRef}>
        <div className="welcome-card">
          <h2>Welcome to {channelTag(currentChannel, currentUser)}!</h2>
          <p>This is the beginning of this channel.</p>
          <p>Send a message to start the conversation.</p>
        </div>
        <div ref={messagesEndRef} />
      </div>
    );
  }

  return (
    <div
      className="messages"
      ref={containerRef}
      onScroll={handleScroll}
      // We keep the position ourselves; the browser's own scroll anchoring
      // (Chromium / WebView2 only) would add a second, conflicting correction,
      // and CSS smooth scrolling would turn our correction into a visible slide.
      style={{ overflowAnchor: 'none', scrollBehavior: 'auto' }}
    >
      {loadingOlder && (
        <div className="history-note">
          Loading older messages…
        </div>
      )}
      {!hasMoreOlder && !loadingOlder && (
        <div className="history-note">
          This is the beginning of {channelTag(currentChannel, currentUser)}
        </div>
      )}
      {messages.map((msg, i) => (
        <MessageItem
          key={msg.id}
          isGrouped={startsGroup(messages[i - 1], msg)}
          message={msg}
          currentUser={currentUser}
          onAddReaction={onAddReaction}
          onEditMessage={onEditMessage}
          onDeleteMessage={onDeleteMessage}
          knownUsernames={knownUsernames}
        />
      ))}
      <div ref={messagesEndRef} />
    </div>
  );
};
