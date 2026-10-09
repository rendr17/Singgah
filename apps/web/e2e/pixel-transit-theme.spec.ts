import { expect, test } from '@playwright/test';

test('map workspace follows the coastal floating-planner reference', async ({ page }) => {
	await page.setViewportSize({ width: 1440, height: 900 });
	await page.route('**/api/v1/**', (route) => route.abort());
	await page.goto('/app?panel=plan', { waitUntil: 'domcontentloaded' });

	const planner = page.locator('.sheet--planner');
	await expect(planner).toBeVisible();
	await planner.evaluate(async (element) =>
		Promise.all(
			element.getAnimations().map((animation) => animation.finished.catch(() => undefined))
		)
	);
	await expect(page.getByRole('heading', { name: 'Rencanakan perjalanan' })).toBeVisible();
	await expect(page.locator('.planner-head__eyebrow')).toHaveText('SINGGAH / PERJALANAN');
	await expect(
		page.getByText('Temukan rute transportasi publik yang paling sesuai untuk perjalananmu.')
	).toBeVisible();
	await expect(page.locator('.planner-preferences__intro')).toBeVisible();
	await expect(page.locator('.planner-empty')).toBeVisible();
	await expect(page.locator('.planner-submit svg')).toBeVisible();
	await expect(page.locator('.app-header--map-home')).toBeVisible();
	await expect(page.locator('.map-brand')).toHaveCount(0);

	await expect(planner).toHaveCSS('position', 'absolute');
	await expect(planner).toHaveCSS('top', '21px');
	await expect(planner).toHaveCSS('width', '460px');
	await expect(planner).toHaveCSS('border-top-left-radius', '22px');
	await expect(page.locator('.sg-mode-switch button[aria-pressed="true"]')).toHaveCSS(
		'background-color',
		'rgb(7, 91, 117)'
	);

	const bounds = await page.evaluate(() => {
		const panel = document.querySelector('.sheet--planner')?.getBoundingClientRect();
		const nav = document.querySelector('.app-nav--map-home')?.getBoundingClientRect();
		const chrome = document.querySelector('.map-chrome')?.getBoundingClientRect();
		const search = document.querySelector('.map-chrome__search')?.getBoundingClientRect();
		const lineToggle = document.querySelector('.map-chrome .lines-toggle')?.getBoundingClientRect();
		const from = document
			.querySelector('.planner-field:first-child .sg-search')
			?.getBoundingClientRect();
		const to = document
			.querySelector('.planner-field:last-child .sg-search')
			?.getBoundingClientRect();
		const swap = document.querySelector('.sheet--planner .swap')?.getBoundingClientRect();
		const modeElement = document.querySelector('.sg-mode-switch');
		const mode = modeElement?.getBoundingClientRect();
		const modeButtons = Array.from(document.querySelectorAll('.sg-mode-switch button')).map(
			(button) => button.getBoundingClientRect().width
		);
		const mapWrap = document.querySelector('.map-wrap');
		const mapPane = document.querySelector('.map-pane:not([hidden])')?.getBoundingClientRect();
		const skyline = mapWrap ? getComputedStyle(mapWrap, '::before') : null;
		const modeStyle = modeElement ? getComputedStyle(modeElement) : null;
		const activeMode = document.querySelector('.sg-mode-switch button[aria-pressed="true"]');
		return {
			panelLeft: panel?.left ?? 0,
			navRight: nav?.right ?? 0,
			navCenter: nav ? nav.left + nav.width / 2 : 0,
			chromeRight: chrome?.right ?? 0,
			searchLeft: search?.left ?? 0,
			searchWidth: search?.width ?? 0,
			lineToggleLeft: lineToggle?.left ?? 0,
			lineToggleWidth: lineToggle?.width ?? 0,
			mapColor: getComputedStyle(mapWrap!).color,
			swapCenter: swap ? swap.top + swap.height / 2 : 0,
			stationFieldsCenter:
				from && to ? (from.top + from.height / 2 + to.top + to.height / 2) / 2 : 0,
			modeCenter: mode ? mode.left + mode.width / 2 : 0,
			modeRight: mode?.right ?? 0,
			modeButtonWidthDifference: Math.abs((modeButtons[0] ?? 0) - (modeButtons[1] ?? 0)),
			modeRadius: modeStyle?.borderTopLeftRadius ?? '',
			activeModeRadius: activeMode ? getComputedStyle(activeMode).borderTopLeftRadius : '',
			mapPaneTop: mapPane?.top ?? 0,
			skylineHeight: skyline?.height ?? '',
			skylineImage: skyline?.backgroundImage ?? ''
		};
	});
	expect(bounds.navRight).toBeLessThanOrEqual(bounds.panelLeft);
	expect(bounds.chromeRight).toBeLessThanOrEqual(bounds.panelLeft);
	expect(bounds.mapColor).toBe('rgb(18, 55, 70)');
	expect(bounds.searchLeft).toBe(165);
	expect(bounds.searchWidth).toBe(280);
	expect(bounds.lineToggleLeft).toBe(455);
	expect(bounds.lineToggleWidth).toBe(130);
	expect(Math.abs(bounds.swapCenter - bounds.stationFieldsCenter)).toBeLessThanOrEqual(2);
	expect(Math.abs(bounds.modeCenter - bounds.panelLeft / 2)).toBeLessThanOrEqual(2);
	expect(Math.abs(bounds.modeCenter - bounds.navCenter)).toBeLessThanOrEqual(2);
	expect(bounds.modeRight).toBeLessThanOrEqual(bounds.panelLeft);
	expect(bounds.modeButtonWidthDifference).toBeLessThanOrEqual(1);
	expect(bounds.activeModeRadius).toBe(bounds.modeRadius);
	expect(bounds.mapPaneTop).toBe(112);
	expect(bounds.skylineHeight).toBe('112px');
	expect(bounds.skylineImage).toContain('map-workspace-skyline.svg');
});

test('map-only mode switch is centered and keeps the active radius consistent', async ({
	page
}) => {
	await page.setViewportSize({ width: 1005, height: 264 });
	await page.route('**/api/v1/**', (route) => route.abort());
	await page.goto('/app', { waitUntil: 'domcontentloaded' });

	const mode = page.locator('.sg-mode-switch');
	await expect(mode).toBeVisible();

	const geometry = await page.evaluate(() => {
		const modeElement = document.querySelector('.sg-mode-switch')!;
		const active = document.querySelector('.sg-mode-switch button[aria-pressed="true"]')!;
		const nav = document.querySelector('.app-nav--map-home')!.getBoundingClientRect();
		const bounds = modeElement.getBoundingClientRect();
		return {
			center: bounds.left + bounds.width / 2,
			viewportCenter: window.innerWidth / 2,
			navCenter: nav.left + nav.width / 2,
			outerRadius: getComputedStyle(modeElement).borderTopLeftRadius,
			activeRadius: getComputedStyle(active).borderTopLeftRadius
		};
	});

	expect(Math.abs(geometry.center - geometry.viewportCenter)).toBeLessThanOrEqual(2);
	expect(Math.abs(geometry.navCenter - geometry.viewportCenter)).toBeLessThanOrEqual(2);
	expect(geometry.activeRadius).toBe(geometry.outerRadius);
});

test('tablet map mode switch is centered in the exposed map area', async ({ page }) => {
	await page.setViewportSize({ width: 820, height: 1024 });
	await page.route('**/api/v1/**', (route) => route.abort());
	await page.goto('/app?panel=plan', { waitUntil: 'domcontentloaded' });
	const planner = page.locator('.sheet--planner');
	await expect(planner).toBeVisible();
	await planner.evaluate(async (element) =>
		Promise.all(
			element.getAnimations().map((animation) => animation.finished.catch(() => undefined))
		)
	);

	const alignment = await page.evaluate(() => {
		const panel = document.querySelector('.sheet--planner')!.getBoundingClientRect();
		const mode = document.querySelector('.sg-mode-switch')!.getBoundingClientRect();
		return {
			modeCenter: mode.left + mode.width / 2,
			mapCenter: panel.left / 2
		};
	});
	expect(Math.abs(alignment.modeCenter - alignment.mapCenter)).toBeLessThanOrEqual(2);
});

test('mobile keeps the resizable planner sheet and avoids horizontal overflow', async ({
	page
}) => {
	await page.setViewportSize({ width: 390, height: 844 });
	await page.route('**/api/v1/**', (route) => route.abort());
	await page.goto('/app?panel=plan', { waitUntil: 'domcontentloaded' });

	const planner = page.locator('.sheet--planner');
	await expect(planner).toBeVisible();
	await expect(page.getByRole('heading', { name: 'Rencanakan perjalanan' })).toBeVisible();
	await expect(page.getByRole('button', { name: 'Perbesar panel' })).toBeVisible();
	await expect(page.getByRole('button', { name: 'Tutup dan hapus rencana' })).toBeVisible();
	await expect(page.locator('.app-nav--map-home')).toBeVisible();

	const layout = await page.evaluate(() => ({
		pageWidth: document.documentElement.scrollWidth,
		viewportWidth: window.innerWidth,
		panelHeight: document.querySelector('.sheet--planner')?.getBoundingClientRect().height ?? 0,
		requestedHeight: window.innerHeight * 0.61,
		titleWidth: document.querySelector('.sheet--planner h2')?.getBoundingClientRect().width ?? 0,
		titleScrollWidth: document.querySelector('.sheet--planner h2')?.scrollWidth ?? 0
	}));
	expect(layout.pageWidth).toBeLessThanOrEqual(layout.viewportWidth);
	expect(layout.panelHeight).toBeCloseTo(layout.requestedHeight, 0);
	expect(layout.titleScrollWidth).toBeLessThanOrEqual(layout.titleWidth);
});

test('map-only palette does not re-theme other functional pages', async ({ page }) => {
	await page.setViewportSize({ width: 1280, height: 900 });
	await page.goto('/plan', { waitUntil: 'domcontentloaded' });

	await expect(page.locator('.map-wrap')).toHaveCount(0);
	const mapPalette = await page.evaluate(() =>
		getComputedStyle(document.body).getPropertyValue('--journey-ink').trim()
	);
	expect(mapPalette).toBe('');
});
