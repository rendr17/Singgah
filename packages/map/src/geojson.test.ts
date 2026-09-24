import { describe, expect, it } from 'vitest';

import { stationsToGeoJSON } from './geojson';

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
			code: 'SUD'
		});
		expect(fc.features[1].properties?.kind).toBe('stop');
	});

	it('returns an empty collection for no stations', () => {
		expect(stationsToGeoJSON([]).features).toHaveLength(0);
	});
});
