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
DOMAIN=chat.example.com docker compose up -d --build
```

Check it: open `https://chat.example.com/healthz`. You should see `{"ok":true,"app":"gothermo"}`.

## Connect the apps

On the sign-in screen of the app click **Server: this computer, Change**, type `chat.example.com`, press **Connect**, then register or sign in as usual.

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

## Good to know

- Data lives in Redis (volume `redis-data`) and uploaded files in volume `uploads`. Back both up.
- Anyone who can reach the server can register. Keep it behind a VPN or firewall until invite codes are added.
- Not available over a server yet: Google sign-in and channel export.
- Never expose Redis (port 6379) to the internet. The compose file keeps it internal.
