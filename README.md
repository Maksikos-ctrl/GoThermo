# GoThermo

[![CI](https://github.com/Maksikos-ctrl/GoThermo/actions/workflows/ci.yml/badge.svg)](https://github.com/Maksikos-ctrl/GoThermo/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go](https://img.shields.io/badge/Go-00ADD8?logo=go&logoColor=white)](https://golang.org/)
[![Wails](https://img.shields.io/badge/Wails-DF0024?logo=wails&logoColor=white)](https://wails.io/)
[![React](https://img.shields.io/badge/React-20232A?logo=react&logoColor=61DAFB)](https://reactjs.org/)
[![TypeScript](https://img.shields.io/badge/TypeScript-007ACC?logo=typescript&logoColor=white)](https://www.typescriptlang.org/)

GoThermo is a real-time team chat desktop app in the spirit of Slack and Teams. The backend is Go, the UI is React + TypeScript, they are glued together with [Wails v2](https://wails.io/), and messages and accounts are stored in Postgres (sessions and caches in Redis).

It also runs in the browser: a GoThermo server serves the same interface, so teammates open a link and register, with nothing to install.

> Status: actively developed, tested on macOS. Windows and Linux builds are produced by GitHub Actions but have not been tested by hand yet.

**Website:** https://maksikos-ctrl.github.io/GoThermo/ · **Downloads:** [latest release](https://github.com/Maksikos-ctrl/GoThermo/releases/latest)

## Download

Grab the build for your system from the [Releases page](https://github.com/Maksikos-ctrl/GoThermo/releases/latest):

| System | File |
|---|---|
| Windows | [`GoThermo-windows-amd64.exe`](https://github.com/Maksikos-ctrl/GoThermo/releases/latest/download/GoThermo-windows-amd64.exe) (a `.zip` is there too) |
| macOS | [`GoThermo-macos.dmg`](https://github.com/Maksikos-ctrl/GoThermo/releases/latest/download/GoThermo-macos.dmg) (a `.zip` is there too) |
| Linux | [`GoThermo-linux-amd64.tar.gz`](https://github.com/Maksikos-ctrl/GoThermo/releases/latest/download/GoThermo-linux-amd64.tar.gz) |

No download needed: the same chat runs in any browser at your server's address.

Things to know before you run it:
- Release builds start **already connected to the team server**: open the app, choose Sign Up and enter the invite code you were given. To use another server or none, click the server name on the sign-in screen. Run on its own, a copy talks to a **Redis on `localhost:6379`** and its own WebSocket on `127.0.0.1:8081`; install and start Redis first (`brew install redis`, `apt install redis-server` or `docker run -d -p 6379:6379 redis`).
- On its own a copy only chats with itself and with other windows on the same machine. To share one workspace between computers, run a GoThermo server and connect the apps to it, or let people use it in a browser (see [Server mode](#server-mode)).
- The builds are not code-signed. Windows SmartScreen: More info, then Run anyway. macOS: open the `.dmg`, drag GoThermo to Applications and try to open it; then System Settings, Privacy & Security, **Open Anyway**. **If macOS says the app is damaged or will not open, run this once in Terminal, then open it again:** `xattr -cr /Applications/GoThermo.app && codesign --force --deep --sign - /Applications/GoThermo.app`
- Calls: the first time you call, macOS asks for microphone (and camera) access. Allow it; if you said no by mistake, System Settings, Privacy & Security, Microphone, switch GoThermo on.
- Sign in with Google works in the app and in the browser when the server is set up for it (see [SERVER.md](SERVER.md)); the app opens Google in your system browser and picks the login up by itself. Only an app running on its own, without a server, needs your own `google_oauth.json` (see below). Email + password works everywhere.

## Features

**Messaging**
- Public channels, **private channels** (invite-only, hidden from non-members) and direct messages
- Real-time delivery over WebSocket, typing indicators, online / away presence (auto-away after 10 minutes without activity)
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

One backend on a server, many clients connected to it. The same Go code runs headless (`go build -tags server`), the clients send their calls to `POST /rpc/<Method>` and receive live updates over `wss://`. Setup with Docker, HTTPS included: [SERVER.md](SERVER.md). Creating a cloud machine from scratch: [deploy/DEPLOY.md](deploy/DEPLOY.md).

**Desktop app:** on the sign-in screen click **Server: this computer, Change**, type your server address and press **Connect**. Leave it empty to keep using the local mode.

**Browser:** open the server address. The Docker image contains the interface (`VITE_WEB=1 npm run build`), the server serves it at `/app/` and the page uses its own address as the server. Registering needs the invite code; send people a link like `https://chat.example.com/app/?invite=CODE` and it is filled in automatically (the browser has no code field). Sign in with Google also works in the browser when the server has `GOOGLE_WEB_CLIENT_ID` set (see [SERVER.md](SERVER.md)).

## Tech stack

| Layer | What |
| --- | --- |
| Desktop shell | Wails v2 |
| Backend | Go, gorilla/websocket, pgx, go-redis |
| Frontend | React, TypeScript, Vite, plain CSS |
| Storage | Postgres (messages, users, files; schema in `db/migrations`) and Redis (sessions, login limits, caches) |
| Real-time | WebSocket hub on `127.0.0.1:8081`, WebRTC for calls |
| Tests | `go test` with miniredis; the Postgres tests run when `TEST_DATABASE_URL` is set (CI does), GitHub Actions on macOS |

## Project structure

```
GoThermo/
├── main.go                  # Wails entry point
├── server_main.go           # headless server entry point (build tag `server`)
├── rpc.go                   # POST /rpc/<Method> for remote clients
├── web.go                   # serves the browser build of the interface
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
├── postgres.go              # Postgres connection, which store is active (GOTHERMO_STORE)
├── store*.go                # Store interface, Redis and Postgres implementations
├── migrate.go, db/migrations/  # schema migrations
├── redis_to_postgres.go     # one-off copy of the data (gothermo-server migrate)
├── models.go                # Data models
├── *_test.go                # Go tests (miniredis)
├── cmd/gothermo-admin/      # Admin CLI (users, locked accounts, sessions, orphan uploads)
├── deploy/                  # setup-server.sh, Caddyfile, DEPLOY.md
├── frontend/
│   └── src/
│       ├── components/      # Login, ChannelSidebar, ChatHeader, MessageList, MessageComposer,
│       │                    # CalendarModal, EventReminders, ChannelModal, ChannelMembers,
│       │                    # call UI (CallBar, GroupCallGrid, IncomingCallModal, ...)
│       ├── hooks/           # useWebSocket, useIdleStatus, group call hook
│       ├── services/        # api.ts, transport.ts (Wails or server), server.ts, session.ts,
│       │                    # platform.ts and save.ts (browser vs desktop)
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

Pushing a tag builds a Windows `.exe`, a macOS `.dmg` and a Linux package in GitHub Actions (`.github/workflows/release.yml`) and attaches them to a GitHub Release. The apps are built to open already connected to `https://gothermo-team.duckdns.org`; to point them at another server set the repository variable `GOTHERMO_SERVER` (Settings, Secrets and variables, Actions, Variables).

```bash
git tag v0.2.0
git push origin v0.2.0
```

The download links on the website and in this README (`releases/latest/download/...`) work as soon as the first release finishes building (the Actions tab shows progress, about 10 minutes).

The landing page lives in `docs/` and is served by GitHub Pages (Settings, Pages, branch `master`, folder `/docs`).

## Tests

```bash
go test ./...            # all Go tests (miniredis; the Postgres tests run when TEST_DATABASE_URL is set)
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
go run ./cmd/gothermo-admin reports          # newest reports of messages
go run ./cmd/gothermo-admin ban <user>       # block an account (no sign-in, sessions end)
go run ./cmd/gothermo-admin unban <user>     # let it back in
go run ./cmd/gothermo-admin banned           # list banned accounts
go run ./cmd/gothermo-admin uploads          # find uploaded files no message uses (add -clean to delete)
# accounts and uploads are read from Postgres when -database-url (or $DATABASE_URL) is given
go run ./cmd/gothermo-admin -database-url "$DATABASE_URL" users
```

## How data is stored

Messages, users, channels, files, read marks and calendar events live in **Postgres** (schema in `db/migrations`, applied automatically at startup). **Redis** keeps what is short-lived: sessions, login limits and link previews. Without `DATABASE_URL` everything is kept in Redis, which is what a copy of the app does when it runs alone for development. Redis layout of the durable data (that mode, and the source of `gothermo-server migrate`):

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

Moving existing data from Redis to Postgres is one command, see [SERVER.md](SERVER.md#moving-to-postgres).

## Security notes

- Run on its own, the WebSocket hub listens on `127.0.0.1` with plain `ws://`, which is fine for one machine. A shared server must sit behind HTTPS (`wss://`); the Docker setup does that with Caddy and a free certificate.
- Set an invite code on a shared server, otherwise anyone who finds the address can register. A link that carries the code lets whoever has the link register, so send it only to your team.
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

Also done since: server mode with an invite code, **the browser version**, a one-command server setup (`deploy/setup-server.sh`), channel and thread export, pinned messages, accent colors, unread markers in threads, and the move of users, channels and messages to Postgres.

Next up:
- [x] Sign in with Google in the browser
- [ ] Keep the session after a page reload in the browser
- [ ] Thread search
- [ ] Integrations (GitHub first, then Jira, ...)
- [ ] Screen sharing in group calls
- [ ] Code signing for Windows and macOS builds
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
