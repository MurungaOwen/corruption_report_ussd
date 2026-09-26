// One-off screenshot capture for demo/review purposes — not part of the
// test suite. Run with: node screenshot.js <outDir>
// Requires the server running at BASE_URL with e2eseed fixtures loaded.
const { chromium } = require('@playwright/test');
const path = require('path');
const fs = require('fs');
const F = require('./fixtures');

const BASE_URL = process.env.E2E_BASE_URL || 'http://localhost:8097';
const OUT_DIR = process.argv[2] || './screenshots';

async function main() {
  fs.mkdirSync(OUT_DIR, { recursive: true });
  const browser = await chromium.launch({
    executablePath: process.env.PLAYWRIGHT_CHROMIUM_PATH || undefined,
  });
  const page = await browser.newPage({ viewport: { width: 1280, height: 900 } });

  async function shot(name) {
    await page.screenshot({ path: path.join(OUT_DIR, name), fullPage: true });
    console.log('saved', name);
  }

  // 1. Citizen homepage
  await page.goto(BASE_URL + '/');
  await shot('01-citizen-home.png');

  // 2. Verify an official — verified, with photo
  await page.goto(BASE_URL + '/v/' + F.WORK_ID_VERIFIED_WITH_PHOTO);
  await page.waitForSelector('img.official-photo-lg');
  await page.waitForTimeout(300);
  await shot('02-verify-photo-match.png');

  // 3. Verify — under investigation
  await page.goto(BASE_URL + '/v/' + F.WORK_ID_INVESTIGATION);
  await page.waitForTimeout(300);
  await shot('03-verify-under-investigation.png');

  // 4. Verify — impersonation report form open
  await page.goto(BASE_URL + '/v/' + F.WORK_ID_VERIFIED_WITH_PHOTO);
  await page.waitForTimeout(300);
  await page.getByRole('button', { name: "This doesn't match — report it" }).click();
  await shot('04-verify-impersonation-form.png');

  // 5. Report corruption form, with official preview
  await page.goto(BASE_URL + '/report');
  await page.getByLabel(/What happened\?/).fill(
    'Officer demanded KES 500 at a roadblock on Thika Road to avoid a fabricated fine.'
  );
  await page.getByLabel("Official's work ID (optional, if known)").fill(F.WORK_ID_VERIFIED_WITH_PHOTO);
  await page.waitForTimeout(600);
  await shot('05-report-form-with-official-preview.png');

  // 6. Report confirmation screen
  await page.getByLabel('Your phone number (optional)').fill('0722334455');
  await page.getByRole('button', { name: 'Submit Report' }).click();
  await page.waitForSelector('text=Your report ID');
  await shot('06-report-confirmation.png');

  // 7. Track a report
  await page.goto(BASE_URL + '/track');
  await page.getByLabel('Report ID').fill('RPT-2026-A4SJ79');
  await page.getByRole('button', { name: 'Check status' }).click();
  await page.waitForTimeout(300);
  await shot('07-track-report.png');

  // 8. Admin login screen
  await page.goto(BASE_URL + '/admin');
  await shot('08-admin-login.png');

  // Log in as admin
  await page.getByLabel('Email').fill(F.ADMIN_EMAIL);
  await page.getByLabel('Password').fill(F.ADMIN_PASSWORD);
  await page.getByRole('button', { name: 'Sign in' }).click();
  await page.waitForSelector('text=Dashboard');
  await page.waitForTimeout(300);

  // 9. Admin dashboard
  await shot('09-admin-dashboard.png');

  // 10. Admin reports table
  await page.getByRole('button', { name: 'Reports', exact: true }).click();
  await page.waitForTimeout(300);
  await shot('10-admin-reports-table.png');

  // 11. Admin report detail drawer with evidence photo
  await page.locator('table tbody tr', { hasText: 'RPT-2026-A4SJ79' }).click();
  await page.waitForTimeout(600);
  await shot('11-admin-report-detail-evidence.png');
  await page.getByRole('button', { name: 'Close' }).click();

  // 12. Admin officials table
  await page.getByRole('button', { name: 'Officials' }).click();
  await page.waitForTimeout(300);
  await shot('12-admin-officials-table.png');

  await browser.close();
}

main().catch((e) => {
  console.error(e);
  process.exit(1);
});
