import React, { useRef, useEffect } from 'react';
import { Message } from '../types';
import { MessageItem } from './MessageItem';

interface MessagesListProps {
  messages: Message[];
  currentChannel: string;
  currentUser: string;
  onAddReaction: (messageId: string, emoji: string) => void;
}

export const MessagesList: React.FC<MessagesListProps> = ({
  messages,
  currentChannel,
  currentUser,
  onAddReaction,
}) => {
  const messagesEndRef = useRef<HTMLDivElement>(null);

  const scrollToBottom = () => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  };

  useEffect(() => {
    scrollToBottom();
  }, [messages]);

  if (messages.length === 0) {
    return (
      <div className="messages">
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
    <div className="messages">
      {messages.map((msg) => (
        <MessageItem
          key={msg.id}
          message={msg}
          currentUser={currentUser}
          onAddReaction={onAddReaction}
        />
      ))}
      <div ref={messagesEndRef} />
    </div>
  );
};