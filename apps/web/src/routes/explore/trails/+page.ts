import { api } from '$lib/api';
import { ApiError, unwrap } from '@singgah/api-client';
import type { components } from '@singgah/api-client';
import type { PageLoad } from './$types';

export type TrailSummary = components['schemas']['TrailSummary'];

export const load: PageLoad = async ({ fetch }) => {
	try {
		const result = await unwrap(api.GET('/api/v1/trails', { fetch }));
		return { trails: result.trails, error: '' };
	} catch (error) {
		const message =
			error instanceof ApiError ? `Jalur gagal dimuat (${error.status}).` : 'Jalur gagal dimuat.';
		return { trails: [] as TrailSummary[], error: message };
	}
};
