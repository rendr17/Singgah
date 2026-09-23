import type { components } from './generated/singgah';

export class ApiError extends Error {
	readonly status: number;
	readonly code: string;
	readonly requestId?: string;

	constructor(status: number, code: string, message: string, requestId?: string) {
		super(message);
		this.name = 'ApiError';
		this.status = status;
		this.code = code;
		this.requestId = requestId;
	}
}

type ErrorEnvelope = components['schemas']['ErrorEnvelope'];

interface FetchResult<T> {
	data?: T;
	error?: unknown;
	response: Response;
}

// unwrap collapses openapi-fetch's {data, error, response} union into either
// the typed payload or a thrown ApiError carrying the contract envelope.
export async function unwrap<T>(result: Promise<FetchResult<T>>): Promise<T> {
	const { data, error, response } = await result;
	if (!response.ok) {
		throw toApiError(response.status, error);
	}
	return data as T;
}

// toApiError preserves envelope fields when the body matches the contract,
// and stays truthful (UNKNOWN) when it does not — e.g. a proxy 502 page.
export function toApiError(status: number, body: unknown): ApiError {
	const env = (body as ErrorEnvelope | undefined)?.error;
	return new ApiError(
		status,
		env?.code ?? 'UNKNOWN',
		env?.message ?? `Request failed with status ${status}`,
		env?.requestId
	);
}
