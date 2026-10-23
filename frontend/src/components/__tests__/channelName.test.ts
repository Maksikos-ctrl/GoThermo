import { describe, it, expect } from 'vitest';
import { channelLabel, channelTag } from '../channelName';

describe('channelLabel', () => {
  it('shows the other person in a direct message', () => {
    expect(channelLabel('dm_alice_bob', 'alice')).toEqual({ isDM: true, text: 'bob' });
    expect(channelLabel('dm_alice_bob', 'bob')).toEqual({ isDM: true, text: 'alice' });
  });

  it('labels group chats', () => {
    expect(channelLabel('dm_group_123', 'alice')).toEqual({ isDM: true, text: 'Group chat' });
  });

  it('leaves normal channels alone', () => {
    expect(channelLabel('general', 'alice')).toEqual({ isDM: false, text: 'general' });
  });
});

describe('channelTag', () => {
  it('uses # for channels and @ for DMs', () => {
    expect(channelTag('general', 'alice')).toBe('#general');
    expect(channelTag('dm_alice_bob', 'alice')).toBe('@bob');
  });
});
