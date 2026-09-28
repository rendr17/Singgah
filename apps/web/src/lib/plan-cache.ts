import type { components } from '@singgah/api-client';

type Plan = components['schemas']['JourneyPlan'];

// Recent plans persist per query — reopening a shared link while offline
// (or during an outage) shows the last fetched copy. Storage is injected
// so the helpers stay testable outside a browser.
const PLAN_CACHE = 'singgah:plan:';

type StorageLike = Pick<Storage, 'getItem' | 'setItem'>;

// Plans for different inputs are different answers — the caller passes the
// full serialized query (from, to, at|arriveBy, filters) so a filtered or
// timed plan never shadows the leave-now one.
export function loadCachedPlan(storage: StorageLike, queryKey: string): Plan | null {
	try {
		const raw = storage.getItem(PLAN_CACHE + queryKey);
		const cached = raw ? (JSON.parse(raw) as Plan) : null;
		// A cache without provenance can't be labeled honestly — treat it as absent.
		return cached?.source?.requestedAt ? cached : null;
	} catch {
		return null; // private mode / corrupt entry — cache is best-effort
	}
}

export function saveCachedPlan(storage: StorageLike, queryKey: string, p: Plan) {
	try {
		storage.setItem(PLAN_CACHE + queryKey, JSON.stringify(p));
	} catch {
		// quota/private mode — the plan still renders, it just isn't persisted
	}
}
