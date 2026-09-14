# Architecture overview

MGP is a modular monolith. HTTP handlers parse requests and render templ components; services and repositories own business rules and SQL; the database is the source of truth for workflow state, counters, and audit history.

```text
Browser
  -> net/http routes and middleware
     -> handlers
        -> auth / gatepass / masterdata / reports services
           -> repositories and PostgreSQL
     -> templ pages + HTMX fragments
Filesystem documents <- documents service -> PostgreSQL metadata
```

The fixed role policy is in `internal/rbac`. Workflow state transitions are in `internal/gatepass`; persistence locks the record with `FOR UPDATE` before rechecking policy. The UI may hide actions for clarity, but it is never trusted for authorization.
