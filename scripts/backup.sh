#!/usr/bin/env sh
set -eu

: "${POSTGRES_DB:?Set POSTGRES_DB in .env}"
: "${POSTGRES_USER:?Set POSTGRES_USER in .env}"
mkdir -p backups
stamp="$(date -u +%Y%m%dT%H%M%SZ)"
archive="backups/mgp-${stamp}.dump"
docker compose exec -T postgres pg_dump -U "$POSTGRES_USER" -d "$POSTGRES_DB" -Fc > "$archive"
sha256sum "$archive" > "${archive}.sha256"
printf 'Created %s and checksum manifest. Restore only with a disposable validation database first.\n' "$archive"
