import { api } from '$lib/api';
import { ApiError, unwrap } from '@singgah/api-client';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ params, fetch }) => {
	try {
		const data = await unwrap(
			api.GET('/api/v1/routes/{id}', { params: { path: { id: params.id } }, fetch })
		);
		return { route: data.route, error: '' };
	} catch (e) {
		const status = e instanceof ApiError ? e.status : 0;
		const message =
			status === 404
				? 'Rute tidak ditemukan.'
				: e instanceof Error
					? e.message
					: 'Data rute gagal dimuat.';
		return { route: null, error: message };
	}
};
