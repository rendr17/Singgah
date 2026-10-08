import { api } from '$lib/api';
import { ApiError, unwrap } from '@singgah/api-client';
import type { components } from '@singgah/api-client';
import type { PageLoad } from './$types';

export type Trail = components['schemas']['TrailDetail'];

export const load: PageLoad = async ({ fetch, params }) => {
	try {
		const result = await unwrap(
			api.GET('/api/v1/trails/{slug}', { params: { path: { slug: params.slug } }, fetch })
		);
		return { trail: result, error: '' };
	} catch (error) {
		const message =
			error instanceof ApiError && error.status === 404
				? 'Jalur ini tidak ditemukan.'
				: error instanceof ApiError
					? `Detail jalur gagal dimuat (${error.status}).`
					: 'Detail jalur gagal dimuat.';
		return { trail: null as Trail | null, error: message };
	}
};
