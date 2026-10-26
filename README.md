# GoThermo

[![CI](https://github.com/Maksikos-ctrl/GoThermo/actions/workflows/ci.yml/badge.svg)](https://github.com/Maksikos-ctrl/GoThermo/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go](https://img.shields.io/badge/Go-00ADD8?logo=go&logoColor=white)](https://golang.org/)
[![Wails](https://img.shields.io/badge/Wails-DF0024?logo=wails&logoColor=white)](https://wails.io/)
[![React](https://img.shields.io/badge/React-20232A?logo=react&logoColor=61DAFB)](https://reactjs.org/)
[![TypeScript](https://img.shields.io/badge/TypeScript-007ACC?logo=typescript&logoColor=white)](https://www.typescriptlang.org/)

GoThermo is a real-time team chat desktop app in the spirit of Slack and Teams. The backend is Go, the UI is React + TypeScript, they are glued together with [Wails v2](https://wails.io/), and everything is stored in Redis.

> Status: actively developed, tested on macOS. Windows and Linux builds are produced by GitHub Actions but have not been tested by hand yet.

**Website:** https://maksikos-ctrl.github.io/GoThermo/ · **Downloads:** [latest release](https://github.com/Maksikos-ctrl/GoThermo/releases/latest)

## Download

Grab the build for your system from the [Releases page](https://github.com/Maksikos-ctrl/GoThermo/releases/latest):

| System | File |
|---|---|
| Windows | `GoThermo-windows-amd64.zip` |
| macOS | `GoThermo-macos.zip` |
| Linux | `GoThermo-linux-amd64.tar.gz` |

Things to know before you run it:
- Each copy talks to a **Redis on `localhost:6379`** and its own WebSocket on `127.0.0.1:8081`. Install and start Redis first (`brew install redis`, `apt install redis-server` or `docker run -d -p 6379:6379 redis`).
- By default a copy only chats with itself and with other windows on the same machine. To share one workspace between computers, run a GoThermo server and connect the apps to it (see [Server mode](#server-mode)).
- The builds are not code-signed. Windows SmartScreen: More info, then Run anyway. macOS: right-click the app, then Open.

## Features

**Messaging**
- Public channels, **private channels** (invite-only, hidden from non-members) and direct messages
- Real-time delivery over WebSocket, typing indicators, online / away / DND presence (auto-away when idle)
- **Threads**: reply to any message in a side panel; the parent shows the reply count, avatars and the time of the last reply, and everything updates live
- Markdown and code blocks (with a copy button), @mentions
- Edit and delete your messages, emoji reactions, "Post" messages, unread badges per channel
- Message search, link previews, export of a channel to JSON / CSV / TXT
- File sharing: file picker, **drag & drop** and **paste from clipboard** (screenshots and images from other apps), 25 MB limit

**Calls**
- 1:1 audio and video calls (WebRTC)
- Group calls (mesh WebRTC); in a private channel the call goes to the channel members
- Call history is written into the chat

**Team calendar**
- Events belong to a channel and are visible only to people who can see that channel
- RSVP: Going / Maybe / Can't go, with a list of who answered what
- Reminder 15 minutes before the start for everyone who answered Going or Maybe
- Month view, edit and delete (creator only), a chat message is posted when an event is created

**Accounts and security**
- Email + password registration (the email domain is checked via DNS) and **Sign in with Google** (OAuth 2.0 with PKCE)
- Passwords are hashed with bcrypt, sessions are random 256-bit tokens that expire after 30 days
- Every backend method checks the session token, the WebSocket is authenticated too
- Brute-force protection: 5 wrong passwords lock the email for 15 minutes
- Private channel events are delivered only to members, never by subscription alone
- Upload limits and content-type sniffing, SSRF protection in link previews

## Server mode

One backend on a server, many desktop apps connected to it. The same Go code runs headless (`go build -tags server`), the app sends its calls to `POST /rpc/<Method>` and receives live updates over `wss://`. Setup with Docker, HTTPS included: [SERVER.md](SERVER.md).

On the sign-in screen click **Server: this computer, Change**, type your server address and press **Connect**. Leave it empty to keep using the local mode.

## Tech stack

| Layer | What |
| --- | --- |
| Desktop shell | Wails v2 |
| Backend | Go, gorilla/websocket, go-redis |
| Frontend | React, TypeScript, Vite, plain CSS |
| Storage | Redis (AOF + RDB persistence) |
| Real-time | WebSocket hub on `127.0.0.1:8081`, WebRTC for calls |
| Tests | `go test` with miniredis (never touches a real Redis), GitHub Actions on macOS |

## Project structure

```
GoThermo/
├── main.go                  # Wails entry point
├── app.go                   # App methods exposed to the frontend (messages, channels, files, calls)
├── auth.go                  # Register, login, logout
├── sessions.go              # Session tokens in Redis
├── authz.go                 # Token → user, private channel access checks
├── ratelimit.go             # Login brute-force protection
├── google_oauth.go          # Sign in with Google
├── emailcheck.go            # Email DNS validation
├── users.go                 # Users, in-memory cache loaded from Redis
├── privatechannels.go       # Private channel members and visibility
├── events.go                # Team calendar and reminders
├── export.go                # Channel export (json / csv / txt)
├── linkpreview.go           # Link previews with SSRF guard and Redis cache
├── maintenance.go           # Cleanup tasks (deleted-message compaction, etc.)
├── websocket.go             # WebSocket hub and delivery rules
├── redis.go                 # Redis client
├── models.go                # Data models
├── *_test.go                # Go tests (miniredis)
├── cmd/gothermo-admin/      # Admin CLI (users, locked accounts, sessions, orphan uploads)
├── frontend/
│   └── src/
│       ├── components/      # Login, ChannelSidebar, ChatHeader, MessageList, MessageComposer,
│       │                    # CalendarModal, EventReminders, ChannelModal, ChannelMembers,
│       │                    # call UI (CallBar, GroupCallGrid, IncomingCallModal, ...)
│       ├── hooks/           # useWebSocket, useIdleStatus, group call hook
│       ├── services/        # api.ts (Wails calls + session token), session.ts
│       └── App.tsx
├── uploads/                 # Uploaded files (not tracked in git)
├── .github/workflows/ci.yml # Go tests on every push
└── wails.json
```

## Installation

### Prerequisites

- Go 1.26 or newer (see `go.mod`)
- Node.js 18 or newer and npm
- [Wails v2](https://wails.io/docs/gettingstarted/installation): `go install github.com/wailsapp/wails/v2/cmd/wails@latest`
- Redis 7 or newer
- macOS: Xcode Command Line Tools (`xcode-select --install`)

### Redis with persistence

Redis keeps data in memory, so persistence has to be switched on or everything is lost on restart. On macOS:

```bash
brew install redis
```

Add this to the Redis config (`/opt/homebrew/etc/redis.conf` on Apple Silicon, `/usr/local/etc/redis.conf` on Intel):

```
appendonly yes
appendfsync everysec
```

Start it as a background service and check:

```bash
brew services start redis
redis-cli ping                    # PONG
redis-cli CONFIG GET appendonly   # appendonly yes
```

With `everysec`, a sudden power loss costs at most the last second of data. GoThermo expects Redis on `localhost:6379`.

### Run the app

```bash
git clone https://github.com/Maksikos-ctrl/GoThermo.git
cd GoThermo
go mod download
cd frontend && npm install && cd ..

wails dev            # desktop window with hot reload
```

`wails dev` also serves the same UI at `http://localhost:34115`, so you can open a second account in a browser and test chats and calls between two users.

### Sign in with Google (optional)

1. In Google Cloud Console create an OAuth client of type **Desktop app**.
2. Download the JSON and save it as `google_oauth.json` next to the app (in the project root while developing). The file is in `.gitignore`, never commit it.
   Alternatively set the `GOOGLE_CLIENT_ID` and `GOOGLE_CLIENT_SECRET` environment variables.

Without it the email + password login still works.

### Production build

```bash
wails build            # binary in build/bin/
wails build -clean
```

### Releasing

Pushing a tag builds Windows, macOS and Linux packages in GitHub Actions (`.github/workflows/release.yml`) and attaches them to a GitHub Release:

```bash
git tag v0.1.0
git push origin v0.1.0
```

The landing page lives in `docs/` and is served by GitHub Pages (Settings, Pages, branch `master`, folder `/docs`).

## Tests

```bash
go test ./...            # all Go tests, uses miniredis, never touches real Redis
go test -race ./...      # with the race detector (this is what CI runs)
go test -cover ./...

cd frontend && npm test   # Vitest: components and thread state helpers
```

CI runs the Go tests on every push and pull request. The frontend job (type check and build) is informational for now.

## Admin tool

```bash
go run ./cmd/gothermo-admin users            # list accounts
go run ./cmd/gothermo-admin locked           # emails blocked after failed logins
go run ./cmd/gothermo-admin unlock <email>   # unblock an email
go run ./cmd/gothermo-admin sessions         # active sessions per user
go run ./cmd/gothermo-admin revoke <user>    # log a user out everywhere
go run ./cmd/gothermo-admin uploads          # find uploaded files no message uses (add -clean to delete)
```

## How data is stored

Redis is the primary database, not a cache.

| Key | Contents |
| --- | --- |
| `user:<email>` / `user:<email>:password` | profile JSON / bcrypt hash |
| `channel:<name>`, set `channels` | channel info (private flag, members) |
| `channel:<name>:messages` | list of messages |
| `file:<id>` | file metadata (the file itself is in `uploads/`) |
| `user:<name>:lastread:<channel>` | last read position for unread badges |
| `event:<id>`, ZSET `events:index` | calendar events, indexed by start time |
| `session:<token>` | session → username, TTL 30 days |
| `loginfail:<email>` | failed login counter, TTL 15 minutes |
| `linkpreview:<hash>` | cached link previews, TTL 24 hours |

When the user base grows, the plan is to move users, channels and messages to PostgreSQL and keep Redis for sessions, counters and caches.

## Security notes

- The WebSocket hub listens on `127.0.0.1` and uses plain `ws://`. That is fine for a local desktop app, but a hosted multi-user version needs TLS (`wss://`) and a reverse proxy.
- Messages are not end-to-end encrypted.
- Keep `google_oauth.json`, `.env` files and `uploads/` out of git.

## Roadmap

Done:
- [x] Authentication: email + password, Sign in with Google, session tokens, brute-force protection
- [x] Real-time messaging, typing indicators, presence
- [x] Public and private channels, direct messages
- [x] File sharing with drag & drop and clipboard paste
- [x] Voice and video calls (1:1 and group)
- [x] Message search, reactions, edit / delete
- [x] Link previews, channel export
- [x] Team calendar with RSVP and reminders
- [x] Go test suite and CI

Also done: Markdown and code blocks, infinite scroll for history, leaving a private channel, threads, frontend tests (Vitest), landing page and release builds.

Next up:
- [x] **Server mode**: run the backend on a server and connect any client by URL (first version; export and Google sign-in are not available over a server yet)
- [x] Invite code for registering on a server
- [ ] Export and Google sign-in in server mode
- [ ] Thread search and export, unread markers inside threads
- [ ] Pinned messages
- [ ] Custom themes
- [ ] Integrations (GitHub first, then Jira, ...)
- [ ] Video in group calls, screen sharing
- [ ] Code signing for Windows and macOS builds
- [ ] Migrate users / channels / messages to PostgreSQL
- [ ] Mobile companion app

## Contributing

1. Fork the repository
2. Create a branch (`git checkout -b feature/my-feature`)
3. Run `go vet ./... && go test ./...` before committing
4. Push and open a pull request

## License

MIT, see [LICENSE](LICENSE).

## Contact

**Maintainer:** Max Chernikov, [@Maksikos-ctrl](https://github.com/Maksikos-ctrl), maksikos973@gmail.com, [portfolio](https://maksymchernikovportfolio.vercel.app/)
