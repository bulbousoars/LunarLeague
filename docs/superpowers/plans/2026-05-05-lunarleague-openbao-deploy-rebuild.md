# LunarLeague OpenBao Deploy Rebuild Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Recreate the VM 218 LunarLeague deployment from `https://github.com/bulbousoars/LunarLeague` while preserving data and adding a narrow OpenBao-agent deploy path.

**Architecture:** Preserve `/mnt/storage/docker/lunarleague/postgres`, `/mnt/storage/docker/lunarleague/redis`, and existing production `.env` values. Replace only the app tree with a real git checkout, then add a root-owned deploy wrapper that agent cert principals can run via sudo without broad Docker group membership.

**Tech Stack:** Git, Docker Compose, OpenBao SSH CA principals, Linux groups/sudoers, Traefik/AdGuard existing routing.

---

### Task 1: Establish VM 218 Admin Session

**Files:**
- Read: `C:\Users\danie\Scripts\homelab\New-AgentSshSession.ps1`
- Target host: `192.168.1.218`

- [ ] Mint or use an OpenBao token that can sign `human-admin-30m`.
- [ ] Start an SSH cert session as `dduggan` with `New-AgentSshSession.ps1 -Agent human -HostName 192.168.1.218`.
- [ ] Verify `whoami`, `id`, `hostname`, and `sudo -n true` or identify whether interactive sudo is required.

### Task 2: Preserve Current Deployment State

**Files:**
- Backup source: `/mnt/storage/docker/lunarleague/app`
- Preserve data: `/mnt/storage/docker/lunarleague/postgres`
- Preserve data: `/mnt/storage/docker/lunarleague/redis`

- [ ] Stop the current stack from `/mnt/storage/docker/lunarleague/app/deploy` if compose files are readable.
- [ ] Create a timestamped backup directory under `/mnt/storage/docker/lunarleague/backups/`.
- [ ] Copy the current app tree metadata and `deploy/.env` into the backup directory with restrictive permissions.
- [ ] Do not remove or reinitialize Postgres or Redis bind mounts.

### Task 3: Recreate App As Real Git Checkout

**Files:**
- Replace: `/mnt/storage/docker/lunarleague/app`
- Source: `https://github.com/bulbousoars/LunarLeague.git`

- [ ] Move the old app directory to the backup path.
- [ ] Clone `https://github.com/bulbousoars/LunarLeague.git` to `/mnt/storage/docker/lunarleague/app`.
- [ ] Restore the preserved `deploy/.env` to `/mnt/storage/docker/lunarleague/app/deploy/.env` with mode `0640`.
- [ ] Verify `git status --short --branch` and `git remote -v`.

### Task 4: Add Narrow Deploy Authorization

**Files:**
- Create: `/usr/local/sbin/deploy-lunarleague`
- Create: `/etc/sudoers.d/lunarleague-agent-deploy`

- [ ] Create group `lunarleague-deploy` if missing.
- [ ] Add `cursor-agent`, `codex-agent`, `claude-agent`, and `gemini-agent` to `lunarleague-deploy` if the accounts exist.
- [ ] Set app checkout group to `lunarleague-deploy` with group read/write on source files and no world access to `deploy/.env`.
- [ ] Create root-owned wrapper that changes to `/mnt/storage/docker/lunarleague/app`, fetches `main`, resets to `origin/main`, and runs `docker compose -f docker-compose.yml -f docker-compose.realestate.yml --env-file .env up -d --build --remove-orphans`.
- [ ] Add sudoers allowing `lunarleague-deploy` to run only `/usr/local/sbin/deploy-lunarleague` without a password.
- [ ] Validate sudoers with `visudo -cf /etc/sudoers.d/lunarleague-agent-deploy`.

### Task 5: Deploy And Verify

**Files:**
- Compose: `/mnt/storage/docker/lunarleague/app/deploy/docker-compose.yml`
- Compose overlay: `/mnt/storage/docker/lunarleague/app/deploy/docker-compose.realestate.yml`

- [ ] Run `/usr/local/sbin/deploy-lunarleague`.
- [ ] Run migrations if the current release requires them.
- [ ] Verify containers are running with `docker compose ps`.
- [ ] Verify API health at `http://192.168.1.218:8020/healthz`.
- [ ] Verify web route at `http://192.168.1.218:3020/`.
- [ ] Verify internal Traefik route `https://lunarleague.dugganco.com` if reachable from the workstation.

### Task 6: Update Docs

**Files:**
- Update: `\\wsl$\Ubuntu\home\dduggan\.claude\projects\-mnt-c-Users-danie\memory\proxmox-infrastructure.md`
- Update: `D:\Projects\LunarLeague\docs\AGENT_HOMELAB_SSH.md`

- [ ] Document the new deploy checkout and wrapper.
- [ ] Document that OpenBao supplies identity and sudoers/group membership supplies deploy authorization.
- [ ] Do not write secret values into docs.
