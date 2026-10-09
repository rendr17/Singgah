import { expect, test } from '@playwright/test';

test('hero hint cards repeat a restrained product-demo sheen and respect motion controls', async ({
	page
}) => {
	await page.emulateMedia({ reducedMotion: 'no-preference' });
	await page.goto('/');
	await page.getByRole('button', { name: 'Jeda animasi', exact: true }).waitFor();
	const first = page.locator('.hero__notes > div').first();
	const motion = await first.evaluate(async (element) => {
		const animation = document
			.getAnimations()
			.find(
				(item) => item.effect?.target === element && item.animationName.endsWith('hero-step-sheen')
			);
		if (!animation) return null;
		const duration = Number(animation.effect!.getTiming().duration);
		const sample = async (phase: number) => {
			animation.currentTime = duration * phase;
			await new Promise<void>((resolve) => requestAnimationFrame(() => resolve()));
			const style = getComputedStyle(element, '::before');
			return {
				x: new DOMMatrixReadOnly(style.transform).m41,
				opacity: Number(style.opacity)
			};
		};
		return {
			content: getComputedStyle(element, '::before').content,
			iterations: animation.effect!.getTiming().iterations,
			waiting: await sample(0.3),
			active: await sample(0.1),
			repeated: await sample(1.1)
		};
	});
	expect(motion).not.toBeNull();
	expect(motion!.content).not.toBe('none');
	expect(motion!.iterations).toBe(Infinity);
	expect(motion!.active.opacity).toBeGreaterThan(0.4);
	expect(motion!.waiting.opacity).toBe(0);
	expect(motion!.active.x).not.toBe(motion!.waiting.x);
	expect(motion!.repeated.x).toBeCloseTo(motion!.active.x, 0);
	expect(motion!.repeated.opacity).toBeCloseTo(motion!.active.opacity, 2);
	await page.getByRole('button', { name: 'Jeda animasi', exact: true }).click();
	await expect
		.poll(() =>
			first.evaluate((element) => getComputedStyle(element, '::before').animationPlayState)
		)
		.toBe('paused');
	await page.getByRole('button', { name: 'Putar animasi', exact: true }).click();
	await expect
		.poll(() =>
			first.evaluate((element) => getComputedStyle(element, '::before').animationPlayState)
		)
		.toBe('running');
	await page.emulateMedia({ reducedMotion: 'reduce' });
	await expect
		.poll(() =>
			first.evaluate((element) => getComputedStyle(element, '::before').animationPlayState)
		)
		.toBe('paused');
});
