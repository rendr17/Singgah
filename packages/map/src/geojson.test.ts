import { describe, expect, it } from 'vitest';

import { journeyToGeoJSON, stationsToGeoJSON } from './geojson';

describe('stationsToGeoJSON', () => {
	it('maps stations to WGS84 point features', () => {
		const fc = stationsToGeoJSON([
			{ id: 'a', name: 'Sudirman', lat: -6.2024, lon: 106.8237, kind: 'station', code: 'SUD' },
			{ id: 'b', name: 'Halte Tosari', lat: -6.19, lon: 106.82 }
		]);
		expect(fc.type).toBe('FeatureCollection');
		expect(fc.features).toHaveLength(2);
		expect(fc.features[0].geometry).toEqual({
			type: 'Point',
			coordinates: [106.8237, -6.2024]
		});
		expect(fc.features[0].properties).toEqual({
			id: 'a',
			name: 'Sudirman',
			kind: 'station',
			code: 'SUD',
			operator: '',
			icon: ''
		});
		expect(fc.features[1].properties?.kind).toBe('stop');
	});

	it('maps known operators to their mark, unknown ones stay unmarked', () => {
		const fc = stationsToGeoJSON([
			{ id: 'a', name: 'Bundaran HI', lat: -6.19, lon: 106.82, operator: 'MRTJ' },
			{ id: 'b', name: 'Halte Tosari', lat: -6.19, lon: 106.82, operator: 'TJ' },
			{ id: 'c', name: 'Skytrain', lat: -6.12, lon: 106.65, operator: 'APCGK' }
		]);
		expect(fc.features[0].properties?.icon).toBe('op-mrtj');
		expect(fc.features[0].properties?.operator).toBe('MRTJ');
		expect(fc.features[1].properties?.icon).toBe('op-tj');
		// unsurveyed/unsupported operators must not be guessed
		expect(fc.features[2].properties?.icon).toBe('');
		expect(fc.features[2].properties?.operator).toBe('APCGK');
	});

	it('returns an empty collection for no stations', () => {
		expect(stationsToGeoJSON([]).features).toHaveLength(0);
	});
});

describe('journeyToGeoJSON', () => {
	const line = (coords: [number, number][]) =>
		({ type: 'LineString', coordinates: coords }) as const;

	it('draws ride legs as LineStrings and endpoints from outermost coords', () => {
		const fc = journeyToGeoJSON([
			{
				type: 'ride',
				from: { name: 'A' },
				to: { name: 'B' },
				color: '312F92',
				geometry: line([
					[106.8, -6.2],
					[106.81, -6.21]
				])
			},
			{
				type: 'ride',
				from: { name: 'B' },
				to: { name: 'C' },
				geometry: line([
					[106.81, -6.21],
					[106.82, -6.22]
				])
			}
		]);
		const [leg1, leg2, orig, dest] = fc.features;
		expect(leg1.geometry.type).toBe('LineString');
		expect(leg1.properties?.color).toBe('312F92');
		expect(leg2.properties?.color).toBe('');
		expect(leg2.geometry.type).toBe('LineString');
		expect(orig.geometry).toEqual({ type: 'Point', coordinates: [106.8, -6.2] });
		expect(orig.properties).toEqual({ endpoint: true, endpointRole: 'origin', name: 'A' });
		expect(dest.geometry).toEqual({ type: 'Point', coordinates: [106.82, -6.22] });
		expect(dest.properties).toEqual({ endpoint: true, endpointRole: 'destination', name: 'C' });
	});

	it('keeps both endpoints on a single-leg journey', () => {
		const fc = journeyToGeoJSON([
			{
				type: 'ride',
				from: { name: 'A' },
				to: { name: 'B' },
				geometry: line([
					[106.8, -6.2],
					[106.9, -6.3]
				])
			}
		]);
		expect(fc.features).toHaveLength(3);
		expect(fc.features[1].geometry).toEqual({ type: 'Point', coordinates: [106.8, -6.2] });
		expect(fc.features[2].geometry).toEqual({ type: 'Point', coordinates: [106.9, -6.3] });
	});

	it('skips walk legs and legs without geometry — never invented coordinates', () => {
		const fc = journeyToGeoJSON([
			{ type: 'walk', from: { name: 'A' }, to: { name: 'B' } },
			{ type: 'ride', from: { name: 'B' }, to: { name: 'C' } },
			{
				type: 'ride',
				from: { name: 'C' },
				to: { name: 'D' },
				geometry: line([
					[106.8, -6.2],
					[106.9, -6.3]
				])
			}
		]);
		expect(fc.features).toHaveLength(3); // one line + two endpoints
		expect(fc.features[0].geometry.type).toBe('LineString');
	});

	it('returns an empty collection when nothing is drawable', () => {
		expect(journeyToGeoJSON([]).features).toHaveLength(0);
		expect(journeyToGeoJSON([{ type: 'walk' }]).features).toHaveLength(0);
	});
});
