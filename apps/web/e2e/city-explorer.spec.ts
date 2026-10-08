import { expect, test, type BrowserContext, type Page } from '@playwright/test';

const apiBase = process.env.SINGGAH_API_BASE_URL ?? 'http://localhost:8080';
const privateSessionKey = 'singgah.session';

async function deleteTestAccount(context: BrowserContext, page: Page): Promise<void> {
	let token: string | null = null;
	try {
		token = await page.evaluate((key) => {
			const raw = localStorage.getItem(key);
			return raw ? ((JSON.parse(raw) as { token?: string }).token ?? null) : null;
		}, privateSessionKey);
	} catch {
		// The page may not have reached the app origin if an earlier step failed.
	}

	if (!token) return;
	const response = await context.request.delete(`${apiBase}/api/v1/auth/account`, {
		headers: { Authorization: `Bearer ${token}` }
	});
	if (response.status() !== 204) {
		throw new Error(`Test-account cleanup failed (${response.status()}).`);
	}
}

test('City Explorer supports curated trails and private save/visit actions', async ({
	browser
}) => {
	const ownerContext = await browser.newContext();
	const otherContext = await browser.newContext();
	const ownerPage = await ownerContext.newPage();
	const otherPage = await otherContext.newPage();
	const cleanupFailures: string[] = [];

	try {
		await ownerPage.goto('/explore/trails');
		for (const title of [
			'Cikini: seni & sejarah',
			'Blok M sore-sore',
			'Glodok lapar mata',
			'Museum hopping Kota Tua'
		]) {
			await expect(ownerPage.getByRole('link').filter({ hasText: title })).toBeVisible();
		}

		await ownerPage.getByRole('link').filter({ hasText: 'Blok M sore-sore' }).click();
		await expect(ownerPage).toHaveURL(/\/explore\/trails\/blok-m-sore-sore\/?$/);
		await expect(ownerPage.getByRole('heading', { name: 'Blok M sore-sore' })).toBeVisible();
		await expect(
			ownerPage.getByRole('region', { name: 'Estimasi untuk seluruh jalur' })
		).toContainText('669 m');

		await ownerPage.getByRole('link', { name: 'Taman Literasi Martha Tiahahu' }).click();
		await expect(ownerPage).toHaveURL(/\/places\/[0-9a-f-]{36}\/?$/i);
		const placeId = new URL(ownerPage.url()).pathname.split('/').pop();
		expect(placeId).toMatch(/^[0-9a-f-]{36}$/i);
		await expect(
			ownerPage.getByRole('heading', { name: 'Taman Literasi Martha Tiahahu' })
		).toBeVisible();
		await expect(ownerPage.getByRole('region', { name: 'Akses transit terdekat' })).toContainText(
			/\d+ m/
		);
		await expect(ownerPage.getByRole('region', { name: 'Sumber data' })).toContainText(
			'OpenStreetMap'
		);

		await ownerPage.getByRole('button', { name: 'Simpan tempat' }).click();
		await expect(ownerPage.getByRole('status')).toContainText(
			'Tempat tersimpan di akun sesi perangkat ini.'
		);
		await ownerPage.getByRole('button', { name: 'Tandai sudah dikunjungi' }).click();
		await expect(ownerPage.getByRole('status')).toContainText('Kunjungan dicatat secara manual.');

		await ownerPage.goto('/explore/saved');
		await expect(
			ownerPage.getByRole('link', { name: 'Taman Literasi Martha Tiahahu' })
		).toBeVisible();
		await expect(ownerPage.getByText(/Dikunjungi/)).toBeVisible();

		await otherPage.goto(`/places/${placeId}`);
		await otherPage.getByRole('button', { name: 'Simpan tempat' }).click();
		await expect(otherPage.getByRole('status')).toContainText(
			'Tempat tersimpan di akun sesi perangkat ini.'
		);
		await otherPage.goto(`/places/${placeId}`);
		await expect(otherPage.getByRole('button', { name: 'Hapus dari tersimpan' })).toBeEnabled();
		await expect(otherPage.getByText(/Kunjungan terakhir:/)).toHaveCount(0);
		await otherPage.goto('/explore/saved');
		await expect(
			otherPage.getByRole('link', { name: 'Taman Literasi Martha Tiahahu' })
		).toBeVisible();

		await otherPage.goto(`/places/${placeId}`);
		await otherPage.getByRole('button', { name: 'Hapus dari tersimpan' }).click();
		await expect(otherPage.getByRole('status')).toContainText(
			'Tempat dihapus dari daftar tersimpan.'
		);
		await otherPage.goto('/explore/saved');
		await expect(otherPage.getByText(/Belum ada tempat tersimpan/)).toBeVisible();

		await ownerPage.goto('/explore/saved');
		await expect(
			ownerPage.getByRole('link', { name: 'Taman Literasi Martha Tiahahu' })
		).toBeVisible();
		await expect(ownerPage.getByText(/Dikunjungi/)).toBeVisible();

		await ownerPage.getByRole('button', { name: 'Hapus', exact: true }).click();
		await expect(ownerPage.getByText(/Belum ada tempat tersimpan/)).toBeVisible();
		await ownerPage.goto(`/places/${placeId}`);
		await expect(ownerPage.getByRole('button', { name: 'Simpan tempat' })).toBeEnabled();
		await expect(ownerPage.getByText(/Kunjungan terakhir:/)).toBeVisible();
	} finally {
		for (const [context, page] of [
			[ownerContext, ownerPage],
			[otherContext, otherPage]
		] as const) {
			try {
				await deleteTestAccount(context, page);
			} catch {
				cleanupFailures.push('A temporary anonymous account could not be deleted.');
			} finally {
				await context.close();
			}
		}
	}

	if (cleanupFailures.length) throw new Error(cleanupFailures.join(' '));
});
