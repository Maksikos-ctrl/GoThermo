import { avatarColor } from './avatarColor';
import { useAvatar } from './avatarStore';

interface Props {
  username: string;
  className?: string;
  title?: string;
}

// A person's picture, or their colored initial when they have none.
export function Avatar({ username, className, title }: Props) {
  const url = useAvatar(username);
  if (url) {
    return (
      <div className={className} title={title} style={{ background: 'none', overflow: 'hidden' }}>
        <img className="avatar-img" src={url} alt={username} />
      </div>
    );
  }
  return (
    <div className={className} title={title} style={{ background: avatarColor(username) }}>
      {username.charAt(0).toUpperCase()}
    </div>
  );
}
