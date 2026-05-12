#!/usr/bin/env bash
# Periodic deploy for a self-hosted Lunar League VM (no GitHub Actions).
# Pulls public origin, rebuilds containers, runs migrations, optional health check.
#
# Install once on the VM:
#   sudo install -m 755 deploy/scripts/vm-periodic-deploy.sh /usr/local/sbin/lunarleague-auto-deploy
#   sudo install -m 640 deploy/systemd/auto-deploy.env.example /etc/lunarleague/auto-deploy.env
#   sudoedit /etc/lunarleague/auto-deploy.env   # set paths/branch/overlay only — no SMTP secrets here
#   sudo install deploy/systemd/lunarleague-auto-deploy.service /etc/systemd/system/
#   sudo install deploy/systemd/lunarleague-auto-deploy.timer /etc/systemd/system/
#   sudo systemctl daemon-reload && sudo systemctl enable --now lunarleague-auto-deploy.timer
#
# Or run from cron:
#   0 */6 * * * LUNARLEAGUE_COMPOSE_OVERLAY=realestate /path/to/LunarLeague/deploy/scripts/vm-periodic-deploy.sh >>/var/log/lunarleague-auto-deploy.log 2>&1
#
# Git auth: configure on the VM once (deploy key read-only to GitHub, or https + credential helper).
# Never commit deploy keys or /etc/lunarleague/auto-deploy.env with secrets into the public repo.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

if [[ -f /etc/lunarleague/auto-deploy.env ]]; then
  set -a
  # shellcheck disable=SC1091
  source /etc/lunarleague/auto-deploy.env
  set +a
fi

: "${LUNARLEAGUE_DEPLOY_BRANCH:=main}"
export LUNARLEAGUE_DEPLOY_BRANCH
export LUNARLEAGUE_COMPOSE_OVERLAY="${LUNARLEAGUE_COMPOSE_OVERLAY:-}"
export MIGRATE=1

log() { echo "[$(date -Iseconds)] $*"; }

log "starting auto-deploy repo_root=$REPO_ROOT branch=$LUNARLEAGUE_DEPLOY_BRANCH"

if [[ ! -d "$REPO_ROOT/.git" ]]; then
  log "error: no .git at $REPO_ROOT"
  exit 1
fi

if [[ ! -f "$REPO_ROOT/deploy/.env" ]]; then
  log "error: missing $REPO_ROOT/deploy/.env"
  exit 1
fi

# Refresh repo (including this script) before prod-deploy. Avoids a chicken-and-egg where an old
# vm-periodic-deploy.sh cannot reach prod-deploy.sh fixes until someone manually git pull's on the VM.
git -C "$REPO_ROOT" fetch origin "$LUNARLEAGUE_DEPLOY_BRANCH"
git -C "$REPO_ROOT" checkout "$LUNARLEAGUE_DEPLOY_BRANCH"
git -C "$REPO_ROOT" reset --hard "origin/$LUNARLEAGUE_DEPLOY_BRANCH"

# prod-deploy.sh is tracked as non-executable (100644); invoke with bash so git pull does not break the timer.
bash "$SCRIPT_DIR/prod-deploy.sh"

cd "$REPO_ROOT/deploy"
API_PORT="$(grep -E '^API_PORT=' .env 2>/dev/null | cut -d= -f2- | tr -d '\r"' | head -1 || true)"
API_PORT="${API_PORT:-8000}"
if curl -fsS "http://127.0.0.1:${API_PORT}/healthz" >/dev/null; then
  log "healthz OK on :${API_PORT}"
else
  log "warning: healthz failed on :${API_PORT}"
  exit 1
fi

log "auto-deploy finished OK"
