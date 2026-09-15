# Material Gate Pass Control Room — Product Requirements

## Product objective

Provide a dependable, offline-LAN system for controlling material movement through a gate. The product must make the person preparing a pass, the person approving it, and the person physically releasing or receiving material independently accountable. Historical records must remain useful even after master data is archived.

## Scope and constraints

The first release targets one site and approximately 100 users. It uses a fresh PostgreSQL database, local accounts, desktop browsers, filesystem document storage, and five fixed roles. It is not a public SaaS, a mobile app, or a legally certified digital-signature service. Internal electronic approvals are recorded with approver identity, role, timestamp, reference, and record hash.

The application is an offline-LAN-first modular monolith: Go, PostgreSQL, templ, HTMX, Alpine.js, TailwindCSS, and Docker Compose. The browser is a presentation layer; authorization and workflow state are always enforced on the server.

## Roles

| Role | Responsibilities | Must not do |
| --- | --- | --- |
| ADMIN | Manage users, settings, master data, records, reports, and operational actions subject to separation of duty | Bypass audit or approve their own pass |
| INVENTORY | Create and own drafts, maintain inventory/consignee data, submit and revise owned passes | Approve or pass out material |
| ISSUING | Independently approve or reject submitted passes | Pass out or return material |
| SECURITY | Record physical pass-out and return for approved passes | Create or approve passes |
| VIEWER | Read permitted registers, reports, and audit history | Mutate any record |

Every decision evaluates `role + action + current state + ownership + separation of duty`. Hiding a button is only a usability feature; it is not an authorization control.

## Functional requirements

### Authentication and administration

- Local username + six-digit PIN accounts use Argon2id hashes. Login failures are rate-limited and PINs are never displayed.
- Sessions are server-side, expiring, HttpOnly, SameSite cookies.
- All authenticated mutations require a session CSRF token.
- Admins can create, suspend, reset, and assign fixed-role accounts.
- Suspended accounts cannot authenticate or continue using an existing session.
- Authentication and account changes produce audit events.

### Gate-pass workflow

1. Inventory or Admin creates a validated draft.
2. The database allocates a unique pass number atomically by directorate, project, and year.
3. The creator edits and submits the draft.
4. Issuing or an eligible Admin independently approves or rejects it.
5. Rejection requires a reason; the creator can create a linked revision.
6. Security records pass-out and a control number only after approval.
7. Security records return for returnable material.
8. Non-returnable passes complete after pass-out; returnable passes complete after return.
9. Every mutation is an append-only audit event. Concurrent transitions are serialized with row locks and current-state checks.

Required routes are documented in [workflow.md](workflow.md). Invalid transitions must return a clear error and must not change the record.

### Master data

Inventory items and consignees support archive behavior. Imports validate every CSV row before transaction commit. Failed imports roll back completely. Pass records store snapshots of operational values so future master-data changes do not rewrite history.

### Documents and reports

- Register search/filter, dashboard counts, overdue returns, role-scoped reports, and audit history are server-calculated.
- A4 HTML preview and server-generated PDF are stored as filesystem artifacts with PostgreSQL metadata and SHA-256 hashes.
- Approval details are internal electronic records, not qualified/certificate-based signatures.

## Non-functional requirements

- Fresh checkout builds with no Node runtime in the production image.
- The application starts from a prebuilt image without downloading packages on the target server.
- PostgreSQL migrations are versioned and run on startup.
- Health and readiness endpoints expose application/database state.
- Daily backup gives a maximum 24-hour recovery point; restore is tested monthly.
- Updates preserve named database/document volumes and support rollback to the previous image.
- Keyboard navigation, visible focus, clear errors, semantic form labels, and print-friendly A4 output are required.

## Release acceptance

The release is accepted only when the unit policy matrix, PostgreSQL integration lifecycle, concurrent transition tests, authenticated browser tests for all five roles, A4/PDF checks, clean offline installation, restart checks, backup/restore verification, and LAN access checks pass. A Go build alone is not release evidence for browser, PDF, backup, or deployment behavior.
