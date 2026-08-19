import React, { useState, useEffect, useRef } from 'react';
import { Message } from '../types';
import { api } from '../services/api';

interface SearchModalProps {
  isOpen: boolean;
  currentUser: string;
  onClose: () => void;
  onSelectChannel: (channelName: string) => void;
}

export const SearchModal: React.FC<SearchModalProps> = ({
  isOpen,
  currentUser,
  onClose,
  onSelectChannel,
}) => {
  const [query, setQuery] = useState('');
  const [results, setResults] = useState<Message[]>([]);
  const [isSearching, setIsSearching] = useState(false);
  const inputRef = useRef<HTMLInputElement>(null);
  const debounceRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(() => {
    if (isOpen) {
      setTimeout(() => inputRef.current?.focus(), 50);
    } else {
      setQuery('');
      setResults([]);
    }
  }, [isOpen]);

  useEffect(() => {
    if (debounceRef.current) clearTimeout(debounceRef.current);

    if (!query.trim()) {
      setResults([]);
      return;
    }

    setIsSearching(true);
    debounceRef.current = setTimeout(async () => {
      try {
        const found = await api.messages.search(currentUser, query.trim());
        setResults(found || []);
      } catch (error) {
        console.error('Search error:', error);
        setResults([]);
      } finally {
        setIsSearching(false);
      }
    }, 300);

    return () => {
      if (debounceRef.current) clearTimeout(debounceRef.current);
    };
  }, [query, currentUser]);

  const getChannelDisplayName = (channelName: string): string => {
    if (channelName.startsWith('dm_')) {
      const names = channelName.replace('dm_', '').split('_');
      const partner = names.find(n => n !== currentUser) || channelName;
      return `@${partner}`;
    }
    return `#${channelName}`;
  };

  const highlightMatch = (text: string, q: string): React.ReactNode => {
    if (!q.trim()) return text;
    const idx = text.toLowerCase().indexOf(q.toLowerCase());
    if (idx === -1) return text;
    return (
      <>
        {text.substring(0, idx)}
        <mark style={{ background: '#5865f2', color: 'white', borderRadius: '3px', padding: '0 2px' }}>
          {text.substring(idx, idx + q.length)}
        </mark>
        {text.substring(idx + q.length)}
      </>
    );
  };

  const formatTime = (timestamp: string) => {
    const d = new Date(timestamp);
    return d.toLocaleDateString([], { day: '2-digit', month: 'short' }) + ' · ' +
      d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
  };

  if (!isOpen) return null;

  return (
    <div
      style={{
        position: 'fixed',
        inset: 0,
        background: 'rgba(0,0,0,0.5)',
        display: 'flex',
        alignItems: 'flex-start',
        justifyContent: 'center',
        paddingTop: '10vh',
        zIndex: 10000,
      }}
      onClick={onClose}
    >
      <div
        style={{
          background: '#2b2d31',
          borderRadius: '10px',
          width: '560px',
          maxWidth: '90vw',
          maxHeight: '70vh',
          display: 'flex',
          flexDirection: 'column',
          boxShadow: '0 12px 32px rgba(0,0,0,0.4)',
          overflow: 'hidden',
        }}
        onClick={(e) => e.stopPropagation()}
      >
        <div style={{ padding: '14px 16px', borderBottom: '1px solid #3f4147', display: 'flex', alignItems: 'center', gap: '8px' }}>
          <span style={{ color: '#8a8f98' }}>🔍</span>
          <input
            ref={inputRef}
            type="text"
            placeholder="Search messages everywhere..."
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            onKeyDown={(e) => { if (e.key === 'Escape') onClose(); }}
            style={{
              flex: 1,
              background: 'transparent',
              border: 'none',
              outline: 'none',
              color: '#f2f3f5',
              fontSize: '15px',
            }}
          />
          <button
            onClick={onClose}
            style={{ background: 'transparent', border: 'none', color: '#8a8f98', cursor: 'pointer', fontSize: '18px' }}
          >
            ×
          </button>
        </div>

        <div style={{ overflowY: 'auto', flex: 1 }}>
          {isSearching && (
            <div style={{ padding: '20px', textAlign: 'center', color: '#8a8f98', fontSize: '13px' }}>
              Searching...
            </div>
          )}

          {!isSearching && query.trim() && results.length === 0 && (
            <div style={{ padding: '20px', textAlign: 'center', color: '#8a8f98', fontSize: '13px' }}>
              No messages found for "{query}"
            </div>
          )}

          {!isSearching && results.map((msg) => (
            <div
              key={msg.id}
              onClick={() => {
                onSelectChannel(msg.channel);
                onClose();
              }}
              style={{
                padding: '10px 16px',
                borderBottom: '1px solid #313338',
                cursor: 'pointer',
              }}
              onMouseEnter={(e) => (e.currentTarget.style.background = '#35373c')}
              onMouseLeave={(e) => (e.currentTarget.style.background = 'transparent')}
            >
              <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: '2px' }}>
                <span style={{ fontWeight: 700, color: '#f2f3f5', fontSize: '13px' }}>
                  {msg.user} <span style={{ color: '#8a8f98', fontWeight: 400 }}>in {getChannelDisplayName(msg.channel)}</span>
                </span>
                <span style={{ fontSize: '11px', color: '#6d6f78' }}>{formatTime(msg.timestamp)}</span>
              </div>
              <div style={{ fontSize: '14px', color: '#dbdee1' }}>
                {highlightMatch(msg.text, query)}
              </div>
            </div>
          ))}
        </div>
      </div>
    </div>
  );
};