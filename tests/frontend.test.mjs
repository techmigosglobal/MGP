import test from 'node:test';
import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';

const source = await readFile(new URL('../frontend/src/app.js', import.meta.url), 'utf8');
const api = await readFile(new URL('../frontend/src/api.js', import.meta.url), 'utf8');
const pdf = await readFile(new URL('../frontend/src/pdf.js', import.meta.url), 'utf8');

test('frontend has no browser-local business store or exposed prototype mutation API', () => {
  assert.doesNotMatch(source, /indexedDB|localStorage|window\.MGP|admin123|inventory123|issuing123|security123|viewer123/);
  assert.match(api, /credentials:\s*['"]include['"]/);
});

test('workflow actions use server endpoints', () => {
  assert.match(source, /\/mgp\/gate-passes\/\$\{form\.dataset\.id\}\/\$\{form\.dataset\.transition\}/);
  assert.match(source, /method:\s*id \? 'PATCH' : 'POST'/);
});

test('desktop workflow has official PDF, revision, user and settings surfaces', () => {
  assert.match(source, /\/mgp\/gate-passes\/\$\{node\.dataset\.id\}\/revise/);
  assert.match(source, /\/mgp\/users/);
  assert.match(source, /\/mgp\/settings\/organization/);
  assert.match(pdf, /jsPDF/);
  assert.match(pdf, /OFFICIAL MATERIAL GATE PASS/);
});
