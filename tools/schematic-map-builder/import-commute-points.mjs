// Imports the commute repo's authored FDTJ hit-targets (provider-coded point
// IDs in the FDTJ artwork's own 9513x6727 world space) and rewrites them to
// Singgah canonical station UUIDs.
//
//   node tools/schematic-map-builder/import-commute-points.mjs [commute-repo-path]
//
// Mapping key: "<OPERATOR>-<station code>" — commute point "KCI-SUDB" maps to
// the catalog station where operator=="KCI" && code=="SUDB". HUB-* points are
// interchange-complex shapes with no station entity and are skipped.
// Marker dot/label coordinates come from the FDTJ tiles; points here carry
// hit geometry only (manifest.json "markers": "baked").

const API = process.env.SINGGAH_API ?? 'http://localhost:8080';
const COMMUTE = process.env.COMMUTE_ROOT ?? process.argv[2] ?? '../../commute-main/commute-main';
const OUT = new URL('../../apps/web/static/maps/integration/points.json', import.meta.url);

const { readFile, writeFile } = await import('node:fs/promises');
const { resolve } = await import('node:path');
const SRC = resolve(COMMUTE, 'apps/web/app/data/points.json');

const stationsRes = await fetch(`${API}/api/v1/stations?limit=500`);
if (!stationsRes.ok) throw new Error(`/stations -> ${stationsRes.status}`);
const stations = (await stationsRes.json()).stations;

const byProviderKey = new Map();
for (const s of stations) {
	if (s.operator && s.code) byProviderKey.set(`${s.operator}-${s.code}`, s.id);
}

const src = JSON.parse(await readFile(SRC, 'utf8'));
const out = [];
const skipped = [];
for (const p of src.points) {
	// "-b" ids are second marker positions for the same halt on the artwork;
	// they select the base station.
	const stationId = byProviderKey.get(p.id) ?? byProviderKey.get(p.id.replace(/-b$/, ''));
	if (!stationId) {
		skipped.push(p.id);
		continue;
	}
	out.push({
		id: p.id,
		stationId,
		ax: Math.round(p.ax * 10) / 10,
		ay: Math.round(p.ay * 10) / 10,
		bx: Math.round(p.bx * 10) / 10,
		by: Math.round(p.by * 10) / 10,
		radius: p.r
	});
}
out.sort((a, b) => a.id.localeCompare(b.id));

await writeFile(OUT, JSON.stringify({ version: src.version, points: out }, null, 2));
console.log(`mapped ${out.length}/${src.points.length} points -> ${OUT.pathname}`);
console.log(`skipped (no catalog match): ${skipped.join(', ') || 'none'}`);
