import type {
	SchematicLabelPoint,
	SchematicLine,
	SchematicPoint,
	SchematicTile
} from '@singgah/map';

// Integration-map assets are authored static files — loaded once per page,
// not viewport-scoped like the geographic station fetch. The manifest
// carries the tile grid in the artwork's own world space
// (docs/INTEGRATION_MAP_BLUEPRINT.md).
export interface IntegrationManifest {
	version: string;
	build?: string;
	viewBox: [number, number, number, number];
	assetDir: string;
	grid: { rows: number; cols: number };
	tileSize: { w: number; h: number };
	raster: { format: string; tiers: number[] };
	preview?: { url: string };
	points: string;
	/** Traced corridor geometry + authored label capsules (line isolation). */
	lines?: string;
	labels?: string;
	markers?: 'overlay' | 'baked';
	attribution?: string;
}

export async function loadIntegrationMap(): Promise<{
	manifest: IntegrationManifest;
	points: SchematicPoint[];
	lines: SchematicLine[];
	labels: SchematicLabelPoint[];
}> {
	const res = await fetch('/maps/integration/manifest.json');
	if (!res.ok) throw new Error('manifest unavailable');
	const manifest = (await res.json()) as IntegrationManifest;
	const pr = await fetch(manifest.points);
	if (!pr.ok) throw new Error('points unavailable');
	const { points } = (await pr.json()) as { points: SchematicPoint[] };
	// Lines/labels power isolation; a missing set degrades to a map that
	// still selects stations but doesn't dim for a tapped corridor.
	const lines = manifest.lines
		? (((await (await fetch(manifest.lines)).json()) as { lines: SchematicLine[] }).lines ?? [])
		: [];
	const labels = manifest.labels
		? (((await (await fetch(manifest.labels)).json()) as { points: SchematicLabelPoint[] })
				.points ?? [])
		: [];
	return { manifest, points, lines, labels };
}

// One tile per grid cell; each carries its SVG plus the pre-rasterized webp
// tiers so the renderer can swap artwork resolution with zoom.
export function buildIntegrationTiles(m: IntegrationManifest): SchematicTile[] {
	const v = m.build ? `?v=${m.build}` : '';
	const out: SchematicTile[] = [];
	for (let r = 0; r < m.grid.rows; r++) {
		for (let c = 0; c < m.grid.cols; c++) {
			const rasters: Record<string, string> = {};
			for (const t of m.raster.tiers) {
				rasters[String(t)] = `${m.assetDir}tile-${r}-${c}@${t}x.${m.raster.format}${v}`;
			}
			out.push({
				x: c * m.tileSize.w,
				y: r * m.tileSize.h,
				w: m.tileSize.w,
				h: m.tileSize.h,
				svg: `${m.assetDir}tile-${r}-${c}.svg${v}`,
				rasters
			});
		}
	}
	return out;
}
