import { describe, it, expect } from 'vitest';
import { avatarColor } from '../avatarColor';

describe('avatarColor', () => {
  it('is deterministic', () => {
    expect(avatarColor('maksikos')).toBe(avatarColor('maksikos'));
  });

  it('always returns a hex colour, even for odd input', () => {
    for (const n of ['', 'a', 'Ирина', '😀', 'x'.repeat(500)]) {
      expect(avatarColor(n)).toMatch(/^#[0-9a-f]{6}$/i);
    }
  });

  it('spreads different users over several colours', () => {
    const set = new Set(['a', 'b', 'c', 'd', 'e', 'f', 'g', 'h'].map(avatarColor));
    expect(set.size).toBeGreaterThan(3);
  });
});
