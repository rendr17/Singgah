import { describe, expect, it } from 'vitest';

import {
	backoffMs,
	memoryMutationStore,
	onSettled,
	queueCheckin,
	replayQueue,
	type CheckinBody,
	type MutationStore,
	type SendResult
} from './mutation-queue';

function body(id: string): CheckinBody {
	return {
		stopId: '00000000-0000-0000-0000-000000000099',
		clientMutationId: id,
		observedAt: '2026-01-01T08:00:00Z',
		lat: -6.2,
		lon: 106.8
	};
}

async function enqueue(store: MutationStore, id: string, createdAt = '2026-01-01T08:00:00Z') {
	await queueCheckin(store, body(id));
	const m = (await store.get(id))!;
	m.createdAt = createdAt;
	await store.put(m);
}

describe('queueCheckin', () => {
	it('stores the mutation under its idempotency key as pending', async () => {
		const store = memoryMutationStore();
		await queueCheckin(store, body('m-1'));
		const m = await store.get('m-1');
		expect(m?.kind).toBe('checkin');
		expect(m?.status).toBe('pending');
		expect(m?.retryCount).toBe(0);
	});
});

describe('replayQueue', () => {
	it('sends due items oldest-first and removes them on ok', async () => {
		const store = memoryMutationStore();
		await enqueue(store, 'later', '2026-01-01T09:00:00Z');
		await enqueue(store, 'first', '2026-01-01T08:00:00Z');
		const order: string[] = [];
		const r = await replayQueue(store, async (b) => {
			order.push(b.clientMutationId);
			return { kind: 'ok' };
		});
		expect(order).toEqual(['first', 'later']);
		expect(r).toEqual({ sent: 2, deferred: 0 });
		expect(await store.all()).toEqual([]);
	});

	it('notifies settled listeners with the terminal result', async () => {
		const store = memoryMutationStore();
		await enqueue(store, 'm-2');
		let seen: SendResult | undefined;
		onSettled('m-2', (r) => (seen = r));
		await replayQueue(store, async () => ({ kind: 'ok', visitStatus: 'confirmed' }));
		expect(seen).toEqual({ kind: 'ok', visitStatus: 'confirmed' });
	});

	it('keeps network failures pending with backoff instead of dropping them', async () => {
		const store = memoryMutationStore();
		await enqueue(store, 'm-3');
		const now = Date.now();
		const r = await replayQueue(store, async () => ({ kind: 'network' }), now);
		expect(r).toEqual({ sent: 0, deferred: 1 });
		const m = (await store.get('m-3'))!;
		expect(m.status).toBe('pending');
		expect(m.retryCount).toBe(1);
		expect(m.nextAttemptAt).toBe(now + backoffMs(1));
	});

	it('skips items whose backoff window has not arrived yet', async () => {
		const store = memoryMutationStore();
		await enqueue(store, 'm-4');
		const m = (await store.get('m-4'))!;
		m.nextAttemptAt = Date.now() + 60_000;
		await store.put(m);
		let calls = 0;
		await replayQueue(store, async () => {
			calls++;
			return { kind: 'ok' };
		});
		expect(calls).toBe(0);
		expect(await store.get('m-4')).toBeDefined();
	});

	it('marks rejected items failed and leaves them for inspection', async () => {
		const store = memoryMutationStore();
		await enqueue(store, 'm-5');
		await replayQueue(store, async () => ({ kind: 'rejected' }));
		expect((await store.get('m-5'))?.status).toBe('failed');
	});

	it('does not re-send failed items on later replays', async () => {
		const store = memoryMutationStore();
		await enqueue(store, 'm-6');
		await replayQueue(store, async () => ({ kind: 'rejected' }));
		let calls = 0;
		await replayQueue(store, async () => {
			calls++;
			return { kind: 'ok' };
		});
		expect(calls).toBe(0);
	});
});

describe('backoffMs', () => {
	it('doubles each retry and caps at one hour', () => {
		expect(backoffMs(0)).toBe(30_000);
		expect(backoffMs(1)).toBe(60_000);
		expect(backoffMs(20)).toBe(3_600_000);
	});
});
