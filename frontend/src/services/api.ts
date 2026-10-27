import {
  SendMessage,
  SendPost,
  GetMessages,
  GetMessagesPage,
  Login,
  Register,
  AddReaction,
  CreateEvent,
  GetEvents,
  RespondToEvent,
  UpdateEvent,
  DeleteEvent,
  GetDueReminders,
  AddChannelMember,
  LeaveChannel,
  CreateChannel,
  GetChannels,
  DeleteChannel,
  JoinChannel,
  GetUsers,
  UpdateUserStatus,
  GetOrCreateDMChannel,
  GetOrCreateGroupChannel,
  GetDMChannels,
  DeleteDMChannel,
  MarkChannelRead,
  GetUnreadCounts,
  EditMessage,
  DeleteMessage,
  SearchMessages,
  SendFile,
  GetFileData,
  SaveFileToDisk,
  LogCall,
  Logout,
  LoginWithGoogle,
  CancelGoogleLogin,
  ExportChannel,
  GetLinkPreview,
  SendReply,
  GetThread,
  EditReply,
  DeleteReply,
  AddReplyReaction,
  PinMessage,
  UnpinMessage,
  GetPinnedMessages,
} from './transport';
import { getAuthToken } from './session';

// The backend checks the session token on every call. The token is added
// here, so components keep calling api.* exactly as before.
const t = () => getAuthToken();

export const api = {
  auth: {
    login: Login,
    register: Register,
    logout: Logout,
    loginWithGoogle: LoginWithGoogle,
    cancelGoogleLogin: CancelGoogleLogin,
  },
  users: {
    getAll: () => GetUsers(t()),
    updateStatus: (username: string, status: string) => UpdateUserStatus(t(), username, status),
  },
  channels: {
    getAll: () => GetChannels(t()),
    create: (
      name: string,
      description: string,
      createdBy: string,
      isPrivate: boolean = false,
      members: string[] = []
    ) => CreateChannel(t(), name, description, createdBy, isPrivate, members),
    addMember: (channelName: string, caller: string, newMember: string) =>
      AddChannelMember(t(), channelName, caller, newMember),
    delete: (name: string, username: string) => DeleteChannel(t(), name, username),
    leave: (name: string, username: string) => LeaveChannel(t(), name, username),
    join: (channelName: string, username: string) => JoinChannel(t(), channelName, username),
  },
  dm: {
    getOrCreate: (user1: string, user2: string) => GetOrCreateDMChannel(t(), user1, user2),
    getOrCreateGroup: (usernames: string[], createdBy: string) =>
      GetOrCreateGroupChannel(t(), usernames, createdBy),
    getAll: (username: string) => GetDMChannels(t(), username),
    delete: (channelName: string, username: string) => DeleteDMChannel(t(), channelName, username),
  },
  unread: {
    markRead: (username: string, channel: string) => MarkChannelRead(t(), username, channel),
    getCounts: (username: string) => GetUnreadCounts(t(), username),
  },
  messages: {
    getByChannel: (channel: string) => GetMessages(t(), channel),
    // beforeID '' = newest page, otherwise the page right before that message
    getPage: (channel: string, beforeID: string, limit: number) =>
      GetMessagesPage(t(), channel, beforeID, limit),
    send: (user: string, text: string, channel: string) => SendMessage(t(), user, text, channel),
    sendPost: (user: string, text: string, channel: string) => SendPost(t(), user, text, channel),
    addReaction: (messageId: string, emoji: string, username: string, channel: string) =>
      AddReaction(t(), messageId, emoji, username, channel),
    edit: (messageId: string, channel: string, username: string, newText: string) =>
      EditMessage(t(), messageId, channel, username, newText),
    delete: (messageId: string, channel: string, username: string) =>
      DeleteMessage(t(), messageId, channel, username),
    search: (username: string, query: string) => SearchMessages(t(), username, query),
  },
  threads: {
    reply: (user: string, channel: string, parentId: string, text: string) =>
      SendReply(t(), user, channel, parentId, text),
    get: (channel: string, parentId: string) => GetThread(t(), channel, parentId),
    edit: (channel: string, parentId: string, replyId: string, username: string, newText: string) =>
      EditReply(t(), channel, parentId, replyId, username, newText),
    delete: (channel: string, parentId: string, replyId: string, username: string) =>
      DeleteReply(t(), channel, parentId, replyId, username),
    addReaction: (channel: string, parentId: string, replyId: string, emoji: string, username: string) =>
      AddReplyReaction(t(), channel, parentId, replyId, emoji, username),
  },
  pins: {
    get: (channel: string) => GetPinnedMessages(t(), channel),
    pin: (channel: string, messageId: string, username: string) => PinMessage(t(), channel, messageId, username),
    unpin: (channel: string, messageId: string, username: string) => UnpinMessage(t(), channel, messageId, username),
  },
  files: {
    send: (user: string, channel: string, fileName: string, mimeType: string, base64Data: string) =>
      SendFile(t(), user, channel, fileName, mimeType, base64Data),
    getData: (fileId: string) => GetFileData(t(), fileId),
    saveToDisk: (fileId: string) => SaveFileToDisk(t(), fileId),
    exportChannel: (channel: string, username: string, format: string) =>
      ExportChannel(t(), channel, username, format),
  },
  links: {
    getPreview: (url: string) => GetLinkPreview(t(), url),
  },
  calendar: {
    getEvents: (fromISO: string, toISO: string) => GetEvents(t(), fromISO, toISO),
    create: (channel: string, title: string, description: string, startISO: string, durationMin: number) =>
      CreateEvent(t(), channel, title, description, startISO, durationMin),
    update: (eventId: string, username: string, title: string, description: string, startISO: string, durationMin: number) =>
      UpdateEvent(t(), eventId, username, title, description, startISO, durationMin),
    respond: (eventId: string, username: string, status: string) =>
      RespondToEvent(t(), eventId, username, status),
    delete: (eventId: string, username: string) => DeleteEvent(t(), eventId, username),
    getDueReminders: () => GetDueReminders(t()),
  },
  calls: {
    log: (channel: string, initiator: string, callStatus: string, durationSeconds: number) =>
      LogCall(t(), channel, initiator, callStatus, durationSeconds),
  },
};