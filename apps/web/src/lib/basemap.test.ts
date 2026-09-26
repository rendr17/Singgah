import { beforeEach, describe, expect, it, vi } from 'vitest';

// $env/dynamic/public resolves statically under vitest — mock it so the
// resolution logic is what gets exercised.
vi.mock('$env/dynamic/public', () => ({ env: {} }));

import { env } from '$env/dynamic/public';
import { basemapStyleUrl } from './basemap';

const envVars = env as Record<string, string | undefined>;

describe('basemapStyleUrl', () => {
	beforeEach(() => {
		for (const k of Object.keys(envVars)) delete envVars[k];
	});

	it('prefers an explicit style URL override', () => {
		envVars.PUBLIC_MAP_STYLE_URL = 'https://example.com/style.json';
		envVars.PUBLIC_PROTOMAPS_API_KEY = 'k';
		expect(basemapStyleUrl()).toBe('https://example.com/style.json');
	});

	it('embeds the Protomaps key in the hosted style URL', () => {
		envVars.PUBLIC_PROTOMAPS_API_KEY = 'testkey';
		expect(basemapStyleUrl()).toBe('https://api.protomaps.com/styles/v5/light/en.json?key=testkey');
	});

	it('warns and still returns the URL when no key is configured', () => {
		const warn = vi.spyOn(console, 'warn').mockImplementation(() => {});
		expect(basemapStyleUrl()).toBe('https://api.protomaps.com/styles/v5/light/en.json?key=');
		expect(warn).toHaveBeenCalled();
		warn.mockRestore();
	});
});
