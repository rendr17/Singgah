import { api } from '$lib/api';
import { ApiError, unwrap } from '@singgah/api-client';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ fetch }) => {
	try {
		const data = await unwrap(api.GET('/api/v1/providers', { fetch }));
		return { providers: data.providers, error: '' };
	} catch (e) {
		const message = e instanceof ApiError ? e.message : 'Data penyedia gagal dimuat.';
		return { providers: [], error: message };
	}
};
