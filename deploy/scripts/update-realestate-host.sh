#!/usr/bin/env bash
# Live stack on VM 218 (and similar): base compose + realestate overlay (bind mounts, 8020/3020).
# Run on the Linux host inside the repo clone, e.g.
#   cd /mnt/storage/docker/lunarleague/app && ./deploy/scripts/update-realestate-host.sh
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"

BRANCH="${LUNARLEAGUE_DEPLOY_BRANCH:-main}"
git fetch origin "$BRANCH"
git checkout "$BRANCH"
git reset --hard "origin/$BRANCH"

cd "$ROOT/deploy"
docker compose -f docker-compose.yml -f docker-compose.realestate.yml --env-file .env up -d --build --remove-orphans

echo "Stack updated. API :${API_PORT:-8020}, web :${WEB_PORT:-3020} (from deploy/.env)."
