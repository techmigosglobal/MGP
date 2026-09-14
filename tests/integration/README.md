# Integration tests

Integration tests run against PostgreSQL and verify migrations, transaction boundaries, concurrent workflow transitions, audit immutability, CSV rollback, and backup/restore behavior. They are intentionally separate from unit tests so a green `go test ./...` is not mistaken for live database evidence.
