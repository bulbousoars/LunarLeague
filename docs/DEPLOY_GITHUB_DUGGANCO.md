# GitHub → homelab deploy (lunarleague.dugganco.com)

This document wires **GitHub Actions** so every merge to `main` (that touches `apps/`, `deploy/`, or this workflow) SSHs into your box, **fast-forwards git**, **rebuilds Docker**, runs **`api migrate up`**, and curls **`/healthz`** on the host-published API port.

## 1. On the server (one-time)

1. **Clone** the repo somewhere persistent (example matches the homelab layout):

   ```bash
   sudo mkdir -p /mnt/storage/docker/lunarleague
   sudo chown "$USER:$USER" /mnt/storage/docker/lunarleague
   git clone https://github.com/bulbousoars/LunarLeague.git /mnt/storage/docker/lunarleague/app
   cd /mnt/storage/docker/lunarleague/app/deploy
   cp .env.production.example .env
   # edit .env: PUBLIC_WEB_URL=https://lunarleague.dugganco.com, SMTP, secrets, API_PORT if not 8000, etc.
   ```

2. **Bring the stack up once** (compose files you actually use — often base + homelab overlay):

   ```bash
   docker compose -f docker-compose.yml -f docker-compose.realestate.yml --env-file .env up -d --build
   docker compose -f docker-compose.yml -f docker-compose.realestate.yml --env-file .env run --rm api migrate up
   docker compose -f docker-compose.yml -f docker-compose.realestate.yml --env-file .env run --rm api seed
   ```

3. **SSH deploy user** must be able to run `git` in **`…/app`** (repo root) and `docker compose` in **`…/app/deploy`**. Typical options:

   - Dedicated user in `docker` group, or
   - Passwordless `sudo` only for `docker compose` (narrower), or
   - The root-owned wrapper pattern from `docs/superpowers/plans/2026-05-05-lunarleague-openbao-deploy-rebuild.md` if you use agent principals.

4. **GitHub deploy key**: create an SSH key pair **without a passphrase**, add the **public** key to `~/.ssh/authorized_keys` for that user on the VM, store the **private** key in `LUNARLEAGUE_SSH_KEY`.

## 2. In GitHub (repo **bulbousoars/LunarLeague**)

**Secrets** → Actions:

| Name | Example |
| --- | --- |
| `LUNARLEAGUE_SSH_HOST` | `192.168.1.218` or internal DNS |
| `LUNARLEAGUE_SSH_USER` | `dduggan` |
| `LUNARLEAGUE_SSH_KEY` | Full PEM private key |
| `LUNARLEAGUE_DEPLOY_DIR` | `/mnt/storage/docker/lunarleague/app/deploy` |

**Variables** → Actions (optional):

| Name | Purpose |
| --- | --- |
| `LUNARLEAGUE_DEPLOY_BRANCH` | Branch to track (default `main` if unset/empty) |
| `LUNARLEAGUE_COMPOSE_OVERLAY` | Extra file suffix, e.g. `realestate` → adds `-f docker-compose.realestate.yml` |

Important: **`LUNARLEAGUE_DEPLOY_DIR` must be the `deploy` folder** inside the clone. The workflow runs `git` in `$(dirname "$DEPLOY_DIR")` (the repo root). If `git fetch` ran only inside `deploy/`, it would fail — that parent-dir behavior is intentional.

## 3. What runs on each deploy

1. `git fetch` / `checkout` / `reset --hard origin/<branch>` at repo root  
2. `docker compose … up -d --build --remove-orphans` from `deploy/`  
3. `docker compose … run --rm api migrate up`  
4. `curl http://127.0.0.1:$API_PORT/healthz` where `API_PORT` is read from `deploy/.env` (default `8000`)

If Traefik terminates TLS and the API is only on a host port, this health check still validates the API container from the VM itself.

## 4. Pushes vs manual

- **Automatic:** any push to **`main`** that changes paths under `apps/`, `deploy/`, or this workflow file.  
- **Manual:** Actions → **Deploy production** → **Run workflow** (useful for a one-off redeploy without a dummy commit).

To deploy on **documentation-only** commits, use **Run workflow** or temporarily widen `paths` in `deploy.yml`.

## 5. Runner reachability

GitHub-hosted runners must reach `LUNARLEAGUE_SSH_HOST`. A **private** `192.168.x.x` address is **not** reachable from GitHub’s cloud. Options:

- **Self-hosted runner** on your LAN (same machine as dev PC or a small LXC) that runs this job, or  
- **Tailscale / Cloudflare Tunnel** SSH, or  
- Expose SSH only via a bastion you trust (not recommended on the open Internet without strict controls).

If you already use another CI that can touch the LAN, mirror the shell block from this workflow there.

## 6. Rollback

On the server:

```bash
cd /mnt/storage/docker/lunarleague/app
git log --oneline -5
git reset --hard <good_sha>
cd deploy
docker compose -f docker-compose.yml -f docker-compose.realestate.yml --env-file .env up -d --build
```

Then fix `main` in git and redeploy.

## 7. Prefer no GitHub deploy at all?

Use **[DEPLOY_VM_PERIODIC.md](DEPLOY_VM_PERIODIC.md)** — systemd (or cron) on the VM pulls `main` and rebuilds; no SSH secrets in GitHub.
