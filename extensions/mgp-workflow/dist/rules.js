export const ROLES = Object.freeze({ ADMIN: 'ADMIN', INVENTORY: 'INVENTORY', ISSUING: 'ISSUING', SECURITY: 'SECURITY', VIEWER: 'VIEWER' });
export const STATUS = Object.freeze({ DRAFT: 'DRAFT', SUBMITTED: 'SUBMITTED', APPROVED: 'APPROVED', NOT_APPROVED: 'NOT_APPROVED', PASSED_OUT: 'PASSED_OUT', RETURNED: 'RETURNED' });

export const SECURITY_VISIBLE_STATUSES = Object.freeze([STATUS.APPROVED, STATUS.PASSED_OUT, STATUS.RETURNED]);

const transitions = Object.freeze({
  submit: { from: STATUS.DRAFT, to: STATUS.SUBMITTED, role: ROLES.INVENTORY },
  approve: { from: STATUS.SUBMITTED, to: STATUS.APPROVED, role: ROLES.ISSUING },
  reject: { from: STATUS.SUBMITTED, to: STATUS.NOT_APPROVED, role: ROLES.ISSUING },
  'pass-out': { from: STATUS.APPROVED, to: STATUS.PASSED_OUT, role: ROLES.SECURITY },
  return: { from: STATUS.PASSED_OUT, to: STATUS.RETURNED, role: ROLES.SECURITY },
});

export function assertTransition(action, gatePass, actor) {
  const spec = transitions[action];
  if (!spec) throw Object.assign(new Error('Unknown workflow action'), { status: 404 });
  if (gatePass.status !== spec.from) throw Object.assign(new Error(`Cannot ${action} a ${gatePass.status} gate pass`), { status: 409 });
  if (actor.role !== ROLES.ADMIN && actor.role !== spec.role) throw Object.assign(new Error('Role is not allowed to perform this action'), { status: 403 });
  if (action === 'return' && gatePass.pass_type !== 'RETURNABLE') throw Object.assign(new Error('Only returnable gate passes can be returned'), { status: 422 });
  if (['approve', 'reject'].includes(action) && actor.id && gatePass.created_by === actor.id) throw Object.assign(new Error('The pass creator cannot approve or reject the same pass'), { status: 403 });
  // MGP Admin is an exception-management role, never a substitute for a complete operational chain.
  if (actor.role === ROLES.ADMIN && actor.id && [gatePass.created_by, gatePass.approved_by, gatePass.security_officer_id, gatePass.returned_by].includes(actor.id)) {
    throw Object.assign(new Error('MGP Admin cannot perform more than one operational stage for the same pass'), { status: 403 });
  }
  return spec;
}

export function assertDraftOwner(gatePass, actor) {
  if (gatePass.status !== STATUS.DRAFT) throw Object.assign(new Error('Only drafts can be changed'), { status: 409 });
  if (actor.role !== ROLES.ADMIN && gatePass.created_by !== actor.id) throw Object.assign(new Error('Only the draft creator can change this gate pass'), { status: 403 });
}

export function assertRevisionCreator(gatePass, actor) {
  if (gatePass.status !== STATUS.NOT_APPROVED) throw Object.assign(new Error('Only rejected gate passes can be revised'), { status: 409 });
  if (actor.role !== ROLES.ADMIN && actor.role !== ROLES.INVENTORY) throw Object.assign(new Error('Only Admin or Inventory can create a revision'), { status: 403 });
  if (actor.role === ROLES.INVENTORY && gatePass.created_by !== actor.id) throw Object.assign(new Error('Only the original creator can revise this rejected pass'), { status: 403 });
}

export function normalizeCode(value, fallback) {
  const output = String(value || fallback || '').trim().toUpperCase().replace(/[^A-Z0-9]/g, '');
  if (!output) throw Object.assign(new Error('Directorate and project are required'), { status: 422 });
  return output;
}
