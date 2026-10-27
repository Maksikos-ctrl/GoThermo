import * as wails from '../../wailsjs/go/main/App';
import { getInviteCode, getServerUrl } from './server';

// Every backend call goes through here. With no server address it is the
// Wails binding, exactly as before. With one, the same call is sent to the
// server as POST /rpc/<Method> with the arguments as a JSON array.

type Fn = (...args: any[]) => Promise<any>;

// These open dialogs or a browser on the machine that runs the backend.
const DESKTOP_ONLY = new Set(['SaveFileToDisk']);

export async function rpc(name: string, args: unknown[]): Promise<any> {
  const base = getServerUrl();
  const headers: Record<string, string> = { 'Content-Type': 'application/json' };
  const invite = getInviteCode();
  if (invite && (name === 'Register' || name === 'LoginWithGoogleToken')) headers['X-Invite-Code'] = invite;
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
export const GetMentionCounts = bind('GetMentionCounts');
export const EditMessage = bind('EditMessage');
export const DeleteMessage = bind('DeleteMessage');
export const SearchMessages = bind('SearchMessages');
export const SendFile = bind('SendFile');
export const GetFileData = bind('GetFileData');
export const SaveFileToDisk = bind('SaveFileToDisk');
export const LogCall = bind('LogCall');
export const Logout = bind('Logout');
// Google sign-in. On a server the browser part still runs here, on this
// computer; the server then checks the token with Google and signs the user in.
export const LoginWithGoogle = ((signUp: boolean) => {
  if (!getServerUrl()) return wails.LoginWithGoogle(signUp);
  return wails.GoogleAccessToken().then((token: string) => rpc('LoginWithGoogleToken', [token, signUp]));
}) as typeof wails.LoginWithGoogle;

// Cancelling only closes the local browser wait, so it is always local.
export const CancelGoogleLogin = ((...args: unknown[]) =>
  (wails.CancelGoogleLogin as unknown as Fn)(...args)) as typeof wails.CancelGoogleLogin;
// Export over a server: the server builds the file, and it is saved on this
// computer through the usual save dialog. Resolves to the saved path, or ""
// when the dialog was closed.
export const ExportChannel = ((token: string, channel: string, username: string, format: string) => {
  if (!getServerUrl()) return wails.ExportChannel(token, channel, username, format);
  return rpc('BuildChannelExport', [token, channel, username, format]).then(
    (f: { filename: string; content: string }) => wails.SaveTextFile(f.filename, f.content),
  );
}) as typeof wails.ExportChannel;
export const GetLinkPreview = bind('GetLinkPreview');
export const SendReply = bind('SendReply');
export const GetThread = bind('GetThread');
export const EditReply = bind('EditReply');
export const DeleteReply = bind('DeleteReply');
export const AddReplyReaction = bind('AddReplyReaction');
export const PinMessage = bind('PinMessage');
export const UnpinMessage = bind('UnpinMessage');
export const GetPinnedMessages = bind('GetPinnedMessages');
export const GetThreadUnread = bind('GetThreadUnread');
export const MarkThreadRead = bind('MarkThreadRead');
export const BuildThreadExport = bind('BuildThreadExport');

// Export one thread: built by the backend (local or server), saved on this
// computer through the save dialog. Resolves to the saved path, or "" when the
// dialog was closed.
export const ExportThread = (token: string, channel: string, parentId: string, format: string): Promise<string> =>
  (BuildThreadExport as unknown as Fn)(token, channel, parentId, format).then(
    (f: { filename: string; content: string }) => wails.SaveTextFile(f.filename, f.content),
  );
