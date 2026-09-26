// UAT: staff portal login and role-based access control. Directly checks
// the fix made during the CoVe pass — the UI must not show controls a
// role isn't actually allowed to use.
const { test, expect } = require('@playwright/test');
const F = require('../fixtures');

async function login(page, email, password) {
  await page.goto('/admin');
  await page.getByLabel('Email').fill(email);
  await page.getByLabel('Password').fill(password);
  await page.getByRole('button', { name: 'Sign in' }).click();
}

// Each RBAC test needs at least one report in the table but must not
// depend on other spec files having run first (file execution order is
// not a contract), so it creates its own via the public API directly.
async function ensureAReportExists(request) {
  await request.post('/api/v1/public/reports', {
    multipart: { description: 'RBAC fixture report — created for admin UAT, ignore.' },
  });
}

test.describe('Admin: authentication', () => {
  test('a wrong password is rejected with a clear message, not a silent failure', async ({ page }) => {
    await login(page, F.ADMIN_EMAIL, 'totally-wrong-password');
    await expect(page.getByText('invalid email or password')).toBeVisible();
    await expect(page.getByRole('heading', { name: 'Dashboard' })).toHaveCount(0);
  });

  test('a valid admin login reaches the dashboard and can log out', async ({ page }) => {
    await login(page, F.ADMIN_EMAIL, F.ADMIN_PASSWORD);
    await expect(page.getByRole('heading', { name: 'Dashboard' })).toBeVisible();
    await expect(page.getByText(F.ADMIN_EMAIL)).toBeVisible();

    await page.getByRole('button', { name: 'Log out' }).click();
    await expect(page.getByRole('button', { name: 'Sign in' })).toBeVisible();
  });
});

test.describe('Admin: role-based access control', () => {
  test('admin sees every nav item, including Audit Log and Staff Accounts', async ({ page }) => {
    await login(page, F.ADMIN_EMAIL, F.ADMIN_PASSWORD);
    await expect(page.getByRole('button', { name: 'Audit Log' })).toBeVisible();
    await expect(page.getByRole('button', { name: 'Staff Accounts' })).toBeVisible();
  });

  test('viewer cannot see admin-only nav items or officials-management controls', async ({ page }) => {
    await login(page, F.VIEWER_EMAIL, F.VIEWER_PASSWORD);
    await expect(page.getByRole('heading', { name: 'Dashboard' })).toBeVisible();

    await expect(page.getByRole('button', { name: 'Audit Log' })).toHaveCount(0);
    await expect(page.getByRole('button', { name: 'Staff Accounts' })).toHaveCount(0);

    await page.getByRole('button', { name: 'Officials' }).click();
    await expect(page.getByRole('button', { name: '+ Register Official' })).toHaveCount(0);
    await expect(page.getByText('View only').first()).toBeVisible();
  });

  test('viewer cannot update a report status (control is hidden, not just backend-blocked)', async ({ page, request }) => {
    await ensureAReportExists(request);
    await login(page, F.VIEWER_EMAIL, F.VIEWER_PASSWORD);
    await page.getByRole('button', { name: 'Reports', exact: true }).click();
    const firstRow = page.locator('table tbody tr').first();
    await expect(firstRow).toBeVisible({ timeout: 5000 });
    await firstRow.click();
    await expect(page.getByRole('heading', { level: 2 })).toBeVisible();
    await expect(page.getByRole('heading', { name: 'Update status' })).toHaveCount(0);
  });

  test('officer can update report status but still cannot manage officials', async ({ page, request }) => {
    await ensureAReportExists(request);
    await login(page, F.OFFICER_EMAIL, F.OFFICER_PASSWORD);
    await page.getByRole('button', { name: 'Reports', exact: true }).click();
    const firstRow = page.locator('table tbody tr').first();
    await expect(firstRow).toBeVisible({ timeout: 5000 });
    await firstRow.click();
    await expect(page.getByRole('heading', { name: 'Update status' })).toBeVisible();
    await page.getByRole('button', { name: 'Close' }).click(); // the drawer overlay otherwise intercepts the nav click below

    await page.getByRole('button', { name: 'Officials' }).click();
    await expect(page.getByRole('button', { name: '+ Register Official' })).toHaveCount(0);
    await expect(page.getByText('View only').first()).toBeVisible();
  });
});
