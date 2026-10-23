import {
  SendMessage,
  SendPost,
  GetMessages,
  Login,
  Register,
  AddReaction,
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
} from '../../wailsjs/go/main/App';
import { getAuthToken } from './session';


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
    create: (name: string, description: string, createdBy: string) =>
      CreateChannel(t(), name, description, createdBy),
    delete: (name: string, username: string) => DeleteChannel(t(), name, username),
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
  calls: {
    log: (channel: string, initiator: string, callStatus: string, durationSeconds: number) =>
      LogCall(t(), channel, initiator, callStatus, durationSeconds),
  },
};