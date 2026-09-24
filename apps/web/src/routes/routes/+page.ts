import { api } from '$lib/api';
import { unwrap } from '@singgah/api-client';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ fetch }) => {
	try {
		const data = await unwrap(api.GET('/api/v1/routes', { fetch }));
		return { routes: data.routes, error: '' };
	} catch (e) {
		return { routes: [], error: e instanceof Error ? e.message : 'Daftar rute gagal dimuat.' };
	}
};
