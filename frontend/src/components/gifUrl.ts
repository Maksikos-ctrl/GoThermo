// A message that is nothing but a Giphy media link is shown as the picture.
// Only https links on giphy.com hosts ever become an <img>.
export function gifUrlOf(text: string): string | null {
  const t = text.trim();
  if (!t || /\s/.test(t) || t.length > 400) return null;
  let u: URL;
  try {
    u = new URL(t);
  } catch {
    return null;
  }
  const host = u.hostname.toLowerCase();
  const ok = host === 'giphy.com' || host.endsWith('.giphy.com');
  if (u.protocol !== 'https:' || u.username || !ok) return null;
  if (!/\.(gif|webp)$/i.test(u.pathname)) return null;
  return u.toString();
}
