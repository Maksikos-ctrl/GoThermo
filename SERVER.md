# Running a GoThermo server

One server, many desktop apps: everybody connects to the same workspace.

## What you need

- A small VPS (1 GB RAM is enough) with Docker and Docker Compose
- A domain name pointing at the VPS (an `A` record), for example `chat.example.com`
- Ports 80 and 443 open

## Start it

```bash
git clone https://github.com/Maksikos-ctrl/GoThermo.git
cd GoThermo
printf 'DOMAIN=chat.example.com\nINVITE_CODE=%s\n' "$(openssl rand -hex 12)" > .env
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

## Settings

| Variable | Default | Meaning |
|---|---|---|
| `GOTHERMO_ADDR` | `0.0.0.0:8081` | address the server listens on |
| `REDIS_ADDR` | `localhost:6379` | where Redis is |
| `REDIS_PASSWORD` | empty | Redis password, if any |
| `GOTHERMO_INVITE_CODE` | empty | when set, registering needs this code. Empty means anyone can register |
| `GOTHERMO_TRUST_PROXY` | empty | set to `1` behind a reverse proxy so wrong-code limits count each real client (the compose file sets it) |

## Good to know

- Data lives in Redis (volume `redis-data`) and uploaded files in volume `uploads`. Back both up.
- Set an invite code. Without one anyone who finds the address can register. Wrong guesses are limited to 10 per client per 15 minutes.
- To change the code, edit `.env` and run `docker compose up -d`.
- Not available over a server yet: Google sign-in and channel export.
- Never expose Redis (port 6379) to the internet. The compose file keeps it internal.
