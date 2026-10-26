import { useState } from 'react';
import { getInviteCode, getServerUrl, parseServerInput, pingServer, setInviteCode, setServerUrl } from '../services/server';
import '../styles/server.css';

// Small control on the sign-in screen: use the backend inside this app
// (default) or connect to a shared GoThermo server.
export function ServerPicker() {
  const [current, setCurrent] = useState(getServerUrl());
  const [open, setOpen] = useState(false);
  const [text, setText] = useState(current);
  const [invite, setInvite] = useState(getInviteCode());
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);

  const label = current ? current.replace(/^https?:\/\//, '') : 'this computer';

  const connect = async () => {
    const parsed = parseServerInput(text);
    if (parsed.error !== undefined) {
      setError(parsed.error);
      return;
    }
    if (!parsed.url) {
      useLocal();
      return;
    }
    setBusy(true);
    setError('');
    const problem = await pingServer(parsed.url);
    setBusy(false);
    if (problem) {
      setError(problem);
      return;
    }
    setServerUrl(parsed.url);
    setInviteCode(invite);
    setCurrent(parsed.url);
    setText(parsed.url);
    setOpen(false);
  };

  const useLocal = () => {
    setServerUrl('');
    setCurrent('');
    setText('');
    setError('');
    setOpen(false);
  };

  return (
    <div className="server-picker">
      {!open ? (
        <button type="button" className="server-chip" onClick={() => setOpen(true)} aria-label="Change server">
          <span className={`server-dot ${current ? 'remote' : ''}`} />
          Server: {label}
          <span className="server-edit">Change</span>
        </button>
      ) : (
        <form
          className="server-form"
          onSubmit={(e) => {
            e.preventDefault();
            void connect();
          }}
        >
          <label htmlFor="server-url">Server address</label>
          <input
            id="server-url"
            type="text"
            value={text}
            placeholder="chat.example.com"
            autoFocus
            spellCheck={false}
            autoCapitalize="off"
            onChange={(e) => setText(e.target.value)}
          />
          <label htmlFor="server-invite">Invite code (only needed to register)</label>
          <input
            id="server-invite"
            type="text"
            value={invite}
            placeholder="optional"
            spellCheck={false}
            autoCapitalize="off"
            onChange={(e) => setInvite(e.target.value)}
          />
          {error && (
            <div className="server-error" role="alert">
              {error}
            </div>
          )}
          <div className="server-actions">
            <button type="submit" className="server-btn primary" disabled={busy}>
              {busy ? 'Checking…' : 'Connect'}
            </button>
            <button type="button" className="server-btn" onClick={useLocal}>
              Use this computer
            </button>
            <button type="button" className="server-btn" onClick={() => setOpen(false)}>
              Cancel
            </button>
          </div>
        </form>
      )}
    </div>
  );
}
