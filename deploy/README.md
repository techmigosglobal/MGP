# MGP offline deployment bundle

This directory is the server-PC deployment definition for MGP. The offline release archive also contains the prebuilt MGP and PostgreSQL images, so the target server does not need Node.js, Go, npm, or internet access.

## Install

1. Install Docker Engine or Docker Desktop with Compose on the server PC.
2. Extract the release archive into a dedicated directory.
3. Copy `.env.example` to `.env` and replace both placeholder secrets.
4. Start the services:

```sh
docker load < images/mgp-images.tar.gz
docker compose --env-file .env up -d
docker compose ps
curl http://127.0.0.1:8080/health/ready
```

5. Create the first administrator. The utility is already inside the MGP image:

```sh
docker compose --env-file .env run --rm --entrypoint /usr/local/bin/mgp-admin app \
  -username admin -name 'MGP Admin' -pin 482617 -role ADMIN
```

Create the INVENTORY, ISSUING, SECURITY, and VIEWER accounts in the same way. Change the sample PINs before operational use. The application accepts username + six-digit PIN only; email/password login is not enabled.

## Operations

```sh
./scripts/backup.sh
./scripts/restore-verify.sh backups/<database-dump>
```

Before an upgrade, create and verify a backup. Load the new image archive, update `MGP_IMAGE` in `.env`, and run `docker compose --env-file .env up -d`. For rollback, restore the previous image tag and use the documented backup/restore procedure. Never run `docker compose down -v` on an installation containing real data.

The included `SHA256SUMS` file verifies the release archive contents. Keep `.env`, backup files, and the Docker volumes outside source control.
