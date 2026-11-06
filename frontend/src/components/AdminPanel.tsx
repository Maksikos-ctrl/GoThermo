import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { api } from '../services/api';
import logoUrl from '../assets/logo.svg';
import '../styles/admin.css';

interface Stats { users: number; online: number; banned: number; reports: number }
interface AdminUser {
  username: string;
  email: string;
  provider?: string;
  status: string;
  lastSeen?: string;
  hasAvatar: boolean;
  banned: boolean;
}
interface Report {
  id: string;
  reporter: string;
  author: string;
  channel: string;
  messageId: string;
  reason: string;
  snippet: string;
  fileName?: string;
  at: string;
}

type Tab = 'overview' | 'users' | 'reports';

const ago = (iso?: string) => {
  if (!iso) return '—';
  const t = new Date(iso).getTime();
  if (Number.isNaN(t)) return '—';
  const m = Math.round((Date.now() - t) / 60000);
  if (m < 1) return 'just now';
  if (m < 60) return `${m} min ago`;
  if (m < 60 * 24) return `${Math.round(m / 60)} h ago`;
  return new Date(t).toLocaleDateString();
};

interface Props {
  onLogout: () => void;
}

// What the admin sees instead of the chat. Everything here is checked again
// on the server, so this screen is only a view.
export const AdminPanel: React.FC<Props> = ({ onLogout }) => {
  const [tab, setTab] = useState<Tab>('overview');
  const [stats, setStats] = useState<Stats | null>(null);
  const [users, setUsers] = useState<AdminUser[]>([]);
  const [reports, setReports] = useState<Report[]>([]);
  const [query, setQuery] = useState('');
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);

  const load = useCallback(async () => {
    try {
      const [s, u, r] = await Promise.all([api.admin.overview(), api.admin.users(), api.admin.reports()]);
      setStats(s as Stats);
      setUsers((u || []) as AdminUser[]);
      setReports((r || []) as Report[]);
      setError('');
    } catch (e: any) {
      setError(String(e?.message ?? e));
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  const act = async (fn: () => Promise<unknown>) => {
    setBusy(true);
    try {
      await fn();
      await load();
    } catch (e: any) {
      setError(String(e?.message ?? e));
    } finally {
      setBusy(false);
    }
  };

  const shown = useMemo(() => {
    const q = query.trim().toLowerCase();
    return q ? users.filter(u => u.username.toLowerCase().includes(q) || (u.email || '').toLowerCase().includes(q)) : users;
  }, [users, query]);

  const isBanned = useMemo(() => new Set(users.filter(u => u.banned).map(u => u.username.toLowerCase())), [users]);

  return (
    <div className="admin">
      <header className="admin-top">
        <div className="admin-brand">
          <img src={logoUrl} alt="" width={28} height={28} />
          <span>GoThermo admin</span>
        </div>
        <nav className="admin-tabs" role="tablist">
          {(['overview', 'users', 'reports'] as Tab[]).map(t => (
            <button
              key={t}
              role="tab"
              aria-selected={tab === t}
              className={'admin-tab' + (tab === t ? ' on' : '')}
              onClick={() => setTab(t)}
            >
              {t === 'overview' ? 'Overview' : t === 'users' ? 'Users' : 'Reports'}
              {t === 'reports' && reports.length > 0 && <span className="admin-pill">{reports.length}</span>}
            </button>
          ))}
        </nav>
        <div className="admin-actions">
          <button className="admin-btn" onClick={() => void load()} disabled={busy}>Refresh</button>
          <button className="admin-btn" onClick={onLogout}>Sign out</button>
        </div>
      </header>

      <main className="admin-main">
        {error && <div className="admin-error" role="alert">{error}</div>}

        {tab === 'overview' && (
          <section>
            <h2>Overview</h2>
            <div className="admin-cards">
              <div className="admin-card"><b>{stats?.users ?? '…'}</b><span>Users</span></div>
              <div className="admin-card"><b>{stats?.online ?? '…'}</b><span>Online now</span></div>
              <div className="admin-card"><b>{stats?.reports ?? '…'}</b><span>Open reports</span></div>
              <div className="admin-card"><b>{stats?.banned ?? '…'}</b><span>Banned</span></div>
            </div>
            <p className="admin-hint">
              Reports come from the Report button on messages. Review them in the Reports tab, then remove the
              message, ban the author, or dismiss the report.
            </p>
          </section>
        )}

        {tab === 'users' && (
          <section>
            <h2>Users <span className="admin-count">{users.length}</span></h2>
            <input
              className="admin-search"
              placeholder="Search by name or email"
              aria-label="Search users"
              value={query}
              onChange={e => setQuery(e.target.value)}
            />
            <div className="admin-table" role="table">
              <div className="admin-row admin-head" role="row">
                <span>User</span><span>Email</span><span>Status</span><span>Last seen</span><span />
              </div>
              {shown.map(u => (
                <div className="admin-row" role="row" key={u.username}>
                  <span className="admin-name">
                    {u.username}
                    {u.provider === 'google' && <em className="admin-tag">Google</em>}
                    {u.banned && <em className="admin-tag bad">Banned</em>}
                  </span>
                  <span className="admin-dim">{u.email || '—'}</span>
                  <span><i className={`admin-dot ${u.status}`} />{u.status}</span>
                  <span className="admin-dim">{ago(u.lastSeen)}</span>
                  <span className="admin-end">
                    {u.banned ? (
                      <button className="admin-btn" disabled={busy} onClick={() => act(() => api.admin.unban(u.username))}>Unban</button>
                    ) : (
                      <button
                        className="admin-btn danger"
                        disabled={busy}
                        onClick={() => {
                          if (window.confirm(`Ban ${u.username}? They will be signed out and unable to sign in.`)) {
                            void act(() => api.admin.ban(u.username));
                          }
                        }}
                      >
                        Ban
                      </button>
                    )}
                  </span>
                </div>
              ))}
              {shown.length === 0 && <div className="admin-empty">No users found.</div>}
            </div>
          </section>
        )}

        {tab === 'reports' && (
          <section>
            <h2>Reports <span className="admin-count">{reports.length}</span></h2>
            {reports.length === 0 && <div className="admin-empty">Nothing to review. 🎉</div>}
            {reports.map(r => (
              <article className="admin-report" key={r.id}>
                <div className="admin-report-head">
                  <span className={`admin-reason ${r.reason}`}>{r.reason}</span>
                  <span>
                    <b>{r.author}</b> in #{r.channel} · reported by {r.reporter} · {ago(r.at)}
                  </span>
                </div>
                {r.fileName && <div className="admin-dim">File: {r.fileName}</div>}
                {r.snippet && <blockquote className="admin-quote">{r.snippet}</blockquote>}
                <div className="admin-report-actions">
                  <button className="admin-btn danger" disabled={busy} onClick={() => act(() => api.admin.deleteReported(r.id))}>
                    Delete message
                  </button>
                  {isBanned.has(r.author.toLowerCase()) ? (
                    <button className="admin-btn" disabled={busy} onClick={() => act(() => api.admin.unban(r.author))}>Unban author</button>
                  ) : (
                    <button className="admin-btn danger" disabled={busy} onClick={() => act(() => api.admin.ban(r.author))}>Ban author</button>
                  )}
                  <button className="admin-btn" disabled={busy} onClick={() => act(() => api.admin.dismissReport(r.id))}>Dismiss</button>
                </div>
              </article>
            ))}
          </section>
        )}
      </main>
    </div>
  );
};
