import { describe, it, expect } from 'vitest';
import { gifUrlOf } from '../gifUrl';

describe('gifUrlOf', () => {
  it('accepts a bare giphy media link', () => {
    expect(gifUrlOf(' https://media1.giphy.com/media/x/200.gif ')).toBe('https://media1.giphy.com/media/x/200.gif');
  });
  it.each([
    'http://media.giphy.com/a.gif',
    'https://evil.example/a.gif',
    'https://giphy.com.evil.io/a.gif',
    'https://evilgiphy.com/a.gif',
    'https://media.giphy.com/page',
    'look https://media.giphy.com/a.gif',
    'https://u:p@media.giphy.com/a.gif',
    'not a url',
    '',
  ])('refuses %s', t => {
    expect(gifUrlOf(t)).toBeNull();
  });
});
