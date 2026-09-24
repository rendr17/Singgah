import type { Handle } from '@sveltejs/kit';

// openapi-fetch inspects Content-Length/Transfer-Encoding to pick a body
// parser. Load-function fetches are serialized for hydration, and reading a
// header outside this allowlist throws — so both must be forwarded.
export const handle: Handle = ({ event, resolve }) =>
	resolve(event, {
		filterSerializedResponseHeaders: (name) =>
			name === 'content-length' || name === 'transfer-encoding'
	});
