import React, { useRef, useEffect } from 'react';
import { Message } from '../types';
import { MessageItem } from './MessageItem';

interface MessagesListProps {
  messages: Message[];
  currentChannel: string;
  currentUser: string;
  onAddReaction: (messageId: string, emoji: string) => void;
  onEditMessage: (messageId: string, newText: string) => void;
  onDeleteMessage: (messageId: string) => void;
  knownUsernames?: string[];
}

export const MessagesList: React.FC<MessagesListProps> = ({
  messages,
  currentChannel,
  currentUser,
  onAddReaction,
  onEditMessage,
  onDeleteMessage,
  knownUsernames = [],
}) => {
  const messagesEndRef = useRef<HTMLDivElement>(null);
  const containerRef = useRef<HTMLDivElement>(null);
  const lastMessageIdRef = useRef<string | null>(null); 
  const lastChannelRef = useRef<string>(currentChannel); 


  const isNearBottom = (): boolean => {
    const el = containerRef.current;
    if (!el) return true;
    const threshold = 120;
    return el.scrollHeight - el.scrollTop - el.clientHeight < threshold;
  };

  const scrollToBottom = () => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  };

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
          <h2>Welcome to #{currentChannel}!</h2>
          <p>This is the beginning of this channel.</p>
          <p>Send a message to start the conversation.</p>
        </div>
        <div ref={messagesEndRef} />
      </div>
    );
  }

  return (
    <div className="messages" ref={containerRef}>
      {messages.map((msg) => (
        <MessageItem
          key={msg.id}
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