import createClient from 'openapi-fetch';
import type { paths } from './generated/singgah';

export type SinggahClient = ReturnType<typeof createSinggahClient>;

// fetchFn is injectable so tests and non-browser runtimes can swap it.
export function createSinggahClient(baseUrl: string, fetchFn?: typeof fetch) {
	return createClient<paths>({ baseUrl, fetch: fetchFn });
}
