import { describe, it, expect, beforeEach } from 'vitest';
import { render, screen, act } from '@testing-library/react';
import { Avatar } from '../Avatar';
import { setAvatar, setAvatarsFromUsers, getAvatar } from '../avatarStore';

describe('avatars', () => {
  beforeEach(() => setAvatarsFromUsers([]));

  it('shows the initial when there is no picture', () => {
    const { container } = render(<Avatar username="bob" className="x" />);
    expect(container.textContent).toBe('B');
    expect(container.querySelector('img')).toBeNull();
  });

  it('switches to the picture live and back', () => {
    const { container } = render(<Avatar username="Bob" className="x" />);
    act(() => setAvatar('bob', 'data:image/jpeg;base64,AAAA'));
    expect(screen.getByAltText('Bob').getAttribute('src')).toBe('data:image/jpeg;base64,AAAA');
    act(() => setAvatar('BOB', undefined));
    expect(container.querySelector('img')).toBeNull();
    expect(container.textContent).toBe('B');
  });

  it('loads from a user list and ignores case', () => {
    setAvatarsFromUsers([{ username: 'Ann', avatar: 'data:x' }, { username: 'Cy' }]);
    expect(getAvatar('ann')).toBe('data:x');
    expect(getAvatar('cy')).toBeUndefined();
  });
});
