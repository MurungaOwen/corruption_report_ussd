// UAT: a citizen filing a corruption report from the web form (with a
// photo attached directly, and an official looked up by work ID as they
// type), then tracking it by the ID they were given.
const path = require('path');
const { test, expect } = require('@playwright/test');
const F = require('../fixtures');

test.describe('Citizen: report corruption and track status', () => {
  test('submitting the report form with evidence yields a trackable report ID', async ({ page }) => {
    await page.goto('/report');

    await page.getByLabel(/What happened\?/).fill(
      'Officer demanded KES 500 at a roadblock on Thika Road to avoid a fabricated fine.'
    );

    // Official lookup-as-you-type: typing a known work ID should show a
    // live preview card before the report is even submitted.
    await page.getByLabel("Official's work ID (optional, if known)").fill(F.WORK_ID_VERIFIED_WITH_PHOTO);
    await expect(page.getByText('Insp. Jane Wanjiru')).toBeVisible({ timeout: 3000 });

    await page.getByLabel('Your phone number (optional)').fill('0722334455');
    await page.locator('#files').setInputFiles(path.join(__dirname, '..', 'assets', 'evidence.png'));

    await page.getByRole('button', { name: 'Submit Report' }).click();

    await expect(page.getByRole('heading', { name: 'Your report ID' })).toBeVisible();
    const reportIdEl = page.locator('div').filter({ hasText: /^RPT-\d{4}-[0-9A-Z]{6}$/ }).last();
    const reportId = (await reportIdEl.textContent()).trim();
    expect(reportId).toMatch(/^RPT-\d{4}-[0-9A-Z]{6}$/);

    // Evidence was attached directly in the same request, so there should
    // be no "attach photos later" prompt — that path is only for reports
    // with zero evidence attached at creation time.
    await expect(page.getByText('Want to add photos later?')).toHaveCount(0);

    // Follow the "track your report" link from the confirmation screen.
    await page.getByRole('link', { name: /track your report/ }).click();
    await expect(page).toHaveURL(new RegExp('/track\\?id=' + reportId));
    await expect(page.getByText('Pending review', { exact: true })).toBeVisible();
  });

  test('a report with no work ID entered still files successfully, unlinked', async ({ page }) => {
    await page.goto('/report');
    await page.getByLabel(/What happened\?/).fill('Asked to pay a bribe to speed up a land title transfer.');
    await page.getByRole('button', { name: 'Submit Report' }).click();
    await expect(page.getByRole('heading', { name: 'Your report ID' })).toBeVisible();
    // No evidence was attached this time, so the "add photos later" path
    // should be offered.
    await expect(page.getByText('Want to add photos later?')).toBeVisible();
  });

  test('an unknown work ID in the report form does not block submission, just warns', async ({ page }) => {
    await page.goto('/report');
    await page.getByLabel(/What happened\?/).fill('Reporting an incident, official ID uncertain.');
    await page.getByLabel("Official's work ID (optional, if known)").fill('NOT-A-REAL-WORK-ID');
    await expect(page.getByText('No matching official found')).toBeVisible({ timeout: 3000 });
    await page.getByRole('button', { name: 'Submit Report' }).click();
    await expect(page.getByRole('heading', { name: 'Your report ID' })).toBeVisible();
  });

  test('tracking an unknown report ID says so clearly', async ({ page }) => {
    await page.goto('/track');
    await page.getByLabel('Report ID').fill('RPT-0000-NOPE99');
    await page.getByRole('button', { name: 'Check status' }).click();
    await expect(page.getByText('No report found with that ID.')).toBeVisible();
  });

  test('the report form requires a description before it can be submitted', async ({ page }) => {
    await page.goto('/report');
    await expect(page.getByRole('button', { name: 'Submit Report' })).toBeDisabled();
    await page.getByLabel(/What happened\?/).fill('x');
    await expect(page.getByRole('button', { name: 'Submit Report' })).toBeEnabled();
  });
});
