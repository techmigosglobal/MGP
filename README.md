# MGP Control Room

MGP is an offline-LAN material gate-pass control system for one site and approximately 100 users. It records preparation, independent approval, physical pass-out, return, revisions, documents, and immutable audit history.

## Stack

- Go modular monolith with `net/http` and PostgreSQL.
- `templ` server-rendered HTML, HTMX for partial interactions, Alpine.js for local UI state, and TailwindCSS for generated utilities/design tokens.
- Docker Compose with PostgreSQL and a prebuilt application image.
- Local accounts with Argon2id password hashes, database-backed sessions, CSRF protection, and backend-authoritative RBAC.

## Development

```sh
cp .env.example .env
# Set POSTGRES_PASSWORD and a random MGP_SESSION_KEY in .env
npm --prefix web ci
make generate
make assets
make test
make build
```

The server requires `MGP_DATABASE_URL` and `MGP_SESSION_KEY`. For a local process, set them explicitly, for example:

```sh
export MGP_DATABASE_URL='postgres://mgp_app:password@127.0.0.1:5432/mgp?sslmode=disable'
export MGP_SESSION_KEY='replace-with-at-least-32-random-characters'
export MGP_SECURE_COOKIE=false
make dev
```

Create the first application account after PostgreSQL is ready:

```sh
go run ./cmd/mgp-admin -email admin@example.com -name "MGP Admin" -password 'use-a-long-password' -role ADMIN
```

`mgp-admin` is a deployment/operator command. It is separate from the permissions of an MGP ADMIN account inside the application.

## Docker/LAN deployment

Build the image on a connected build machine, transfer the image or an image archive to the offline server, then run:

```sh
make docker
docker compose up -d
docker compose ps
curl http://127.0.0.1:8080/health/ready
```

The production container contains no Node.js runtime. Node is used only in the Docker build stage to generate CSS and vendor static assets. PostgreSQL data and generated documents are named volumes and are not removed by application updates.

See [docs/operations/deployment.md](docs/operations/deployment.md) and [docs/operations/backup.md](docs/operations/backup.md) for server setup, backups, upgrades, and restore verification.

## Commands

| Command | Purpose |
| --- | --- |
| `make generate` | Generate Go code from templ files |
| `make assets` | Install locked web dependencies and build/vendor assets |
| `make dev` | Generate assets and run the Go server |
| `make test` | Run unit and package tests |
| `make lint` | Run formatting and `go vet` checks |
| `make build` | Produce a standalone server binary |
| `make docker` | Build the multi-stage production image |

## Repository layout

`cmd/` contains executable entrypoints. `internal/` contains domain services, repositories, policy, HTTP handlers, sessions, database, files, and reports. `web/templates/` contains templ layouts and pages. `db/migrations/` is the versioned schema source. `deploy/` contains the production image and Compose definition. `tests/` is reserved for integration and browser acceptance suites.

The former Vite/Directus application is preserved outside the active tree at `/home/vinay/Documents/MGP-legacy-20260914.tar.gz`; its SHA-256 is recorded beside the archive.
