import { api } from '$lib/api';
import { ApiError, unwrap } from '@singgah/api-client';
import type { components } from '@singgah/api-client';
import type { PageLoad } from './$types';

export type Place = components['schemas']['PlaceDetail'];

export const load: PageLoad = async ({ fetch, params }) => {
	try {
		const result = await unwrap(
			api.GET('/api/v1/places/{id}', { params: { path: { id: params.id } }, fetch })
		);
		return { place: result.place, error: '' };
	} catch (error) {
		const message =
			error instanceof ApiError && error.status === 404
				? 'Tempat tidak ditemukan atau sudah tidak tersedia.'
				: error instanceof ApiError
					? `Detail tempat gagal dimuat (${error.status}).`
					: 'Detail tempat gagal dimuat.';
		return { place: null as Place | null, error: message };
	}
};
