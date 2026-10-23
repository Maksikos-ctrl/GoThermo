import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { MessageText, parseBlocks, trimUrl } from '../MessageText';

const r = (text: string, known: string[] = [], me = 'alice') =>
  render(<MessageText text={text} knownUsernames={known} currentUser={me} />).container;

describe('trimUrl', () => {
  it('drops trailing sentence punctuation', () => {
    expect(trimUrl('https://x.com/a.')).toBe('https://x.com/a');
    expect(trimUrl('https://x.com/a,')).toBe('https://x.com/a');
    expect(trimUrl('https://x.com/a?!')).toBe('https://x.com/a');
  });
  it('drops an unbalanced closing paren but keeps a balanced one', () => {
    expect(trimUrl('https://x.com/a)')).toBe('https://x.com/a');
    expect(trimUrl('https://en.wikipedia.org/wiki/Go_(game)')).toBe('https://en.wikipedia.org/wiki/Go_(game)');
  });
});

describe('parseBlocks', () => {
  it('parses a closed fenced code block with language', () => {
    expect(parseBlocks('```go\nfmt.Println()\n```')).toEqual([
      { type: 'code', lang: 'go', code: 'fmt.Println()' },
    ]);
  });
  it('treats an unclosed fence as plain text', () => {
    const b = parseBlocks('```go\nno end');
    expect(b.every(x => x.type !== 'code')).toBe(true);
  });
  it('parses quotes and lists', () => {
    expect(parseBlocks('> hi')[0]).toEqual({ type: 'quote', lines: ['hi'] });
    expect(parseBlocks('- a\n- b')[0]).toEqual({ type: 'ul', items: ['a', 'b'] });
    expect(parseBlocks('3. a\n4. b')[0]).toEqual({ type: 'ol', items: ['a', 'b'], start: 3 });
  });
  it('splits paragraphs on blank lines', () => {
    expect(parseBlocks('a\nb\n\nc')).toEqual([
      { type: 'p', lines: ['a', 'b'] },
      { type: 'p', lines: ['c'] },
    ]);
  });
  it('handles CRLF', () => {
    expect(parseBlocks('a\r\nb')).toEqual([{ type: 'p', lines: ['a', 'b'] }]);
  });
});

describe('MessageText inline', () => {
  it('renders bold, italic, strike and code', () => {
    const c = r('**b** *i* _u_ ~~s~~ `c`');
    expect(c.querySelector('strong')?.textContent).toBe('b');
    expect(c.querySelectorAll('em')).toHaveLength(2);
    expect(c.querySelector('del')?.textContent).toBe('s');
    expect(c.querySelector('code.md-code')?.textContent).toBe('c');
  });

  it('does not italicise snake_case words', () => {
    const c = r('use my_var_name here');
    expect(c.querySelector('em')).toBeNull();
    expect(c.textContent).toBe('use my_var_name here');
  });

  it('keeps markdown inside inline code literal', () => {
    const c = r('`**not bold**`');
    expect(c.querySelector('strong')).toBeNull();
    expect(c.querySelector('code')?.textContent).toBe('**not bold**');
  });

  it('turns known @mentions into highlights, others stay text', () => {
    const c = r('hi @bob and @nobody', ['bob']);
    expect(c.querySelectorAll('.md-mention')).toHaveLength(1);
    expect(c.textContent).toContain('@nobody');
  });

  it('marks a mention of the current user', () => {
    const c = r('@Alice look', ['alice'], 'alice');
    expect(c.querySelector('.md-mention-self')).not.toBeNull();
  });
});

describe('MessageText links and safety', () => {
  it('links bare URLs without the trailing dot', () => {
    const c = r('see https://example.com/a.');
    const a = c.querySelector('a')!;
    expect(a.getAttribute('href')).toBe('https://example.com/a');
    expect(c.textContent).toBe('see https://example.com/a.');
  });

  it('renders [text](url) only for http(s)', () => {
    const ok = r('[go](https://example.com)');
    expect(ok.querySelector('a')?.textContent).toBe('go');
    const bad = r('[x](javascript:alert(1))');
    expect(bad.querySelector('a')).toBeNull();
  });

  it('never injects HTML', () => {
    const c = r('<img src=x onerror=alert(1)> <script>alert(1)</script>');
    expect(c.querySelector('img')).toBeNull();
    expect(c.querySelector('script')).toBeNull();
    expect(c.textContent).toContain('<script>');
  });

  it('opens links through the Wails runtime when present', () => {
    const open = vi.fn();
    (window as any).runtime = { BrowserOpenURL: open };
    const c = r('https://example.com');
    fireEvent.click(c.querySelector('a')!);
    expect(open).toHaveBeenCalledWith('https://example.com');
    delete (window as any).runtime;
  });

  it('survives pathological input quickly', () => {
    const t0 = Date.now();
    r('*'.repeat(5000) + '_'.repeat(5000) + '['.repeat(2000) + '`'.repeat(3000));
    expect(Date.now() - t0).toBeLessThan(2000);
  });
});

describe('MessageText blocks', () => {
  it('renders a code block with a language label and Copy button', () => {
    const c = r('```js\nlet a = 1\n```');
    expect(c.querySelector('.md-codeblock-lang')?.textContent).toBe('js');
    expect(c.querySelector('pre code')?.textContent).toBe('let a = 1');
    expect(screen.getByText('Copy')).toBeTruthy();
  });

  it('renders lists and quotes', () => {
    const c = r('- a\n- b\n\n> q');
    expect(c.querySelectorAll('ul li')).toHaveLength(2);
    expect(c.querySelector('blockquote')?.textContent).toBe('q');
  });
});
