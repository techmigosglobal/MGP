import { ROLES, STATUS, SECURITY_VISIBLE_STATUSES, assertDraftOwner, assertRevisionCreator, assertTransition, normalizeCode } from './rules.js';

const roleByName = Object.freeze({
  'MGP Admin': ROLES.ADMIN,
  'Inventory Holder': ROLES.INVENTORY,
  'Issuing Officer': ROLES.ISSUING,
  'Security Officer': ROLES.SECURITY,
  Viewer: ROLES.VIEWER,
});
const roleByCode = Object.freeze(Object.fromEntries(Object.entries(roleByName).map(([name, code]) => [code, name])));
const allStatuses = Object.values(STATUS);

function appError(message, status = 400) { return Object.assign(new Error(message), { status }); }
function required(value, label) {
  if (value === undefined || value === null || String(value).trim() === '') throw appError(`${label} is required`, 422);
  return value;
}
function cleanItems(items) {
  if (!Array.isArray(items) || items.length === 0) throw appError('At least one material item is required', 422);
  return items.map((item) => ({
    inventory_item_id: item.inventory_item_id || null,
    item_code: String(item.item_code || '').trim(),
    item_name: String(required(item.item_name, 'Item name')).trim(),
    category: String(item.category || '').trim(),
    serial_no: String(item.serial_no || '').trim(),
    batch_no: String(item.batch_no || '').trim(),
    unit_of_measure: String(item.unit_of_measure || 'NOS').trim(),
    full_or_part: String(item.full_or_part || 'FULL').trim(),
    quantity: Number(item.quantity),
    description: String(item.description || '').trim(),
  })).map((item) => {
    if (!Number.isFinite(item.quantity) || item.quantity <= 0) throw appError('Each item quantity must be greater than zero', 422);
    return item;
  });
}
function cleanDraft(input) {
  const pass_type = required(input.pass_type, 'Pass type');
  if (!['RETURNABLE', 'NON_RETURNABLE'].includes(pass_type)) throw appError('Invalid pass type', 422);
  const draft = {
    pass_type,
    pass_date: required(input.pass_date, 'Pass date'),
    directorate: normalizeCode(input.directorate),
    project: normalizeCode(input.project),
    inventory_no: String(input.inventory_no || '').trim(),
    inventory_holder: String(input.inventory_holder || '').trim(),
    consignee_id: input.consignee_id || null,
    consignee_name: String(required(input.consignee_name, 'Consignee name')).trim(),
    consignee_address: String(input.consignee_address || '').trim(),
    reference_no: String(input.reference_no || '').trim(),
    packages: Number(input.packages),
    purpose: String(required(input.purpose, 'Purpose')).trim(),
    authority: String(required(input.authority, 'Authority')).trim(),
    vehicle_no: String(input.vehicle_no || '').trim(),
    loaded_in_presence_of: String(input.loaded_in_presence_of || '').trim(),
    carrier_name: String(input.carrier_name || '').trim(),
    carrier_designation: String(input.carrier_designation || '').trim(),
    expected_return_date: input.expected_return_date || null,
  };
  if (!Number.isInteger(draft.packages) || draft.packages < 1) throw appError('Packages must be a positive whole number', 422);
  if (draft.pass_type === 'RETURNABLE' && !draft.expected_return_date) throw appError('Expected return date is required for returnable passes', 422);
  if (draft.pass_type === 'NON_RETURNABLE') draft.expected_return_date = null;
  return { draft, items: cleanItems(input.items) };
}
function parseDate(value, label) {
  if (!value) return null;
  if (!/^\d{4}-\d{2}-\d{2}$/.test(String(value))) throw appError(`${label} must be YYYY-MM-DD`, 422);
  return value;
}

export default {
  id: 'mgp',
  handler: (router, { database, services, getSchema, logger }) => {
    async function actor(req) {
      if (!req.accountability?.user) throw appError('Authentication is required', 401);
      const row = await database('directus_users as u').leftJoin('directus_roles as r', 'u.role', 'r.id')
        .select('u.id', 'u.email', 'u.first_name', 'u.last_name', 'u.status', 'r.name as role_name').where('u.id', req.accountability.user).first();
      if (!row || row.status !== 'active') throw appError('Authenticated user is not active', 401);
      const role = roleByName[row.role_name];
      if (!role) throw appError('This Directus account is not assigned an MGP role', 403);
      return { id: row.id, role, name: [row.first_name, row.last_name].filter(Boolean).join(' ') || row.email, email: row.email };
    }
    function handle(fn) {
      return async (req, res) => {
        try { res.json({ data: await fn(req, res) }); }
        catch (error) {
          const status = Number(error.status) || 500;
          if (status >= 500) logger.error(error, 'MGP endpoint failed');
          res.status(status).json({ errors: [{ message: error.message || 'Unexpected server error' }] });
        }
      };
    }
    async function appendEvent(db, values) { await db('gate_pass_events').insert(values); }
    async function fetchPass(db, id) {
      const pass = await db('gate_passes').where({ id }).first();
      if (!pass) throw appError('Gate pass was not found', 404);
      const items = await db('gate_pass_items').where({ gate_pass_id: id }).orderBy('created_at');
      return { ...pass, items };
    }
    function scopedPasses(db, user) {
      return user.role === ROLES.SECURITY ? db.whereIn('status', SECURITY_VISIBLE_STATUSES) : db;
    }
    async function getScopedPass(db, id, user) {
      const pass = await scopedPasses(db('gate_passes'), user).where({ id }).first();
      if (!pass) throw appError('Gate pass was not found or is outside your operational scope', 404);
      return pass;
    }
    function applyFilters(query, params) {
      const status = params.status;
      if (status) {
        const values = String(status).split(',').filter(Boolean);
        if (!values.every((value) => allStatuses.includes(value))) throw appError('Invalid pass status filter', 422);
        query.whereIn('status', values);
      }
      const from = parseDate(params.from, 'From date');
      const to = parseDate(params.to, 'To date');
      if (from) query.where('pass_date', '>=', from);
      if (to) query.where('pass_date', '<=', to);
      const search = String(params.q || '').trim();
      if (search) query.where((builder) => builder.whereILike('pass_no', `%${search}%`).orWhereILike('consignee_name', `%${search}%`).orWhereILike('reference_no', `%${search}%`));
      return query;
    }
    async function requireAdmin(user) { if (user.role !== ROLES.ADMIN) throw appError('Only MGP Admin can perform this action', 403); }
    async function requireMasterRole(user) { if (![ROLES.ADMIN, ROLES.INVENTORY].includes(user.role)) throw appError('Only Admin and Inventory roles can manage master data', 403); }
    async function requireExportRole(user) { if (![ROLES.ADMIN, ROLES.INVENTORY, ROLES.ISSUING].includes(user.role)) throw appError('This role cannot export master data', 403); }
    async function roleRecord(code) {
      const name = roleByCode[code];
      if (!name) throw appError('Invalid MGP role', 422);
      const role = await database('directus_roles').where({ name }).first();
      if (!role) throw appError(`MGP role ${name} has not been bootstrapped`, 503);
      return role;
    }
    async function usersService() { return new services.UsersService({ schema: await getSchema(), accountability: null }); }

    router.get('/me', handle(async (req) => {
      const user = await actor(req);
      const settings = await database('system_settings').where('key', 'organization').first();
      return { user, capabilities: {
        createGatePass: [ROLES.ADMIN, ROLES.INVENTORY].includes(user.role),
        manageMasters: [ROLES.ADMIN, ROLES.INVENTORY].includes(user.role),
        approve: [ROLES.ADMIN, ROLES.ISSUING].includes(user.role),
        security: [ROLES.ADMIN, ROLES.SECURITY].includes(user.role),
        audit: [ROLES.ADMIN, ROLES.ISSUING].includes(user.role),
        manageUsers: user.role === ROLES.ADMIN,
        manageSettings: user.role === ROLES.ADMIN,
      }, organization: settings?.value || {} };
    }));

    router.get('/gate-passes', handle(async (req) => {
      const user = await actor(req);
      const query = applyFilters(scopedPasses(database('gate_passes'), user), req.query || {}).orderBy('updated_at', 'desc').limit(200);
      const rows = await query;
      return Promise.all(rows.map((row) => fetchPass(database, row.id)));
    }));
    router.get('/gate-passes/:id', handle(async (req) => {
      const user = await actor(req); const pass = await getScopedPass(database, req.params.id, user); return fetchPass(database, pass.id);
    }));
    router.post('/gate-passes', handle(async (req) => {
      const user = await actor(req);
      if (![ROLES.ADMIN, ROLES.INVENTORY].includes(user.role)) throw appError('Only Admin and Inventory roles can create gate passes', 403);
      const { draft, items } = cleanDraft(req.body || {});
      return database.transaction(async (trx) => {
        const year = new Date(`${draft.pass_date}T00:00:00Z`).getUTCFullYear();
        const result = await trx.raw('select next_gate_pass_number(?, ?, ?) as pass_no', [draft.directorate, draft.project, year]);
        const passNo = result.rows[0].pass_no;
        const [pass] = await trx('gate_passes').insert({ ...draft, pass_no: passNo, status: STATUS.DRAFT, created_by: user.id, updated_by: user.id }).returning('*');
        await trx('gate_pass_items').insert(items.map((item) => ({ ...item, gate_pass_id: pass.id })));
        await appendEvent(trx, { gate_pass_id: pass.id, action: 'CREATE_DRAFT', from_status: null, to_status: STATUS.DRAFT, actor_id: user.id, actor_role: user.role, metadata: { pass_no: passNo } });
        return fetchPass(trx, pass.id);
      });
    }));
    router.patch('/gate-passes/:id', handle(async (req) => {
      const user = await actor(req); const { draft, items } = cleanDraft(req.body || {});
      return database.transaction(async (trx) => {
        const current = await trx('gate_passes').where('id', req.params.id).forUpdate().first();
        if (!current) throw appError('Gate pass was not found', 404);
        assertDraftOwner(current, user);
        await trx('gate_passes').where('id', current.id).update({ ...draft, updated_by: user.id, updated_at: trx.fn.now() });
        await trx('gate_pass_items').where('gate_pass_id', current.id).delete();
        await trx('gate_pass_items').insert(items.map((item) => ({ ...item, gate_pass_id: current.id })));
        await appendEvent(trx, { gate_pass_id: current.id, action: 'EDIT_DRAFT', from_status: STATUS.DRAFT, to_status: STATUS.DRAFT, actor_id: user.id, actor_role: user.role });
        return fetchPass(trx, current.id);
      });
    }));
    router.post('/gate-passes/:id/revise', handle(async (req) => {
      const user = await actor(req);
      return database.transaction(async (trx) => {
        const rejected = await trx('gate_passes').where('id', req.params.id).forUpdate().first();
        if (!rejected) throw appError('Gate pass was not found', 404);
        assertRevisionCreator(rejected, user);
        const items = await trx('gate_pass_items').where('gate_pass_id', rejected.id).orderBy('created_at');
        const year = new Date(`${rejected.pass_date}T00:00:00Z`).getUTCFullYear();
        const result = await trx.raw('select next_gate_pass_number(?, ?, ?) as pass_no', [rejected.directorate, rejected.project, year]);
        const passNo = result.rows[0].pass_no;
        const rootId = rejected.revision_of || rejected.id;
        const [{ next_revision }] = await trx('gate_passes').where((builder) => builder.where('id', rootId).orWhere('revision_of', rootId)).count('* as next_revision');
        const fields = ['pass_type', 'pass_date', 'directorate', 'project', 'inventory_no', 'inventory_holder', 'consignee_id', 'consignee_name', 'consignee_address', 'reference_no', 'packages', 'purpose', 'authority', 'vehicle_no', 'loaded_in_presence_of', 'carrier_name', 'carrier_designation', 'expected_return_date'];
        const draft = Object.fromEntries(fields.map((field) => [field, rejected[field]]));
        const [revision] = await trx('gate_passes').insert({ ...draft, pass_no: passNo, status: STATUS.DRAFT, revision_of: rootId, revision_no: Number(next_revision), created_by: user.id, updated_by: user.id }).returning('*');
        await trx('gate_pass_items').insert(items.map(({ id, gate_pass_id, created_at, ...item }) => ({ ...item, gate_pass_id: revision.id })));
        await appendEvent(trx, { gate_pass_id: rejected.id, action: 'CREATE_REVISION', from_status: STATUS.NOT_APPROVED, to_status: STATUS.NOT_APPROVED, actor_id: user.id, actor_role: user.role, metadata: { revision_id: revision.id, revision_pass_no: passNo } });
        await appendEvent(trx, { gate_pass_id: revision.id, action: 'CREATE_DRAFT_REVISION', from_status: null, to_status: STATUS.DRAFT, actor_id: user.id, actor_role: user.role, metadata: { revision_of: rejected.id, root_pass_id: rootId } });
        return fetchPass(trx, revision.id);
      });
    }));
    router.post('/gate-passes/:id/:action', handle(async (req) => {
      const user = await actor(req); const action = req.params.action;
      if (!['submit', 'approve', 'reject', 'pass-out', 'return'].includes(action)) throw appError('Unknown workflow action', 404);
      return database.transaction(async (trx) => {
        const current = await trx('gate_passes').where('id', req.params.id).forUpdate().first();
        if (!current) throw appError('Gate pass was not found', 404);
        const spec = assertTransition(action, current, user);
        const patch = { status: spec.to, updated_by: user.id, updated_at: trx.fn.now() };
        const reason = String(req.body?.reason || '').trim();
        if (action === 'reject') { if (!reason) throw appError('A rejection reason is required', 422); patch.rejection_reason = reason; patch.approved_by = user.id; patch.approved_at = trx.fn.now(); }
        if (action === 'approve') { patch.approved_by = user.id; patch.approved_at = trx.fn.now(); }
        if (action === 'submit') patch.submitted_at = trx.fn.now();
        if (action === 'pass-out') { const control = String(req.body?.security_control_no || '').trim(); if (!control) throw appError('Security control number is required', 422); patch.security_control_no = control; patch.security_officer_id = user.id; patch.passed_out_at = trx.fn.now(); }
        if (action === 'return') { const actual = parseDate(req.body?.actual_return_date, 'Actual return date'); if (!actual) throw appError('Actual return date is required', 422); patch.actual_return_date = actual; patch.returned_by = user.id; patch.returned_at = trx.fn.now(); }
        const count = await trx('gate_passes').where({ id: current.id, status: spec.from }).update(patch);
        if (count !== 1) throw appError('Gate pass changed concurrently; reload and try again', 409);
        await appendEvent(trx, { gate_pass_id: current.id, action: action.toUpperCase(), from_status: spec.from, to_status: spec.to, actor_id: user.id, actor_role: user.role, reason: reason || null, metadata: action === 'pass-out' ? { security_control_no: patch.security_control_no } : {} });
        return fetchPass(trx, current.id);
      });
    }));

    router.get('/inventory', handle(async (req) => { await actor(req); return database('inventory_items').whereNull('archived_at').orderBy('item_name'); }));
    router.get('/inventory/export', handle(async (req) => { const user = await actor(req); await requireExportRole(user); return database('inventory_items').whereNull('archived_at').orderBy('item_name'); }));
    router.post('/inventory', handle(async (req) => { const user = await actor(req); await requireMasterRole(user); const input = req.body || {}; required(input.item_code, 'Item code'); required(input.item_name, 'Item name'); const [item] = await database('inventory_items').insert({ item_code: String(input.item_code).trim(), item_name: String(input.item_name).trim(), category: String(input.category || ''), serial_no: String(input.serial_no || ''), batch_no: String(input.batch_no || ''), unit_of_measure: String(input.unit_of_measure || 'NOS'), quantity: Number(input.quantity || 0), holder: String(input.holder || ''), description: String(input.description || ''), created_by: user.id, updated_by: user.id }).returning('*'); await appendEvent(database, { action: 'CREATE_INVENTORY', actor_id: user.id, actor_role: user.role, metadata: { inventory_item_id: item.id } }); return item; }));
    router.patch('/inventory/:id', handle(async (req) => { const user = await actor(req); await requireMasterRole(user); const changes = { ...req.body }; ['id', 'created_at', 'created_by', 'archived_at', 'archived_by'].forEach((key) => delete changes[key]); const [item] = await database('inventory_items').where({ id: req.params.id }).whereNull('archived_at').update({ ...changes, updated_by: user.id, updated_at: database.fn.now() }).returning('*'); if (!item) throw appError('Active inventory item was not found', 404); await appendEvent(database, { action: 'UPDATE_INVENTORY', actor_id: user.id, actor_role: user.role, metadata: { inventory_item_id: item.id } }); return item; }));
    router.post('/inventory/:id/archive', handle(async (req) => { const user = await actor(req); await requireMasterRole(user); const count = await database('inventory_items').where({ id: req.params.id }).whereNull('archived_at').update({ archived_at: database.fn.now(), archived_by: user.id, updated_at: database.fn.now(), updated_by: user.id }); if (!count) throw appError('Active inventory item was not found', 404); await appendEvent(database, { action: 'ARCHIVE_INVENTORY', actor_id: user.id, actor_role: user.role, metadata: { inventory_item_id: req.params.id } }); return { archived: true }; }));
    router.get('/consignees', handle(async (req) => { await actor(req); return database('consignees').whereNull('archived_at').orderBy('name'); }));
    router.get('/consignees/export', handle(async (req) => { const user = await actor(req); await requireExportRole(user); return database('consignees').whereNull('archived_at').orderBy('name'); }));
    router.post('/consignees', handle(async (req) => { const user = await actor(req); await requireMasterRole(user); const input = req.body || {}; const [item] = await database('consignees').insert({ name: String(required(input.name, 'Consignee name')).trim(), address: String(input.address || ''), contact: String(input.contact || ''), created_by: user.id, updated_by: user.id }).returning('*'); await appendEvent(database, { action: 'CREATE_CONSIGNEE', actor_id: user.id, actor_role: user.role, metadata: { consignee_id: item.id } }); return item; }));
    router.patch('/consignees/:id', handle(async (req) => { const user = await actor(req); await requireMasterRole(user); const input = req.body || {}; const [item] = await database('consignees').where({ id: req.params.id }).whereNull('archived_at').update({ name: String(required(input.name, 'Consignee name')).trim(), address: String(input.address || ''), contact: String(input.contact || ''), updated_by: user.id, updated_at: database.fn.now() }).returning('*'); if (!item) throw appError('Active consignee was not found', 404); await appendEvent(database, { action: 'UPDATE_CONSIGNEE', actor_id: user.id, actor_role: user.role, metadata: { consignee_id: item.id } }); return item; }));
    router.post('/consignees/:id/archive', handle(async (req) => { const user = await actor(req); await requireMasterRole(user); const count = await database('consignees').where({ id: req.params.id }).whereNull('archived_at').update({ archived_at: database.fn.now(), archived_by: user.id, updated_at: database.fn.now(), updated_by: user.id }); if (!count) throw appError('Active consignee was not found', 404); await appendEvent(database, { action: 'ARCHIVE_CONSIGNEE', actor_id: user.id, actor_role: user.role, metadata: { consignee_id: req.params.id } }); return { archived: true }; }));

    router.get('/reports/overview', handle(async (req) => { const user = await actor(req); const rows = await scopedPasses(database('gate_passes'), user).select('status').count('* as count').groupBy('status').orderBy('status'); return { statuses: rows, visibleScope: user.role === ROLES.SECURITY ? SECURITY_VISIBLE_STATUSES : allStatuses }; }));
    router.get('/reports/returns-due', handle(async (req) => { const user = await actor(req); const rows = await scopedPasses(database('gate_passes'), user).where({ pass_type: 'RETURNABLE', status: STATUS.PASSED_OUT }).where('expected_return_date', '<', database.raw('CURRENT_DATE')).orderBy('expected_return_date').limit(100); return Promise.all(rows.map((row) => fetchPass(database, row.id))); }));
    router.get('/events', handle(async (req) => { const user = await actor(req); if (![ROLES.ADMIN, ROLES.ISSUING].includes(user.role)) throw appError('Audit access is restricted', 403); return database('gate_pass_events').orderBy('created_at', 'desc').limit(200); }));

    router.get('/settings/organization', handle(async (req) => { await actor(req); return (await database('system_settings').where('key', 'organization').first())?.value || {}; }));
    router.patch('/settings/organization', handle(async (req) => { const user = await actor(req); await requireAdmin(user); const input = req.body || {}; const value = { name: String(required(input.name, 'Organization name')).trim(), address: String(input.address || '').trim(), defaultDirectorate: String(input.defaultDirectorate || '').trim(), defaultProject: String(input.defaultProject || '').trim() }; await database('system_settings').where('key', 'organization').update({ value, updated_at: database.fn.now(), updated_by: user.id }); await appendEvent(database, { action: 'UPDATE_ORGANIZATION', actor_id: user.id, actor_role: user.role, metadata: {} }); return value; }));
    router.get('/backup/status', handle(async (req) => { const user = await actor(req); await requireAdmin(user); return (await database('system_settings').where('key', 'backup').first())?.value || {}; }));

    router.get('/users', handle(async (req) => { const user = await actor(req); await requireAdmin(user); return database('directus_users as u').join('directus_roles as r', 'u.role', 'r.id').whereIn('r.name', Object.keys(roleByName)).select('u.id', 'u.email', 'u.first_name', 'u.last_name', 'u.status', 'r.name as role_name').orderBy('u.email'); }));
    router.post('/users', handle(async (req) => { const user = await actor(req); await requireAdmin(user); const input = req.body || {}; const role = await roleRecord(String(input.role || '')); const password = String(required(input.password, 'Temporary password')); if (password.length < 12) throw appError('Temporary password must contain at least 12 characters', 422); const service = await usersService(); const id = await service.createOne({ email: String(required(input.email, 'Email')).trim().toLowerCase(), password, first_name: String(input.first_name || '').trim(), last_name: String(input.last_name || '').trim(), role: role.id, status: 'active' }); await appendEvent(database, { action: 'CREATE_USER', actor_id: user.id, actor_role: user.role, metadata: { user_id: id, role: input.role } }); return { id }; }));
    router.patch('/users/:id', handle(async (req) => { const user = await actor(req); await requireAdmin(user); const existing = await database('directus_users as u').join('directus_roles as r', 'u.role', 'r.id').where('u.id', req.params.id).whereIn('r.name', Object.keys(roleByName)).select('u.id').first(); if (!existing) throw appError('MGP user was not found', 404); const input = req.body || {}; const changes = {}; if (input.first_name !== undefined) changes.first_name = String(input.first_name).trim(); if (input.last_name !== undefined) changes.last_name = String(input.last_name).trim(); if (input.status !== undefined) { if (!['active', 'suspended'].includes(input.status)) throw appError('User status must be active or suspended', 422); changes.status = input.status; } if (input.role !== undefined) changes.role = (await roleRecord(String(input.role))).id; if (input.password) { if (String(input.password).length < 12) throw appError('Temporary password must contain at least 12 characters', 422); changes.password = String(input.password); } if (!Object.keys(changes).length) throw appError('No user changes were supplied', 422); await (await usersService()).updateOne(req.params.id, changes); await appendEvent(database, { action: 'UPDATE_USER', actor_id: user.id, actor_role: user.role, metadata: { user_id: req.params.id } }); return { id: req.params.id, updated: true }; }));
  },
};
