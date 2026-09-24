// UAT: officials management — the source-of-truth data that the whole
// public verification feature depends on. Register an official, upload
// their reference photo, change their status, and issue a signed
// ID-card token, then confirm each change is visible where a citizen
// would actually see it (the public verify page), not just in the
// admin table.
const path = require('path');
const { test, expect } = require('@playwright/test');
const F = require('../fixtures');

async function login(page, email, password) {
  await page.goto('/admin');
  await page.getByLabel('Email').fill(email);
  await page.getByLabel('Password').fill(password);
  await page.getByRole('button', { name: 'Sign in' }).click();
  await expect(page.getByRole('heading', { name: 'Dashboard' })).toBeVisible();
}

test.describe('Admin: officials management', () => {
  test('registering a new official makes them immediately verifiable by citizens', async ({ page }) => {
    const workId = 'E2E-REGISTER-' + Date.now();

    await login(page, F.ADMIN_EMAIL, F.ADMIN_PASSWORD);
    await page.getByRole('button', { name: 'Officials' }).click();
    await page.getByRole('button', { name: '+ Register Official' }).click();

    await page.getByLabel('Work ID').fill(workId);
    await page.getByLabel('Name').fill('Test Officer Newly Registered');
    await page.getByLabel('Position').fill('Trainee Inspector');
    await page.getByLabel('Department').fill('National Police Service');
    await page.getByRole('button', { name: 'Register' }).click();

    await expect(page.getByText('Official registered')).toBeVisible();
    await expect(page.locator('table tbody tr', { hasText: workId })).toBeVisible();

    // A newly registered official starts unverified until an admin marks
    // them — confirm the citizen-facing page reflects that honestly.
    await page.goto('/v/' + workId);
    await expect(page.getByRole('heading', { name: 'Test Officer Newly Registered' })).toBeVisible();
    await expect(page.getByText('Unverified', { exact: true })).toBeVisible();
  });

  test("changing an official's status in the admin table updates what citizens see", async ({ page }) => {
    const workId = 'E2E-STATUS-FLOW-' + Date.now();

    await login(page, F.ADMIN_EMAIL, F.ADMIN_PASSWORD);
    await page.getByRole('button', { name: 'Officials' }).click();
    await page.getByRole('button', { name: '+ Register Official' }).click();
    await page.getByLabel('Work ID').fill(workId);
    await page.getByLabel('Name').fill('Status Flow Officer');
    await page.getByLabel('Position').fill('Officer');
    await page.getByRole('button', { name: 'Register' }).click();
    await expect(page.locator('table tbody tr', { hasText: workId })).toBeVisible();

    await page.getByLabel('Status for ' + workId).selectOption('verified');
    await expect(page.getByText('Status updated')).toBeVisible();
    await expect(page.locator('table tbody tr', { hasText: workId }).locator('.pill')).toHaveText('Verified');

    await page.goto('/v/' + workId);
    await expect(page.getByText('Verified', { exact: true })).toBeVisible();
  });

  test('uploading an official photo makes it visible on the public verification page', async ({ page }) => {
    const workId = 'E2E-PHOTO-FLOW-' + Date.now();

    await login(page, F.ADMIN_EMAIL, F.ADMIN_PASSWORD);
    await page.getByRole('button', { name: 'Officials' }).click();
    await page.getByRole('button', { name: '+ Register Official' }).click();
    await page.getByLabel('Work ID').fill(workId);
    await page.getByLabel('Name').fill('Photo Flow Officer');
    await page.getByLabel('Position').fill('Officer');
    await page.getByRole('button', { name: 'Register' }).click();
    const row = page.locator('table tbody tr', { hasText: workId });
    await expect(row).toBeVisible();

    await row.locator('input[type=file]').setInputFiles(path.join(__dirname, '..', 'assets', 'evidence.png'));
    await expect(page.getByText('Photo uploaded')).toBeVisible();

    await page.goto('/v/' + workId);
    const photo = page.locator('img.official-photo-lg');
    await expect
      .poll(async () => photo.evaluate((img) => img.naturalWidth), { timeout: 5000 })
      .toBeGreaterThan(0);
  });

  test('an issued ID-card link validates on the verification page', async ({ page }) => {
    await login(page, F.ADMIN_EMAIL, F.ADMIN_PASSWORD);
    await page.getByRole('button', { name: 'Officials' }).click();
    const row = page.locator('table tbody tr', { hasText: F.WORK_ID_VERIFIED_WITH_PHOTO });
    await expect(row).toBeVisible();

    await row.getByRole('button', { name: 'ID card link' }).click();
    const link = row.getByRole('link');
    await expect(link).toBeVisible();
    const href = await link.getAttribute('href');
    expect(href).toContain('/v/' + F.WORK_ID_VERIFIED_WITH_PHOTO);
    expect(href).toContain('?t=');

    await page.goto(href);
    await expect(page.getByText('✓ Scanned ID card token is currently valid')).toBeVisible();
  });

  test('a tampered ID-card token is flagged as invalid, not silently accepted', async ({ page }) => {
    await page.goto('/v/' + F.WORK_ID_VERIFIED_WITH_PHOTO + '?t=not-a-real-token');
    await expect(page.getByText(/expired or is invalid/)).toBeVisible();
  });
});
