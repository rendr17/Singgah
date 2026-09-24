import type { components } from '@singgah/api-client';

type Plan = components['schemas']['JourneyPlan'];

// Recent plans persist per station pair — reopening a shared link while
// offline (or during a provider outage) shows the last fetched copy.
// Storage is injected so the helpers stay testable outside a browser.
const PLAN_CACHE = 'singgah:plan:';

type StorageLike = Pick<Storage, 'getItem' | 'setItem'>;

export function loadCachedPlan(storage: StorageLike, fromId: string, toId: string): Plan | null {
	try {
		const raw = storage.getItem(PLAN_CACHE + fromId + ':' + toId);
		const cached = raw ? (JSON.parse(raw) as Plan) : null;
		// A cache without provenance can't be labeled honestly — treat it as absent.
		return cached?.source?.requestedAt ? cached : null;
	} catch {
		return null; // private mode / corrupt entry — cache is best-effort
	}
}

export function saveCachedPlan(storage: StorageLike, fromId: string, toId: string, p: Plan) {
	try {
		storage.setItem(PLAN_CACHE + fromId + ':' + toId, JSON.stringify(p));
	} catch {
		// quota/private mode — the plan still renders, it just isn't persisted
	}
}
