import React, { useCallback, useEffect, useMemo, useState } from 'react';
import './Calendar.css';
import { api } from '../services/api';
import { Channel } from '../types';

export interface CalendarEvent {
  id: string;
  title: string;
  description: string;
  channel: string;
  createdBy: string;
  start: string;
  end: string;
  rsvp: Record<string, string> | null;
  createdAt: string;
}

type Answer = 'going' | 'maybe' | 'declined';

interface CalendarModalProps {
  isOpen: boolean;
  onClose: () => void;
  currentUser: string;
  channels: Channel[];
  currentChannel: string;
  showToast: (message: string, type?: 'error' | 'success') => void;
}

interface FormState {
  id: string | null;
  title: string;
  description: string;
  date: string;
  time: string;
  duration: number;
  channel: string;
}

const pad = (n: number) => String(n).padStart(2, '0');
const ymd = (d: Date) => `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`;
const hm = (d: Date) => `${pad(d.getHours())}:${pad(d.getMinutes())}`;
const addDays = (d: Date, n: number) => new Date(d.getFullYear(), d.getMonth(), d.getDate() + n);
const sameDay = (a: Date, b: Date) => ymd(a) === ymd(b);

// The grid starts on Monday and always shows six weeks.
const gridStart = (month: Date) => {
  const first = new Date(month.getFullYear(), month.getMonth(), 1);
  return addDays(first, -((first.getDay() + 6) % 7));
};

const DURATIONS = [15, 30, 45, 60, 90, 120, 180, 240];

export const CalendarModal: React.FC<CalendarModalProps> = ({
  isOpen,
  onClose,
  currentUser,
  channels,
  currentChannel,
  showToast,
}) => {
  const today = new Date();
  const [month, setMonth] = useState(() => new Date(today.getFullYear(), today.getMonth(), 1));
  const [selected, setSelected] = useState(() => new Date());
  const [events, setEvents] = useState<CalendarEvent[]>([]);
  const [onlyThisChannel, setOnlyThisChannel] = useState(false);
  const [form, setForm] = useState<FormState | null>(null);
  const [busy, setBusy] = useState(false);

  const start = useMemo(() => gridStart(month), [month]);

  const loadEvents = useCallback(async () => {
    try {
      const from = start;
      const to = addDays(start, 42);
      const list = await api.calendar.getEvents(from.toISOString(), to.toISOString());
      setEvents((list || []) as unknown as CalendarEvent[]);
    } catch (error) {
      console.error('Error loading events:', error);
    }
  }, [start]);

  useEffect(() => {
    if (!isOpen) return;
    loadEvents();
    const timer = setInterval(loadEvents, 30000);
    return () => clearInterval(timer);
  }, [isOpen, loadEvents]);

  useEffect(() => {
    if (!isOpen) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose();
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [isOpen, onClose]);

  useEffect(() => {
    if (!isOpen) setForm(null);
  }, [isOpen]);

  const visible = useMemo(
    () => (onlyThisChannel ? events.filter(e => e.channel === currentChannel) : events),
    [events, onlyThisChannel, currentChannel]
  );

  const byDay = useMemo(() => {
    const map = new Map<string, CalendarEvent[]>();
    visible.forEach(ev => {
      const key = ymd(new Date(ev.start));
      map.set(key, [...(map.get(key) || []), ev]);
    });
    return map;
  }, [visible]);

  if (!isOpen) return null;

  const days = Array.from({ length: 42 }, (_, i) => addDays(start, i));
  const weekdayNames = days.slice(0, 7).map(d => d.toLocaleDateString(undefined, { weekday: 'short' }));
  const monthTitle = month.toLocaleDateString(undefined, { month: 'long', year: 'numeric' });
  const dayEvents = byDay.get(ymd(selected)) || [];
  const channelByName = (name: string) => channels.find(c => c.name === name);

  const goToMonth = (delta: number) =>
    setMonth(m => new Date(m.getFullYear(), m.getMonth() + delta, 1));

  const goToday = () => {
    const now = new Date();
    setMonth(new Date(now.getFullYear(), now.getMonth(), 1));
    setSelected(now);
  };

  const openNewForm = () => {
    const nextHour = new Date();
    nextHour.setMinutes(0, 0, 0);
    nextHour.setHours(nextHour.getHours() + 1);
    const isToday = sameDay(selected, new Date());
    const defaultChannel = channelByName(currentChannel) ? currentChannel : channels[0]?.name || '';
    setForm({
      id: null,
      title: '',
      description: '',
      date: ymd(selected),
      time: isToday ? hm(nextHour) : '09:00',
      duration: 60,
      channel: defaultChannel,
    });
  };

  const openEditForm = (ev: CalendarEvent) => {
    const s = new Date(ev.start);
    const minutes = Math.max(5, Math.round((new Date(ev.end).getTime() - s.getTime()) / 60000));
    setForm({
      id: ev.id,
      title: ev.title,
      description: ev.description,
      date: ymd(s),
      time: hm(s),
      duration: minutes,
      channel: ev.channel,
    });
  };

  const submitForm = async () => {
    if (!form) return;
    if (!form.title.trim()) {
      showToast('Please enter a title', 'error');
      return;
    }
    if (!form.channel) {
      showToast('Please choose a channel', 'error');
      return;
    }
    const startDate = new Date(`${form.date}T${form.time}`);
    if (isNaN(startDate.getTime())) {
      showToast('Please enter a valid date and time', 'error');
      return;
    }
    setBusy(true);
    try {
      if (form.id) {
        await api.calendar.update(
          form.id, currentUser, form.title, form.description, startDate.toISOString(), form.duration
        );
      } else {
        await api.calendar.create(
          form.channel, form.title, form.description, startDate.toISOString(), form.duration
        );
      }
      showToast(form.id ? 'Event updated' : 'Event created', 'success');
      setSelected(startDate);
      setMonth(new Date(startDate.getFullYear(), startDate.getMonth(), 1));
      setForm(null);
      await loadEvents();
    } catch (error: any) {
      showToast(`${error}`, 'error');
    } finally {
      setBusy(false);
    }
  };

  const respond = async (ev: CalendarEvent, answer: Answer) => {
    const current = (ev.rsvp || {})[currentUser];
    try {
      await api.calendar.respond(ev.id, currentUser, current === answer ? '' : answer);
      await loadEvents();
    } catch (error: any) {
      showToast(`${error}`, 'error');
    }
  };

  const remove = async (ev: CalendarEvent) => {
    if (!window.confirm(`Delete "${ev.title}"?`)) return;
    try {
      await api.calendar.delete(ev.id, currentUser);
      await loadEvents();
    } catch (error: any) {
      showToast(`${error}`, 'error');
    }
  };

  const people = (ev: CalendarEvent, answer: Answer) =>
    Object.entries(ev.rsvp || {})
      .filter(([, a]) => a === answer)
      .map(([u]) => u);

  const timeRange = (ev: CalendarEvent) =>
    `${hm(new Date(ev.start))}–${hm(new Date(ev.end))}`;

  return (
    <div className="cal-overlay" onMouseDown={onClose}>
      <div className="cal-modal" onMouseDown={e => e.stopPropagation()}>
        <div className="cal-header">
          <div className="cal-nav">
            <button className="cal-icon-btn" onClick={() => goToMonth(-1)} title="Previous month">‹</button>
            <h3 className="cal-title">{monthTitle}</h3>
            <button className="cal-icon-btn" onClick={() => goToMonth(1)} title="Next month">›</button>
            <button className="cal-today-btn" onClick={goToday}>Today</button>
          </div>
          <div className="cal-filters">
            <button
              className={`cal-chip ${!onlyThisChannel ? 'active' : ''}`}
              onClick={() => setOnlyThisChannel(false)}
            >
              All channels
            </button>
            <button
              className={`cal-chip ${onlyThisChannel ? 'active' : ''}`}
              onClick={() => setOnlyThisChannel(true)}
            >
              #{currentChannel}
            </button>
          </div>
          <button className="cal-icon-btn" onClick={onClose} title="Close">✕</button>
        </div>

        <div className="cal-body">
          <div className="cal-grid-wrap">
            <div className="cal-grid cal-weekdays">
              {weekdayNames.map(n => (
                <div key={n} className="cal-weekday">{n}</div>
              ))}
            </div>
            <div className="cal-grid">
              {days.map(day => {
                const list = byDay.get(ymd(day)) || [];
                const classes = [
                  'cal-cell',
                  day.getMonth() !== month.getMonth() ? 'other-month' : '',
                  sameDay(day, new Date()) ? 'today' : '',
                  sameDay(day, selected) ? 'selected' : '',
                ].join(' ');
                return (
                  <div key={ymd(day)} className={classes} onClick={() => setSelected(day)}>
                    <span className="cal-daynum">{day.getDate()}</span>
                    {list.slice(0, 2).map(ev => (
                      <div key={ev.id} className="cal-pill" title={ev.title}>
                        {hm(new Date(ev.start))} {ev.title}
                      </div>
                    ))}
                    {list.length > 2 && <div className="cal-more">+{list.length - 2} more</div>}
                  </div>
                );
              })}
            </div>
          </div>

          <div className="cal-side">
            {form ? (
              <div className="cal-form">
                <h4>{form.id ? 'Edit event' : 'New event'}</h4>
                <label>Title</label>
                <input
                  className="cal-input"
                  value={form.title}
                  maxLength={100}
                  autoFocus
                  placeholder="Team sync"
                  onChange={e => setForm({ ...form, title: e.target.value })}
                />
                <div className="cal-row">
                  <div>
                    <label>Date</label>
                    <input
                      type="date"
                      className="cal-input"
                      value={form.date}
                      onChange={e => setForm({ ...form, date: e.target.value })}
                    />
                  </div>
                  <div>
                    <label>Time</label>
                    <input
                      type="time"
                      className="cal-input"
                      value={form.time}
                      onChange={e => setForm({ ...form, time: e.target.value })}
                    />
                  </div>
                </div>
                <div className="cal-row">
                  <div>
                    <label>Duration</label>
                    <select
                      className="cal-input"
                      value={form.duration}
                      onChange={e => setForm({ ...form, duration: Number(e.target.value) })}
                    >
                      {(DURATIONS.includes(form.duration) ? DURATIONS : [...DURATIONS, form.duration].sort((a, b) => a - b)).map(m => (
                        <option key={m} value={m}>
                          {m >= 60 && m % 60 === 0 ? `${m / 60} h` : `${m} min`}
                        </option>
                      ))}
                    </select>
                  </div>
                  <div>
                    <label>Channel</label>
                    <select
                      className="cal-input"
                      value={form.channel}
                      disabled={!!form.id}
                      onChange={e => setForm({ ...form, channel: e.target.value })}
                    >
                      {channels.map(c => (
                        <option key={c.name} value={c.name}>
                          {c.isPrivate ? '🔒 ' : '# '}{c.name}
                        </option>
                      ))}
                    </select>
                  </div>
                </div>
                <label>Description (optional)</label>
                <textarea
                  className="cal-input"
                  rows={3}
                  maxLength={500}
                  value={form.description}
                  onChange={e => setForm({ ...form, description: e.target.value })}
                />
                <div className="cal-form-actions">
                  <button className="modal-cancel-btn" onClick={() => setForm(null)}>Cancel</button>
                  <button className="modal-create-btn" onClick={submitForm} disabled={busy}>
                    {form.id ? 'Save' : 'Create event'}
                  </button>
                </div>
              </div>
            ) : (
              <>
                <div className="cal-side-head">
                  <h4>
                    {selected.toLocaleDateString(undefined, { weekday: 'long', day: 'numeric', month: 'long' })}
                  </h4>
                  <button className="modal-create-btn" onClick={openNewForm} disabled={channels.length === 0}>
                    + New event
                  </button>
                </div>
                {dayEvents.length === 0 ? (
                  <div className="cal-empty">No events this day</div>
                ) : (
                  dayEvents.map(ev => {
                    const mine = (ev.rsvp || {})[currentUser];
                    const ch = channelByName(ev.channel);
                    return (
                      <div key={ev.id} className="cal-event">
                        <div className="cal-event-time">{timeRange(ev)}</div>
                        <div className="cal-event-title">{ev.title}</div>
                        <div className="cal-event-meta">
                          {ch?.isPrivate ? '🔒' : '#'}{ev.channel} · by {ev.createdBy}
                        </div>
                        {ev.description && <div className="cal-event-desc">{ev.description}</div>}
                        <div className="cal-event-people">
                          ✅ {people(ev, 'going').join(', ') || '—'}
                          {people(ev, 'maybe').length > 0 && <> · 🤔 {people(ev, 'maybe').join(', ')}</>}
                          {people(ev, 'declined').length > 0 && <> · ❌ {people(ev, 'declined').join(', ')}</>}
                        </div>
                        <div className="cal-event-actions">
                          {(['going', 'maybe', 'declined'] as Answer[]).map(a => (
                            <button
                              key={a}
                              className={`cal-rsvp ${mine === a ? 'active' : ''}`}
                              onClick={() => respond(ev, a)}
                            >
                              {a === 'going' ? 'Going' : a === 'maybe' ? 'Maybe' : "Can't go"}
                            </button>
                          ))}
                          {ev.createdBy === currentUser && (
                            <>
                              <button className="cal-link-btn" onClick={() => openEditForm(ev)}>Edit</button>
                              <button className="cal-link-btn danger" onClick={() => remove(ev)}>Delete</button>
                            </>
                          )}
                        </div>
                      </div>
                    );
                  })
                )}
              </>
            )}
          </div>
        </div>
      </div>
    </div>
  );
};
