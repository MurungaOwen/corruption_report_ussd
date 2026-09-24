// UAT: a citizen verifying a government official's identity — the core
// anti-impersonation journey (ARCHITECTURE.md "Anti-impersonation defense
// in depth"). Covers: photo shown for a verified official, no-photo
// warning surfaced honestly, investigation status flagged, not-found
// guidance, and the one-tap impersonation report from the results page.
const { test, expect } = require('@playwright/test');
const F = require('../fixtures');

test.describe('Citizen: verify a government official', () => {
  test('a verified official with a photo on file is shown with their photo', async ({ page }) => {
    await page.goto('/verify');
    await page.getByLabel('Work / Badge ID').fill(F.WORK_ID_VERIFIED_WITH_PHOTO);
    await page.getByRole('button', { name: 'Check' }).click();

    await expect(page.getByRole('heading', { name: 'Insp. Jane Wanjiru' })).toBeVisible();
    await expect(page.getByText('Verified', { exact: true })).toBeVisible();
    await expect(page.getByText('National Police Service')).toBeVisible();

    const photo = page.locator('img.official-photo-lg');
    await expect(photo).toBeVisible();
    const src = await photo.getAttribute('src');
    expect(src).toContain('/uploads/officials/');
    // The photo must actually load, not just have a src attribute set.
    const naturalWidth = await photo.evaluate((img) => img.naturalWidth);
    expect(naturalWidth).toBeGreaterThan(0);

    // No "no photo on file" warning should appear when a photo exists.
    await expect(page.getByText('No reference photo is on file')).toHaveCount(0);
  });

  test('an official with no reference photo shows an honest warning, not a broken image', async ({ page }) => {
    await page.goto('/verify');
    await page.getByLabel('Work / Badge ID').fill(F.WORK_ID_UNVERIFIED);
    await page.getByRole('button', { name: 'Check' }).click();

    await expect(page.getByRole('heading', { name: 'Peter Otieno' })).toBeVisible();
    await expect(page.getByText('Unverified', { exact: true })).toBeVisible();
    await expect(page.getByText('No reference photo is on file for this official yet.')).toBeVisible();
  });

  test('an official under investigation is flagged, not just labeled', async ({ page }) => {
    await page.goto('/verify');
    await page.getByLabel('Work / Badge ID').fill(F.WORK_ID_INVESTIGATION);
    await page.getByRole('button', { name: 'Check' }).click();

    await expect(page.getByRole('heading', { name: 'Mohamed Abdi' })).toBeVisible();
    await expect(page.getByText('Under investigation', { exact: true })).toBeVisible();
  });

  test('an unknown work ID gets guidance, not a dead end', async ({ page }) => {
    await page.goto('/verify');
    await page.getByLabel('Work / Badge ID').fill('NO-SUCH-ID-EXISTS-999');
    await page.getByRole('button', { name: 'Check' }).click();

    await expect(page.getByText('No official found with that ID.')).toBeVisible();
    await expect(page.getByRole('link', { name: 'file a report' })).toBeVisible();
  });

  test('deep link /v/{work_id} auto-looks-up on page load, as a USSD-issued link would', async ({ page }) => {
    await page.goto('/v/' + F.WORK_ID_INVESTIGATION);
    await expect(page.getByRole('heading', { name: 'Mohamed Abdi' })).toBeVisible();
    // The work ID input should be pre-filled from the URL, confirming the
    // client-side path parsing (not just a lucky race).
    await expect(page.getByLabel('Work / Badge ID')).toHaveValue(F.WORK_ID_INVESTIGATION);
  });

  test('one-tap impersonation reporting files a real case from the verification page', async ({ page }) => {
    await page.goto('/v/' + F.WORK_ID_VERIFIED_WITH_PHOTO);
    await expect(page.getByRole('heading', { name: 'Insp. Jane Wanjiru' })).toBeVisible();

    await page.getByRole('button', { name: "This doesn't match — report it" }).click();
    await page.getByLabel('What happened? (optional but helpful)').fill(
      'The person showing this ID at the roadblock looked nothing like the photo.'
    );
    await page.getByLabel('Your phone number (optional)').fill('0733445566');
    await page.getByRole('button', { name: 'Submit report' }).click();

    await expect(page.getByText(/Report filed\. Your report ID is/)).toBeVisible();
    const reportId = await page.locator('strong').filter({ hasText: /^RPT-/ }).first().textContent();
    expect(reportId).toMatch(/^RPT-\d{4}-[0-9A-Z]{6}$/);
  });
});
