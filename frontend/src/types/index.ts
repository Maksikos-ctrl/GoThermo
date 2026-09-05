export interface Message {
  id: string;
  user: string;
  text: string;
  channel: string;
  timestamp: string;
  reactions?: { [emoji: string]: string[] };
  isPost?: boolean;
  isEdited?: boolean;
  isFile?: boolean;     
  fileId?: string;     
  fileName?: string;    
  fileSize?: number;    
  mimeType?: string;    
  isCall?: boolean;
  callStatus?: string;
  callDuration?: number;
  callInitiator?: string;
}


export interface Channel {
  id: string;
  name: string;
  description: string;
  members: string[];
  createdBy: string;
  createdAt: string;
  isPrivate: boolean;
  order?: number;
}

export interface User {
  id: string;
  username: string;
  email: string;
  isOnline: boolean;
  lastSeen?: string;
  status?: 'online' | 'away' | 'offline';
}

export type StatusType = 'online' | 'away' | 'offline';

export const EMOJI_LIST = ['👍', '❤️', '😂', '🎉', '🚀', '👏', '🔥', '💯'] as const;

export const STATUS_OPTIONS = [
  { value: 'online', label: 'Online', color: '#3ba55d' },
  { value: 'away', label: 'Away', color: '#faa81a' },
  { value: 'offline', label: 'Offline', color: '#747f8d' }
] as const;