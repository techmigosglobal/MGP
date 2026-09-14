#!/bin/sh
set -eu

DUMP=${1:?usage: restore-verify.sh path/to/mgp.dump}
NAME="mgp-restore-$RANDOM"
docker run --rm -d --name "$NAME" -e POSTGRES_PASSWORD=verify postgres:16-alpine >/dev/null
cleanup() { docker rm -f "$NAME" >/dev/null 2>&1 || true; }
trap cleanup EXIT
until docker exec "$NAME" pg_isready -U postgres >/dev/null 2>&1; do sleep 1; done
docker exec -i "$NAME" createdb -U postgres mgp_restore
docker exec -i "$NAME" pg_restore -U postgres -d mgp_restore < "$DUMP"
docker exec "$NAME" psql -U postgres -d mgp_restore -Atc "SELECT count(*) FROM audit_events;"
echo "restore verification completed"
