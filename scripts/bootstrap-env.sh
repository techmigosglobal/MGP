#!/usr/bin/env sh
set -eu

if [ -e .env ]; then
  echo ".env already exists; refusing to overwrite it." >&2
  exit 1
fi
: "${MGP_TECHNICAL_PASSWORD:?Set MGP_TECHNICAL_PASSWORD for the initial technical Directus administrator}"
technical_email="${MGP_TECHNICAL_EMAIL:-technical-admin@example.com}"
postgres_password="$(openssl rand -hex 32)"
directus_secret="$(openssl rand -hex 48)"
umask 077
{
  printf 'POSTGRES_DB=mgp\n'
  printf 'POSTGRES_USER=mgp_app\n'
  printf 'POSTGRES_PASSWORD=%s\n' "$postgres_password"
  printf 'DIRECTUS_SECRET=%s\n' "$directus_secret"
  printf 'DIRECTUS_ADMIN_EMAIL=%s\n' "$technical_email"
  printf 'DIRECTUS_ADMIN_PASSWORD=%s\n' "$MGP_TECHNICAL_PASSWORD"
  printf 'PUBLIC_URL=http://localhost\n'
} > .env
echo "Created .env with mode 600. Start with: docker compose up -d"
