#!/usr/bin/env sh
set -eu

archive="${1:?Usage: scripts/restore-validate.sh backups/mgp-*.dump}"
test -f "$archive"
test -f "${archive}.sha256"
sha256sum -c "${archive}.sha256"
echo "Archive integrity passed. Restore this archive only into a disposable PostgreSQL instance; this script deliberately never changes the running MGP database."
