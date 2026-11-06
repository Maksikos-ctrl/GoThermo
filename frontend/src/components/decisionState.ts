// Decisions and tasks of a channel (see decisions.go on the server).

export interface DecisionEntry {
  messageId: string;
  channel: string;
  kind: 'decision' | 'task';
  text: string;
  author: string;
  markedBy: string;
  assignee?: string;
  done: boolean;
  doneBy?: string;
  at: string;
}

export type DecisionFilter = 'all' | 'decisions' | 'open' | 'done';

export function normalizeDecisions(raw: unknown): DecisionEntry[] {
  if (!Array.isArray(raw)) return [];
  return raw.filter(
    (d): d is DecisionEntry =>
      !!d && typeof d.messageId === 'string' && (d.kind === 'decision' || d.kind === 'task')
  );
}

export function filterDecisions(list: DecisionEntry[], filter: DecisionFilter): DecisionEntry[] {
  switch (filter) {
    case 'decisions':
      return list.filter(d => d.kind === 'decision');
    case 'open':
      return list.filter(d => d.kind === 'task' && !d.done);
    case 'done':
      return list.filter(d => d.kind === 'task' && d.done);
    default:
      return list;
  }
}

export const openTaskCount = (list: DecisionEntry[]): number =>
  list.filter(d => d.kind === 'task' && !d.done).length;

// Only the one who marked it or the author of the message may take it off.
export const canRemoveDecision = (d: DecisionEntry, user: string): boolean =>
  d.markedBy === user || d.author === user;

export function decisionMeta(d: DecisionEntry): string {
  const parts = [`by ${d.author}`];
  if (d.markedBy !== d.author) parts.push(`marked by ${d.markedBy}`);
  if (d.kind === 'task' && d.assignee) parts.push(`for @${d.assignee}`);
  if (d.done && d.doneBy) parts.push(`done by ${d.doneBy}`);
  return parts.join(' · ');
}
