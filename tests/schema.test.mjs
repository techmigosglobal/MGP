import test from 'node:test';
import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';

const sql = await readFile(new URL('../db/init/001_schema.sql', import.meta.url), 'utf8');

test('schema uses an atomic sequence, pass uniqueness, archive markers, and immutable events', () => {
  assert.match(sql, /CREATE TABLE IF NOT EXISTS gate_passes/);
  assert.match(sql, /DROP TRIGGER IF EXISTS gate_pass_events_immutable/);
  assert.match(sql, /CREATE TABLE IF NOT EXISTS pass_sequences/);
  assert.match(sql, /ON CONFLICT \(directorate, project, year\)/);
  assert.match(sql, /pass_no text NOT NULL UNIQUE/);
  assert.match(sql, /revision_of uuid REFERENCES gate_passes/);
  assert.match(sql, /revision_no integer NOT NULL DEFAULT 0/);
  assert.match(sql, /archived_at timestamptz/);
  assert.match(sql, /BEFORE UPDATE OR DELETE ON gate_pass_events/);
});
