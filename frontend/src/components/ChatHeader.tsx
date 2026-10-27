import React from 'react';
import { Channel } from '../types';
import { Pin, Search, CalendarDays, Users, UsersRound, Phone, Video, Lock, Hash, AtSign } from 'lucide-react';
import { ExportMenu } from './ExportMenu';
import { channelLabel } from './channelName';

interface ChatHeaderProps {
  currentChannel: string;
  channels: Channel[];
  messagesCount: number;
  onStartVideoCall: () => void;
  onStartAudioCall: () => void;
  onShowMembers?: () => void;
  onOpenSearch?: () => void;
  onOpenCalendar?: () => void;
  onStartGroupCall?: () => void;
  currentUser?: string;
  // Pins: shows the pin button with the number of pinned messages.
  pinnedCount?: number;
  pinsOpen?: boolean;
  onTogglePins?: () => void;
}

export const ChatHeader: React.FC<ChatHeaderProps> = ({
  currentChannel,
  channels,
  messagesCount,
  onStartVideoCall,
  onStartAudioCall,
  onShowMembers,
  onOpenSearch, 
  onOpenCalendar,
  onStartGroupCall,
  currentUser,
  pinnedCount = 0,
  pinsOpen = false,
  onTogglePins,
}) => {
  const channel = channels.find(ch => ch.name === currentChannel);

  // What to show as the title: a DM is "@person", a private channel gets a lock.
  const { isDM, text: title } = channelLabel(currentChannel, currentUser);
  const TitleIcon = isDM ? AtSign : channel?.isPrivate ? Lock : Hash;

  return (
    <div className="chat-header">
      <div className="channel-info">
        <div className="channel-title">
          <h2>
            <TitleIcon size={20} strokeWidth={2} className="title-icon" />
            {title}
          </h2>
          {channel?.description && (
            <span className="channel-description-badge">{channel.description}</span>
          )}
        </div>
        <div className="channel-stats">
          <span className="messages-count">{messagesCount} messages</span>
        </div>
      </div>
      
      <div className="header-actions">
        
        <button
          className="search-btn hdr-btn"
          onClick={onOpenSearch}
          title="Search messages"
          aria-label="Search messages"
        >
          <Search size={18} />
        </button>
        {onTogglePins && (
          <button
            className={`search-btn hdr-btn pins-btn ${pinsOpen ? 'active-icon' : ''}`}
            onClick={onTogglePins}
            title={pinnedCount > 0 ? `Pinned messages (${pinnedCount})` : 'Pinned messages'}
            aria-label="Pinned messages"
            aria-pressed={pinsOpen}
          >
            <Pin size={18} />
            {pinnedCount > 0 && <span className="pins-btn-badge">{pinnedCount}</span>}
          </button>
        )}
        {onOpenCalendar && (
          <button
            className="search-btn hdr-btn"
            onClick={onOpenCalendar}
            title="Calendar"
            aria-label="Calendar"
          >
            <CalendarDays size={18} />
          </button>
        )}
        {currentUser && (
          <ExportMenu channel={currentChannel} username={currentUser} />
        )}
        <button 
          className="members-btn hdr-btn" 
          onClick={onShowMembers}
          title="Show channel members"
          aria-label="Show channel members"
        >
          <Users size={18} />
        </button>
        <button 
          className="call-btn audio-call-btn hdr-btn" 
          onClick={onStartAudioCall} 
          title="Voice call"
          aria-label="Voice call"
        >
          <Phone size={18} />
        </button>
        {onStartGroupCall && (
          <button
            className="call-btn group-call-btn hdr-btn"
            onClick={onStartGroupCall}
            title="Start a group call"
            aria-label="Start a group call"
          >
            <span className="icon-combo">
              <UsersRound size={18} />
              <Phone size={9} strokeWidth={3} className="icon-combo-badge" />
            </span>
          </button>
        )}
        <button 
          className="call-btn video-call-btn hdr-btn" 
          onClick={onStartVideoCall} 
          title="Video call"
          aria-label="Video call"
        >
          <Video size={18} />
        </button>
      </div>
    </div>
  );
};
