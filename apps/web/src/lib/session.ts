import { api, apiBase } from '$lib/api';
import { createSinggahClient } from '@singgah/api-client';

// Anonymous-first session (ADR-010). The raw token lives only in localStorage —
// never in URLs or logs. Losing it means losing the account; that trade-off is
// surfaced in the passport UX, not hidden here.
const STORAGE_KEY = 'singgah.session';
// Refresh the token ahead of expiry so a session opened days later still works.
const EXPIRY_MARGIN_MS = 24 * 60 * 60 * 1000;

export type StoredSession = { token: string; expiresAt: string; userId: string };

export type StorageLike = Pick<Storage, 'getItem' | 'setItem' | 'removeItem'>;

export function parseStoredSession(raw: string | null, now = new Date()): StoredSession | null {
	if (!raw) return null;
	try {
		const s = JSON.parse(raw) as Partial<StoredSession>;
		if (typeof s.token !== 'string' || typeof s.expiresAt !== 'string' || typeof s.userId !== 'string')
			return null;
		const expiresAt = Date.parse(s.expiresAt);
		if (Number.isNaN(expiresAt) || now.getTime() + EXPIRY_MARGIN_MS >= expiresAt) return null;
		return { token: s.token, expiresAt: s.expiresAt, userId: s.userId };
	} catch {
		return null;
	}
}

export function readSession(storage: StorageLike): StoredSession | null {
	return parseStoredSession(storage.getItem(STORAGE_KEY));
}

// Browser-only by contract — all callers run in client event handlers/onMount.
export function clearSession(storage: StorageLike = localStorage): void {
	storage.removeItem(STORAGE_KEY);
}

// Concurrent callers share one mint — a station page and the passport view can
// race to the token without creating two accounts.
let minting: Promise<StoredSession> | null = null;

export async function ensureSession(): Promise<StoredSession> {
	const stored = readSession(localStorage);
	if (stored) return stored;
	minting ??= mint().finally(() => (minting = null));
	return minting;
}

async function mint(): Promise<StoredSession> {
	const { data, error, response } = await api.POST('/api/v1/auth/session');
	if (error || !data) throw new Error(`Sesi gagal dibuat (${response.status})`);
	const session: StoredSession = {
		token: data.token,
		expiresAt: data.expiresAt,
		userId: data.userId
	};
	localStorage.setItem(STORAGE_KEY, JSON.stringify(session));
	return session;
}

export async function authedFetch(input: RequestInfo | URL, init: RequestInit = {}) {
	const session = await ensureSession();
	const headers = new Headers(init.headers);
	headers.set('Authorization', `Bearer ${session.token}`);
	return fetch(input, { ...init, headers });
}

// Browser-only client — every call lazily ensures a session. SSR code paths
// must keep using `api` (no Authorization header on the server).
export const authedApi = createSinggahClient(apiBase, authedFetch);
