// Grouping/ordering for the /routes catalog. Routes are grouped by agency so
// rail operators aren't drowned out by the long TransJakarta corridor list —
// TJ alone is ~100 rows while every other operator is single digits.
import type { components } from '@singgah/api-client';

type Route = components['schemas']['RouteSummary'];
export type RouteMode = Route['mode'];

export interface RouteGroup {
	/** agencyCode — stable key for chip state and each-blocks. */
	key: string;
	name: string;
	mode: RouteMode;
	routes: Route[];
}

// Rail-first ordering mirrors how riders think of the network (fixed lines
// first, the bus corridor tail last); alphabetical inside the same mode.
const MODE_RANK: Record<RouteMode, number> = {
	rail: 0,
	subway: 1,
	tram: 2,
	bus: 3,
	ferry: 4,
	other: 5
};

export const MODE_LABELS: Record<RouteMode, string> = {
	rail: 'Kereta',
	subway: 'MRT',
	tram: 'LRT',
	bus: 'Bus',
	ferry: 'Feri',
	other: 'Lainnya'
};

// Mode → ModeIcon glyph key. Modes without a pictogram render label-only.
export const MODE_ICON: Partial<Record<RouteMode, 'commuter' | 'metro' | 'bus' | 'light-rail'>> = {
	rail: 'commuter',
	subway: 'metro',
	tram: 'light-rail',
	bus: 'bus'
};

// Numeric collation so corridor codes order the way riders read them:
// 1, 1A, 2, 9, 10 — not the lexicographic 1, 10, 1A, 2 the DB gives.
const collator = new Intl.Collator('id', { numeric: true, sensitivity: 'base' });

function compareRoutes(a: Route, b: Route): number {
	if (!!a.shortName !== !!b.shortName) return a.shortName ? -1 : 1;
	return collator.compare(a.shortName ?? a.longName ?? '', b.shortName ?? b.longName ?? '');
}

export function groupRoutes(routes: Route[]): RouteGroup[] {
	const byKey = new Map<string, RouteGroup>();
	for (const r of routes) {
		const key = r.agencyCode || r.agencyName || 'lainnya';
		let g = byKey.get(key);
		if (!g) {
			g = { key, name: r.agencyName || r.agencyCode || 'Operator lain', mode: r.mode, routes: [] };
			byKey.set(key, g);
		}
		g.routes.push(r);
		// Agencies are single-mode in practice; if a provider ever mixes modes
		// in one agency, the "most rail" mode drives ordering and the label.
		if (MODE_RANK[r.mode] < MODE_RANK[g.mode]) g.mode = r.mode;
	}
	const groups = [...byKey.values()];
	for (const g of groups) g.routes.sort(compareRoutes);
	groups.sort((a, b) => MODE_RANK[a.mode] - MODE_RANK[b.mode] || collator.compare(a.name, b.name));
	return groups;
}

/** Case-insensitive match on the fields a rider would type: line code, corridor name, operator. */
export function matchesQuery(r: Route, q: string): boolean {
	const needle = q.trim().toLowerCase();
	if (!needle) return true;
	return [r.shortName, r.longName, r.agencyName].some((f) => f?.toLowerCase().includes(needle));
}

const INK_LUMINANCE = 0.0095; // --sg-text #141719, precomputed sRGB luminance

function channel(c: number): number {
	const s = c / 255;
	return s <= 0.04045 ? s / 12.92 : Math.pow((s + 0.055) / 1.055, 2.4);
}

function parseHex(hex: string): [number, number, number] | null {
	const h = hex.replace(/^#/, '');
	const full = h.length === 3 ? [...h].map((c) => c + c).join('') : h;
	if (!/^[0-9a-fA-F]{6}$/.test(full)) return null;
	return [
		parseInt(full.slice(0, 2), 16),
		parseInt(full.slice(2, 4), 16),
		parseInt(full.slice(4, 6), 16)
	];
}

/**
 * Readable text color on a route-color badge. Provider colors aren't audited
 * for contrast, so pick whichever of white/ink wins a WCAG ratio rather than
 * assuming dark lines always take white.
 */
export function badgeTextColor(hex: string): string {
	const rgb = parseHex(hex);
	if (!rgb) return '#ffffff';
	const L = 0.2126 * channel(rgb[0]) + 0.7152 * channel(rgb[1]) + 0.0722 * channel(rgb[2]);
	const vsWhite = 1.05 / (L + 0.05);
	const vsInk = (L + 0.05) / (INK_LUMINANCE + 0.05);
	return vsWhite >= vsInk ? '#ffffff' : '#141719';
}
