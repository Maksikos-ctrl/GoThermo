import React, { useState } from 'react';
import './Markdown.css';

// Chat-style Markdown for message text. No dependencies and no HTML is ever
// injected: everything is built from React elements, so a message can never
// run script or inject markup.
//
// Supported:  **bold**  *italic*  _italic_  ~~strike~~  `code`
//             ```fenced code blocks``` (with an optional language, Copy button)
//             > quotes   - bullet lists   1. numbered lists
//             https://links, [text](https://links), @mentions

interface MessageTextProps {
  text: string;
  knownUsernames?: string[];
  currentUser: string;
}

type Ctx = { known: Set<string>; currentUser: string };

// ───────────────────────── links ─────────────────────────

const count = (s: string, ch: string) => s.split(ch).length - 1;

// Drops the punctuation that usually trails a link in a sentence:
// "see https://x.com/a." or "(https://x.com/a)". A closing ")" stays when the
// link itself has the opening one (Wikipedia URLs).
export function trimUrl(url: string): string {
  for (;;) {
    const last = url.slice(-1);
    if (/[.,;:!?\]}'"]/.test(last)) {
      url = url.slice(0, -1);
    } else if (last === ')' && count(url, ')') > count(url, '(')) {
      url = url.slice(0, -1);
    } else {
      return url;
    }
  }
}

function openLink(url: string) {
  // Native app: open in the system browser. Dev in a normal browser: new tab.
  try {
    const rt = (window as any).runtime;
    if (rt && typeof rt.BrowserOpenURL === 'function') {
      rt.BrowserOpenURL(url);
      return;
    }
  } catch {
    /* fall through */
  }
  window.open(url, '_blank', 'noopener,noreferrer');
}

function Link({ href, children }: { href: string; children: React.ReactNode }) {
  return (
    <a
      className="md-link"
      href={href}
      rel="noopener noreferrer"
      onClick={e => {
        e.preventDefault();
        openLink(href);
      }}
    >
      {children}
    </a>
  );
}

// ───────────────────────── inline ─────────────────────────

const isWordChar = (c: string | undefined) => !!c && /[A-Za-z0-9]/.test(c);

// One alternation, in priority order. Groups:
// 1 ```x```  2 `x`  3 **x**  4 ~~x~~  5 [t](url)  6 bare url  7 *x*  8 _x_  9 @user
const INLINE_RE =
  /(```[^`\n]+```)|(`[^`\n]+`)|(\*\*(?:[^*\n]|\*(?!\*))+?\*\*)|(~~[^~\n]+?~~)|(\[[^\]\n]+\]\(https?:\/\/[^\s)]+\))|(https?:\/\/[^\s<>"'`]+)|(\*[^*\s][^*\n]*?\*)|(_[^_\s][^_\n]*?_)|(@[a-zA-Z0-9_]+)/g;

export function renderInline(text: string, ctx: Ctx, prefix = 'i'): React.ReactNode[] {
  const out: React.ReactNode[] = [];
  const re = new RegExp(INLINE_RE.source, 'g');
  let last = 0;
  let n = 0;
  let m: RegExpExecArray | null;

  const pushText = (s: string) => {
    if (s) out.push(s);
  };

  while ((m = re.exec(text)) !== null) {
    const start = m.index;
    let token = m[0];
    const key = `${prefix}-${n++}`;
    let node: React.ReactNode | null = null;

    if (m[1]) {
      node = <code key={key} className="md-code">{token.slice(3, -3)}</code>;
    } else if (m[2]) {
      node = <code key={key} className="md-code">{token.slice(1, -1)}</code>;
    } else if (m[3]) {
      node = <strong key={key}>{renderInline(token.slice(2, -2), ctx, key)}</strong>;
    } else if (m[4]) {
      node = <del key={key}>{renderInline(token.slice(2, -2), ctx, key)}</del>;
    } else if (m[5]) {
      const close = token.indexOf('](');
      const label = token.slice(1, close);
      const url = token.slice(close + 2, -1);
      node = <Link key={key} href={url}>{label}</Link>;
    } else if (m[6]) {
      const url = trimUrl(token);
      // give the trimmed punctuation back to the text that follows
      re.lastIndex = start + url.length;
      token = url;
      node = <Link key={key} href={url}>{url}</Link>;
    } else if (m[7]) {
      node = <em key={key}>{renderInline(token.slice(1, -1), ctx, key)}</em>;
    } else if (m[8]) {
      // snake_case_names must not turn into italics
      if (isWordChar(text[start - 1]) || isWordChar(text[start + token.length])) {
        re.lastIndex = start + 1;
        continue;
      }
      node = <em key={key}>{renderInline(token.slice(1, -1), ctx, key)}</em>;
    } else if (m[9]) {
      const name = token.slice(1);
      if (isWordChar(text[start - 1]) || !ctx.known.has(name.toLowerCase())) {
        re.lastIndex = start + 1; // "@" before a random word stays plain text
        continue;
      }
      const self = name.toLowerCase() === ctx.currentUser.toLowerCase();
      node = (
        <span key={key} className={`md-mention ${self ? 'md-mention-self' : ''}`}>
          @{name}
        </span>
      );
    }

    if (node === null) continue;
    pushText(text.slice(last, start));
    out.push(node);
    last = start + token.length;
  }

  pushText(text.slice(last));
  return out;
}

// ───────────────────────── blocks ─────────────────────────

export type Block =
  | { type: 'code'; lang: string; code: string }
  | { type: 'quote'; lines: string[] }
  | { type: 'ul'; items: string[] }
  | { type: 'ol'; items: string[]; start: number }
  | { type: 'p'; lines: string[] };

const FENCE_RE = /^\s*```([\w+#.-]*)\s*$/;
const QUOTE_RE = /^\s*>(?:\s(.*))?$/;
const UL_RE = /^\s*[-*•]\s+(.*)$/;
const OL_RE = /^\s*(\d{1,9})[.)]\s+(.*)$/;

export function parseBlocks(text: string): Block[] {
  const lines = text.replace(/\r\n?/g, '\n').split('\n');
  const blocks: Block[] = [];
  let i = 0;

  let newPara = true; // the next plain line starts a new paragraph

  const paragraph = (): Block & { type: 'p' } => {
    const last = blocks[blocks.length - 1];
    if (!newPara && last && last.type === 'p') return last;
    const p: Block & { type: 'p' } = { type: 'p', lines: [] };
    blocks.push(p);
    newPara = false;
    return p;
  };

  while (i < lines.length) {
    const line = lines[i];

    const fence = FENCE_RE.exec(line);
    if (fence) {
      // only a fence that is closed later is a code block
      let end = -1;
      for (let j = i + 1; j < lines.length; j++) {
        if (/^\s*```\s*$/.test(lines[j])) {
          end = j;
          break;
        }
      }
      if (end !== -1) {
        newPara = true;
        blocks.push({ type: 'code', lang: fence[1].toLowerCase(), code: lines.slice(i + 1, end).join('\n') });
        i = end + 1;
        continue;
      }
    }

    if (QUOTE_RE.test(line)) {
      const q: string[] = [];
      while (i < lines.length && QUOTE_RE.test(lines[i])) {
        q.push(QUOTE_RE.exec(lines[i])![1] ?? '');
        i++;
      }
      newPara = true;
      blocks.push({ type: 'quote', lines: q });
      continue;
    }

    if (UL_RE.test(line)) {
      const items: string[] = [];
      while (i < lines.length && UL_RE.test(lines[i])) {
        items.push(UL_RE.exec(lines[i])![1]);
        i++;
      }
      newPara = true;
      blocks.push({ type: 'ul', items });
      continue;
    }

    if (OL_RE.test(line)) {
      const first = OL_RE.exec(line)!;
      const items: string[] = [];
      while (i < lines.length && OL_RE.test(lines[i])) {
        items.push(OL_RE.exec(lines[i])![2]);
        i++;
      }
      newPara = true;
      blocks.push({ type: 'ol', items, start: parseInt(first[1], 10) });
      continue;
    }

    if (line.trim() === '') {
      newPara = true;
      i++;
      continue;
    }

    paragraph().lines.push(line);
    i++;
  }

  return blocks.filter(b => !(b.type === 'p' && b.lines.length === 0));
}

// ───────────────────────── code block ─────────────────────────

function copyText(text: string): Promise<void> {
  if (navigator.clipboard && window.isSecureContext) {
    return navigator.clipboard.writeText(text);
  }
  return new Promise((resolve, reject) => {
    try {
      const ta = document.createElement('textarea');
      ta.value = text;
      ta.style.position = 'fixed';
      ta.style.opacity = '0';
      document.body.appendChild(ta);
      ta.select();
      const ok = document.execCommand('copy');
      document.body.removeChild(ta);
      ok ? resolve() : reject(new Error('copy failed'));
    } catch (e) {
      reject(e);
    }
  });
}

function CodeBlock({ lang, code }: { lang: string; code: string }) {
  const [copied, setCopied] = useState(false);

  const onCopy = async () => {
    try {
      await copyText(code);
      setCopied(true);
      setTimeout(() => setCopied(false), 1500);
    } catch {
      /* clipboard blocked: nothing to do */
    }
  };

  return (
    <div className="md-codeblock">
      <div className="md-codeblock-bar">
        <span className="md-codeblock-lang">{lang || 'code'}</span>
        <button type="button" className="md-codeblock-copy" onClick={onCopy}>
          {copied ? 'Copied ✓' : 'Copy'}
        </button>
      </div>
      <pre>
        <code>{code}</code>
      </pre>
    </div>
  );
}

// ───────────────────────── component ─────────────────────────

const withBreaks = (lines: string[], ctx: Ctx, prefix: string): React.ReactNode[] =>
  lines.flatMap((l, idx) => {
    const nodes = renderInline(l, ctx, `${prefix}-${idx}`);
    return idx === 0 ? nodes : [<br key={`${prefix}-br${idx}`} />, ...nodes];
  });

export const MessageText: React.FC<MessageTextProps> = ({ text, knownUsernames = [], currentUser }) => {
  const ctx: Ctx = { known: new Set(knownUsernames.map(u => u.toLowerCase())), currentUser };
  const blocks = parseBlocks(text);

  return (
    <div className="md">
      {blocks.map((b, i) => {
        const key = `b${i}`;
        switch (b.type) {
          case 'code':
            return <CodeBlock key={key} lang={b.lang} code={b.code} />;
          case 'quote':
            return <blockquote key={key} className="md-quote">{withBreaks(b.lines, ctx, key)}</blockquote>;
          case 'ul':
            return (
              <ul key={key} className="md-list">
                {b.items.map((it, j) => <li key={j}>{renderInline(it, ctx, `${key}-${j}`)}</li>)}
              </ul>
            );
          case 'ol':
            return (
              <ol key={key} className="md-list" start={b.start}>
                {b.items.map((it, j) => <li key={j}>{renderInline(it, ctx, `${key}-${j}`)}</li>)}
              </ol>
            );
          default:
            return <div key={key} className="md-p">{withBreaks(b.lines, ctx, key)}</div>;
        }
      })}
    </div>
  );
};

export default MessageText;