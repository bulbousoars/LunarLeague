# VM-side periodic deploy (no GitHub secrets)

Use this when **lunarleague.dugganco.com** (or any host) should **pull the public repo on a schedule** and **rebuild Docker** without tying credentials to GitHub Actions.

## What it does

[`deploy/scripts/vm-periodic-deploy.sh`](../deploy/scripts/vm-periodic-deploy.sh) runs [`prod-deploy.sh`](../scripts/prod-deploy.sh) with **`MIGRATE=1`**, then **`curl` `/healthz`** on the API port from `deploy/.env` (`API_PORT`, default `8000`).

`prod-deploy.sh` already:

- `git fetch` / `checkout` / `reset --hard` to `origin/$LUNARLEAGUE_DEPLOY_BRANCH` (default `main`)
- `docker compose up -d --build --remove-orphans` with optional **`docker-compose.caddy.yml`** and **`LUNARLEAGUE_COMPOSE_OVERLAY`**
- `api migrate up` when `MIGRATE=1`

## One-time VM setup

1. **Clone** the public repo (or your fork) and configure **`deploy/.env`** as in [SELF_HOSTING.md](SELF_HOSTING.md) (SMTP, `PUBLIC_*`, etc.). That file stays on the VM and is **gitignored** — nothing private goes into GitHub.

2. **Git read access** on the VM only, e.g.  
   - SSH deploy key with **read** access to the repo (add public key in GitHub → Deploy keys), or  
   - `https` remote + credential helper.  
   Do **not** commit keys into the repo.

3. **Install the script** (pick one path; adjust if your clone is not under `/mnt/storage/docker/lunarleague/app`):

   ```bash
   sudo install -m 755 /mnt/storage/docker/lunarleague/app/deploy/scripts/vm-periodic-deploy.sh /usr/local/sbin/lunarleague-auto-deploy
   ```

4. **Optional config** (branch + compose overlay — still not your SMTP secrets):

   ```bash
   sudo mkdir -p /etc/lunarleague
   sudo cp /mnt/storage/docker/lunarleague/app/deploy/systemd/auto-deploy.env.example /etc/lunarleague/auto-deploy.env
   sudo chmod 640 /etc/lunarleague/auto-deploy.env
   sudo chown root:root /etc/lunarleague/auto-deploy.env
   sudoedit /etc/lunarleague/auto-deploy.env
   ```

5. **systemd timer** (edit paths in the unit files if your clone differs, then install):

   ```bash
   sudo cp /mnt/storage/docker/lunarleague/app/deploy/systemd/lunarleague-auto-deploy.service /etc/systemd/system/
   sudo cp /mnt/storage/docker/lunarleague/app/deploy/systemd/lunarleague-auto-deploy.timer /etc/systemd/system/
   # Edit WorkingDirectory, User, ExecStart in .service if needed
   sudo systemctl daemon-reload
   sudo systemctl enable --now lunarleague-auto-deploy.timer
   sudo systemctl list-timers | grep lunarleague
   ```

   The default timer runs **every 6 hours** with a **5-minute** random delay. Adjust `OnCalendar` in the timer file.

6. **User permissions**: the `User=` in the service must be able to run **`docker compose`** and **`git`** on that tree. Common pattern: user in the **`docker`** group, or a narrow **`sudo`** wrapper (see homelab OpenBao deploy plan).

## Manual test

```bash
sudo systemctl start lunarleague-auto-deploy.service
journalctl -u lunarleague-auto-deploy.service -n 50 --no-pager
```

## Cron alternative

```cron
0 */6 * * * dduggan LUNARLEAGUE_COMPOSE_OVERLAY=realestate /mnt/storage/docker/lunarleague/app/deploy/scripts/vm-periodic-deploy.sh >>/var/log/lunarleague-auto-deploy.log 2>&1
```

## Relation to GitHub Actions

[DEPLOY_GITHUB_DUGGANCO.md](DEPLOY_GITHUB_DUGGANCO.md) is optional CI from GitHub. This VM timer is **fully local**: no `LUNARLEAGUE_SSH_*` secrets in the public repo—only this documented flow on your server.

## Install from Windows without manual SSH

If you prefer not to open an interactive SSH session, run from this repo on **danspc** (PowerShell):

```powershell
Set-Location D:\Projects\LunarLeague   # or your clone
.\deploy\scripts\Install-LunarLeagueVmAutoDeploy.ps1
```

That script uses [`deploy/scripts/Invoke-RealestateAdmin.ps1`](../deploy/scripts/Invoke-RealestateAdmin.ps1), which obtains a short-lived **root** certificate via the homelab **cursor-admin** SSH broker (`New-AgentSshSession.ps1`) and runs the install steps on **192.168.1.218** (realestate). The installer:

- Writes `vm-periodic-deploy.sh`, `prod-deploy.sh`, and systemd units on the VM
- Strips CR characters from shell scripts, `/etc/lunarleague/auto-deploy.env`, and unit files (Windows-sourced line endings)
- Enables `lunarleague-auto-deploy.timer` and starts the service once

The shipped systemd unit uses **`User=root`** on this homelab layout because the bind-mounted clone under `/mnt/storage/docker/lunarleague/app` is root-owned and `dduggan` is not in `lunarleague-deploy`; adjust if your tree is owned by a deploy user.

After changing the installer or units locally, re-run `Install-LunarLeagueVmAutoDeploy.ps1` so the VM picks up updates. Application code still arrives via **`git pull`** inside `vm-periodic-deploy.sh`, so push to `origin/main` (or your configured branch) before expecting the timer to deploy new commits.
