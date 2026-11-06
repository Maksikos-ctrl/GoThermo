import { isWeb } from './platform';
import { Notify } from './transport';

// Message notifications: the browser's Notification API on the web, the
// system notifier (through the Go side) in the desktop app. Plus an unread
// count in the window title, which works in both.

const BASE_TITLE = 'GoThermo';
let unseen = 0;

export function windowIsAway(): boolean {
  return document.hidden || !document.hasFocus();
}

export function titleFor(count: number): string {
  return count > 0 ? `(${count > 99 ? '99+' : count}) ${BASE_TITLE}` : BASE_TITLE;
}

export function addUnseen() {
  unseen += 1;
  document.title = titleFor(unseen);
}

export function clearUnseen() {
  if (unseen === 0 && document.title === BASE_TITLE) return;
  unseen = 0;
  document.title = BASE_TITLE;
}

export function notificationsSupported(): boolean {
  return !isWeb() || typeof Notification !== 'undefined';
}

// Browsers only allow asking from a click, so this is called from the
// settings toggle. The desktop app needs no permission.
export async function askNotificationPermission(): Promise<boolean> {
  if (!isWeb()) return true;
  if (typeof Notification === 'undefined') return false;
  if (Notification.permission === 'granted') return true;
  if (Notification.permission === 'denied') return false;
  try {
    return (await Notification.requestPermission()) === 'granted';
  } catch {
    return false;
  }
}

export function showNotice(title: string, body: string, tag: string, onClick?: () => void) {
  try {
    if (!isWeb()) {
      void Promise.resolve(Notify(title, body)).catch(() => {});
      return;
    }
    if (typeof Notification === 'undefined' || Notification.permission !== 'granted') return;
    const n = new Notification(title, { body, tag, icon: './icon-192.png' });
    n.onclick = () => {
      window.focus();
      onClick?.();
      n.close();
    };
  } catch {
    // a notification that cannot show is not worth breaking the chat for
  }
}
