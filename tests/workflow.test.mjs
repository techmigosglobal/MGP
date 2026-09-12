import test from 'node:test';
import assert from 'node:assert/strict';
import { ROLES, STATUS, SECURITY_VISIBLE_STATUSES, assertDraftOwner, assertRevisionCreator, assertTransition, normalizeCode } from '../extensions/mgp-workflow/dist/rules.js';

const draft = { status: STATUS.DRAFT, created_by: 'creator' };

test('only a draft creator or MGP Admin can edit a draft', () => {
  assert.doesNotThrow(() => assertDraftOwner(draft, { id: 'creator', role: ROLES.INVENTORY }));
  assert.doesNotThrow(() => assertDraftOwner(draft, { id: 'admin', role: ROLES.ADMIN }));
  assert.throws(() => assertDraftOwner(draft, { id: 'other', role: ROLES.INVENTORY }), { status: 403 });
  assert.throws(() => assertDraftOwner({ ...draft, status: STATUS.SUBMITTED }, { id: 'creator', role: ROLES.INVENTORY }), { status: 409 });
});

test('workflow requires the correct previous state and role', () => {
  assert.equal(assertTransition('approve', { status: STATUS.SUBMITTED }, { role: ROLES.ISSUING }).to, STATUS.APPROVED);
  assert.doesNotThrow(() => assertTransition('approve', { status: STATUS.SUBMITTED }, { role: ROLES.ADMIN }));
  assert.throws(() => assertTransition('approve', { status: STATUS.DRAFT }, { role: ROLES.ISSUING }), { status: 409 });
  assert.throws(() => assertTransition('approve', { status: STATUS.SUBMITTED }, { role: ROLES.INVENTORY }), { status: 403 });
});

test('approval is independent and Admin cannot repeat an operational stage', () => {
  const submitted = { status: STATUS.SUBMITTED, created_by: 'creator', approved_by: null, security_officer_id: null, returned_by: null };
  assert.throws(() => assertTransition('approve', submitted, { id: 'creator', role: ROLES.ISSUING }), { status: 403 });
  assert.throws(() => assertTransition('approve', submitted, { id: 'creator', role: ROLES.ADMIN }), { status: 403 });
  assert.throws(() => assertTransition('pass-out', { ...submitted, status: STATUS.APPROVED, approved_by: 'admin' }, { id: 'admin', role: ROLES.ADMIN }), { status: 403 });
});

test('return is restricted to passed-out returnable records', () => {
  assert.doesNotThrow(() => assertTransition('return', { status: STATUS.PASSED_OUT, pass_type: 'RETURNABLE' }, { role: ROLES.SECURITY }));
  assert.throws(() => assertTransition('return', { status: STATUS.PASSED_OUT, pass_type: 'NON_RETURNABLE' }, { role: ROLES.SECURITY }), { status: 422 });
});

test('only an Inventory creator can create a rejected-pass revision and security has a fixed visibility scope', () => {
  const rejected = { status: STATUS.NOT_APPROVED, created_by: 'inventory-user' };
  assert.doesNotThrow(() => assertRevisionCreator(rejected, { id: 'inventory-user', role: ROLES.INVENTORY }));
  assert.throws(() => assertRevisionCreator(rejected, { id: 'another-user', role: ROLES.INVENTORY }), { status: 403 });
  assert.throws(() => assertRevisionCreator(rejected, { id: 'issuer', role: ROLES.ISSUING }), { status: 403 });
  assert.deepEqual(SECURITY_VISIBLE_STATUSES, [STATUS.APPROVED, STATUS.PASSED_OUT, STATUS.RETURNED]);
});

test('pass number components are normalized and cannot be empty', () => {
  assert.equal(normalizeCode(' astra-1 '), 'ASTRA1');
  assert.throws(() => normalizeCode('***'), { status: 422 });
});
