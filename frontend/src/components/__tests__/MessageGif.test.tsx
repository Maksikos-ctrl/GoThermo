import { describe, it, expect } from 'vitest';
import { render } from '@testing-library/react';
import { MessageText } from '../MessageText';

describe('GIF messages', () => {
  it('renders a giphy link as an image', () => {
    const { container } = render(<MessageText text="https://media.giphy.com/media/x/200.gif" currentUser="a" />);
    const img = container.querySelector('img.md-gif');
    expect(img?.getAttribute('src')).toBe('https://media.giphy.com/media/x/200.gif');
  });
  it('never renders other hosts as images', () => {
    const { container } = render(<MessageText text="https://evil.example/a.gif" currentUser="a" />);
    expect(container.querySelector('img')).toBeNull();
  });
});
