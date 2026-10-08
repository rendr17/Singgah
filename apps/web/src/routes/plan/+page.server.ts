import { redirect } from '@sveltejs/kit';
import type { PageServerLoad } from './$types';

export const load: PageServerLoad = ({ url }) => {
	const params = new URLSearchParams(url.searchParams);
	params.set('panel', 'plan');
	redirect(307, `/app?${params.toString()}`);
};
