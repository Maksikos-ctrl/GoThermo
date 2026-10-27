import React from 'react';
import { typingLabel } from './typingState';
import '../styles/presence.css';

interface TypingIndicatorProps {
  users: string[];
}

// One line above the composer. It always takes its height so the chat does
// not jump when someone starts or stops typing.
export const TypingIndicator: React.FC<TypingIndicatorProps> = ({ users }) => {
  const label = typingLabel(users);
  return (
    <div className="typing-indicator" aria-live="polite">
      {label && (
        <>
          <span className="typing-dots" aria-hidden="true">
            <i />
            <i />
            <i />
          </span>
          <span className="typing-text">{label}</span>
        </>
      )}
    </div>
  );
};
