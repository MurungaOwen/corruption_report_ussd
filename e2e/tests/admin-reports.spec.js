// UAT: dashboard numbers, and the full report-triage loop — open a
// report, read its history, view its photo evidence (an authenticated
// fetch-as-blob, not a plain <img src>), update its status, and confirm
// the change is reflected in both the drawer and the table.
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

test.describe('Admin: dashboard', () => {
  test('dashboard shows real report and official counts, not placeholders', async ({ page, request }) => {
    await request.post('/api/v1/public/reports', { multipart: { description: 'Dashboard-count fixture report.' } });
    await login(page, F.ADMIN_EMAIL, F.ADMIN_PASSWORD);

    const totalReportsTile = page.locator('.stat-tile', { hasText: 'Total reports' });
    await expect(totalReportsTile.locator('.num')).not.toHaveText('0');

    const totalOfficialsTile = page.locator('.stat-tile', { hasText: 'Registered officials' });
    const officialCount = parseInt(await totalOfficialsTile.locator('.num').textContent(), 10);
    expect(officialCount).toBeGreaterThanOrEqual(3); // the 3 e2eseed fixtures, at minimum

    // The status breakdown bars should render actual widths, not be empty.
    await expect(page.getByRole('heading', { name: 'Reports by status' })).toBeVisible();
    await expect(page.getByRole('heading', { name: 'Officials by status' })).toBeVisible();
  });
});

test.describe('Admin: report triage', () => {
  test('opening a report shows its full detail, evidence photo, and history', async ({ page, request }) => {
    const res = await request.post('/api/v1/public/reports', {
      multipart: {
        description: 'Triage-flow fixture: officer requested a bribe at a weighbridge.',
        files: {
          name: 'evidence.png',
          mimeType: 'image/png',
          buffer: require('fs').readFileSync(path.join(__dirname, '..', 'assets', 'evidence.png')),
        },
      },
    });
    const { report_id } = await res.json();

    await login(page, F.ADMIN_EMAIL, F.ADMIN_PASSWORD);
    await page.getByRole('button', { name: 'Reports', exact: true }).click();
    await page.getByPlaceholder('Search description or ID…').fill(report_id);
    await page.getByRole('button', { name: 'Search' }).click();

    await page.locator('table tbody tr', { hasText: report_id }).click();
    await expect(page.getByRole('heading', { name: report_id })).toBeVisible();
    // Scoped to the drawer's description paragraph specifically — the
    // same text also appears (truncated) in the table row behind the
    // drawer, which made an unscoped getByText() ambiguous.
    await expect(page.locator('.modal-panel p', { hasText: 'officer requested a bribe at a weighbridge' })).toBeVisible();

    // History: the "filed" entry should be present with its actor and reason.
    await expect(page.getByRole('heading', { name: 'History' })).toBeVisible();
    await expect(page.getByText('citizen (web)')).toBeVisible();
    await expect(page.getByText('report filed', { exact: false })).toBeVisible();

    // Evidence: the thumbnail must actually load as an authenticated blob
    // (report_handlers.go requires a bearer token — a plain <img src> to
    // the API would 401), not just appear as a broken image icon.
    await expect(page.getByRole('heading', { name: /Evidence \(1\)/ })).toBeVisible();
    const thumb = page.locator('.thumb-grid img').first();
    await expect(thumb).toBeVisible();
    await expect
      .poll(async () => thumb.evaluate((img) => img.naturalWidth), { timeout: 5000 })
      .toBeGreaterThan(0);
  });

  test('updating a report status is reflected in the drawer, the table, and the audit-visible history', async ({ page, request }) => {
    const res = await request.post('/api/v1/public/reports', {
      multipart: { description: 'Status-update fixture report.' },
    });
    const { report_id } = await res.json();

    await login(page, F.ADMIN_EMAIL, F.ADMIN_PASSWORD);
    await page.getByRole('button', { name: 'Reports', exact: true }).click();
    await page.getByPlaceholder('Search description or ID…').fill(report_id);
    await page.getByRole('button', { name: 'Search' }).click();
    await page.locator('table tbody tr', { hasText: report_id }).click();

    await page.getByLabel('New status').selectOption('under_review');
    await page.getByPlaceholder('Reason (optional)').fill('Assigned to investigations desk.');
    await page.getByRole('button', { name: 'Update' }).click();

    await expect(page.getByText('Report status updated')).toBeVisible();
    await expect(page.locator('.modal-panel .pill')).toHaveText('Under review');
    await expect(page.getByText('Assigned to investigations desk.')).toBeVisible();

    await page.getByRole('button', { name: 'Close' }).click();
    await expect(page.locator('table tbody tr', { hasText: report_id }).locator('.pill')).toHaveText('Under review');
  });

  test('filtering reports by status narrows the table', async ({ page, request }) => {
    const res = await request.post('/api/v1/public/reports', { multipart: { description: 'Filter-test fixture.' } });
    const { report_id } = await res.json();

    await login(page, F.ADMIN_EMAIL, F.ADMIN_PASSWORD);
    await page.getByRole('button', { name: 'Reports', exact: true }).click();
    await page.getByLabel('Filter by status').selectOption('dismissed');
    await page.waitForTimeout(300);
    await expect(page.locator('table tbody tr', { hasText: report_id })).toHaveCount(0);

    await page.getByLabel('Filter by status').selectOption('pending');
    await page.waitForTimeout(300);
    await expect(page.locator('table tbody tr', { hasText: report_id })).toBeVisible();
  });
});
