// One stable color per username, used for avatars everywhere.
const COLORS = [
  '#5865f2', '#ed4245', '#2da160', '#c79a12',
  '#d6409f', '#e8603c', '#4f7fd9', '#9b59b6',
];

export function avatarColor(username: string): string {
  let hash = 0;
  for (let i = 0; i < username.length; i++) {
    hash = username.charCodeAt(i) + ((hash << 5) - hash);
  }
  return COLORS[Math.abs(hash) % COLORS.length];
}
