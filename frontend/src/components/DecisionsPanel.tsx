import React, { useCallback, useEffect, useState } from 'react';
import { CheckSquare, Square, Trash2, X } from 'lucide-react';
import { api } from '../services/api';
import {
  DecisionEntry,
  DecisionFilter,
  canRemoveDecision,
  decisionMeta,
  filterDecisions,
  normalizeDecisions,
} from './decisionState';
import '../styles/decisions.css';

interface DecisionsPanelProps {
  channel: string;
  channelTag: string;
  currentUser: string;
  // Changes when something was marked elsewhere, so the list reloads.
  refreshKey: number;
  onClose: () => void;
  onCountChange?: (openTasks: number, total: number) => void;
}

const FILTERS: { id: DecisionFilter; label: string }[] = [
  { id: 'all', label: 'All' },
  { id: 'decisions', label: 'Decisions' },
  { id: 'open', label: 'Open tasks' },
  { id: 'done', label: 'Done' },
];

// The decisions and tasks of the open channel, shown on the right.
export const DecisionsPanel: React.FC<DecisionsPanelProps> = ({
  channel,
  channelTag,
  currentUser,
  refreshKey,
  onClose,
  onCountChange,
}) => {
  const [list, setList] = useState<DecisionEntry[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [filter, setFilter] = useState<DecisionFilter>('all');

  const load = useCallback(async () => {
    try {
      const items = normalizeDecisions(await api.decisions.get(channel));
      setList(items);
      setError(null);
    } catch (e) {
      setError(`Could not load the list: ${e}`);
    } finally {
      setLoading(false);
    }
  }, [channel]);

  useEffect(() => {
    setLoading(true);
    setList([]);
    void load();
  }, [load, refreshKey]);

  useEffect(() => {
    onCountChange?.(list.filter(d => d.kind === 'task' && !d.done).length, list.length);
  }, [list, onCountChange]);

  const toggleDone = async (d: DecisionEntry) => {
    try {
      await api.decisions.setDone(channel, d.messageId, currentUser, !d.done);
    } catch (e) {
      setError(`${e}`);
    }
    void load();
  };

  const remove = async (d: DecisionEntry) => {
    try {
      await api.decisions.remove(channel, d.messageId, currentUser);
    } catch (e) {
      setError(`${e}`);
    }
    void load();
  };

  const shown = filterDecisions(list, filter);

  return (
    <aside className="thread-panel decisions-panel" aria-label="Decisions and tasks">
      <header className="thread-header">
        <div className="thread-title">
          <strong>Decisions</strong>
          <span className="thread-channel">{channelTag}</span>
        </div>
        <button type="button" className="thread-close" onClick={onClose} aria-label="Close decisions" title="Close">
          <X size={16} />
        </button>
      </header>

      <div className="decisions-filters" role="tablist">
        {FILTERS.map(f => (
          <button
            key={f.id}
            type="button"
            role="tab"
            aria-selected={filter === f.id}
            className={`decisions-filter${filter === f.id ? ' active' : ''}`}
            onClick={() => setFilter(f.id)}
          >
            {f.label}
          </button>
        ))}
      </div>

      <div className="thread-body">
        {error && <div className="thread-error" role="alert">{error}</div>}
        {!error && shown.length === 0 && (
          <div className="pins-empty">
            {loading ? (
              'Loading…'
            ) : list.length === 0 ? (
              <>
                <strong>Nothing here yet</strong>
                <span>Hover a message and choose the checklist button to mark it as a decision or a task.</span>
              </>
            ) : (
              <span>Nothing in this view.</span>
            )}
          </div>
        )}
        {shown.map(d => (
          <section className={`decision-item${d.done ? ' done' : ''}`} key={d.messageId}>
            <div className="decision-head">
              {d.kind === 'task' ? (
                <button
                  type="button"
                  className="decision-check"
                  onClick={() => toggleDone(d)}
                  aria-label={d.done ? 'Mark task as open' : 'Mark task as done'}
                  aria-pressed={d.done}
                >
                  {d.done ? <CheckSquare size={16} /> : <Square size={16} />}
                </button>
              ) : null}
              <span className={`decision-kind ${d.kind}`}>{d.kind === 'task' ? 'Task' : 'Decision'}</span>
              {canRemoveDecision(d, currentUser) && (
                <button
                  type="button"
                  className="decision-remove"
                  onClick={() => remove(d)}
                  title="Remove from the list"
                  aria-label={`Remove ${d.kind} by ${d.author}`}
                >
                  <Trash2 size={13} />
                </button>
              )}
            </div>
            <div className="decision-text">{d.text}</div>
            <div className="decision-meta">{decisionMeta(d)}</div>
          </section>
        ))}
      </div>
    </aside>
  );
};
