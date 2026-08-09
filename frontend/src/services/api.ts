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
  GetDMChannels,
  DeleteDMChannel,     
  MarkChannelRead,     
  GetUnreadCounts,     
} from '../../wailsjs/go/main/App';

export const api = {
  auth: {
    login: Login,
    register: Register
  },
  users: {
    getAll: GetUsers,
    updateStatus: UpdateUserStatus,
  },
  channels: {
    getAll: GetChannels,
    create: CreateChannel,
    delete: DeleteChannel,
    join: JoinChannel,
  },
  dm: {
    getOrCreate: GetOrCreateDMChannel,
    getAll: GetDMChannels,
    delete: DeleteDMChannel, 
  },

  unread: {
    markRead: MarkChannelRead,
    getCounts: GetUnreadCounts,
  },
  messages: {
    getByChannel: GetMessages,
    send: SendMessage,
    sendPost: SendPost,
    addReaction: AddReaction,
  },
};