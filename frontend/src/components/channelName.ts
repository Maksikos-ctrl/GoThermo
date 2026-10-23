// How a channel name is shown to people.
// Direct messages are stored as "dm_<a>_<b>"; the user sees "@<other person>".
export function channelLabel(
  channel: string,
  currentUser?: string
): { isDM: boolean; text: string } {
  if (channel.startsWith('dm_group_')) return { isDM: true, text: 'Group chat' };
  if (channel.startsWith('dm_')) {
    const other = channel
      .replace('dm_', '')
      .split('_')
      .find(n => n !== currentUser);
    return { isDM: true, text: other || channel };
  }
  return { isDM: false, text: channel };
}

// "#general" for channels, "@maksikos" for direct messages.
export function channelTag(channel: string, currentUser?: string): string {
  const { isDM, text } = channelLabel(channel, currentUser);
  return (isDM ? '@' : '#') + text;
}
