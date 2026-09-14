#!/bin/sh
set -eu

BACKUP_DIR=${MGP_BACKUP_DIR:-./backups}
STAMP=$(date -u +%Y%m%dT%H%M%SZ)
mkdir -p "$BACKUP_DIR"

docker compose exec -T postgres pg_dump -Fc -U "${POSTGRES_USER:-mgp_app}" -d "${POSTGRES_DB:-mgp}" > "$BACKUP_DIR/mgp-$STAMP.dump"
docker compose exec -T app tar -C /var/lib/mgp/documents -czf - . > "$BACKUP_DIR/mgp-documents-$STAMP.tar.gz"
sha256sum "$BACKUP_DIR/mgp-$STAMP.dump" "$BACKUP_DIR/mgp-documents-$STAMP.tar.gz" > "$BACKUP_DIR/mgp-$STAMP.sha256"
find "$BACKUP_DIR" -type f -mtime +30 -delete
echo "backup complete: $STAMP"
