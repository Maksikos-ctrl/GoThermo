# Running a GoThermo server

One server, many desktop apps: everybody connects to the same workspace.

## What you need

No server yet? [deploy/DEPLOY.md](deploy/DEPLOY.md) walks through creating a cloud machine and a one-command setup.

- A small VPS (1 GB RAM is enough) with Docker and Docker Compose
- A domain name pointing at the VPS (an `A` record), for example `chat.example.com`
- Ports 80 and 443 open

## Start it

```bash
git clone https://github.com/Maksikos-ctrl/GoThermo.git
cd GoThermo
printf 'DOMAIN=chat.example.com\nINVITE_CODE=%s\nPOSTGRES_PASSWORD=%s\n' "$(openssl rand -hex 12)" "$(openssl rand -hex 16)" > .env
cat .env          # note the invite code, you will give it to your team
docker compose up -d --build
```

Check it: open `https://chat.example.com/healthz`. You should see `{"ok":true,"app":"gothermo"}`.

## Connect the apps

On the sign-in screen of the app click **Server: this computer, Change**, type `chat.example.com`, enter the invite code and press **Connect**, then register or sign in as usual. The invite code is only needed to create an account; signing in does not use it.

## In the browser

The Docker image also contains the interface, so nobody has to install anything: open `https://chat.example.com` and it goes straight to the chat. Register with the invite code, or send a link that carries it: `https://chat.example.com/app/?invite=THE-CODE` opens the sign-up form with the code filled in and removes it from the address bar. Sign in with Google is not available in the browser yet.

Without Docker, build the interface with `cd frontend && VITE_WEB=1 npm run build`, put the `dist` folder at `<dir>/app` and start the server with `GOTHERMO_WEB_DIR=<dir>`.

## Without Docker

```bash
go build -tags server -o gothermo-server .
REDIS_ADDR=localhost:6379 GOTHERMO_ADDR=127.0.0.1:8081 ./gothermo-server
```

Put a reverse proxy (Caddy, nginx) with HTTPS in front of it.

## Moving to Postgres

Postgres is the durable store (messages, users, channels, events, files); Redis stays for sessions, login limits and previews. A new install uses Postgres from the first start.

If you already have data in Redis, the server refuses to start until you copy it over. The copy is one-off and Redis is only read, so nothing is lost if you stop halfway:

```bash
docker compose run --rm gothermo gothermo-server migrate -dry-run   # what would be copied
docker compose run --rm gothermo gothermo-server migrate            # copy and verify
docker compose up -d
```

If the copy fails or does not match, nothing is switched: fix the cause and run `migrate -reset` to start over. To stay on Redis on purpose, put `GOTHERMO_STORE=redis` in `.env`. Going back to Redis after the switch loses what was written to Postgres since.

## Settings

| Variable | Default | Meaning |
|---|---|---|
| `GOTHERMO_ADDR` | `0.0.0.0:8081` | address the server listens on |
| `REDIS_ADDR` | `localhost:6379` | where Redis is |
| `REDIS_PASSWORD` | empty | Redis password, if any |
| `GOTHERMO_INVITE_CODE` | empty | when set, registering needs this code. Empty means anyone can register |
| `GOOGLE_CLIENT_ID` | empty | the Google OAuth client id the server accepts sign-ins from. Needed for Google sign-in over a server |
| `GOOGLE_WEB_CLIENT_ID` | empty | the Google OAuth client id (type **Web application**) used by the browser version. When set, the browser sign-in page shows "Sign in with Google". See "Google sign-in in the browser" below |
| `GOOGLE_WEB_CLIENT_SECRET` | empty | the secret of the same Web client. With it the desktop apps sign in with Google through the server, with nothing installed on the computer. Needs the redirect address below |
| `GOTHERMO_PUBLIC_URL` | empty | the public address of the server, `https://chat.example.com`. The compose file sets it from `DOMAIN` |
| `DATABASE_URL` | empty | Postgres, `postgres://user:password@host:5432/dbname`. The schema is created and updated automatically. The compose file sets it |
| `GOTHERMO_STORE` | empty | where messages, users, channels and the rest are kept: `postgres`, `redis`, or empty (Postgres when `DATABASE_URL` is set, otherwise Redis). See "Moving to Postgres" |
| `GOTHERMO_WEB_DIR` | empty | folder with the browser version, its `app` subfolder holds the built interface. The Docker image sets it. Empty means the server only answers the apps |
| `GOTHERMO_TRUST_PROXY` | empty | set to `1` behind a reverse proxy so wrong-code limits count each real client (the compose file sets it) |

## Reports and bans

Anyone can report another person's message (flag button on hover, then a reason).
Reports are kept on the server; you read them and act with the admin tool that is
inside the container:

```
sudo docker compose exec gothermo gothermo-admin -addr redis:6379 reports
sudo docker compose exec gothermo gothermo-admin -addr redis:6379 ban some-username
sudo docker compose exec gothermo gothermo-admin -addr redis:6379 unban some-username
sudo docker compose exec gothermo gothermo-admin -addr redis:6379 banned
```

A banned account cannot sign in and its sessions stop working at once.

## Calls on mobile networks

Calls go straight between two devices. On some phone networks and strict
firewalls that is impossible, and the call hangs on "Connecting". A small relay
(coturn) fixes it. It is part of the compose file and off by default.

1. Open these ports in the firewall (on AWS: the Security Group): **UDP and TCP 3478** and **UDP 49152-49252**.
2. Add to `.env`: `TURN_SECRET=<any long random string>`, `COMPOSE_PROFILES=turn` and, on AWS or any host behind NAT, `TURN_EXTERNAL_IP=<the server's public IP>`.
3. `docker compose up -d --build`

New installs made by `deploy/setup-server.sh` get the first two lines automatically
(they still need the ports and, on AWS, the public IP). Without `TURN_SECRET`
calls use STUN only, as before.

## Good to know

- Messages, users, channels and the rest live in Redis (volume `redis-data`) or, after the move below, in Postgres (volume `pg-data`). Uploaded files are in volume `uploads`. Back up the volumes. Sessions and login limits always stay in Redis.
- Set an invite code. Without one anyone who finds the address can register. Wrong guesses are limited to 10 per client per 15 minutes.
- To change the code, edit `.env` and run `docker compose up -d`.
- Google sign-in over a server: set `GOOGLE_CLIENT_ID` in `.env` to the same client id that is in `google_oauth.json` on the desktop apps. The browser step happens on the user's computer, so each desktop app still needs its own `google_oauth.json`. The server checks the token was issued to that client and reads the profile from Google itself. Google sign-up needs the invite code, like normal registration.
- Google sign-in in the browser: in Google Cloud Console (APIs & Services, Credentials) create an OAuth client of type **Web application**. Under "Authorized JavaScript origins" add your address, for example `https://chat.example.com` (no path, no trailing slash). Put the client id in `.env` as `GOOGLE_WEB_CLIENT_ID=...apps.googleusercontent.com` and run `docker compose up -d`. The browser needs no client secret. For the desktop apps to sign in with Google through the server as well, add under "Authorized redirect URIs" the address `https://chat.example.com/auth/google/callback` and put the client's secret in `.env` as `GOOGLE_WEB_CLIENT_SECRET=...`: the app then opens Google in the system browser, Google returns to the server, and the app picks the login up by itself, so no `google_oauth.json` is needed on anyone's computer. While the consent screen is in "Testing", only the test users you list can sign in: add your teammates there, or press "Publish app" (the basic profile and email scopes need no review). The server accepts tokens from `GOOGLE_CLIENT_ID` (desktop) and `GOOGLE_WEB_CLIENT_ID` (browser); both can be set, and `GOOGLE_CLIENT_ID` may hold several ids separated by commas. A Google account is a separate account from an email+password one, even with the same address.
- Channel export works over a server: the server builds the file and the app saves it on your computer. Saving received attachments works in the browser and, in the app, falls back to a normal download.
- Never expose Redis (port 6379) to the internet. The compose file keeps it internal.

## GIFs

The GIF button appears when the server has a Giphy key.

1. Create a free app at <https://developers.giphy.com> (type: API) and copy the key.
2. Add `GIPHY_API_KEY=<key>` to `.env` and run `docker compose up -d`.

The key stays on the server; browsers only talk to GoThermo. Results are limited to
PG-13 and each user gets 30 searches a minute. Without a key nothing changes.

## Avatars and notifications

Pictures are re-encoded on the server (128x128 JPEG), so nothing a user uploads is
served as-is. Notifications need no setup: the browser asks once when the toggle in
Settings is switched on, and the desktop app uses the system notifier.
