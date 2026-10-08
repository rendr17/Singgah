import type { Feature, FeatureCollection, LineString, Point, Position } from 'geojson';

import { operatorIcon } from './operators';

// Minimal shape the map needs — structurally compatible with the generated
// StationSummary/StopRef without coupling this package to the API client.
export interface MapStation {
	id: string;
	name: string;
	lat: number;
	lon: number;
	kind?: string;
	code?: string;
	operator?: string;
}

export function stationsToGeoJSON(stations: MapStation[]): FeatureCollection<Point> {
	return {
		type: 'FeatureCollection',
		features: stations.map((s): Feature<Point> => ({
			type: 'Feature',
			geometry: { type: 'Point', coordinates: [s.lon, s.lat] },
			properties: {
				id: s.id,
				name: s.name,
				kind: s.kind ?? 'stop',
				code: s.code ?? '',
				operator: s.operator ?? '',
				icon: operatorIcon(s.operator)
			}
		}))
	};
}

// Minimal leg shape the overlay needs — compatible with the contract's
// JourneyLeg without coupling to the API client. `JourneyStopRef` carries no
// coordinates, so walk legs and stop markers can't be drawn from the plan
// alone: ride legs render as LineStrings, endpoints are recovered from the
// outermost geometry coordinates (transfer markers are Phase 6).
export interface MapJourneyLeg {
	type: string;
	from?: { name: string };
	to?: { name: string };
	/** Corridor color, hex without '#' — the trail wears the line's hue. */
	color?: string;
	geometry?: { type: 'LineString'; coordinates: Position[] };
}

export function journeyToGeoJSON(legs: MapJourneyLeg[]): FeatureCollection {
	const features: Feature[] = [];
	for (const leg of legs) {
		if (leg.type === 'walk' || !leg.geometry) continue;
		features.push({
			type: 'Feature',
			geometry: leg.geometry as LineString,
			properties: { dashed: false, color: leg.color ?? '' }
		});
	}
	const first = features[0]?.geometry as LineString | undefined;
	const last = features[features.length - 1]?.geometry as LineString | undefined;
	const fromName = legs.find((l) => l.geometry)?.from?.name;
	const toName = [...legs].reverse().find((l) => l.geometry)?.to?.name;
	if (first) {
		features.push({
			type: 'Feature',
			geometry: { type: 'Point', coordinates: first.coordinates[0] },
			properties: { endpoint: true, endpointRole: 'origin', name: fromName ?? '' }
		});
	}
	// Same LineString for a single-leg journey still gets both endpoints —
	// first[0] and last[end] are different coordinates.
	if (last) {
		features.push({
			type: 'Feature',
			geometry: { type: 'Point', coordinates: last.coordinates[last.coordinates.length - 1] },
			properties: { endpoint: true, endpointRole: 'destination', name: toName ?? '' }
		});
	}
	return { type: 'FeatureCollection', features };
}
