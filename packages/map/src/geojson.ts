import type { Feature, FeatureCollection, Point } from 'geojson';

// Minimal shape the map needs — structurally compatible with the generated
// StationSummary/StopRef without coupling this package to the API client.
export interface MapStation {
	id: string;
	name: string;
	lat: number;
	lon: number;
	kind?: string;
	code?: string;
}

export function stationsToGeoJSON(stations: MapStation[]): FeatureCollection<Point> {
	return {
		type: 'FeatureCollection',
		features: stations.map((s): Feature<Point> => ({
			type: 'Feature',
			geometry: { type: 'Point', coordinates: [s.lon, s.lat] },
			properties: { id: s.id, name: s.name, kind: s.kind ?? 'stop', code: s.code ?? '' }
		}))
	};
}
