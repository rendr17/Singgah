import { api } from '$lib/api';
import { ApiError, unwrap } from '@singgah/api-client';
import type { components } from '@singgah/api-client';
import type { PageLoad } from './$types';

type StationDetail = components['schemas']['StationDetail'];
type StationDepartures = components['schemas']['StationDepartures'];
type PlaceSummary = components['schemas']['PlaceSummary'];

export const load: PageLoad = async ({ params, fetch }) => {
	let station: StationDetail | null = null;
	let stationError = '';
	try {
		const data = await unwrap(
			api.GET('/api/v1/stations/{id}', { params: { path: { id: params.id } }, fetch })
		);
		station = data.station;
	} catch (e) {
		const status = e instanceof ApiError ? e.status : 0;
		stationError =
			status === 404
				? 'Stasiun tidak ditemukan.'
				: e instanceof Error
					? e.message
					: 'Data stasiun gagal dimuat.';
	}

	// Full-day board for the "Jadwal" section. A timetable outage degrades to
	// no board — it must not take the station detail down with it.
	let departures: StationDepartures | null = null;
	if (station) {
		try {
			const data = await unwrap(
				api.GET('/api/v1/stations/{id}/departures', {
					params: { path: { id: params.id }, query: { window: 1440 } },
					fetch
				})
			);
			departures = data.departures;
		} catch {
			departures = null;
		}
	}

	// City Explorer candidates anchored on this stop (ADR-011). Same
	// degradation rule as departures: a places outage must not sink the page.
	let places: PlaceSummary[] | null = null;
	if (station) {
		try {
			const data = await unwrap(
				api.GET('/api/v1/places', {
					params: { query: { near_stop_id: params.id, limit: 50 } },
					fetch
				})
			);
			places = data.places;
		} catch {
			places = null;
		}
	}

	return { station, departures, places, error: stationError };
};
