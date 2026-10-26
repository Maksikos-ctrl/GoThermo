import * as wails from '../../wailsjs/go/main/App';
import { getInviteCode, getServerUrl } from './server';

// Every backend call goes through here. With no server address it is the
// Wails binding, exactly as before. With one, the same call is sent to the
// server as POST /rpc/<Method> with the arguments as a JSON array.

type Fn = (...args: any[]) => Promise<any>;

// These open dialogs or a browser on the machine that runs the backend.
const DESKTOP_ONLY = new Set(['SaveFileToDisk', 'ExportChannel', 'LoginWithGoogle', 'CancelGoogleLogin']);

export async function rpc(name: string, args: unknown[]): Promise<any> {
  const base = getServerUrl();
  const headers: Record<string, string> = { 'Content-Type': 'application/json' };
  const invite = getInviteCode();
  if (name === 'Register' && invite) headers['X-Invite-Code'] = invite;
  let res: Response;
  try {
    res = await fetch(`${base}/rpc/${name}`, {
      method: 'POST',
      headers,
      body: JSON.stringify(args),
    });
  } catch {
    throw `Cannot reach the server at ${base}`;
  }
  let body: { result?: unknown; error?: string } = {};
  try {
    body = await res.json();
  } catch {
    throw `Unexpected answer from the server (${res.status})`;
  }
  // Wails rejects with the error text, so do the same: callers already
  // handle that shape.
  if (body.error) throw body.error;
  if (!res.ok) throw `Server error (${res.status})`;
  return body.result ?? null;
}

function bind<K extends keyof typeof wails>(name: K): (typeof wails)[K] {
  const call: Fn = (...args) => {
    if (!getServerUrl()) return (wails[name] as unknown as Fn)(...args);
    if (DESKTOP_ONLY.has(name as string)) {
      return Promise.reject('This is not available when connected to a server yet.');
    }
    return rpc(name as string, args);
  };
  return call as unknown as (typeof wails)[K];
}

export const SendMessage = bind('SendMessage');
export const SendPost = bind('SendPost');
export const GetMessages = bind('GetMessages');
export const GetMessagesPage = bind('GetMessagesPage');
export const Login = bind('Login');
export const Register = bind('Register');
export const AddReaction = bind('AddReaction');
export const CreateEvent = bind('CreateEvent');
export const GetEvents = bind('GetEvents');
export const RespondToEvent = bind('RespondToEvent');
export const UpdateEvent = bind('UpdateEvent');
export const DeleteEvent = bind('DeleteEvent');
export const GetDueReminders = bind('GetDueReminders');
export const AddChannelMember = bind('AddChannelMember');
export const LeaveChannel = bind('LeaveChannel');
export const CreateChannel = bind('CreateChannel');
export const GetChannels = bind('GetChannels');
export const DeleteChannel = bind('DeleteChannel');
export const JoinChannel = bind('JoinChannel');
export const GetUsers = bind('GetUsers');
export const UpdateUserStatus = bind('UpdateUserStatus');
export const GetOrCreateDMChannel = bind('GetOrCreateDMChannel');
export const GetOrCreateGroupChannel = bind('GetOrCreateGroupChannel');
export const GetDMChannels = bind('GetDMChannels');
export const DeleteDMChannel = bind('DeleteDMChannel');
export const MarkChannelRead = bind('MarkChannelRead');
export const GetUnreadCounts = bind('GetUnreadCounts');
export const EditMessage = bind('EditMessage');
export const DeleteMessage = bind('DeleteMessage');
export const SearchMessages = bind('SearchMessages');
export const SendFile = bind('SendFile');
export const GetFileData = bind('GetFileData');
export const SaveFileToDisk = bind('SaveFileToDisk');
export const LogCall = bind('LogCall');
export const Logout = bind('Logout');
export const LoginWithGoogle = bind('LoginWithGoogle');
export const CancelGoogleLogin = bind('CancelGoogleLogin');
export const ExportChannel = bind('ExportChannel');
export const GetLinkPreview = bind('GetLinkPreview');
export const SendReply = bind('SendReply');
export const GetThread = bind('GetThread');
export const EditReply = bind('EditReply');
export const DeleteReply = bind('DeleteReply');
export const AddReplyReaction = bind('AddReplyReaction');
