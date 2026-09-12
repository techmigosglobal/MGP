#!/bin/sh
set -eu

if [ ! -f .env ]; then
  echo "Create .env first with scripts/bootstrap-env.sh" >&2
  exit 1
fi

set -a
. ./.env
set +a

docker compose exec -T postgres psql -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d "$POSTGRES_DB" < db/migrations/002_desktop_workflow.sql
echo "MGP desktop workflow migration applied."
