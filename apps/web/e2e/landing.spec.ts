import { expect, test } from '@playwright/test';

test.use({ reducedMotion: 'reduce' });

test('landing entrances are sequenced and settle without shifting the layout', async ({ page }) => {
	await page.emulateMedia({ reducedMotion: 'no-preference' });
	await page.goto('/');
	const entrance = await page.locator('.hero__content > *').evaluateAll((elements) =>
		elements.map((element) => {
			const style = getComputedStyle(element);
			return { duration: style.animationDuration, delay: style.animationDelay };
		})
	);
	expect(entrance).toEqual([
		{ duration: '0.6s', delay: '0.1s' },
		{ duration: '0.6s', delay: '0.5s' },
		{ duration: '0.6s', delay: '0.9s' }
	]);
	await expect(page.locator('.hero__actions')).toHaveCSS('opacity', '1');
	await expect
		.poll(() =>
			page.locator('.hero__actions').evaluate((element) => {
				const transform = getComputedStyle(element).transform;
				return transform === 'none' || new DOMMatrixReadOnly(transform).isIdentity;
			})
		)
		.toBe(true);
	const before = await page.locator('.hero__content').boundingBox();
	await page.getByRole('button', { name: 'Jeda animasi', exact: true }).click();
	expect(await page.locator('.hero__content').boundingBox()).toEqual(before);
});

test('landing previews, feature details, and demo tabs work without booting a map', async ({
	page
}) => {
	await page.setViewportSize({ width: 1440, height: 1000 });
	const pageErrors: string[] = [];
	page.on('pageerror', (error) => pageErrors.push(error.message));
	await page.goto('/');
	await expect(page).toHaveTitle('Singgah — Pergi boleh spontan. Rute jangan.');
	await expect(page.getByRole('heading', { level: 1 })).toHaveText(
		'Pergi boleh spontan.Rute jangan.'
	);
	await expect(
		page.locator('.hero').getByRole('link', { name: 'Cari Jalan', exact: true })
	).toHaveAttribute('href', '/app?panel=plan');

	const overview = page.locator('.overview');
	const explore = overview.getByRole('button', { name: 'Jelajah', exact: true });
	await explore.click();
	await expect(explore).toHaveAttribute('aria-pressed', 'true');
	await expect(overview.getByRole('heading', { name: 'Turun. Singgah sebentar.' })).toBeVisible();
	await expect(overview.getByRole('link', { name: 'Buka Jelajah' })).toHaveAttribute(
		'href',
		'/app?panel=explore'
	);
	await overview.getByRole('button', { name: 'Paspor', exact: true }).click();
	await expect(overview.getByRole('heading', { name: 'Cerita yang ikut pulang.' })).toBeVisible();
	await expect(overview.getByRole('link', { name: 'Buka Paspor' })).toHaveAttribute(
		'href',
		'/app?panel=passport'
	);
	await expect(overview).toContainText('Ilustrasi antarmuka');

	const topic = page.locator('#plan details').first();
	await topic.locator('summary').click();
	await expect(topic).toHaveAttribute('open', '');
	await expect(topic.locator('p')).toBeVisible();
	await topic.locator('summary').press('Enter');
	await expect(topic).not.toHaveAttribute('open', '');

	const firstTab = page.getByRole('tab', { name: /Peta & perjalanan/ });
	const secondTab = page.getByRole('tab', { name: /Jelajah kota/ });
	const thirdTab = page.getByRole('tab', { name: /Paspor & jurnal/ });
	await firstTab.click();
	await firstTab.press('ArrowRight');
	await expect(secondTab).toBeFocused();
	await expect(secondTab).toHaveAttribute('aria-selected', 'true');
	await expect(page.locator('#demo-panel-explore')).toBeVisible();
	await expect(page.locator('#demo-panel-network')).toBeHidden();
	await secondTab.press('End');
	await expect(thirdTab).toBeFocused();
	await expect(page.locator('#demo-panel-passport')).toBeVisible();
	await thirdTab.press('Home');
	await expect(firstTab).toBeFocused();
	await expect(page.locator('#demo-panel-network')).toBeVisible();

	await expect(page.locator('.site-header')).toHaveClass(/site-header--solid/);
	await page.locator('.site-header .wordmark').click();
	await expect(page.locator('.site-header')).not.toHaveClass(/site-header--solid/);
	const heavyResources = await page.evaluate(() =>
		performance
			.getEntriesByType('resource')
			.map((entry) => entry.name)
			.filter((url) => /maplibre|\/tiles\//i.test(url))
	);
	expect(heavyResources).toEqual([]);
	expect(pageErrors).toEqual([]);
});

for (const width of [320, 390, 768]) {
	test(`landing fits ${width}px and keeps mobile navigation usable`, async ({ page }) => {
		await page.setViewportSize({ width, height: 844 });
		await page.goto('/');
		await expect(page).toHaveTitle('Singgah — Pergi boleh spontan. Rute jangan.');
		const menu = page.getByLabel('Buka menu navigasi');
		await menu.click();
		const mobileNav = page.getByRole('navigation', { name: 'Navigasi halaman mobile' });
		await expect(mobileNav).toBeVisible();
		await page.keyboard.press('Escape');
		await expect(mobileNav).toBeHidden();
		await expect(menu).toBeFocused();
		await menu.click();
		await mobileNav.getByRole('link', { name: 'Jelajah kota' }).click();
		await expect(page).toHaveURL(/\/#explore$/);
		await expect(mobileNav).toBeHidden();

		const layout = await page.evaluate(() => ({
			width: innerWidth,
			scrollWidth: document.documentElement.scrollWidth
		}));
		expect(layout.scrollWidth).toBeLessThanOrEqual(layout.width);
		const menuBox = await menu.boundingBox();
		expect(menuBox?.width).toBeGreaterThanOrEqual(44);
		expect(menuBox?.height).toBeGreaterThanOrEqual(44);
		await expect(page.locator('.hero video')).toHaveCount(0);
		await expect(page.locator('.scene-train')).toHaveCSS('animation-play-state', 'paused');
	});
}

test('primary landing CTA opens the actual planner state', async ({ page }) => {
	await page.goto('/');
	await page.locator('.hero').getByRole('link', { name: 'Cari Jalan', exact: true }).click();
	await expect(page).toHaveURL(/\/app\?panel=plan$/);
	await expect(
		page
			.getByRole('navigation', { name: 'Navigasi utama' })
			.getByRole('link', { name: 'Perjalanan' })
	).toHaveAttribute('aria-current', 'page');
});

test('hero animation can be paused and follows live reduced-motion preferences', async ({
	page
}) => {
	await page.emulateMedia({ reducedMotion: 'no-preference' });
	await page.goto('/');
	const train = page.locator('.scene-train');
	await expect(page.locator('.hero video')).toHaveCount(0);
	await expect(train).toHaveCSS('animation-play-state', 'running');
	await page.getByRole('button', { name: 'Jeda animasi', exact: true }).click();
	await expect(train).toHaveCSS('animation-play-state', 'paused');
	const pausedTransform = await train.evaluate((element) => getComputedStyle(element).transform);
	await page.waitForTimeout(250);
	expect(await train.evaluate((element) => getComputedStyle(element).transform)).toBe(
		pausedTransform
	);
	await page.emulateMedia({ reducedMotion: 'reduce' });
	await expect(train).toHaveCSS('animation-play-state', 'paused');
	await expect(page.getByRole('button', { name: 'Putar animasi', exact: true })).toHaveCount(0);
	await page.emulateMedia({ reducedMotion: 'no-preference' });
	// Changing the system preference must not override an explicit pause.
	await expect(page.getByRole('button', { name: 'Putar animasi', exact: true })).toBeVisible();
	await expect(train).toHaveCSS('animation-play-state', 'paused');
	await page.getByRole('button', { name: 'Putar animasi', exact: true }).click();
	await expect(train).toHaveCSS('animation-play-state', 'running');
	await page.emulateMedia({ reducedMotion: 'reduce' });
	await expect(train).toHaveCSS('animation-play-state', 'paused');
});

test('hero camera and foreground stay fixed while decorative objects move', async ({ page }) => {
	await page.emulateMedia({ reducedMotion: 'no-preference' });
	await page.goto('/');
	const fixed = page.locator('.hero-scene > svg, .scene-laptop, .scene-journal');
	const boxes = () =>
		fixed.evaluateAll((elements) =>
			elements.map((element) => {
				const box = element.getBoundingClientRect();
				return [box.x, box.y, box.width, box.height];
			})
		);
	const before = await boxes();
	const train = page.locator('.scene-train');
	const transform = await train.evaluate((element) => getComputedStyle(element).transform);
	await expect
		.poll(() => train.evaluate((element) => getComputedStyle(element).transform))
		.not.toBe(transform);
	expect(await boxes()).toEqual(before);
	await page.locator('.overview').scrollIntoViewIfNeeded();
	await expect(train).toHaveCSS('animation-play-state', 'paused');
	await page.locator('.site-header .wordmark').click();
	await expect(train).toHaveCSS('animation-play-state', 'running');
	await expect(page.locator('.hero')).toContainText('bukan pelacakan live');
});

test('hero stays static without JavaScript and preserves its navigation links', async ({
	browser
}) => {
	const context = await browser.newContext({
		javaScriptEnabled: false,
		reducedMotion: 'no-preference'
	});
	try {
		const page = await context.newPage();
		await page.goto('/');
		await expect(page.locator('.scene-train')).toHaveCSS('animation-play-state', 'paused');
		await expect(page.getByRole('button', { name: 'Jeda animasi', exact: true })).toHaveCount(0);
		const href = await page
			.locator('.hero')
			.getByRole('link', { name: 'Cari Jalan', exact: true })
			.getAttribute('href');
		const destination = new URL(href ?? '', page.url());
		expect(destination.pathname + destination.search).toBe('/app?panel=plan');
	} finally {
		await context.close();
	}
});

test('missing decorative artwork does not break hero copy, controls, or original SVG', async ({
	page
}) => {
	await page.emulateMedia({ reducedMotion: 'no-preference' });
	await page.route('**/illustrations/landing/*.webp', (route) => route.abort());
	const errors: string[] = [];
	page.on('pageerror', (error) => errors.push(error.message));
	await page.goto('/');
	await expect(page.getByRole('heading', { level: 1 })).toBeVisible();
	await expect(page.locator('.scene-laptop')).toBeVisible();
	await page.getByRole('button', { name: 'Jeda animasi', exact: true }).click();
	await expect(page.locator('.scene-train')).toHaveCSS('animation-play-state', 'paused');
	await expect(
		page.locator('.hero').getByRole('link', { name: 'Cari Jalan', exact: true })
	).toBeVisible();
	expect(errors).toEqual([]);
});

test('hero train resets entirely outside the visible track at its loop boundary', async ({
	page
}) => {
	await page.emulateMedia({ reducedMotion: 'no-preference' });
	await page.goto('/');
	await page.getByRole('button', { name: 'Jeda animasi', exact: true }).click();
	const outside = await page.locator('.scene-train').evaluate(async (train) => {
		const graphic = train as SVGGraphicsElement;
		const animation = train.getAnimations()[0];
		const timing = animation.effect!.getTiming();
		const boundary = Number(timing.duration) + timing.delay;
		const clip = graphic.ownerSVGElement!.querySelector('#hero-track rect')!;
		const left = Number(clip.getAttribute('x'));
		const right = left + Number(clip.getAttribute('width'));
		const states: boolean[] = [];
		animation.pause();
		for (const time of [boundary - 0.5, boundary + 0.5]) {
			animation.currentTime = time;
			await new Promise<void>((resolve) => requestAnimationFrame(() => resolve()));
			const box = graphic.getBBox();
			const x = new DOMMatrixReadOnly(getComputedStyle(train).transform).m41;
			states.push(box.x + x >= right || box.x + box.width + x <= left);
		}
		return states;
	});
	expect(outside).toEqual([true, true]);
});

test('hero environment loops continuously instead of stopping after its entrance', async ({
	page
}) => {
	await page.emulateMedia({ reducedMotion: 'no-preference' });
	await page.goto('/');
	await page.getByRole('button', { name: 'Jeda animasi', exact: true }).waitFor();
	for (const selector of ['.scene-tree', '.scene-clouds', '.scene-grass', '.scene-screen-route']) {
		const layer = page.locator(selector).first();
		await expect(layer).toHaveCSS('animation-iteration-count', 'infinite', { timeout: 2000 });
		const samples = await layer.evaluate(async (element) => {
			const animation = element.getAnimations()[0];
			const timing = animation.effect!.getTiming();
			const duration = Number(timing.duration);
			const cycle = timing.direction.includes('alternate') ? 2 : 1;
			animation.pause();
			const sample = async (phase: number) => {
				animation.currentTime = timing.delay + duration * phase;
				await new Promise<void>((resolve) => requestAnimationFrame(() => resolve()));
				const style = getComputedStyle(element);
				return { transform: style.transform, trace: style.strokeDashoffset };
			};
			return {
				start: await sample(0.25),
				moved: await sample(0.65),
				repeated: await sample(cycle + 0.25)
			};
		});
		expect(samples.moved, selector).not.toEqual(samples.start);
		expect(samples.repeated, selector).toEqual(samples.start);
	}
	await expect(page.locator('.hero-scene > svg')).toHaveCSS('transform', 'none');
	expect(
		await page.locator('.scene-grass').evaluateAll((elements) =>
			elements.every((element) => {
				const laptop = document.querySelector('.scene-laptop')!;
				const journal = document.querySelector('.scene-journal')!;
				return (
					Boolean(element.compareDocumentPosition(laptop) & Node.DOCUMENT_POSITION_FOLLOWING) &&
					Boolean(element.compareDocumentPosition(journal) & Node.DOCUMENT_POSITION_FOLLOWING)
				);
			})
		)
	).toBe(true);
});

test('hero screenshot typography uses the self-hosted Poppins family', async ({ page }) => {
	await page.setViewportSize({ width: 1440, height: 1000 });
	await page.goto('/');
	await page.evaluate(() => document.fonts.ready);
	for (const selector of [
		'.hero h1',
		'.hero__content > p',
		'.hero__notes span',
		'.hero__notes strong'
	]) {
		await expect(page.locator(selector).first()).toHaveCSS('font-family', /^Poppins,/);
	}
	expect(await page.evaluate(() => document.fonts.check('500 46px Poppins'))).toBe(true);
	expect(
		await page.evaluate(() =>
			performance
				.getEntriesByType('resource')
				.map((entry) => entry.name)
				.filter((url) => /poppins.*woff2/.test(url))
		)
	).not.toHaveLength(0);
	await expect(page.locator('.overview h2')).toHaveCSS('font-family', /Inter Variable/);
});

test('hero steps loop their emphasis and share the environment pause controls', async ({
	page
}) => {
	await page.emulateMedia({ reducedMotion: 'no-preference' });
	await page.goto('/');
	const layers = page.locator('.scene-motion, .hero__notes > div');
	await page.getByRole('button', { name: 'Jeda animasi', exact: true }).waitFor();
	await expect(page.locator('.hero__notes > div').first()).toHaveCSS(
		'animation-iteration-count',
		'infinite',
		{ timeout: 2000 }
	);
	await page.getByRole('button', { name: 'Jeda animasi', exact: true }).click();
	await expect
		.poll(() =>
			layers.evaluateAll((elements) =>
				elements.every((element) => getComputedStyle(element).animationPlayState === 'paused')
			)
		)
		.toBe(true);
	const states = () =>
		layers.evaluateAll((elements) =>
			elements.map((element) => {
				const style = getComputedStyle(element);
				return [
					style.transform,
					style.strokeDashoffset,
					style.offsetDistance,
					style.opacity,
					style.backgroundColor
				];
			})
		);
	const paused = await states();
	await page.waitForTimeout(250);
	expect(await states()).toEqual(paused);
	await page.getByRole('button', { name: 'Putar animasi', exact: true }).click();
	await expect
		.poll(() =>
			layers.evaluateAll((elements) =>
				elements.every((element) => getComputedStyle(element).animationPlayState === 'running')
			)
		)
		.toBe(true);
	await page.locator('.overview').scrollIntoViewIfNeeded();
	await expect
		.poll(() =>
			layers.evaluateAll((elements) =>
				elements.every((element) => getComputedStyle(element).animationPlayState === 'paused')
			)
		)
		.toBe(true);
	await page.locator('.site-header .wordmark').click();
	await page.emulateMedia({ reducedMotion: 'reduce' });
	await expect
		.poll(() =>
			layers.evaluateAll((elements) =>
				elements.every((element) => getComputedStyle(element).animationPlayState === 'paused')
			)
		)
		.toBe(true);
});

test('hero step cards animate a repeating progress sweep from journey step to step', async ({
	page
}) => {
	await page.emulateMedia({ reducedMotion: 'no-preference' });
	await page.goto('/');
	await page.getByRole('button', { name: 'Jeda animasi', exact: true }).waitFor();
	const step = page.locator('.hero__notes > div').first();
	await expect
		.poll(() => step.evaluate((element) => getComputedStyle(element, '::after').content))
		.not.toBe('none');
	const cycle = await step.evaluate(async (element) => {
		const ownAnimations = document
			.getAnimations()
			.filter((animation) => animation.effect?.target === element);
		const focus = ownAnimations.find((animation) =>
			animation.animationName.endsWith('hero-step-loop')
		);
		const progress = ownAnimations.find((animation) =>
			animation.animationName.endsWith('hero-step-progress')
		);
		if (!focus || !progress) return null;
		const timing = focus.effect!.getTiming();
		const duration = Number(timing.duration);
		const sample = async (phase: number) => {
			focus.currentTime = Number(timing.delay) + duration * phase;
			progress.currentTime = Number(progress.effect!.getTiming().delay) + duration * phase;
			await new Promise<void>((resolve) => requestAnimationFrame(() => resolve()));
			return {
				card: getComputedStyle(element).transform,
				progress: new DOMMatrixReadOnly(getComputedStyle(element, '::after').transform).a
			};
		};
		return { waiting: await sample(0.3), active: await sample(0.1), repeated: await sample(1.1) };
	});
	expect(cycle).not.toBeNull();
	expect(cycle!.active.card).not.toBe(cycle!.waiting.card);
	expect(cycle!.active.progress).not.toBe(cycle!.waiting.progress);
	expect(cycle!.repeated.card).toBe(cycle!.active.card);
	expect(cycle!.repeated.progress).toBeCloseTo(cycle!.active.progress, 2);
});

test('hero step progress loops in sequence and pauses with its cards', async ({ page }) => {
	await page.emulateMedia({ reducedMotion: 'no-preference' });
	await page.goto('/');
	await page.getByRole('button', { name: 'Jeda animasi', exact: true }).waitFor();
	const rows = page.locator('.hero__notes > div');
	const first = rows.first();
	await expect(first).toHaveCSS('animation-iteration-count', 'infinite');
	await expect
		.poll(() =>
			first.evaluate((element) => getComputedStyle(element, '::after').animationIterationCount)
		)
		.toBe('infinite');
	await expect
		.poll(() =>
			first.evaluate((element) => getComputedStyle(element, '::after').animationPlayState)
		)
		.toBe('running');
	const delays = await rows.evaluateAll((elements) =>
		elements.map((element) => getComputedStyle(element, '::after').animationDelay)
	);
	expect(delays).toEqual(['0s', '-4s', '-8s']);
	const progress = await first.evaluate(async (element) => {
		const animation = document
			.getAnimations()
			.filter((item) => item.effect?.target === element)
			.find((item) => item.animationName.endsWith('hero-step-progress'));
		if (!animation) return null;
		const duration = Number(animation.effect!.getTiming().duration);
		animation.pause();
		const sample = async (phase: number) => {
			animation.currentTime = duration * phase;
			await new Promise<void>((resolve) => requestAnimationFrame(() => resolve()));
			return getComputedStyle(element, '::after').transform;
		};
		return { waiting: await sample(0.3), active: await sample(0.1), repeated: await sample(1.1) };
	});
	expect(progress).not.toBeNull();
	expect(progress!.active).not.toBe(progress!.waiting);
	expect(progress!.repeated).toBe(progress!.active);
	await page.getByRole('button', { name: 'Jeda animasi', exact: true }).click();
	await expect
		.poll(() =>
			rows.evaluateAll((elements) =>
				elements.every(
					(element) => getComputedStyle(element, '::after').animationPlayState === 'paused'
				)
			)
		)
		.toBe(true);
	await page.getByRole('button', { name: 'Putar animasi', exact: true }).click();
	await expect
		.poll(() =>
			rows.evaluateAll((elements) =>
				elements.every(
					(element) => getComputedStyle(element, '::after').animationPlayState === 'running'
				)
			)
		)
		.toBe(true);
	await page.locator('.overview').scrollIntoViewIfNeeded();
	await expect
		.poll(() =>
			rows.evaluateAll((elements) =>
				elements.every(
					(element) => getComputedStyle(element, '::after').animationPlayState === 'paused'
				)
			)
		)
		.toBe(true);
	await page.locator('.site-header .wordmark').click();
	await page.emulateMedia({ reducedMotion: 'reduce' });
	await expect
		.poll(() =>
			rows.evaluateAll((elements) =>
				elements.every(
					(element) => getComputedStyle(element, '::after').animationPlayState === 'paused'
				)
			)
		)
		.toBe(true);
});
