import { useCallback, useEffect, useState } from 'react';

export type Theme = 'dark' | 'light';
export type DefaultStatus = 'online' | 'away';

export interface Settings {
  theme: Theme;
  messageSoundEnabled: boolean;
  ringtoneVolume: number; // 0 to 1
  defaultStatus: DefaultStatus;
}

const DEFAULT_SETTINGS: Settings = {
  theme: 'dark',
  messageSoundEnabled: true,
  ringtoneVolume: 0.5,
  defaultStatus: 'online',
};

function storageKey(username: string) {
  return `gothermo:settings:${username}`;
}

function loadSettings(username: string): Settings {
  if (!username) return DEFAULT_SETTINGS;
  try {
    const raw = localStorage.getItem(storageKey(username));
    if (!raw) return DEFAULT_SETTINGS;
    return { ...DEFAULT_SETTINGS, ...JSON.parse(raw) };
  } catch {
    return DEFAULT_SETTINGS;
  }
}

export function useSettings(username: string) {
  const [settings, setSettings] = useState<Settings>(() => loadSettings(username));

  // Re-load if the logged-in user changes (e.g. different account in the
  // same session).
  useEffect(() => {
    setSettings(loadSettings(username));
  }, [username]);

  useEffect(() => {
    const root = document.documentElement;
    if (settings.theme === 'light') {
      root.setAttribute('data-theme', 'light');
    } else {
      root.removeAttribute('data-theme');
    }
  }, [settings.theme]);

  const updateSettings = useCallback(
    (partial: Partial<Settings>) => {
      setSettings((prev) => {
        const next = { ...prev, ...partial };
        if (username) {
          try {
            localStorage.setItem(storageKey(username), JSON.stringify(next));
          } catch {
            // storage can fail (quota, private mode) - settings just won't
            // persist across restarts, not worth surfacing an error for.
          }
        }
        return next;
      });
    },
    [username]
  );

  return { settings, updateSettings };
}