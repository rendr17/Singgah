// Schematic map builder — generates the Integration Map's authored artwork
// from live catalog data (docs/INTEGRATION_MAP_BLUEPRINT.md §36–§37).
//
// NOTE: the shipped integration artwork is currently the FDTJ tile set
// (apps/web/static/maps/integration/fdtj/, points via import-commute-
// points.mjs). This generator is the fallback pipeline for rebuilding a
// self-authored map if that artwork ever has to be pulled or extended past
// its coverage — running it overwrites manifest.json/points.json, which are
// committed assets whose diff is reviewable.
//
// Inputs (local API): route line geometry (/map/lines), route stop orders
// (/routes/{id}), and stations (/stations). Output is committed to
// apps/web/static/maps/integration/ — the runtime never derives schematic
// coordinates itself; generation runs offline like every asset pipeline.
//
//   node tools/schematic-map-builder/generate.mjs
//   SINGGAH_API=http://localhost:8080 node tools/schematic-map-builder/generate.mjs
//
// Layout: schematic space is seeded from geography (uniform equirectangular
// scale), then rail lines are snapped to octilinear doglegs and routes
// sharing stations get parallel lane offsets. Bus corridors keep smooth
// thin polylines. This is authored-by-code artwork — not a copy of any
// operator/FDTJ map (licensing: docs/35_DATA_SOURCES.md).

const API = process.env.SINGGAH_API ?? 'http://localhost:8080';
const OUT = new URL('../../apps/web/static/maps/integration/', import.meta.url);

// Full-Jakarta bbox — wider than the station bounds so nothing clips.
const BBOX = '105.90,-6.70,107.25,-5.80';

const WORLD_W = 3600;
const PAD = 140;
const RAIL_AGENCIES = new Set(['KCI', 'MRTJ', 'LRTJ', 'LRTJBDB', 'APCGK']);
const RAIL_W = 15;
const BUS_W = 6;
const BUS_OPACITY = 0.45;
const INTERCHANGE_M = 450; // proximity cluster radius for interchange pills
const SIMPLIFY_PX_RAIL = 60;
const SIMPLIFY_PX_BUS = 45;
const LANE_STEP = 11;
const ANCHOR_MAX_PX = 90; // max distance a station may be pulled onto a path

async function get(path) {
	const res = await fetch(`${API}${path}`);
	if (!res.ok) throw new Error(`${path} -> ${res.status}`);
	return res.json();
}

// --- geometry helpers ------------------------------------------------------

function dpSimplify(pts, eps) {
	if (pts.length <= 2) return pts.slice();
	const keep = new Uint8Array(pts.length);
	keep[0] = keep[pts.length - 1] = 1;
	const stack = [[0, pts.length - 1]];
	while (stack.length) {
		const [a, b] = stack.pop();
		let maxD = 0;
		let idx = -1;
		for (let i = a + 1; i < b; i++) {
			const d = segDist(pts[i], pts[a], pts[b]);
			if (d > maxD) {
				maxD = d;
				idx = i;
			}
		}
		if (idx >= 0 && maxD > eps) {
			keep[idx] = 1;
			stack.push([a, idx], [idx, b]);
		}
	}
	return pts.filter((_, i) => keep[i]);
}

function segDist(p, a, b) {
	const dx = b.x - a.x;
	const dy = b.y - a.y;
	const l2 = dx * dx + dy * dy || 1;
	const t = Math.max(0, Math.min(1, ((p.x - a.x) * dx + (p.y - a.y) * dy) / l2));
	return Math.hypot(p.x - (a.x + dx * t), p.y - (a.y + dy * t));
}

/** Nearest point on a polyline to p — returns [x, y]. */
function nearestOnLine(p, line) {
	let best = null;
	let bestD = Infinity;
	for (let i = 0; i < line.length - 1; i++) {
		const a = line[i];
		const b = line[i + 1];
		const dx = b.x - a.x;
		const dy = b.y - a.y;
		const l2 = dx * dx + dy * dy || 1;
		const t = Math.max(0, Math.min(1, ((p.x - a.x) * dx + (p.y - a.y) * dy) / l2));
		const x = a.x + dx * t;
		const y = a.y + dy * t;
		const d = (p.x - x) ** 2 + (p.y - y) ** 2;
		if (d < bestD) {
			bestD = d;
			best = [x, y];
		}
	}
	return best;
}

/** Octilinear dogleg between two schematic points: straight when already
 *  octilinear, else one elbow — dominant axis first, then the 45° run. */
function dogleg(a, b) {
	const dx = b.x - a.x;
	const dy = b.y - a.y;
	if (Math.abs(dx) < 1e-6 || Math.abs(dy) < 1e-6 || Math.abs(Math.abs(dx) - Math.abs(dy)) < 1e-6) {
		return [a, b];
	}
	let mid;
	if (Math.abs(dx) > Math.abs(dy)) {
		mid = { x: b.x - Math.sign(dx) * Math.abs(dy), y: a.y }; // horizontal, then diagonal
	} else {
		mid = { x: a.x, y: b.y - Math.sign(dy) * Math.abs(dx) }; // vertical, then diagonal
	}
	return [a, mid, b];
}

function octilinearize(pts) {
	if (pts.length < 2) return pts.slice();
	const out = [pts[0]];
	for (let i = 1; i < pts.length; i++) {
		const leg = dogleg(out[out.length - 1], pts[i]);
		out.push(...leg.slice(1));
	}
	return out;
}

// --- fetch -----------------------------------------------------------------

const [linesRes, stationsRes] = await Promise.all([
	get(`/api/v1/map/lines?bbox=${BBOX}`),
	get('/api/v1/stations?limit=500')
]);
const stations = stationsRes.stations;

const routes = linesRes.lines.features.map((f) => ({
	id: f.properties.routeId,
	shortName: f.properties.shortName ?? '',
	longName: f.properties.longName ?? '',
	mode: f.properties.mode,
	color: (f.properties.color || '626A70').replace('#', ''),
	agency: f.properties.agencyName,
	rail: false, // filled below
	geom: f.geometry.coordinates // MultiLineString
}));
// The lines payload carries agencyName not agencyCode — rail agencies are
// recognized by name; fall back to subway/rail mode.
for (const r of routes) {
	r.rail =
		RAIL_AGENCIES.has(r.agency) ||
		/mrt|lrt|commuter|kereta|rail|airport/i.test(r.agency ?? '') ||
		r.mode === 'subway' ||
		r.mode === 'rail';
}

// Ordered stops per rail route — also seeds interchange detection.
const railRoutes = routes.filter((r) => r.rail);
const stopLists = new Map(); // routeId -> [{id, lat, lon, seq}]
await Promise.all(
	railRoutes.map(async (r) => {
		try {
			const d = await get(`/api/v1/routes/${r.id}`);
			stopLists.set(r.id, d.route.stops ?? []);
		} catch {
			stopLists.set(r.id, []);
		}
	})
);

// --- project geography -> schematic world ----------------------------------

const cosLat = Math.cos(((-6.25 * Math.PI) / 180) * 1); // ~Jakarta latitude
const allPts = [];
for (const r of routes) for (const line of r.geom) for (const c of line) allPts.push(c);
for (const s of stations) allPts.push([s.lon, s.lat]);
let minX = Infinity,
	maxX = -Infinity,
	minY = Infinity,
	maxY = -Infinity;
for (const [lon, lat] of allPts) {
	const x = lon * cosLat;
	const y = -lat;
	if (x < minX) minX = x;
	if (x > maxX) maxX = x;
	if (y < minY) minY = y;
	if (y > maxY) maxY = y;
}
const spanX = maxX - minX;
const spanY = maxY - minY;
const worldH = Math.round(((WORLD_W - PAD * 2) * spanY) / spanX + PAD * 2);
const K = (WORLD_W - PAD * 2) / spanX;
const proj = ([lon, lat]) => ({
	x: PAD + (lon * cosLat - minX) * K,
	y: PAD + (-lat - minY) * K
});

// --- per-route schematic paths ---------------------------------------------

// Lane offsets: rail routes sharing stations get parallel lanes. Lanes are
// assigned greedily over the "shares a station" graph so conflicting routes
// never occupy the same lane.
const stopRoutes = new Map(); // stationId -> Set<routeId>
for (const [rid, stops] of stopLists) {
	for (const s of stops) {
		if (!stopRoutes.has(s.id)) stopRoutes.set(s.id, new Set());
		stopRoutes.get(s.id).add(rid);
	}
}
const adj = new Map(railRoutes.map((r) => [r.id, new Set()]));
for (const set of stopRoutes.values()) {
	const ids = [...set];
	for (let i = 0; i < ids.length; i++)
		for (let j = i + 1; j < ids.length; j++) {
			adj.get(ids[i])?.add(ids[j]);
			adj.get(ids[j])?.add(ids[i]);
		}
}
const lane = new Map();
for (const r of [...railRoutes].sort((a, b) => a.id.localeCompare(b.id))) {
	const used = new Set([...(adj.get(r.id) ?? [])].map((o) => lane.get(o)).filter((x) => x != null));
	let l = 0;
	while (used.has(l)) l++;
	lane.set(r.id, l);
}
const laneCenter = (Math.max(...lane.values(), 0) / 2) * LANE_STEP;

// For every route build: simplified path (+ perpendicular lane shift for
// rail), and an unshifted reference path used to anchor station markers.
for (const r of routes) {
	const eps = r.rail ? SIMPLIFY_PX_RAIL : SIMPLIFY_PX_BUS;
	r.paths = r.geom.map((line) => {
		const base = dpSimplify(line.map(proj), eps);
		let pts = base;
		if (r.rail) pts = octilinearize(pts);
		const off = r.rail ? lane.get(r.id) * LANE_STEP - laneCenter : 0;
		const shifted =
			off !== 0
				? pts.map((p, i) => {
						// perpendicular to the local segment direction
						const q = pts[Math.min(i + 1, pts.length - 1)];
						const p0 = pts[Math.max(i - 1, 0)];
						let dx = q.x - p0.x;
						let dy = q.y - p0.y;
						const l = Math.hypot(dx, dy) || 1;
						return { x: p.x + (-dy / l) * off, y: p.y + (dx / l) * off };
					})
				: pts;
		return { draw: shifted, ref: pts };
	});
}

// --- station placement -----------------------------------------------------

const railPaths = railRoutes.flatMap((r) => r.paths.map((p) => p.ref));
const busPaths = routes.filter((r) => !r.rail).flatMap((r) => r.paths.map((p) => p.ref));
function anchor(s) {
	const p = proj([s.lon, s.lat]);
	const rail = s.operator && RAIL_AGENCIES.has(s.operator);
	const pool = rail ? railPaths : [...railPaths, ...busPaths];
	let best = null;
	let bestD = Infinity;
	for (const line of pool) {
		const q = nearestOnLine(p, line);
		if (!q) continue;
		const d = (p.x - q[0]) ** 2 + (p.y - q[1]) ** 2;
		if (d < bestD) {
			bestD = d;
			best = q;
		}
	}
	// Orphan stations (no serving route geometry in the catalog, e.g. the
	// Merak-line stops west of Rangkasbitung) must not be dragged onto an
	// unrelated corridor — past the snap threshold they keep their true
	// projected position and render as off-line markers.
	return bestD <= ANCHOR_MAX_PX * ANCHOR_MAX_PX ? best : [p.x, p.y];
}

for (const s of stations) {
	const [x, y] = anchor(s);
	s.sx = x;
	s.sy = y;
}

// Stations that anchored to the exact same point (interchange complexes like
// the airport, or bus bays at a rail station) get fanned onto a small ring so
// every one remains a separate tap target.
{
	const groups = new Map();
	for (const s of stations) {
		const k = `${Math.round(s.sx)},${Math.round(s.sy)}`;
		if (!groups.has(k)) groups.set(k, []);
		groups.get(k).push(s);
	}
	for (const g of groups.values()) {
		if (g.length < 2) continue;
		g.forEach((s, i) => {
			const a = (i / g.length) * Math.PI * 2;
			s.sx += Math.cos(a) * 18;
			s.sy += Math.sin(a) * 18;
		});
	}
}

// Interchange pills: rail stations that collapse to nearly the same schematic
// spot — OR sit within INTERCHANGE_M in reality — merge into one pill drawn
// under their dots. Schematic distance matters because co-anchored stations
// (e.g. the airport terminals) can be a kilometer apart in reality.
const mPerPx = (111320 * cosLat) / K;
const clusterPx = Math.max(INTERCHANGE_M / mPerPx, 70);
const railStations = stations.filter((s) => s.operator && RAIL_AGENCIES.has(s.operator));
const parent = new Map(railStations.map((s) => [s.id, s.id]));
const find = (a) => {
	while (parent.get(a) !== a) {
		parent.set(a, parent.get(parent.get(a)));
		a = parent.get(a);
	}
	return a;
};
for (let i = 0; i < railStations.length; i++) {
	for (let j = i + 1; j < railStations.length; j++) {
		const a = railStations[i];
		const b = railStations[j];
		const dm = Math.hypot(a.lon - b.lon, a.lat - b.lat) * 111320;
		const dpx = Math.hypot(a.sx - b.sx, a.sy - b.sy);
		if (dm <= INTERCHANGE_M || dpx <= clusterPx) {
			const ra = find(a.id);
			const rb = find(b.id);
			if (ra !== rb) parent.set(ra, rb);
		}
	}
}
const clusters = new Map();
for (const s of railStations) {
	const root = find(s.id);
	if (!clusters.has(root)) clusters.set(root, []);
	clusters.get(root).push(s);
}
const pills = [...clusters.values()].filter((c) => c.length > 1);

// --- emit ------------------------------------------------------------------

const fmt = (n) => Math.round(n * 10) / 10;
const pathD = (pts) => 'M' + pts.map((p) => `${fmt(p.x)} ${fmt(p.y)}`).join('L');

const svg = [];
svg.push(
	`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 ${WORLD_W} ${worldH}" font-family="system-ui, sans-serif">`
);
svg.push(`<rect width="${WORLD_W}" height="${worldH}" fill="#ffffff"/>`);

// bus corridors — thin, muted, underneath
svg.push('<g fill="none" stroke-linecap="round" stroke-linejoin="round">');
for (const r of routes.filter((x) => !x.rail)) {
	for (const p of r.paths) {
		svg.push(
			`<path d="${pathD(p.draw)}" stroke="#${r.color}" stroke-width="${BUS_W}" opacity="${BUS_OPACITY}"/>`
		);
	}
}
// rail lanes — thick, provider colors, on top
for (const r of railRoutes) {
	for (const p of r.paths) {
		svg.push(`<path d="${pathD(p.draw)}" stroke="#${r.color}" stroke-width="${RAIL_W}"/>`);
	}
}
svg.push('</g>');

// interchange pills
svg.push('<g>');
for (const c of pills) {
	const xs = c.map((s) => s.sx);
	const ys = c.map((s) => s.sy);
	const x0 = Math.min(...xs) - 26;
	const x1 = Math.max(...xs) + 26;
	const y0 = Math.min(...ys) - 26;
	const y1 = Math.max(...ys) + 26;
	svg.push(
		`<rect x="${fmt(x0)}" y="${fmt(y0)}" width="${fmt(x1 - x0)}" height="${fmt(
			y1 - y0
		)}" rx="${fmt((y1 - y0) / 2)}" fill="#ffffff" stroke="#141719" stroke-width="7"/>`
	);
}
svg.push('</g>');

// Station dots and labels are NOT baked into the artwork — the interactive
// overlay (IntegrationMap.svelte) draws them from points.json so they stay
// in one place and the selection halo/label behavior keeps working.

// title
svg.push(
	`<text x="${PAD}" y="90" font-size="44" font-weight="700" fill="#141719" stroke="none">Peta Integrasi — Singgah</text>`
);
svg.push('</svg>');

// points.json — every station is a hit target; rail gets labels, buses are
// unlabeled dots off the nearest drawn path.
const labelSide = (s) => (s.sx > WORLD_W * 0.82 ? 'left' : 'right');
const points = stations.map((s) => ({
	id: `pt-${s.id}`,
	stationId: s.id,
	label: s.operator && RAIL_AGENCIES.has(s.operator) ? s.name : undefined,
	labelSide: labelSide(s),
	ax: Math.round(s.sx),
	ay: Math.round(s.sy),
	bx: Math.round(s.sx),
	by: Math.round(s.sy),
	radius: s.operator && RAIL_AGENCIES.has(s.operator) ? 18 : 12
}));

const manifest = {
	version: new Date().toISOString().slice(0, 10),
	viewBox: [0, 0, WORLD_W, worldH],
	artwork: '/maps/integration/integration-map.svg',
	points: '/maps/integration/points.json'
};

const { writeFile } = await import('node:fs/promises');
await writeFile(new URL('integration-map.svg', OUT), svg.join('\n'));
await writeFile(new URL('points.json', OUT), JSON.stringify({ points }, null, 2));
await writeFile(new URL('manifest.json', OUT), JSON.stringify(manifest, null, 2));

console.log(`routes: ${routes.length} (${railRoutes.length} rail), stations: ${stations.length}`);
console.log(`world: ${WORLD_W}x${worldH}, pills: ${pills.length}`);
console.log(`wrote ${new URL('integration-map.svg', OUT).pathname}`);
