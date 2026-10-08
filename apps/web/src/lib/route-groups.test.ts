import { describe, expect, it } from 'vitest';
import type { components } from '@singgah/api-client';

import { badgeTextColor, groupRoutes, matchesQuery } from './route-groups';

type Route = components['schemas']['RouteSummary'];

function route(over: Partial<Route>): Route {
	return { id: crypto.randomUUID(), mode: 'bus', providerCode: 'commute', ...over };
}

describe('groupRoutes', () => {
	it('groups by agencyCode and orders rail modes before bus', () => {
		const groups = groupRoutes([
			route({ agencyCode: 'TJ', agencyName: 'TransJakarta', mode: 'bus', shortName: '1' }),
			route({ agencyCode: 'KCI', agencyName: 'Commuter Line', mode: 'rail', shortName: 'C' }),
			route({ agencyCode: 'MRTJ', agencyName: 'MRT Jakarta', mode: 'subway', shortName: 'M' }),
			route({ agencyCode: 'TJ', agencyName: 'TransJakarta', mode: 'bus', shortName: '2' })
		]);
		expect(groups.map((g) => g.key)).toEqual(['KCI', 'MRTJ', 'TJ']);
		expect(groups[2].routes).toHaveLength(2);
	});

	it('orders same-mode groups alphabetically by agency name', () => {
		const groups = groupRoutes([
			route({ agencyCode: 'LRTJBDB', agencyName: 'LRT Jabodebek', mode: 'tram', shortName: 'CB' }),
			route({ agencyCode: 'LRTJ', agencyName: 'LRT Jakarta', mode: 'tram', shortName: 'S' })
		]);
		expect(groups.map((g) => g.name)).toEqual(['LRT Jabodebek', 'LRT Jakarta']);
	});

	it('sorts route codes numerically, not lexicographically', () => {
		const groups = groupRoutes([
			route({ agencyCode: 'TJ', mode: 'bus', shortName: '10' }),
			route({ agencyCode: 'TJ', mode: 'bus', shortName: '1A' }),
			route({ agencyCode: 'TJ', mode: 'bus', shortName: '2' }),
			route({ agencyCode: 'TJ', mode: 'bus', shortName: '1' })
		]);
		expect(groups[0].routes.map((r) => r.shortName)).toEqual(['1', '1A', '2', '10']);
	});

	it('keeps routes without a shortName at the end of their group', () => {
		const groups = groupRoutes([
			route({ agencyCode: 'X', mode: 'bus', longName: 'Tanpa kode' }),
			route({ agencyCode: 'X', mode: 'bus', shortName: '9' })
		]);
		expect(groups[0].routes.map((r) => r.shortName)).toEqual(['9', undefined]);
	});

	it('falls back to agencyCode then a placeholder for missing names', () => {
		const groups = groupRoutes([
			route({ agencyCode: 'APCGK', mode: 'other' }),
			route({ mode: 'other' })
		]);
		expect(groups[0].name).toBe('APCGK');
		expect(groups[1].name).toBe('Operator lain');
	});
});

describe('matchesQuery', () => {
	const r = route({
		shortName: '10C',
		longName: 'Pelabuhan Tanjung Priok',
		agencyName: 'TransJakarta'
	});
	it('matches code, corridor, and operator case-insensitively', () => {
		expect(matchesQuery(r, '10c')).toBe(true);
		expect(matchesQuery(r, 'priok')).toBe(true);
		expect(matchesQuery(r, 'TRANS')).toBe(true);
		expect(matchesQuery(r, 'bogor')).toBe(false);
	});
	it('matches everything on blank query', () => {
		expect(matchesQuery(r, '   ')).toBe(true);
	});
});

describe('badgeTextColor', () => {
	it('returns white on dark line colors', () => {
		expect(badgeTextColor('312F92')).toBe('#ffffff'); // TJ koridor 11 navy
		expect(badgeTextColor('262262')).toBe('#ffffff'); // KCI A deep indigo
	});
	it('returns ink on light line colors', () => {
		expect(badgeTextColor('F5AB6E')).toBe('#141719'); // TJ koridor 14 peach
		expect(badgeTextColor('25B8EB')).toBe('#141719'); // KCI C light blue
		expect(badgeTextColor('EE3D43')).toBe('#141719'); // KCI B red — ink beats white 4.5:3.9
		expect(badgeTextColor('ED4F98')).toBe('#141719'); // KCI TP pink — ink beats white 5.2:3.4
	});
	it('tolerates 3-digit and #-prefixed hex', () => {
		expect(badgeTextColor('#312F92')).toBe('#ffffff');
		expect(badgeTextColor('fff')).toBe('#141719');
	});
	it('defaults to white on garbage input', () => {
		expect(badgeTextColor('not-a-color')).toBe('#ffffff');
	});
});
