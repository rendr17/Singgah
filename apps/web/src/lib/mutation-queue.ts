import { authedApi, clearSession } from '$lib/session';

// Offline mutation queue (docs/24). Pending check-ins live in IndexedDB and are
// replayed with the same clientMutationId — the server's idempotency contract
// turns an already-recorded retry into a no-op replay, never a duplicate.
//
// A queued check-in is NOT accepted; the UI must keep showing it as pending
// until the server confirms. 'sending' is deliberately not persisted — a tab
// closed mid-send would otherwise wedge the item forever.
export type CheckinBody = {
	stopId: string;
	clientMutationId: string;
	observedAt: string;
	lat?: number;
	lon?: number;
};

export type QueuedMutation = {
	mutationId: string;
	kind: 'checkin';
	payloadVersion: 1;
	body: CheckinBody;
	createdAt: string;
	retryCount: number;
	// Epoch ms — when the next replay may try this item (exponential backoff).
	nextAttemptAt: number;
	status: 'pending' | 'failed';
};

export interface MutationStore {
	put(m: QueuedMutation): Promise<void>;
	get(id: string): Promise<QueuedMutation | undefined>;
	all(): Promise<QueuedMutation[]>;
	remove(id: string): Promise<void>;
}

export function memoryMutationStore(): MutationStore {
	const map = new Map<string, QueuedMutation>();
	return {
		put: async (m) => void map.set(m.mutationId, structuredClone(m)),
		get: async (id) => map.get(id),
		all: async () => [...map.values()],
		remove: async (id) => void map.delete(id)
	};
}

const DB_NAME = 'singgah';
const STORE = 'mutations';
let dbPromise: Promise<IDBDatabase> | null = null;

function openDb(): Promise<IDBDatabase> {
	dbPromise ??= new Promise((resolve, reject) => {
		const req = indexedDB.open(DB_NAME, 1);
		req.onupgradeneeded = () => req.result.createObjectStore(STORE, { keyPath: 'mutationId' });
		req.onsuccess = () => resolve(req.result);
		req.onerror = () => reject(req.error);
	});
	return dbPromise;
}

function req<T>(r: IDBRequest<T>): Promise<T> {
	return new Promise((resolve, reject) => {
		r.onsuccess = () => resolve(r.result);
		r.onerror = () => reject(r.error);
	});
}

// Browser-only — constructed lazily inside client event handlers/onMount.
export function idbMutationStore(): MutationStore {
	async function withStore<T>(
		mode: IDBTransactionMode,
		fn: (s: IDBObjectStore) => IDBRequest<T>
	): Promise<T> {
		const db = await openDb();
		return req(fn(db.transaction(STORE, mode).objectStore(STORE)));
	}
	return {
		put: (m) => withStore('readwrite', (s) => s.put(m)).then(() => undefined),
		get: (id) => withStore('readonly', (s) => s.get(id)),
		all: () => withStore('readonly', (s) => s.getAll()),
		remove: (id) => withStore('readwrite', (s) => s.delete(id)).then(() => undefined)
	};
}

export async function queueCheckin(store: MutationStore, body: CheckinBody): Promise<void> {
	await store.put({
		mutationId: body.clientMutationId,
		kind: 'checkin',
		payloadVersion: 1,
		body,
		createdAt: new Date().toISOString(),
		retryCount: 0,
		nextAttemptAt: Date.now(),
		status: 'pending'
	});
}

export async function listFailed(store: MutationStore): Promise<QueuedMutation[]> {
	return (await store.all())
		.filter((m) => m.status === 'failed')
		.sort((a, b) => a.createdAt.localeCompare(b.createdAt));
}

// Server-rejected items stay visible instead of vanishing — the user can retry
// (transient-looking rejections like a stopped rate-limit) or discard honestly.
export async function retryFailed(store: MutationStore, id: string): Promise<void> {
	const m = await store.get(id);
	if (!m) return;
	m.status = 'pending';
	m.retryCount = 0;
	m.nextAttemptAt = Date.now();
	await store.put(m);
}

export async function discardFailed(store: MutationStore, id: string): Promise<void> {
	await store.remove(id);
}

export type SendResult =
	| { kind: 'ok'; visitStatus?: 'confirmed' | 'low_confidence' }
	| { kind: 'rejected' }
	| { kind: 'network' };

// sendCheckin doubles as the live path — the button uses it directly so the
// offline queue and the immediate send share one interpretation of outcomes.
export const sendCheckin = async (body: CheckinBody): Promise<SendResult> => {
	try {
		const { data, error, response } = await authedApi.POST('/api/v1/visits', { body });
		if (error || !data) {
			if (!response) return { kind: 'network' };
			if (response.status === 401) clearSession();
			// 429/5xx are transient — retry on the next replay like a network
			// blip. Marking them 'rejected' would fail the item permanently.
			return { kind: response.status === 429 || response.status >= 500 ? 'network' : 'rejected' };
		}
		return { kind: 'ok', visitStatus: data.visit.status };
	} catch {
		return { kind: 'network' };
	}
};

export function backoffMs(retryCount: number): number {
	return Math.min(30_000 * 2 ** retryCount, 60 * 60 * 1000);
}

export async function replayQueue(
	store: MutationStore,
	send: (body: CheckinBody) => Promise<SendResult>,
	now = Date.now()
): Promise<{ sent: number; deferred: number }> {
	const due = (await store.all())
		.filter((m) => m.status === 'pending' && m.nextAttemptAt <= now)
		.sort((a, b) => a.createdAt.localeCompare(b.createdAt));
	let sent = 0;
	let deferred = 0;
	for (const m of due) {
		const result = await send(m.body);
		if (result.kind === 'ok') {
			await store.remove(m.mutationId);
			sent++;
		} else if (result.kind === 'rejected') {
			m.status = 'failed';
			await store.put(m);
		} else {
			m.retryCount++;
			m.nextAttemptAt = now + backoffMs(m.retryCount);
			await store.put(m);
			deferred++;
		}
		notifySettled(m.mutationId, result);
	}
	return { sent, deferred };
}

// UI settlement notifications — a queued CheckinButton flips to its final
// state when the background replay resolves its mutationId.
const listeners = new Map<string, Set<(r: SendResult) => void>>();

export function onSettled(mutationId: string, cb: (r: SendResult) => void): () => void {
	let set = listeners.get(mutationId);
	if (!set) listeners.set(mutationId, (set = new Set()));
	set.add(cb);
	return () => set.delete(cb);
}

function notifySettled(mutationId: string, r: SendResult): void {
	listeners.get(mutationId)?.forEach((cb) => cb(r));
	// 'network' is transient — keep listeners so they still hear the terminal
	// result on a later replay.
	if (r.kind !== 'network') listeners.delete(mutationId);
}

// One driver per page lifetime — 'online' events plus an initial drain on load.
let replaying = false;

export function installQueueDriver(): () => void {
	const store = idbMutationStore();
	const drain = async () => {
		if (replaying || !navigator.onLine) return;
		replaying = true;
		try {
			await replayQueue(store, sendCheckin);
		} finally {
			replaying = false;
		}
	};
	const onOnline = () => void drain();
	window.addEventListener('online', onOnline);
	void drain();
	return () => window.removeEventListener('online', onOnline);
}
