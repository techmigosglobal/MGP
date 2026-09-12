import { chromium } from 'playwright-core';
import { mkdtemp, readFile, rm } from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';

const base = process.env.MGP_BROWSER_URL || 'http://127.0.0.1:18080';
const password = process.env.MGP_TEST_PASSWORD || 'MgpAcceptanceOperator-2026!';
const accounts = { inventory: 'inventory.acceptance@example.test', issuing: 'issuing.acceptance@example.test', security: 'security.acceptance@example.test', viewer: 'viewer.acceptance@example.test' };
const browser = await chromium.launch({ executablePath: process.env.CHROME_PATH || '/usr/bin/google-chrome', headless: true, args: ['--no-sandbox'] });
const errors = [];
const downloads = await mkdtemp(path.join(os.tmpdir(), 'mgp-pdf-'));
const assert = (condition, message) => { if (!condition) throw new Error(message); };
async function loginAs(page, email) { await page.goto(base, { waitUntil: 'networkidle' }); await page.locator('input[name="email"]').fill(email); await page.locator('input[name="password"]').fill(password); await page.getByRole('button', { name: 'Sign in' }).click(); await page.waitForSelector('.app-shell'); }
try {
  const page = await browser.newPage({ viewport: { width: 1440, height: 1050 }, acceptDownloads: true });
  page.on('console', (message) => { if (message.type() === 'error' || message.type() === 'warning') errors.push(`${message.type()}: ${message.text()}`); });
  await loginAs(page, accounts.inventory);
  assert(await page.getByText('MGP Control Room').isVisible(), 'Desktop control room did not render');
  await page.getByRole('button', { name: 'Gate passes' }).click();
  assert(await page.getByText('Gate pass records').isVisible(), 'Inventory register did not render');
  await page.getByRole('button', { name: 'Sign out' }).click();
  await loginAs(page, accounts.issuing);
  assert(await page.getByRole('button', { name: 'Audit log' }).isVisible(), 'Issuing audit navigation is missing');
  await page.getByRole('button', { name: 'Sign out' }).click();
  await loginAs(page, accounts.security);
  await page.getByRole('button', { name: 'Gate passes' }).click();
  assert(!(await page.getByText('DRAFT', { exact: true }).count()), 'Security UI exposed a draft');
  await page.getByRole('button', { name: 'Sign out' }).click();
  await loginAs(page, accounts.viewer);
  await page.getByRole('button', { name: 'Gate passes' }).click();
  assert(!(await page.getByRole('button', { name: 'Create' }).count()), 'Viewer received a create control');
  const pdfButton = page.getByRole('button', { name: 'Official PDF' }).first();
  await pdfButton.click();
  assert(await page.getByText('A4 gate-pass preview').isVisible(), 'Official PDF preview did not render');
  const [download] = await Promise.all([page.waitForEvent('download'), page.getByRole('button', { name: 'Download PDF' }).click()]);
  const pdfPath = path.join(downloads, 'gate-pass.pdf'); await download.saveAs(pdfPath);
  assert((await readFile(pdfPath)).subarray(0, 4).toString() === '%PDF', 'Downloaded document is not a PDF');
  await page.screenshot({ path: path.join(downloads, 'desktop-workflow.png'), fullPage: true });
  assert(!errors.length, `Browser console was not clean: ${errors.join('; ')}`);
  console.log(`Browser desktop/PDF acceptance: PASS (${base}); evidence=${downloads}`);
} finally { await browser.close(); if (process.env.MGP_KEEP_BROWSER_EVIDENCE !== '1') await rm(downloads, { recursive: true, force: true }); }
