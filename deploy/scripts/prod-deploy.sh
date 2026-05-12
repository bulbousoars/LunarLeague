#!/usr/bin/env bash
# Run ON your production VPS inside the LunarLeague clone (Linux).
#
# Usage:
#   ./deploy/scripts/prod-deploy.sh              # pull + rebuild containers
#   MIGRATE=1 ./deploy/scripts/prod-deploy.sh    # also run DB migrations
#
# Prerequisites: deploy/.env configured (see deploy/.env.production.example), DNS → this host if using Caddy.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$REPO_ROOT/deploy"

if [[ -f docker-compose.caddy.yml ]]; then
  COMPOSE=(docker compose -f docker-compose.yml -f docker-compose.caddy.yml --env-file .env)
  echo "Using TLS stack: docker-compose.yml + docker-compose.caddy.yml"
else
  COMPOSE=(docker compose -f docker-compose.yml --env-file .env)
  echo "Using base stack only (no docker-compose.caddy.yml here)"
fi

OVERLAY="${LUNARLEAGUE_COMPOSE_OVERLAY:-}"
if [[ -n "$OVERLAY" && -f "docker-compose.${OVERLAY}.yml" ]]; then
  COMPOSE+=(-f "docker-compose.${OVERLAY}.yml")
  echo "Also merging docker-compose.${OVERLAY}.yml"
fi

BRANCH="${LUNARLEAGUE_DEPLOY_BRANCH:-main}"
git fetch origin "$BRANCH"
git checkout "$BRANCH"
git reset --hard "origin/$BRANCH"

"${COMPOSE[@]}" up -d --build --remove-orphans

if [[ "${MIGRATE:-}" == "1" ]]; then
  "${COMPOSE[@]}" run --rm api migrate up
fi

echo "Deploy finished. Tail logs from deploy/: ${COMPOSE[*]} logs -f api worker web caddy"
