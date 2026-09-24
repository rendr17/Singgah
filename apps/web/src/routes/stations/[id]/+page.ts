import { api } from '$lib/api';
import { ApiError, unwrap } from '@singgah/api-client';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ params, fetch }) => {
	try {
		const data = await unwrap(
			api.GET('/api/v1/stations/{id}', { params: { path: { id: params.id } }, fetch })
		);
		return { station: data.station, error: '' };
	} catch (e) {
		const status = e instanceof ApiError ? e.status : 0;
		const message =
			status === 404
				? 'Stasiun tidak ditemukan.'
				: e instanceof Error
					? e.message
					: 'Data stasiun gagal dimuat.';
		return { station: null, error: message };
	}
};
