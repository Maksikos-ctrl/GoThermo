import React from 'react';
import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { ChatHeader } from '../ChatHeader';

const noop = () => {};
const header = (over: Partial<React.ComponentProps<typeof ChatHeader>> = {}) =>
  render(
    <ChatHeader
      currentChannel="general"
      channels={[]}
      messagesCount={3}
      onStartVideoCall={noop}
      onStartAudioCall={noop}
      {...over}
    />
  );

describe('ChatHeader channel drawer button', () => {
  it('is absent without a handler', () => {
    header();
    expect(screen.queryByLabelText('Open channels')).toBeNull();
  });

  it('opens the drawer when pressed', () => {
    const onOpenNav = vi.fn();
    header({ onOpenNav });
    fireEvent.click(screen.getByLabelText('Open channels'));
    expect(onOpenNav).toHaveBeenCalledTimes(1);
  });
});
