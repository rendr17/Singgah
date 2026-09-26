import { browser } from '$app/environment';
import { api } from '$lib/api';
import { loadCachedPlan } from '$lib/plan-cache';
import { unwrap } from '@singgah/api-client';
import type { PageLoad } from './$types';

// Same query shape as /plan — a shared plan link opens the same itinerary
// here. Cache fallback reuses the plan-cache key so an offline reopen still
// renders the last fetched copy.
export const load: PageLoad = async ({ url, fetch }) => {
	const from = url.searchParams.get('from') ?? '';
	const to = url.searchParams.get('to') ?? '';
	const at = url.searchParams.get('at') ?? '';
	if (!from || !to) {
		return { plan: null, fromCache: false, error: 'Tautan perjalanan tidak lengkap.' };
	}
	try {
		const plan = await unwrap(
			api.GET('/api/v1/journeys', {
				params: { query: { from, to, at: at || undefined } },
				fetch
			})
		);
		return { plan, fromCache: false, error: '' };
	} catch (e) {
		const cached = browser ? loadCachedPlan(localStorage, from, to, at) : null;
		if (cached) return { plan: cached, fromCache: true, error: '' };
		return {
			plan: null,
			fromCache: false,
			error: e instanceof Error ? e.message : 'Perjalanan gagal dimuat.'
		};
	}
};
