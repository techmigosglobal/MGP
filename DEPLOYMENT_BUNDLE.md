# MGP deployment bundle

This workspace is a clean-start Material Gate Pass deployment. The Docker image archive is distributed separately from the source bundle so it can be loaded on an offline server.

## Server installation

1. Copy the source bundle to the server and extract it.
2. Load the pinned images:

   ```sh
   docker load -i MGP-docker-images-20260908.tar
   ```

3. Create the local secret file (never commit or distribute it):

   ```sh
   MGP_TECHNICAL_PASSWORD='use-a-long-random-secret' ./scripts/bootstrap-env.sh
   ```

4. Start the stack:

   ```sh
   docker compose up -d --build
   docker compose ps
   ```

The first PostgreSQL start applies `db/init/001_schema.sql`; the bootstrap service then creates the five constrained MGP roles. No MGP users or business records are seeded. Provision users with the separate Directus technical administrator account.

## Included services

- PostgreSQL 16 with persistent `postgres_data` storage
- Directus 11.5.1 with the MGP workflow endpoint extension
- Nginx 1.27 serving the static frontend and same-origin `/api/` proxy
- One-shot bootstrap service for constrained MGP roles

The application is served at port 8080 by default. Set `PUBLIC_URL` and the database/Directus values in `.env` for the target host. Use `scripts/backup.sh` for host-side dumps and `scripts/restore-validate.sh` to validate a dump checksum; restore only into an isolated disposable PostgreSQL instance before production use.

For an existing MGP database, take and validate a backup first, then run `./scripts/apply-migrations.sh`. The desktop web image compiles Tailwind and the official PDF generator locally during its build; it does not load scripts, styles, fonts, or PDF assets from the network.

## Verification evidence

The source bundle was checked with `npm run check`, `npm test` (7 passing tests), shell syntax checks, and `docker compose config`. A live disposable stack passed API workflow and denial checks for all five MGP roles, including concurrent draft-number allocation (20 requests, 20 unique numbers). Browser rendering was checked in desktop and mobile headless Chrome; full manual click-through and print/PDF validation remain deployment acceptance work.
