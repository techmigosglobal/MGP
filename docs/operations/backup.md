# Backup and restore

The target recovery policy is a daily PostgreSQL backup with a maximum 24-hour recovery point and a monthly restore test. Generated documents must be backed up together with PostgreSQL metadata.

The repository provides `deploy/backup/backup.sh`. Configure `MGP_BACKUP_DIR`, run it from the Compose project directory, and schedule it with the site server's task scheduler. It produces a compressed PostgreSQL dump, a document archive, SHA-256 checksums, and a timestamped log.

Verify every backup before retention. Keep at least 30 daily backups and one monthly copy on separate storage. A backup that has not been restored into disposable PostgreSQL is not release evidence.

Restore procedure:

1. Stop normal writes or place the application in maintenance mode.
2. Restore the PostgreSQL dump into a disposable PostgreSQL instance first.
3. Verify migration state, row counts, an audit event, and document hashes.
4. Restore the approved backup into the target database and document volume.
5. Start the app, check readiness, authenticate a test account, and record the restore result.
