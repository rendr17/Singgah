import { defineConfig } from '@playwright/test';

export default defineConfig({
	testDir: './e2e',
	fullyParallel: false,
	workers: 1,
	timeout: 90_000,
	expect: { timeout: 10_000 },
	reporter: 'list',
	outputDir: './test-results',
	use: {
		baseURL: process.env.PLAYWRIGHT_BASE_URL ?? 'http://localhost:5173',
		browserName: 'chromium',
		headless: true,
		trace: 'off',
		screenshot: 'off',
		video: 'off'
	}
});
