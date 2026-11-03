import * as wails from '../../wailsjs/go/main/App';
import { isWeb } from './platform';

export function downloadText(filename: string, content: string): string {
  const blob = new Blob([content], { type: 'text/plain;charset=utf-8' });
  const url = URL.createObjectURL(blob);
  const link = document.createElement('a');
  link.href = url;
  link.download = filename;
  document.body.appendChild(link);
  link.click();
  document.body.removeChild(link);
  setTimeout(() => URL.revokeObjectURL(url), 5000);
  return filename;
}

export function saveText(filename: string, content: string): Promise<string> {
  if (isWeb()) return Promise.resolve(downloadText(filename, content));
  return wails.SaveTextFile(filename, content);
}

export function openExternal(url: string): void {
  if (!isWeb()) {
    const rt = (window as any).runtime;
    if (rt && typeof rt.BrowserOpenURL === 'function') {
      rt.BrowserOpenURL(url);
      return;
    }
  }
  window.open(url, '_blank', 'noopener,noreferrer');
}
