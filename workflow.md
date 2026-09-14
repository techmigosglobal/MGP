# MGP Workflow Contract

This document is the authoritative state and role contract. The server must enforce it even when a request is sent directly without the normal UI.

## State machine

```text
DRAFT -> SUBMITTED -> APPROVED -> PASSED_OUT -> RETURNED
                  \-> NOT_APPROVED -> linked revision DRAFT
```

Non-returnable material ends its operational lifecycle at `PASSED_OUT`. Returnable material ends at `RETURNED`.

## Route contract

| Method | Route | Purpose |
| --- | --- | --- |
| GET | `/login` | Login form |
| POST | `/login` | Authenticate local account |
| POST | `/logout` | CSRF-protected logout |
| GET | `/dashboard` | Role-scoped operational summary |
| GET | `/gate-passes` | Register |
| GET | `/gate-passes/new` | Draft form |
| POST | `/gate-passes` | Create draft or linked revision |
| GET | `/gate-passes/{id}` | Detail and available actions |
| PATCH | `/gate-passes/{id}` | Update an owned draft |
| POST | `/gate-passes/{id}/submit` | Submit draft |
| POST | `/gate-passes/{id}/approve` | Independent approval |
| POST | `/gate-passes/{id}/reject` | Rejection with mandatory reason |
| POST | `/gate-passes/{id}/pass-out` | Security physical release |
| POST | `/gate-passes/{id}/return` | Security physical return |

All POST routes require an active account and a matching server-side CSRF token. All action routes lock the current row, reload state, evaluate policy, update only from the expected state, and append an audit event in the same transaction.

## Role matrix

| Action | ADMIN | INVENTORY | ISSUING | SECURITY | VIEWER |
| --- | --- | --- | --- | --- | --- |
| Create draft | Yes | Yes | No | No | No |
| Edit/submit owned draft | Yes | Owned only | No | No | No |
| Approve/reject submitted | Yes, if independent | No | Yes, if independent | No | No |
| Revise rejected pass | Yes | Owned only | No | No | No |
| Pass out approved pass | Yes, if independent | No | No | Yes | No |
| Return passed-out returnable | Yes, if independent | No | No | Yes | No |
| Read register | Yes | Yes | Yes | Operational states | Yes |
| Manage users/settings | Yes | No | No | No | No |

Admin operational actions still observe separation of duty. The creator cannot approve/reject their own pass. An actor who already participated in an earlier operational stage cannot perform the next independent stage.

## Required validation

- Returnable passes require an expected return date.
- Packages are positive whole numbers.
- Each pass has at least one item with a positive quantity and name.
- Directorate, project, consignee, purpose, and authority are required.
- Rejection requires a non-empty reason.
- Pass-out requires a security control number.
- Return is allowed only for an approved pass that has been passed out and is marked returnable.

## Audit contract

Each mutation records entity, entity ID, action, previous state, next state, actor ID, actor role, reason where applicable, pass number metadata, request ID, and timestamp. The database trigger rejects updates and deletes against `audit_events`.
