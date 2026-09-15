#!/bin/sh
set -eu

VERSION=${MGP_RELEASE_VERSION:-$(date -u +%Y%m%d-%H%M%S)}
IMAGE="mgp:${VERSION}"
OUTPUT_DIR=${MGP_PACKAGE_DIR:-dist}
STAGING_DIR="${OUTPUT_DIR}/MGP-deployment-${VERSION}"
ARCHIVE="${OUTPUT_DIR}/MGP-deployment-${VERSION}.tar.gz"

case "$STAGING_DIR" in
  ""|"/"|"."|"..") echo "refusing unsafe staging directory: $STAGING_DIR" >&2; exit 1 ;;
esac
rm -rf "$STAGING_DIR"
mkdir -p "$STAGING_DIR/images" "$STAGING_DIR/scripts" "$STAGING_DIR/docs"

make generate
make lint
make test
docker build -f deploy/Dockerfile -t "$IMAGE" .

cp deploy/docker-compose.yml "$STAGING_DIR/docker-compose.yml"
cp deploy/README.md "$STAGING_DIR/README.md"
cp .env.example "$STAGING_DIR/.env.example"
sed -i "s/^MGP_IMAGE=.*/MGP_IMAGE=${IMAGE}/" "$STAGING_DIR/.env.example"
cp deploy/backup/backup.sh "$STAGING_DIR/scripts/backup.sh"
cp deploy/backup/restore-verify.sh "$STAGING_DIR/scripts/restore-verify.sh"
cp docs/operations/backup.md "$STAGING_DIR/docs/backup.md"
cp docs/operations/deployment.md "$STAGING_DIR/docs/deployment.md"
chmod 0750 "$STAGING_DIR/scripts/"*.sh

docker save "$IMAGE" postgres:16-alpine | gzip -n > "$STAGING_DIR/images/mgp-images.tar.gz"
tar -C "$OUTPUT_DIR" -czf "$ARCHIVE" "MGP-deployment-${VERSION}"
sha256sum "$ARCHIVE" > "${ARCHIVE}.sha256"

echo "created: $ARCHIVE"
echo "checksum: ${ARCHIVE}.sha256"
