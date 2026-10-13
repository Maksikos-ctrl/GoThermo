import React, { useEffect, useState } from 'react';
import { api } from '../services/api';
import { BrowserOpenURL } from '../../wailsjs/runtime/runtime';

interface Preview {
  url: string;
  title: string;
  description: string;
  siteName: string;
  image: string;
}

const URL_RE = /https?:\/\/[^\s<>"'`]+/i;

const count = (s: string, ch: string) => s.split(ch).length - 1;

// First http(s) link in a text, without the punctuation that usually trails
// it ("see https://x.com/a." / "(https://x.com/a)"). A closing ")" is kept
// when the link itself contains the opening one, as in Wikipedia URLs.
export const extractFirstUrl = (text: string): string | null => {
  const match = text.match(URL_RE);
  if (!match) return null;

  let url = match[0];
  for (;;) {
    const last = url.slice(-1);
    if (/[.,;:!?\]}]/.test(last)) {
      url = url.slice(0, -1);
    } else if (last === ')' && count(url, ')') > count(url, '(')) {
      url = url.slice(0, -1);
    } else {
      break;
    }
  }
  return url.length > 'https://x'.length ? url : null;
};

// One request per link for the whole session, however often the message
// is re-rendered (the backend caches too).
const cache = new Map<string, Promise<Preview | null>>();

const loadPreview = (url: string): Promise<Preview | null> => {
  let pending = cache.get(url);
  if (!pending) {
    pending = api.links
      .getPreview(url)
      .then((p) => p as Preview)
      .catch(() => null);
    cache.set(url, pending);
  }
  return pending;
};

interface LinkPreviewProps {
  text: string;
}

export const LinkPreview: React.FC<LinkPreviewProps> = ({ text }) => {
  const url = extractFirstUrl(text);
  const [preview, setPreview] = useState<Preview | null>(null);

  useEffect(() => {
    let cancelled = false;
    setPreview(null);
    if (url) {
      loadPreview(url).then((p) => {
        if (!cancelled) setPreview(p);
      });
    }
    return () => {
      cancelled = true;
    };
  }, [url]);

  if (!preview) return null;

  const open = () => BrowserOpenURL(preview.url);

  return (
    <div
      role="link"
      tabIndex={0}
      onClick={open}
      onKeyDown={(e) => {
        if (e.key === 'Enter') open();
      }}
      style={{
        display: 'flex',
        gap: '12px',
        marginTop: '6px',
        padding: '10px 12px',
        maxWidth: '460px',
        borderLeft: '3px solid #667eea',
        borderRadius: '6px',
        background: 'rgba(127, 127, 127, 0.12)',
        cursor: 'pointer',
        overflow: 'hidden',
      }}
    >
      {preview.image && (
        <img
          src={preview.image}
          alt=""
          style={{ width: '72px', height: '72px', objectFit: 'cover', borderRadius: '4px', flexShrink: 0 }}
        />
      )}
      <div style={{ minWidth: 0 }}>
        <div style={{ fontSize: '11px', opacity: 0.65, marginBottom: '2px' }}>{preview.siteName}</div>
        <div style={{ fontSize: '14px', fontWeight: 600, lineHeight: 1.3, wordBreak: 'break-word' }}>
          {preview.title || preview.url}
        </div>
        {preview.description && (
          <div style={{ fontSize: '12px', opacity: 0.8, marginTop: '3px', lineHeight: 1.35, wordBreak: 'break-word' }}>
            {preview.description}
          </div>
        )}
      </div>
    </div>
  );
};
