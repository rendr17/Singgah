import { browser } from '$app/environment';
import { api } from '$lib/api';
import { loadCachedPlan } from '$lib/plan-cache';
import { unwrap } from '@singgah/api-client';
import type { PageLoad } from './$types';

// Same query shape as /plan — a shared plan link opens the same itineraries
// here; `i` picks which one the timeline shows. Cache fallback reuses the
// plan-cache key so an offline reopen still renders the last fetched copy.
export const load: PageLoad = async ({ url, fetch }) => {
	const params = new URLSearchParams();
	for (const [k, v] of url.searchParams) {
		if (k !== 'i') params.set(k, v);
	}
	const i = Number(url.searchParams.get('i') ?? '0');
	if (!params.get('from') || !params.get('to')) {
		return { plan: null, index: 0, fromCache: false, error: 'Tautan perjalanan tidak lengkap.' };
	}
	const q = Object.fromEntries(params) as Record<string, string>;
	try {
		const plan = await unwrap(
			api.GET('/api/v1/journeys', {
				params: {
					query: {
						from: q.from,
						to: q.to,
						at: q.at,
						arriveBy: q.arriveBy,
						modes: q.modes,
						maxWalkM: q.maxWalkM ? Number(q.maxWalkM) : undefined,
						maxTransfers: q.maxTransfers ? Number(q.maxTransfers) : undefined,
						stepFree: q.stepFree ? true : undefined
					}
				},
				fetch
			})
		);
		return { plan, index: i, fromCache: false, error: '' };
	} catch (e) {
		const cached = browser ? loadCachedPlan(localStorage, params.toString()) : null;
		if (cached) return { plan: cached, index: i, fromCache: true, error: '' };
		return {
			plan: null,
			index: 0,
			fromCache: false,
			error: e instanceof Error ? e.message : 'Perjalanan gagal dimuat.'
		};
	}
};
