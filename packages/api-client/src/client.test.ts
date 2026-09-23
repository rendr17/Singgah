import { describe, expect, it } from 'vitest';
import { createSinggahClient } from './client';
import { unwrap } from './errors';

describe('unwrap', () => {
	it('returns data on success', async () => {
		const result = Promise.resolve({
			data: { status: 'ok' as const },
			response: new Response(null, { status: 200 })
		});
		await expect(unwrap(result)).resolves.toEqual({ status: 'ok' });
	});

	it('throws ApiError carrying the envelope fields on failure', async () => {
		const result = Promise.resolve({
			error: {
				error: { code: 'NOT_FOUND', message: 'Resource not found', requestId: 'r1' }
			},
			response: new Response(null, { status: 404 })
		});
		await expect(unwrap(result)).rejects.toMatchObject({
			name: 'ApiError',
			status: 404,
			code: 'NOT_FOUND',
			requestId: 'r1'
		});
	});
});

describe('createSinggahClient', () => {
	it('calls a contract endpoint and returns typed data', async () => {
		const fetchFn = async () =>
			new Response(JSON.stringify({ status: 'ok' }), {
				status: 200,
				headers: { 'Content-Type': 'application/json' }
			});
		const client = createSinggahClient('http://api.test', fetchFn);
		const { data } = await client.GET('/health');
		expect(data?.status).toBe('ok');
	});
});
