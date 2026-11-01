# Running a GoThermo server

One server, many desktop apps: everybody connects to the same workspace.

## What you need

No server yet? [deploy/ORACLE.md](deploy/ORACLE.md) walks through a free Oracle Cloud machine and a one-command setup.

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
| `DATABASE_URL` | empty | Postgres, `postgres://user:password@host:5432/dbname`. The schema is created and updated automatically. The compose file sets it |
| `GOTHERMO_STORE` | empty | where messages, users, channels and the rest are kept: `postgres`, `redis`, or empty (Postgres when `DATABASE_URL` is set, otherwise Redis). See "Moving to Postgres" |
| `GOTHERMO_TRUST_PROXY` | empty | set to `1` behind a reverse proxy so wrong-code limits count each real client (the compose file sets it) |

## Good to know

- Messages, users, channels and the rest live in Redis (volume `redis-data`) or, after the move below, in Postgres (volume `pg-data`). Uploaded files are in volume `uploads`. Back up the volumes. Sessions and login limits always stay in Redis.
- Set an invite code. Without one anyone who finds the address can register. Wrong guesses are limited to 10 per client per 15 minutes.
- To change the code, edit `.env` and run `docker compose up -d`.
- Google sign-in over a server: set `GOOGLE_CLIENT_ID` in `.env` to the same client id that is in `google_oauth.json` on the desktop apps. The browser step happens on the user's computer, so each desktop app still needs its own `google_oauth.json`. The server checks the token was issued to that client and reads the profile from Google itself. Google sign-up needs the invite code, like normal registration.
- Channel export works over a server: the server builds the file and the app saves it on your computer. Saving received attachments to disk is not available over a server yet.
- Never expose Redis (port 6379) to the internet. The compose file keeps it internal.
