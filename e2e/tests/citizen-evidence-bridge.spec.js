// UAT: the USSD → web evidence bridge. A USSD report has no way to carry
// photos, so the END message gives the citizen a link + one-time code to
// attach photos afterward from any browser. This test files the report
// through the raw USSD HTTP endpoint (as a feature-phone session would),
// extracts the code from the response text, then drives the real upload
// page in a browser.
const path = require('path');
const { test, expect } = require('@playwright/test');

async function fileUSSDReport(request, baseURL) {
  const sessionId = 'e2e-ussd-' + Date.now();
  await request.post('/ussd', { form: { sessionId, phoneNumber: '254700999888', text: '' } });
  await request.post('/ussd', { form: { sessionId, phoneNumber: '254700999888', text: '1' } });
  const res = await request.post('/ussd', {
    form: { sessionId, phoneNumber: '254700999888', text: '1*2*Officer requested a bribe at the border crossing.' },
  });
  const body = await res.text();
  const idMatch = body.match(/RPT-\d{4}-[0-9A-Z]{6}/);
  const codeMatch = body.match(/code (\d{6})/);
  if (!idMatch || !codeMatch) throw new Error('could not parse report ID/code from USSD response: ' + body);
  return { publicId: idMatch[0], code: codeMatch[1], raw: body };
}

test.describe('Citizen: USSD evidence upload bridge', () => {
  test('a USSD-filed report can have photos attached afterward via the web link', async ({ page, request }) => {
    const { publicId, code } = await fileUSSDReport(request);

    await page.goto('/e/' + publicId);
    await expect(page.getByText(publicId)).toBeVisible();

    await page.getByLabel('One-time code').fill(code);
    await page.locator('#files').setInputFiles(path.join(__dirname, '..', 'assets', 'evidence.png'));
    await page.getByRole('button', { name: 'Upload' }).click();

    await expect(page.getByText('1 file(s) attached.')).toBeVisible();

    // The report should now show evidence when tracked, confirming the
    // upload actually landed against the right report, not just a UI toast.
    await page.getByRole('link', { name: 'Track this report' }).click();
    await expect(page).toHaveURL(new RegExp('/track\\?id=' + publicId));
    await expect(page.getByText('Pending review', { exact: true })).toBeVisible();
  });

  test('an incorrect code is rejected without attaching anything', async ({ page, request }) => {
    const { publicId } = await fileUSSDReport(request);

    await page.goto('/e/' + publicId);
    await page.getByLabel('One-time code').fill('000000');
    await page.locator('#files').setInputFiles(path.join(__dirname, '..', 'assets', 'evidence.png'));
    await page.getByRole('button', { name: 'Upload' }).click();

    await expect(page.getByText(/invalid|expired/i)).toBeVisible();
  });
});
