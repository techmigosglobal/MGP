# 1. Current application assessment

| Area                     | Current status           | My assessment                                       |
| ------------------------ | ------------------------ | --------------------------------------------------- |
| Dashboard                | ✅ Implemented           | Good                                                |
| Returnable gate pass     | ✅ Implemented           | Good                                                |
| Non-returnable gate pass | ✅ Implemented           | Good                                                |
| Save Draft               | ✅ Implemented           | Good                                                |
| Edit Draft               | ✅ Implemented           | Good, but ownership should be enforced              |
| Submit for approval      | ✅ Implemented           | Good                                                |
| Approve / Reject         | ✅ Implemented           | Needs backend state validation                      |
| Security Pass Out        | ✅ Implemented           | Needs stronger validation                           |
| Mark Returned            | ✅ Implemented           | Good for full return                                |
| Inventory Master         | ✅ Implemented           | RBAC currently incorrect                            |
| Consignee Master         | ✅ Implemented           | RBAC currently incorrect                            |
| User Master              | ✅ Implemented           | Admin UI restriction exists                         |
| Search / Filters         | ✅ Implemented           | Good                                                |
| Reports                  | ✅ Implemented           | Good                                                |
| CSV Export               | ✅ Implemented           | Needs export permissions                            |
| CSV Import               | ✅ Implemented           | Needs import permissions                            |
| Audit Log                | ✅ Implemented           | Should be immutable/backend-generated               |
| Backup                   | ✅ Implemented           | Backend architecture must change this               |
| Restore                  | ✅ Implemented           | Admin-only + validation required                    |
| SQL Export               | ✅ Implemented           | Development/migration utility only                  |
| Settings                 | ✅ Implemented           | Good                                                |
| Draft printing           | ✅ Implemented           | Good                                                |
| Official printing        | ✅ Implemented           | Correctly restricted to approved+ states            |
| Auto gate-pass numbering | ✅ Implemented           | Must change for concurrent backend                  |
| Yearly numbering         | ⚠️ Partially           | `yearlyReset` exists but isn't actually consulted |
| Multi-user concurrency   | ❌ Not addressed         | Must be handled by backend                          |
| Separation of duty       | ❌ Not properly enforced | Must be added                                       |

Official printing is already sensibly limited to Approved, Passed Out, or Returned gate passes.

So architecturally, I would call this **feature-complete enough to begin backend implementation**, rather than spending much more time adding frontend features.

---

# 2. Most important problems before backend work

There are eight things I would fix in the backend design:

1. **Viewer can currently reach Inventory/Consignee modification UI.** Inventory has Add/Edit/Delete/Import without role checks.  Consignee Master has the same issue.
2. **Workflow functions don't independently authorize the actor.** `approveGatePass()`, `rejectGatePass()`, `passOutGatePass()` and `returnGatePass()` directly change state once called.  UI buttons are hidden according to permissions, but that is not sufficient.
3. Many privileged functions are deliberately exposed under `window.MGP`, including approval, deletion, restore and imports.  That's acceptable for this single-file prototype but absolutely cannot determine backend security.
4. **Admin can create and approve the same pass.** For your actual organization workflow, routine self-approval should be prohibited.
5. **Pass numbers are calculated as `MAX + 1`.**  Two simultaneous creators could generate the same number after moving to a multi-user backend.
6. Inventory/Consignee records are hard-deleted. Production should normally **archive** master data so historical references remain traceable.
7. Audit data currently lives alongside ordinary browser data. The backend audit/event table should be **append-only** and server-generated.
8. Backend transitions must use conditional/atomic updates so two officers cannot approve/pass-out the same record simultaneously.

OWASP specifically recommends least privilege, deny-by-default, and authorization checks **on every request**, rather than relying on interface visibility. ([OWASP Cheat Sheet Series][1]) NIST RBAC also explicitly supports separation-of-duty constraints, which fits the creator → approver → security workflow particularly well. ([NIST Computer Security Resource Center][2])

---

# 3. Recommended backend RBAC

I recommend keeping your existing five application roles:

| Code          | Role             | Responsibility                                          |
| ------------- | ---------------- | ------------------------------------------------------- |
| `ADMIN`     | System Admin     | Configuration, users, masters, emergency administration |
| `INVENTORY` | Inventory Holder | Prepare material records and gate passes                |
| `ISSUING`   | Issuing Officer  | Independent approval/rejection                          |
| `SECURITY`  | Security Officer | Physical gate movement and returns                      |
| `VIEWER`    | Viewer           | Read-only operational access                            |

But don't implement this as simple pure RBAC.

Use:

**RBAC + Record State + Ownership + Separation of Duty**

For example:

`Inventory Holder + Draft + created_by = current_user → UPDATE allowed`

but:

`Inventory Holder + Submitted → UPDATE denied`

That is much safer.

---

# 4. Backend-ready RBAC matrix

Legend:

**✓** Allowed
**C** Conditional
**—** Denied

| Permission / Action                        | Admin | Inventory Holder | Issuing Officer | Security Officer | Viewer | Backend condition                                          |
| ------------------------------------------ | :----: | :--------------: | :-------------: | :--------------: | :----: | ---------------------------------------------------------- |
| Dashboard read                             |   ✓   |        ✓        |       ✓       |        ✓        |   ✓   | Role authenticated                                         |
| Gate passes read                           |   ✓   |        ✓        |       ✓       |        C        |   C   | Security can be restricted to approved+; Viewer per policy |
| Create Gate Pass                           |   ✓   |        ✓        |       —       |        —        |   —   | Initial status must be`DRAFT`                            |
| Edit Gate Pass                             |   C   |        C        |       —       |        —        |   —   | `status=DRAFT`; Inventory only own draft                 |
| Delete Gate Pass                           |   —   |        —        |       —       |        —        |   —   | Never hard-delete official records                         |
| Submit Draft                               |   C   |        C        |       —       |        —        |   —   | Draft + valid required fields                              |
| Approve                                    |   C   |        —        |       ✓       |        —        |   —   | Must be`SUBMITTED`; approver ≠ creator                  |
| Reject                                     |   C   |        —        |       ✓       |        —        |   —   | Must be`SUBMITTED`; reason mandatory                     |
| Modify after submission                    |   C   |        —        |       —       |        —        |   —   | Immutable business fields                                  |
| Pass Out                                   |   C   |        —        |       —       |        ✓        |   —   | Must be`APPROVED`                                        |
| Security Control No.                       |   C   |        —        |       —       |        ✓        |   —   | Required during Pass Out                                   |
| Mark Return                                |   C   |        —        |       —       |        ✓        |   —   | `RETURNABLE + PASSED_OUT`                                |
| Change expected return date after approval |   C   |        —        |       —       |        —        |   —   | Amendment workflow required                                |
| Draft print                                |   ✓   |        ✓        |       ✓       |        —        |   —   | Draft/Submitted/Rejected                                   |
| Official PDF                               |   ✓   |        ✓        |       ✓       |        ✓        |   ✓   | Approved/Passed Out/Returned only                          |
| Inventory read                             |   ✓   |        ✓        |       ✓       |        ✓        |   ✓   | Read-only reference access                                 |
| Inventory create                           |   ✓   |        ✓        |       —       |        —        |   —   | Validate unique item/serial rules                          |
| Inventory update                           |   ✓   |        ✓        |       —       |        —        |   —   | No modification of historical GP snapshot                  |
| Inventory archive                          |   ✓   |        C        |       —       |        —        |   —   | Prefer Admin                                               |
| Inventory hard delete                      |   C   |        —        |       —       |        —        |   —   | Avoid once referenced                                      |
| Inventory CSV import                       |   ✓   |        ✓        |       —       |        —        |   —   | Validate rows before transaction                           |
| Inventory CSV export                       |   ✓   |        ✓        |       ✓       |        —        |   C   | Viewer only if specifically required                       |
| Consignee read                             |   ✓   |        ✓        |       ✓       |        ✓        |   ✓   | —                                                         |
| Consignee create                           |   ✓   |        ✓        |       —       |        —        |   —   | —                                                         |
| Consignee update                           |   ✓   |        ✓        |       —       |        —        |   —   | —                                                         |
| Consignee archive                          |   ✓   |        C        |       —       |        —        |   —   | No hard delete if referenced                               |
| Consignee import                           |   ✓   |        ✓        |       —       |        —        |   —   | —                                                         |
| Gate Pass CSV export                       |   ✓   |        ✓        |       ✓       |        C        |   C   | Explicit export permission                                 |
| Operational Reports                        |   ✓   |        ✓        |       ✓       |        ✓        |   ✓   | Read-only                                                  |
| Report export                              |   ✓   |        ✓        |       ✓       |        C        |   C   | Separate permission                                        |
| Audit Log read                             |   ✓   |        —        |       ✓       |        —        |   C   | Viewer only if acting as Auditor                           |
| Audit Log export                           |   ✓   |        —        |        C        |        —        |   —   | Explicit permission                                        |
| Audit Log create                           | System |      System      |     System     |      System      | System | Server only                                                |
| Audit Log update/delete                    |   —   |        —        |       —       |        —        |   —   | Never                                                      |
| User list                                  |   ✓   |        —        |       —       |        —        |   —   | Other roles receive only safe user references              |
| Create User                                |   ✓   |        —        |       —       |        —        |   —   | —                                                         |
| Change User Role                           |   ✓   |        —        |       —       |        —        |   —   | Audit mandatory                                            |
| Activate/Deactivate User                   |   ✓   |        —        |       —       |        —        |   —   | Cannot disable final Admin                                 |
| Reset User Password                        |   ✓   |        —        |       —       |        —        |   —   | Separate auth mechanism                                    |
| System Settings read                       |   ✓   |        C        |        C        |        C        |   C   | Expose only non-sensitive settings                         |
| System Settings update                     |   ✓   |        —        |       —       |        —        |   —   | —                                                         |
| Manual Pass Number                         |   ✓   |        —        |       —       |        —        |   —   | Unique constraint                                          |
| Backup Export                              |   ✓   |        —        |       —       |        —        |   —   | Admin-only                                                 |
| Database Restore                           |   ✓   |        —        |       —       |        —        |   —   | Admin + validation + audit                                 |
| SQL/database dump                          |   ✓   |        —        |       —       |        —        |   —   | Maintenance only                                           |

This is the matrix I would use as the **source of truth for backend implementation**.

---

# 5. Gate-pass state-transition RBAC

This is equally important as the normal CRUD table.

| Current State                   | Action               | New State        | Allowed role      | Important condition               |
| ------------------------------- | -------------------- | ---------------- | ----------------- | --------------------------------- |
| New                             | Create               | `DRAFT`        | Inventory / Admin | Creator recorded automatically    |
| `DRAFT`                       | Edit                 | `DRAFT`        | Creator / Admin   | Own draft only                    |
| `DRAFT`                       | Submit               | `SUBMITTED`    | Creator / Admin   | Full validation                   |
| `SUBMITTED`                   | Approve              | `APPROVED`     | Issuing Officer   | Approver must not be creator      |
| `SUBMITTED`                   | Reject               | `NOT_APPROVED` | Issuing Officer   | Reason mandatory                  |
| `NOT_APPROVED`                | Modify original      | —               | Nobody            | Preserve rejected record          |
| `NOT_APPROVED`                | Create revision      | `DRAFT`        | Inventory         | Link to original pass             |
| `APPROVED`                    | Pass Out             | `PASSED_OUT`   | Security          | Security control number mandatory |
| `PASSED_OUT` + Returnable     | Return               | `RETURNED`     | Security          | Actual return date required       |
| `PASSED_OUT` + Non-returnable | Complete             | `PASSED_OUT`   | —                | Can be considered terminal        |
| `RETURNED`                    | Change business data | —               | Nobody            | Record locked                     |

This is much better than allowing generic `PATCH /gatepasses/:id` access.

For example, don't let an Issuing Officer send:

```text
status = "Returned"
```

through a generic update.

Instead expose controlled business operations conceptually such as:

```text
POST /gatepasses/:id/submit
POST /gatepasses/:id/approve
POST /gatepasses/:id/reject
POST /gatepasses/:id/pass-out
POST /gatepasses/:id/return
```

Each transition validates the **role + old state + required fields + actor**.

---

# 6. Separation of duty

This matters particularly for your application.

The ideal normal flow should be:

**Inventory Holder**
→ prepares material pass

**Issuing Officer**
→ independently approves it

**Security Officer**
→ independently verifies physical movement

This is exactly where constrained RBAC/separation of duty is useful. NIST identifies separation of duty as a core capability of more advanced RBAC models. ([NIST Computer Security Resource Center][2])

Therefore:

```text
created_by != approved_by
```

should be a backend rule.

And preferably:

```text
approved_by != security_officer
```

for normal operation.

The Admin account should be **break-glass administration**, not the person routinely processing every workflow stage.

---

# 7. Backend collections I recommend

Instead of copying the current IndexedDB/SQL-dump structure exactly, use approximately:

| Collection/Table              | Purpose                            |
| ----------------------------- | ---------------------------------- |
| `gate_passes`               | Main gate-pass header              |
| `gate_pass_items`           | Individual material rows           |
| `inventory_items`           | Inventory master                   |
| `consignees`                | Consignee master                   |
| `gate_pass_events`          | Immutable state/action history     |
| `pass_sequences`            | Atomic gate-pass numbering         |
| `system_settings`           | Organization/default configuration |
| `directus_users`            | Users                              |
| `directus_roles` / policies | Authorization                      |
| `directus_files`            | Logo/signatures if required        |

I strongly recommend **not storing gate-pass items as one JSON blob** once you move to PostgreSQL/Directus.

Your current SQL migration dump does exactly that through `items_json JSON`.

Normalizing the items gives you much better:

* item-wise reports
* searching
* indexing
* serial-number tracking
* quantity calculations
* return tracking
* future partial-return support

---

# 8. Very important if you implement this in Directus

Directus is actually well suited to this RBAC design. Its access-control model supports permissions per **collection + action**, as well as item filters, field-level permissions, validation and presets. ([Directus][3])

For example, the Inventory Holder's update rule for `gate_passes` can conceptually be:

```text
status = DRAFT
AND
created_by = $CURRENT_USER
```

Issuing Officer:

```text
READ: required gate-pass fields
UPDATE: approval-related fields only
ITEM RULE:
status = SUBMITTED
```

Security Officer:

```text
UPDATE FIELDS:
status
security_control_no
passed_out_at
actual_return_date
returned_at

ITEM RULE:
status IN (APPROVED, PASSED_OUT)
```

Directus specifically supports limiting both **which items** a role may modify and **which fields** that role may modify. ([Directus][3])

### One especially important Directus point

Do **not** make your application's `Admin` role the actual unrestricted **Directus Administrator** role.

Directus states that its Administrator role has unrestricted control over the entire project, including the data model, and cannot be constrained. ([Directus][3])

Instead create:

**Directus Administrator**
→ only technical/backend maintenance account

and

**MGP System Admin**
→ your application's Admin role governed by the matrix above.

Also be careful with multiple Directus policies: permissions are additive, and item rules can combine to expand accessible records. ([Directus][3])

---

# 9. Pass-number implementation

The current frontend generates:

```text
ASTRA/SRSAM/2026/0001
ASTRA/SRSAM/2026/0002
...
```

by scanning existing passes and taking the maximum + 1.

That is okay offline on one browser.

It is **not safe with concurrent backend requests**.

Use a `pass_sequences` table:

| directorate | project | year | current_number |
| ----------- | ------- | ---: | -------------: |
| ASTRA       | SRSAM   | 2026 |            152 |

The server must increment it atomically inside a database transaction.

Then maintain:

```text
UNIQUE(pass_no)
```

at database level.

That eliminates duplicate numbers even if two Inventory Holders submit simultaneously.

---

# 10. Audit design

Your existing audit functionality is already a good foundation: actions record user, role, entity, ID, details and timestamp.

For the backend, upgrade it to something like:

| Field        | Example          |
| ------------ | ---------------- |
| event_id     | UUID             |
| gate_pass_id | UUID             |
| action       | `APPROVE`      |
| from_status  | `SUBMITTED`    |
| to_status    | `APPROVED`     |
| actor_id     | User UUID        |
| actor_role   | `ISSUING`      |
| timestamp    | server timestamp |
| reason       | nullable         |
| metadata     | JSON             |
| request_id   | UUID             |

And make it:

**INSERT only. No role, including MGP Admin, should edit or delete audit entries.**

OWASP also recommends appropriate logging and explicit authorization testing. ([OWASP Cheat Sheet Series][1])

## Deployment Model:

DEVELOPMENT PC — Internet ✅
        		│
        		│ develop + configure + test
        		▼
┌────────────────────────────┐
│ HTML / CSS / JavaScript                                        │
│ Directus                                              		          │
│ PostgreSQL                 						  │
│ NGINX static server        					  │
└─────────────┬──────────────┘
              				    │
         			 Release bundle
              				    │
           				Pendrive
              				    │
              				   ▼
		SERVER 10.2.3.135 — Internet ❌
┌────────────────────────────┐
│ Same exact Docker images   				  │
│ Same application           					  │
│ Same DB schema             					  │
│ Local persistent DB data   					  │
└────────────────────────────┘
