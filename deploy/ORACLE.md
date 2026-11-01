# Running GoThermo on Oracle Cloud Always Free

A free machine that stays on, enough for a small team. About 30 minutes, most of it waiting.

What "Always Free" gives you ([Oracle's list](https://docs.oracle.com/en-us/iaas/Content/FreeTier/resourceref.htm), check it, limits change):

- **Arm machine (VM.Standard.A1.Flex)**: 2 OCPUs and 12 GB of memory in total. This is the one to pick. It is sometimes "out of capacity"; try another availability domain, try again later, or use the next option.
- **AMD machine (VM.Standard.E2.1.Micro)**: up to two, 1 GB of memory each. It works too (the setup script adds swap so the build fits), but slowly.
- Everything must be created in your **home region**, chosen at sign-up and not changeable. Pick one close to your team (for Central Europe: Frankfurt).
- Oracle may reclaim machines that sit almost idle for a week. A chat server with people on it is not idle.

## 1. Account

Sign up at <https://www.oracle.com/cloud/free/>. A card is needed for identity checks; Always Free resources are not charged.

## 2. Create the machine

Console menu, **Compute → Instances → Create instance**:

- **Image**: Canonical Ubuntu 24.04 (or 22.04).
- **Shape**: Change shape → Ampere → VM.Standard.A1.Flex, 2 OCPUs, 12 GB. (Or the AMD Micro if Arm is out of capacity.)
- **Networking**: keep "Assign a public IPv4 address" on.
- **SSH keys**: upload or paste your public key (`cat ~/.ssh/id_ed25519.pub`; make one with `ssh-keygen -t ed25519` if you have none). Without it you cannot log in.

Create it and note the **public IP address** shown on the instance page.

## 3. Open ports 80 and 443 in the cloud firewall

Oracle blocks everything except SSH until you allow it. **Networking → Virtual cloud networks →** your network **→ Security → Default Security List → Add ingress rules**. Add two rules:

| Source CIDR | IP protocol | Destination port |
|---|---|---|
| `0.0.0.0/0` | TCP | `80` |
| `0.0.0.0/0` | TCP | `443` |

(Leave the other fields at their defaults.) The machine has a second firewall of its own; the setup script takes care of that one.

## 4. A domain name

HTTPS needs a name, not a bare IP. Either use a domain you own (add an **A record** pointing at the public IP), or a free name from [DuckDNS](https://www.duckdns.org/): sign in, create a subdomain, put the public IP into it. You get `yourname.duckdns.org`.

## 5. Run the setup script

```bash
ssh ubuntu@<public IP>
curl -fsSL https://raw.githubusercontent.com/Maksikos-ctrl/GoThermo/master/deploy/setup-server.sh -o setup-server.sh
sudo bash setup-server.sh yourname.duckdns.org
```

It installs Docker, opens ports 80 and 443 on the machine, adds swap on small machines, downloads GoThermo, writes `/opt/gothermo/.env` (random invite code and database password), builds and starts everything, then waits until `https://yourname.duckdns.org/healthz` answers. The first build takes a few minutes. At the end it prints the address and the invite code.

Optional: `sudo GOOGLE_CLIENT_ID=... bash setup-server.sh <domain>` to turn on Google sign-in (see SERVER.md).

If it ends with "No answer from https://...": the name does not point at the machine yet (step 4), or the two ingress rules are missing (step 3). Logs: `cd /opt/gothermo && sudo docker compose logs --tail 50 caddy gothermo`.

## 6. Connect the apps

In the app: **Server → Change**, type the domain, the invite code, **Connect**, then register.

## Everyday

```bash
cd /opt/gothermo
sudo git pull && sudo docker compose up -d --build        # update to the latest version
sudo docker compose ps                                      # is everything running
sudo docker compose logs --tail 100 gothermo                # what the server says
cat .env                                                    # invite code (needs sudo)
```

**Backups** (do this before you rely on it): the data is in two Docker volumes, the database and the uploaded files.

```bash
cd /opt/gothermo
sudo docker compose exec -T postgres pg_dump -U gothermo gothermo | gzip > ~/gothermo-$(date +%F).sql.gz
sudo docker run --rm -v gothermo_uploads:/data -v ~:/backup alpine tar czf /backup/uploads-$(date +%F).tgz -C /data .
```

Copy the files off the machine (`scp`). The volume name starts with the folder name; check with `sudo docker volume ls`.

## If you already have data in Redis somewhere

Copy it over once, as described in SERVER.md ("Moving to Postgres"). A fresh install needs nothing.
