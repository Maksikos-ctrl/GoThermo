// Run from the project root:  node apply-pins.mjs
// Makes small, checked edits to existing files. Safe to run twice.
import { readFileSync, writeFileSync } from 'node:fs';

let failed = false;

function patch(file, edits) {
  let s;
  try {
    s = readFileSync(file, 'utf8');
  } catch {
    console.log(`❌ ${file}: file not found`);
    failed = true;
    return;
  }
  for (const { name, find, replace, done } of edits) {
    if (s.includes(done)) continue; // already applied
    const first = s.indexOf(find);
    if (first === -1 || s.indexOf(find, first + 1) !== -1) {
      console.log(`❌ ${file}: could not place "${name}" (${first === -1 ? 'not found' : 'found more than once'})`);
      failed = true;
      return;
    }
    s = s.replace(find, () => replace);
  }
  writeFileSync(file, s);
  console.log(`✅ ${file}`);
}

// ───────── app.go (backend): clean up pins together with their message / channel ─────────
patch('app.go', [
  {
    name: 'deleting a message removes its pin',
    find: '\tdeleteThread(channel, messageID)\n',
    replace:
      '\tdeleteThread(channel, messageID)\n' +
      '\tif removed, _ := removePin(channel, messageID); removed {\n' +
      '\t\ta.announcePins(channel)\n' +
      '\t}\n',
    done: 'removePin(channel, messageID)',
  },
  {
    name: 'deleting a channel removes its pins',
    find: '\tdeleteChannelEvents(name)\n',
    replace: '\tdeleteChannelEvents(name)\n\tdeleteChannelPins(name)\n',
    done: 'deleteChannelPins(name)',
  },
]);

// ───────── services/api.ts ─────────
patch('frontend/src/services/api.ts', [
  {
    name: 'imports',
    find: '  AddReplyReaction,\n} from',
    replace: '  AddReplyReaction,\n  PinMessage,\n  UnpinMessage,\n  GetPinnedMessages,\n} from',
    done: 'GetPinnedMessages,',
  },
  {
    name: 'api.pins',
    find: '  files: {\n',
    replace:
      '  pins: {\n' +
      '    get: (channel: string) => GetPinnedMessages(t(), channel),\n' +
      '    pin: (channel: string, messageId: string, username: string) => PinMessage(t(), channel, messageId, username),\n' +
      '    unpin: (channel: string, messageId: string, username: string) => UnpinMessage(t(), channel, messageId, username),\n' +
      '  },\n' +
      '  files: {\n',
    done: '  pins: {',
  },
]);

// ───────── services/transport.ts (server mode) ─────────
patch('frontend/src/services/transport.ts', [
  {
    name: 'bind pins',
    find: "export const AddReplyReaction = bind('AddReplyReaction');\n",
    replace:
      "export const AddReplyReaction = bind('AddReplyReaction');\n" +
      "export const PinMessage = bind('PinMessage');\n" +
      "export const UnpinMessage = bind('UnpinMessage');\n" +
      "export const GetPinnedMessages = bind('GetPinnedMessages');\n",
    done: "bind('GetPinnedMessages')",
  },
]);

// ───────── hooks/useWebSocket.ts ─────────
patch('frontend/src/hooks/useWebSocket.ts', [
  {
    name: 'parameter',
    find: '  onThreadReplyDeleted?: (channel: string, threadId: string, messageId: string) => void\n) => {',
    replace:
      '  onThreadReplyDeleted?: (channel: string, threadId: string, messageId: string) => void,\n' +
      '  onPinsChanged?: (channel: string, pins: any[]) => void\n) => {',
    done: 'onPinsChanged?:',
  },
  {
    name: 'ref',
    find: '  const onThreadReplyDeletedRef = useRef(onThreadReplyDeleted);\n',
    replace:
      '  const onThreadReplyDeletedRef = useRef(onThreadReplyDeleted);\n' +
      '  const onPinsChangedRef = useRef(onPinsChanged);\n',
    done: 'const onPinsChangedRef',
  },
  {
    name: 'ref update',
    find: '  onThreadReplyDeletedRef.current = onThreadReplyDeleted;\n',
    replace:
      '  onThreadReplyDeletedRef.current = onThreadReplyDeleted;\n' +
      '  onPinsChangedRef.current = onPinsChanged;\n',
    done: 'onPinsChangedRef.current =',
  },
  {
    name: 'event',
    find: "      case 'channel_created':",
    replace:
      "      case 'pins_changed':\n" +
      '        if (onPinsChangedRef.current) {\n' +
      '          onPinsChangedRef.current(data.payload.channel, data.payload.pins || []);\n' +
      '        }\n' +
      '        break;\n\n' +
      "      case 'channel_created':",
    done: "case 'pins_changed'",
  },
]);

// ───────── App.tsx ─────────
const PIN_FUNCTIONS = `  // ───────────── pinned messages ─────────────

  async function loadPins(channel: string) {
    setPinsLoading(true);
    setPinsError(null);
    try {
      const raw = await api.pins.get(channel);
      if (channel !== currentChannelRef.current) return; // switched while loading
      setPins(normalizePins(raw));
    } catch (error) {
      if (channel === currentChannelRef.current) setPinsError(\`Could not load pinned messages: \${error}\`);
    } finally {
      if (channel === currentChannelRef.current) setPinsLoading(false);
    }
  }

  // The server sends the whole list whenever a pin is added or removed.
  function handlePinsChanged(channel: string, list: any[]) {
    if (channel !== currentChannelRef.current) return;
    setPins(normalizePins(list));
  }

  async function handleTogglePin(message: Message) {
    const channel = currentChannel;
    try {
      if (isPinned(pins, message.id)) {
        await api.pins.unpin(channel, message.id, currentUser);
      } else {
        await api.pins.pin(channel, message.id, currentUser);
      }
      void loadPins(channel); // do not rely on the live event alone
    } catch (error) {
      showToast(\`\${error}\`, 'error');
    }
  }

  function handleTogglePins() {
    if (showPins) {
      setShowPins(false);
      return;
    }
    closeThread(); // one side panel at a time
    setShowPins(true);
  }

  // A new channel has its own pins; the panel closes when you leave.
  useEffect(() => {
    setPins([]);
    setPinsError(null);
    setShowPins(false);
    if (isLoggedIn) void loadPins(currentChannel);
  }, [currentChannel, isLoggedIn]);

  // After a dropped connection pins may have changed: load them again.
  useEffect(() => {
    if (isConnected && isLoggedIn) void loadPins(currentChannelRef.current);
  }, [isConnected]);

`;

patch('frontend/src/App.tsx', [
  {
    name: 'imports',
    find: "import { ThreadPanel } from './components/ThreadPanel';\n",
    replace:
      "import { ThreadPanel } from './components/ThreadPanel';\n" +
      "import { PinnedPanel } from './components/PinnedPanel';\n" +
      "import { PinnedMessage, applyMessageToPins, removeMessageFromPins, isPinned, normalizePins, pinnedIdSet } from './components/pinState';\n",
    done: "from './components/PinnedPanel'",
  },
  {
    name: 'css',
    find: "import './styles/thread.css';\n",
    replace: "import './styles/thread.css';\nimport './styles/pins.css';\n",
    done: "import './styles/pins.css'",
  },
  {
    name: 'state',
    find: '  openThreadRef.current = openThread;\n',
    replace:
      '  openThreadRef.current = openThread;\n\n' +
      '  // Pinned messages of the open channel.\n' +
      '  const [pins, setPins] = useState<PinnedMessage[]>([]);\n' +
      '  const [showPins, setShowPins] = useState(false);\n' +
      '  const [pinsLoading, setPinsLoading] = useState(false);\n' +
      '  const [pinsError, setPinsError] = useState<string | null>(null);\n',
    done: 'const [showPins, setShowPins]',
  },
  {
    name: 'websocket handler',
    find: '    handleThreadReplyEvent,\n    handleThreadReplyDeleted\n  );',
    replace: '    handleThreadReplyEvent,\n    handleThreadReplyDeleted,\n    handlePinsChanged\n  );',
    done: '    handlePinsChanged\n  );',
  },
  {
    name: 'new message updates its pin',
    find: '      setThreadData(prev => applyParentUpdate(prev, message));\n',
    replace:
      '      setThreadData(prev => applyParentUpdate(prev, message));\n' +
      '      setPins(prev => applyMessageToPins(prev, message));\n',
    done: 'applyMessageToPins(prev, message)',
  },
  {
    name: 'deleted message loses its pin',
    find: '    setMessages(prev => prev.filter(m => m.id !== messageId));\n    if (openThreadRef.current?.parentId === messageId) {',
    replace:
      '    setMessages(prev => prev.filter(m => m.id !== messageId));\n' +
      '    setPins(prev => removeMessageFromPins(prev, messageId));\n' +
      '    if (openThreadRef.current?.parentId === messageId) {',
    done: 'removeMessageFromPins(prev, messageId)',
  },
  {
    name: 'pin functions',
    find: '  // ───────────── threads ─────────────\n',
    replace: PIN_FUNCTIONS + '  // ───────────── threads ─────────────\n',
    done: 'async function loadPins(',
  },
  {
    name: 'opening a thread closes pins',
    find: '    setOpenThread(target);\n',
    replace: '    setOpenThread(target);\n    setShowPins(false); // one side panel at a time\n',
    done: 'setShowPins(false); // one side panel at a time',
  },
  {
    name: 'logout clears pins',
    find: '    setMessages([]);\n    closeThread();\n',
    replace: '    setMessages([]);\n    closeThread();\n    setPins([]);\n    setShowPins(false);\n',
    done: '    closeThread();\n    setPins([]);',
  },
  {
    name: 'header props',
    find: '          onStartGroupCall={() => setShowStartGroupCallModal(true)}\n          currentUser={currentUser}\n        />',
    replace:
      '          onStartGroupCall={() => setShowStartGroupCallModal(true)}\n          currentUser={currentUser}\n' +
      '          pinnedCount={pins.length}\n          pinsOpen={showPins}\n          onTogglePins={handleTogglePins}\n        />',
    done: 'onTogglePins={handleTogglePins}',
  },
  {
    name: 'list props',
    find: '          onOpenThread={handleOpenThread}\n        />',
    replace:
      '          onOpenThread={handleOpenThread}\n          pinnedIds={pinnedIdSet(pins)}\n          onTogglePin={handleTogglePin}\n        />',
    done: 'onTogglePin={handleTogglePin}',
  },
  {
    name: 'panel',
    find: '          onDeleteMessage={handleThreadDelete}\n        />\n      )}\n',
    replace:
      '          onDeleteMessage={handleThreadDelete}\n        />\n      )}\n\n' +
      '      {showPins && (\n' +
      '        <PinnedPanel\n' +
      '          pins={pins}\n' +
      '          loading={pinsLoading}\n' +
      '          error={pinsError}\n' +
      '          channelTag={channelTag(currentChannel, currentUser)}\n' +
      '          currentUser={currentUser}\n' +
      '          knownUsernames={allUsernames}\n' +
      '          onClose={() => setShowPins(false)}\n' +
      '          onUnpin={handleTogglePin}\n' +
      '          onAddReaction={handleAddReaction}\n' +
      '          onEditMessage={handleEditMessage}\n' +
      '          onDeleteMessage={handleDeleteMessage}\n' +
      '        />\n' +
      '      )}\n',
    done: '<PinnedPanel',
  },
]);

if (failed) {
  console.log('\nSome edits could not be applied. Nothing was changed in those files; send me the messages above.');
  process.exit(1);
}
console.log('\nAll edits applied.');
