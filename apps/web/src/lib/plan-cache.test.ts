import { describe, expect, it } from 'vitest';

import type { components } from '@singgah/api-client';
import { loadCachedPlan, saveCachedPlan } from './plan-cache';

type Plan = components['schemas']['JourneyPlan'];

function fakeStorage(): Storage & { map: Map<string, string> } {
	const map = new Map<string, string>();
	return {
		map,
		getItem: (k: string) => map.get(k) ?? null,
		setItem: (k: string, v: string) => void map.set(k, v),
		removeItem: (k: string) => void map.delete(k),
		clear: () => map.clear(),
		key: () => null,
		length: 0
	} as Storage & { map: Map<string, string> };
}

const plan: Plan = {
	from: { id: 'a', name: 'Sudirman' },
	to: { id: 'b', name: 'Lebak Bulus' },
	itinerary: null,
	source: { provider: 'commute', requestedAt: '2026-09-24T08:00:00Z' }
};

describe('plan cache', () => {
	it('round-trips a plan per station pair', () => {
		const s = fakeStorage();
		saveCachedPlan(s, 'a', 'b', '', plan);
		expect(loadCachedPlan(s, 'a', 'b', '')).toEqual(plan);
		// Different pair, different slot — no cross-talk.
		expect(loadCachedPlan(s, 'b', 'a', '')).toBeNull();
	});

	it('keys by departure context — an at-plan does not shadow the leave-now plan', () => {
		const s = fakeStorage();
		saveCachedPlan(s, 'a', 'b', '', plan);
		const timed: Plan = { ...plan, at: '2026-09-25T08:00:00+07:00' };
		saveCachedPlan(s, 'a', 'b', '2026-09-25T08:00:00+07:00', timed);
		expect(loadCachedPlan(s, 'a', 'b', '')).toEqual(plan);
		expect(loadCachedPlan(s, 'a', 'b', '2026-09-25T08:00:00+07:00')).toEqual(timed);
		expect(loadCachedPlan(s, 'a', 'b', '2026-09-26T08:00:00+07:00')).toBeNull();
	});

	it('returns null for corrupt or provenance-less entries', () => {
		const s = fakeStorage();
		s.map.set('singgah:plan:a:b', '{not json');
		expect(loadCachedPlan(s, 'a', 'b', '')).toBeNull();
		s.map.set('singgah:plan:a:b', JSON.stringify({ ...plan, source: undefined }));
		expect(loadCachedPlan(s, 'a', 'b', '')).toBeNull();
	});

	it('survives a throwing storage (private mode) without breaking the page', () => {
		const dead = {
			getItem: () => {
				throw new Error('denied');
			},
			setItem: () => {
				throw new Error('denied');
			}
		};
		expect(loadCachedPlan(dead, 'a', 'b', '')).toBeNull();
		expect(() => saveCachedPlan(dead, 'a', 'b', '', plan)).not.toThrow();
	});
});
