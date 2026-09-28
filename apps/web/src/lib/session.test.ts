import { describe, expect, it } from 'vitest';

import {
	clearSession,
	parseStoredSession,
	readSession,
	type StoredSession,
	type StorageLike
} from './session';

const valid: StoredSession = {
	token: 'tok-abc',
	expiresAt: '2999-01-01T00:00:00Z',
	userId: '00000000-0000-0000-0000-000000000001'
};

function fakeStorage(initial: Record<string, string> = {}): StorageLike & {
	map: Map<string, string>;
} {
	const map = new Map(Object.entries(initial));
	return {
		map,
		getItem: (k: string) => map.get(k) ?? null,
		setItem: (k: string, v: string) => void map.set(k, v),
		removeItem: (k: string) => void map.delete(k)
	};
}

describe('parseStoredSession', () => {
	it('returns the stored session when the token is fresh', () => {
		expect(parseStoredSession(JSON.stringify(valid), new Date('2025-01-01'))).toEqual(valid);
	});

	it('rejects sessions inside the 24h expiry margin', () => {
		const s = { ...valid, expiresAt: '2025-01-02T00:00:00Z' };
		expect(parseStoredSession(JSON.stringify(s), new Date('2025-01-01T06:00:00Z'))).toBeNull();
	});

	it.each([
		['missing storage entry', null],
		['non-JSON garbage', 'not json'],
		['missing token', JSON.stringify({ expiresAt: valid.expiresAt, userId: valid.userId })],
		['non-string fields', JSON.stringify({ token: 1, expiresAt: 2, userId: 3 })]
	])('returns null on %s', (_label, raw) => {
		expect(parseStoredSession(raw)).toBeNull();
	});
});

describe('readSession / clearSession', () => {
	it('round-trips through the storage key', () => {
		const storage = fakeStorage({ 'singgah.session': JSON.stringify(valid) });
		expect(readSession(storage)).toEqual(valid);
		clearSession(storage);
		expect(storage.map.size).toBe(0);
		expect(readSession(storage)).toBeNull();
	});

	it('treats malformed stored JSON as absent', () => {
		const storage = fakeStorage({ 'singgah.session': '{{{' });
		expect(readSession(storage)).toBeNull();
	});
});
