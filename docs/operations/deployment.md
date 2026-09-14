# Offline-LAN deployment

## Build and transfer

On a connected build machine:

```sh
cp .env.example .env
# Set a real POSTGRES_PASSWORD and MGP_SESSION_KEY
make docker
docker save mgp:local | gzip > mgp-local.tar.gz
```

Transfer the image archive, repository deployment files, and `.env` through the approved operational process. Never commit `.env`, passwords, signing material, or database data.

On the target server:

```sh
docker load < mgp-local.tar.gz
docker compose up -d
docker compose ps
curl http://127.0.0.1:8080/health/ready
```

Create the first MGP Admin from the server operator shell:

```sh
MGP_DATABASE_URL='postgres://mgp_app:...@127.0.0.1:5432/mgp?sslmode=disable' \
MGP_SESSION_KEY='same-value-as-.env' \
go run ./cmd/mgp-admin -email admin@example.com -name 'MGP Admin' -password 'long-password' -role ADMIN
```

The command requires the Go source/toolchain. For an offline production server, run it on a controlled administration workstation or provide a separately built provisioning utility. Do not expose PostgreSQL to the LAN; users access only the app port.

## Upgrade and rollback

1. Take and verify a database backup.
2. Load the new prebuilt image.
3. Update `MGP_IMAGE` in `.env` and run `docker compose up -d`.
4. Confirm `/health/ready`, login, and a read-only register check.
5. Run the acceptance smoke test.

Rollback is the same process using the previous image tag. Do not remove `postgres_data` or `documents_data`. Migrations are forward-only; test rollback behavior on a disposable copy before production use.

## LAN and HTTPS

Bind the app port to the site server's LAN address through the host firewall. For HTTPS, place an approved internal reverse proxy in front of the app, set `MGP_SECURE_COOKIE=true`, and preserve the `X-Forwarded-Proto`/host policy appropriate to that proxy.
