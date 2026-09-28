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
	query: { stepFree: false },
	itineraries: [],
	source: {
		provider: 'schedule',
		requestedAt: '2026-09-24T08:00:00Z',
		snapshotAt: '2026-09-24T07:55:00Z'
	}
};

describe('plan cache', () => {
	it('round-trips a plan per query key', () => {
		const s = fakeStorage();
		saveCachedPlan(s, 'from=a&to=b', plan);
		expect(loadCachedPlan(s, 'from=a&to=b')).toEqual(plan);
		// Different query, different slot — no cross-talk.
		expect(loadCachedPlan(s, 'from=b&to=a')).toBeNull();
	});

	it('keys by full params — filtered plans do not shadow the plain one', () => {
		const s = fakeStorage();
		saveCachedPlan(s, 'from=a&to=b', plan);
		const timed: Plan = {
			...plan,
			query: { departAt: '2026-09-25T08:00:00+07:00', stepFree: false }
		};
		saveCachedPlan(s, 'from=a&to=b&at=x', timed);
		expect(loadCachedPlan(s, 'from=a&to=b')).toEqual(plan);
		expect(loadCachedPlan(s, 'from=a&to=b&at=x')).toEqual(timed);
		expect(loadCachedPlan(s, 'from=a&to=b&modes=rail')).toBeNull();
	});

	it('returns null for corrupt or provenance-less entries', () => {
		const s = fakeStorage();
		s.map.set('singgah:plan:from=a&to=b', '{not json');
		expect(loadCachedPlan(s, 'from=a&to=b')).toBeNull();
		s.map.set('singgah:plan:from=a&to=b', JSON.stringify({ ...plan, source: undefined }));
		expect(loadCachedPlan(s, 'from=a&to=b')).toBeNull();
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
		expect(loadCachedPlan(dead, 'from=a&to=b')).toBeNull();
		expect(() => saveCachedPlan(dead, 'from=a&to=b', plan)).not.toThrow();
	});
});
