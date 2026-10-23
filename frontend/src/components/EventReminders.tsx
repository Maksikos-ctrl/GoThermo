import React from 'react';
import { CalendarEvent } from './CalendarModal';

interface EventRemindersProps {
  reminders: CalendarEvent[];
  onDismiss: (id: string) => void;
  onOpenCalendar: () => void;
}

// Cards in the corner for events that are about to start. They stay until
// dismissed, so a reminder is not missed if the window is in the background.
export const EventReminders: React.FC<EventRemindersProps> = ({
  reminders,
  onDismiss,
  onOpenCalendar,
}) => {
  if (reminders.length === 0) return null;

  const minutesLeft = (ev: CalendarEvent) =>
    Math.max(0, Math.round((new Date(ev.start).getTime() - Date.now()) / 60000));

  return (
    <div className="event-reminders">
      {reminders.map(ev => {
        const mins = minutesLeft(ev);
        return (
          <div key={ev.id} className="event-reminder">
            <div className="event-reminder-title">📅 {ev.title}</div>
            <div className="event-reminder-sub">
              {mins === 0 ? 'Starting now' : `Starts in ${mins} min`} · #{ev.channel}
            </div>
            <div className="event-reminder-actions">
              <button
                className="cal-link-btn"
                onClick={() => {
                  onOpenCalendar();
                  onDismiss(ev.id);
                }}
              >
                Open calendar
              </button>
              <button className="cal-link-btn" onClick={() => onDismiss(ev.id)}>
                Dismiss
              </button>
            </div>
          </div>
        );
      })}
    </div>
  );
};
