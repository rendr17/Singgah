import { createSinggahClient } from '@singgah/api-client';
import { env } from '$env/dynamic/public';

// One client per runtime — base URL is public env, not a secret.
export const apiBase = env.PUBLIC_API_BASE_URL ?? 'http://localhost:8080';
export const api = createSinggahClient(apiBase);
