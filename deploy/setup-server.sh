#!/usr/bin/env bash
# First-time setup of a GoThermo server on a fresh Ubuntu machine (22.04 or 24.04,
# x86 or Arm), on any cloud provider. See deploy/DEPLOY.md for the whole walk-through.
#
#   curl -fsSL https://raw.githubusercontent.com/Maksikos-ctrl/GoThermo/master/deploy/setup-server.sh -o setup-server.sh
#   sudo bash setup-server.sh chat.example.com
#
# It installs Docker, opens ports 80 and 443 in the machine's own firewall,
# adds swap on small machines, fetches the code, writes .env with a random
# invite code and database password, and starts everything.
# Safe to run again: it keeps the existing .env and just updates and restarts.
#
# Optional environment: GOOGLE_CLIENT_ID, REPO_URL, INSTALL_DIR.
# The cloud provider's own firewall (AWS security group, Azure network security
# group, Oracle VCN security list) is not something this script can open.

set -euo pipefail

REPO_URL="${REPO_URL:-https://github.com/Maksikos-ctrl/GoThermo.git}"
INSTALL_DIR="${INSTALL_DIR:-/opt/gothermo}"

say()  { printf '\n==> %s\n' "$*"; }
warn() { printf '\n!! %s\n' "$*" >&2; }
die()  { printf '\nERROR: %s\n' "$*" >&2; exit 1; }

# ───────────────────────── pieces (testable on their own) ─────────────────────────

valid_domain() {
  [[ "$1" =~ ^([A-Za-z0-9]([A-Za-z0-9-]*[A-Za-z0-9])?\.)+[A-Za-z]{2,}$ ]]
}

# Line number of the first REJECT rule in INPUT (some images, Oracle's for one,
# have a rule that blocks everything not allowed above it). Empty when there is none.
reject_position() {
  iptables -L INPUT --line-numbers -n | awk '$2 == "REJECT" { print $1; exit }'
}

open_port() { # open_port tcp 80
  local proto="$1" port="$2" pos
  if iptables -C INPUT -p "$proto" --dport "$port" -j ACCEPT 2>/dev/null; then
    return 0
  fi
  pos="$(reject_position)"
  if [ -n "$pos" ]; then
    iptables -I INPUT "$pos" -p "$proto" --dport "$port" -j ACCEPT
  else
    iptables -A INPUT -p "$proto" --dport "$port" -j ACCEPT
  fi
}

# Writes .env once. An existing .env is never touched (it holds the passwords).
write_env() { # write_env <file> <domain>
  local file="$1" domain="$2"
  if [ -f "$file" ]; then
    echo ".env already exists, keeping it"
    return 0
  fi
  (
    umask 077
    {
      echo "DOMAIN=$domain"
      echo "INVITE_CODE=$(openssl rand -hex 12)"
      echo "POSTGRES_PASSWORD=$(openssl rand -hex 16)"
      if [ -n "${GOOGLE_CLIENT_ID:-}" ]; then
        echo "GOOGLE_CLIENT_ID=$GOOGLE_CLIENT_ID"
      fi
    } > "$file"
  )
}

add_swap_if_small() {
  local mem_kb
  mem_kb="$(awk '/MemTotal/ { print $2 }' /proc/meminfo)"
  if [ "$mem_kb" -ge 3000000 ] || [ "$(swapon --show --noheadings | wc -l)" -gt 0 ]; then
    return 0
  fi
  say "Small machine ($((mem_kb / 1024)) MB): adding 2 GB of swap so the build does not run out of memory"
  fallocate -l 2G /swapfile 2>/dev/null || dd if=/dev/zero of=/swapfile bs=1M count=2048 status=none
  chmod 600 /swapfile
  mkswap /swapfile >/dev/null
  swapon /swapfile
  grep -q '^/swapfile ' /etc/fstab || echo '/swapfile none swap sw 0 0' >> /etc/fstab
}

# ───────────────────────── main ─────────────────────────

main() {
  [ "$(id -u)" -eq 0 ] || die "run it with sudo:  sudo bash $0 chat.example.com"
  local domain="${1:-}"
  [ -n "$domain" ] || die "usage: sudo bash $0 <domain>   (a name that already points at this machine's public IP)"
  valid_domain "$domain" || die "'$domain' does not look like a domain name"
  . /etc/os-release
  [ "${ID:-}" = "ubuntu" ] || warn "this script was written for Ubuntu, you have ${PRETTY_NAME:-something else}; continuing"

  say "Checking that $domain points at this machine"
  local my_ip dns_ip
  my_ip="$(curl -fsS --max-time 10 https://api.ipify.org 2>/dev/null || true)"
  dns_ip="$(getent ahostsv4 "$domain" 2>/dev/null | awk 'NR == 1 { print $1 }' || true)"
  if [ -z "$dns_ip" ]; then
    warn "$domain does not resolve yet. The HTTPS certificate cannot be issued until it does."
  elif [ -n "$my_ip" ] && [ "$dns_ip" != "$my_ip" ]; then
    warn "$domain points at $dns_ip but this machine's public address is $my_ip. Fix the DNS record, or HTTPS will not work."
  else
    echo "ok: $domain -> $dns_ip"
  fi

  say "Installing packages"
  export DEBIAN_FRONTEND=noninteractive
  apt-get update -qq
  apt-get install -y -qq ca-certificates curl git openssl iptables iptables-persistent >/dev/null

  if ! command -v docker >/dev/null 2>&1; then
    say "Installing Docker"
    curl -fsSL https://get.docker.com | sh
  fi
  docker compose version >/dev/null 2>&1 || die "Docker Compose is missing; install the docker-compose-plugin package"
  systemctl enable --now docker >/dev/null 2>&1 || true

  say "Opening ports 80 and 443 in this machine's firewall"
  open_port tcp 80
  open_port tcp 443
  open_port udp 443
  netfilter-persistent save >/dev/null 2>&1 || warn "could not save the firewall rules; they may be lost on reboot"

  add_swap_if_small

  say "Fetching GoThermo into $INSTALL_DIR"
  if [ -d "$INSTALL_DIR/.git" ]; then
    git -C "$INSTALL_DIR" pull --ff-only
  else
    git clone "$REPO_URL" "$INSTALL_DIR"
  fi
  cd "$INSTALL_DIR"

  write_env .env "$domain"

  say "Building and starting (the first build takes a few minutes)"
  docker compose up -d --build

  say "Waiting for https://$domain/healthz"
  local i
  for i in $(seq 1 30); do
    if curl -fsS --max-time 5 "https://$domain/healthz" 2>/dev/null; then
      echo
      say "Done"
      echo "Server:       https://$domain"
      echo "Invite code:  $(grep '^INVITE_CODE=' .env | cut -d= -f2-)   (give it to your team; it is also in $INSTALL_DIR/.env)"
      echo "In a browser: https://$domain"
      echo "In the app:   Server -> Change -> $domain"
      return 0
    fi
    sleep 6
  done
  warn "No answer from https://$domain/healthz yet. Usual causes:"
  warn "  1) the DNS name does not point at this machine's public IP"
  warn "  2) the cloud firewall (security group) does not allow ports 80 and 443 from 0.0.0.0/0"
  warn "Look at the logs:  cd $INSTALL_DIR && docker compose logs --tail 50 caddy gothermo"
  exit 1
}

# Sourced by the tests with GOTHERMO_SETUP_TEST=1: only the pieces above are defined.
if [ "${GOTHERMO_SETUP_TEST:-}" != "1" ]; then
  main "$@"
fi
