# Put a GoThermo server on the internet

A server is one small Linux machine that is always on. The team opens its address in a browser or connects the desktop apps to it. This guide uses AWS EC2 as the worked example; any provider with an Ubuntu virtual machine works the same way, only the buttons differ.

What you need:
- an Ubuntu 22.04 or 24.04 machine with **2 GB of memory or more**, x86 or Arm, a public IP address and 20 GB of disk
- ports **80 and 443** open to everyone, and port 22 (SSH) open to you only
- a domain name that points at the machine's IP (a free DuckDNS name is fine)

## 1. Create the machine

On AWS (EC2, Launch instance):

1. Image: **Ubuntu Server 24.04 LTS**.
2. Type: `t3.small` (2 GB). A 1 GB machine is too small for Postgres, Redis and the server together.
3. Key pair: create one (RSA, `.pem`) and keep the file safe. It is the only way in.
4. Network: allow **SSH from My IP**, and tick **HTTPS** and **HTTP** from the internet.
5. Storage: 20 GB gp3.
6. Launch. Then open Elastic IPs, allocate one and associate it with the machine, so the address survives a stop and start.

Other providers: create an Ubuntu VM, then open ports 80 and 443 in its firewall (Azure: Network security group, inbound rules; Google Cloud: VPC firewall rules; Oracle Cloud: the VCN security list, plus the machine's own iptables, which the setup script handles).

## 2. Point a name at it

Make an A record for your domain with the machine's IP. With DuckDNS: sign in at duckdns.org, add a sub domain, put the IP in the "current ip" field and press update. The field is pre-filled with the address of the computer you are using, so replace it.

Check it from your own computer: `ping chat.example.com` should show the machine's IP.

## 3. Run the setup script

```bash
chmod 400 ~/Downloads/your-key.pem
ssh -i ~/Downloads/your-key.pem ubuntu@MACHINE_IP

curl -fsSLO https://raw.githubusercontent.com/Maksikos-ctrl/GoThermo/master/deploy/setup-server.sh
sudo bash setup-server.sh chat.example.com
```

The user name is `ubuntu` on Ubuntu images of most providers; Azure uses the one you chose. The script installs Docker, opens the machine's own firewall, adds swap on small machines, fetches the code, writes `.env` with a random invite code and database password, starts everything and waits for `https://chat.example.com/healthz`. The first build takes a few minutes. It is safe to run again.

At the end it prints the invite code. It is also in `/opt/gothermo/.env`.

## 4. Use it

- **In a browser:** open `https://chat.example.com`. Registering needs the invite code. A link like `https://chat.example.com/app/?invite=THE-CODE` fills it in.
- **In the desktop app:** on the sign-in screen click **Change** next to "Server", type `chat.example.com`, enter the code and press Connect.

## Looking after it

All commands run on the server, in `/opt/gothermo`.

| What | Command |
|---|---|
| Update to the latest code | `sudo git pull && sudo docker compose up -d --build` |
| Change the invite code | edit `INVITE_CODE` in `.env`, then `sudo docker compose up -d` |
| Look at the logs | `sudo docker compose logs --tail 50 gothermo caddy` |
| Back up the database | `sudo docker compose exec -T postgres pg_dump -U gothermo gothermo \| gzip > gothermo-$(date +%F).sql.gz` |
| Back up uploaded files | `sudo docker run --rm -v gothermo_uploads:/data -v "$PWD":/backup alpine tar czf /backup/uploads-$(date +%F).tgz -C /data .` |

Copy the backups off the machine (for example with `scp`). A backup that lives only on the machine is lost with it.

To save money while nobody uses it, stop the machine in the provider's console. The data stays on the disk.

## When something does not work

- **The script says there is no answer from /healthz.** Either the name does not point at the machine's IP yet, or ports 80 and 443 are closed in the provider's firewall. Fix that and run the script again.
- **SSH times out.** SSH is allowed only from the address you chose. If your IP changed (another network, a VPN), edit the rule in the provider's console.
- **The browser warns about the certificate.** Caddy asks for it on the first request and needs ports 80 and 443 reachable; wait a minute and reload.

More settings and moving old Redis data to Postgres: [SERVER.md](../SERVER.md).
