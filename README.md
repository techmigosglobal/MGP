# Material Gate Pass System

This workspace now contains the backend-authoritative MGP implementation. The former browser-only prototype is retired; the compatibility file [material_gate_pass_offline.html](material_gate_pass_offline.html) points to the Docker-served app.

## Start locally

1. Generate `.env` without exposing the database secret: `MGP_TECHNICAL_PASSWORD='use-a-password-you-choose' scripts/bootstrap-env.sh`.
2. Start the stack with `docker compose up -d --build`, then inspect `docker compose logs -f bootstrap`.
3. Open <http://localhost:8080/>.

The technical Directus Administrator is created from `DIRECTUS_ADMIN_EMAIL` and `DIRECTUS_ADMIN_PASSWORD`. The bootstrap creates only the five constrained MGP roles; it creates no application users or business records. Provision the first MGP Admin from Directus administration, then use that account for normal MGP administration.

## Verification

Run `npm run build`, `npm run check`, `npm test`, and `docker compose --env-file .env config --quiet`.

For an existing persistent deployment, apply `./scripts/apply-migrations.sh` after taking a validated backup and before starting the desktop workflow release. Fresh Docker volumes apply both schema files automatically.

`workflow.md` is the authoritative role, state-transition, denial, audit, and acceptance matrix. `npm run verify:docker` starts an isolated disposable stack for full role/API acceptance; `npm run verify:browser` is its browser/PDF follow-up after the Docker stack is running.

`scripts/backup.sh` creates a checksummed PostgreSQL custom-format archive. `scripts/restore-validate.sh` checks an archive without touching the running database. Restore remains a technical-host maintenance operation.

The endpoint extension enforces workflow role/state checks and uses row locks plus `next_gate_pass_number` for concurrent numbering. Audit events are server-generated and protected by a PostgreSQL mutation trigger.
