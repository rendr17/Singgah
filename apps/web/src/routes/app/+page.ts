import { api } from '$lib/api';
import { ApiError, unwrap } from '@singgah/api-client';
import type { components } from '@singgah/api-client';
import type { PageLoad } from './$types';

type Collection = components['schemas']['CollectionSummary'];

export const load: PageLoad = async ({ fetch, url }) => {
	if (url.searchParams.get('panel') !== 'explore') {
		return { collections: [] as Collection[], error: '' };
	}

	try {
		const data = await unwrap(api.GET('/api/v1/collections', { fetch }));
		return { collections: data.collections, error: '' };
	} catch (e) {
		const error =
			e instanceof ApiError ? `Koleksi gagal dimuat (${e.status}).` : 'Koleksi gagal dimuat.';
		return { collections: [] as Collection[], error };
	}
};
