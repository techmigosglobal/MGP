# ADR 0001: Go modular monolith with server-rendered HTML

## Decision

Use Go with PostgreSQL, templ, HTMX, Alpine.js, TailwindCSS, and Docker Compose. Keep domain modules inside one deployable server.

## Rationale

The application is offline-LAN-first, desktop-focused, and small enough that a modular monolith keeps deployment, backup, authorization, and upgrades understandable. Server-rendered HTML keeps browser authorization concerns narrow while HTMX provides responsive interactions. PostgreSQL transactions and row locks are a strong fit for atomic numbering and workflow transitions.

## Consequences

The system does not require a Node runtime or SPA build at production runtime. The code must keep handler, service, repository, and template boundaries clear. If scale or integration needs change materially, modules can be extracted behind explicit interfaces later.
