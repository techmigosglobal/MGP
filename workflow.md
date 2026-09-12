# Material Gate Pass Workflow

This document is the operational contract for the backend-authoritative MGP application. The technical Directus Administrator is a deployment account and is never an MGP business role.

## State flow

`DRAFT → SUBMITTED → APPROVED → PASSED_OUT → RETURNED`

An Issuing Officer can instead move `SUBMITTED → NOT_APPROVED`. The original rejected record remains immutable; Inventory creates a linked revision as a new `DRAFT`. A non-returnable pass remains `PASSED_OUT` after physical movement.

| Role | Desktop workspace and permitted work | Server enforcement and denials | Audit / acceptance evidence |
| --- | --- | --- | --- |
| `ADMIN` | Manages MGP users, organization details, masters, reports, audit log, backup/restore guidance, and may assist with a workflow stage only when separation rules allow it. | Cannot use the technical Directus account as an application role. Cannot approve/reject a pass they created and cannot complete more than one operational stage of one pass. Only Admin can manage users/settings and view backup status. | `CREATE_USER`, `UPDATE_USER`, `UPDATE_ORGANIZATION`, master, draft and transition events. Test self-approval and repeated-stage denials. |
| `INVENTORY` | Creates and edits only own drafts; maintains inventory/consignee masters; submits complete drafts; creates a revision from their rejected pass; exports masters. | Cannot edit after submission, approve/reject, pass out, return, manage users/settings, or read audit events. Revision requires a rejected original created by that operator. | `CREATE_DRAFT`, `EDIT_DRAFT`, `SUBMIT`, `CREATE_REVISION`, master events. Test draft ownership, required items and expected date for returnable passes. |
| `ISSUING` | Reviews the submitted queue; approves or rejects with a required rejection reason; reads audit and operational reports; exports masters. | Cannot create/edit a pass, pass out or return. Creator approval/rejection is denied even if the actor otherwise has Issuing authority. | `APPROVE` or `REJECT` with prior/current state, actor and optional reason. Test a creator/approver conflict and invalid-state transition. |
| `SECURITY` | Sees only `APPROVED`, `PASSED_OUT`, and `RETURNED` passes; records the security control number at pass-out and actual date at return. | Cannot see drafts, submissions or rejections; cannot prepare, approve/reject, manage masters/users/settings, or read audit. Pass-out requires an approved record and control number; return requires a returnable passed-out record. | `PASS-OUT` includes security control number; `RETURN` includes actual date. Test scoped list/read denial and non-returnable return denial. |
| `VIEWER` | Read-only register, pass details, official PDFs for approved+ records, and operational reports. | No creation, edit, revision, transition, master management/export, audit, users/settings, or backup access. | Read access generates no mutation events. Test every mutation route returns `403`. |

## Official documents

Draft, submitted, and rejected records are not official documents. The A4 preview/print/download PDF is available only for `APPROVED`, `PASSED_OUT`, and `RETURNED` records. It contains organization identity, pass number/status, consignee, authority, material lines, movement/security data, and signature areas. The downloadable PDF and print preview use the same backend pass data.

## Recovery boundary

The UI displays backup status and the approved host procedure only. `scripts/backup.sh` creates the host-side archive; `scripts/restore-validate.sh` validates it in isolation. Restoration into operational data is not an in-application action.

## Required acceptance run

1. Run `npm run build`, `npm run check`, and `npm test`.
2. Run `npm run verify:docker` for an isolated all-role API and Docker check.
3. With the isolated stack still running, run `npm run verify:browser` to validate each role UI, desktop layout, clean browser console, and generated official PDF.
4. Treat target-server backup/restore and real-operator acceptance as separate production evidence.
