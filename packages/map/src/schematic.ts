// Schematic (integration) map math — pure functions, no DOM.
// World coordinates are authored x/y on the artwork canvas, never lat/lon
// (docs/INTEGRATION_MAP_BLUEPRINT.md §17–§21).

export interface SchematicPoint {
	id: string;
	/** Canonical Singgah station UUID — provider codes never appear here. */
	stationId: string;
	/** Schematic label; may differ from the official/sponsored station name. */
	label?: string;
	/** Which side of the marker carries the label; default 'right'. */
	labelSide?: 'top' | 'bottom' | 'left' | 'right';
	// Capsule from (ax,ay) to (bx,by) with `radius`; ax==bx && ay==by is a circle.
	ax: number;
	ay: number;
	bx: number;
	by: number;
	radius: number;
	cornerRadius?: number;
	noRing?: boolean;
}

export interface IntegrationCamera {
	tx: number;
	ty: number;
	scale: number;
}

/** One artwork tile placed in schematic world coordinates. */
export interface SchematicTile {
	x: number;
	y: number;
	w: number;
	h: number;
	/** Vector source — sharp at any zoom, used above the raster ceiling. */
	svg: string;
	/** Pre-rasterized sources keyed by tier ('0.5' | '1' | '2'). */
	rasters: Record<string, string>;
}

export type ArtworkTier = '0.5' | '1' | '2' | 'svg';

// Resolution a given camera scale asks for: pow2 ceiling of scale*dpr — the
// same rule as commute's pickTier — clamped to the raster tiers the manifest
// ships. Past 2x the SVG tile renders instead and stays crisp at any zoom.
export function pickArtworkTier(scale: number, dpr: number): ArtworkTier {
	const target = Math.max(scale * (dpr > 0 && Number.isFinite(dpr) ? dpr : 1), 0.5);
	const raw = 2 ** Math.ceil(Math.log2(target));
	return raw >= 4 ? 'svg' : (String(raw) as ArtworkTier);
}

// --- line isolation ---------------------------------------------------------

/** One drawn direction/branch of a schematic line — artwork-space geometry
 *  traced from the FDTJ map (commute `map-lines.json`). */
export interface SchematicLineSegment {
	kind: string;
	/** Edge quads [ax, ay, bx, by] along the drawn stroke. */
	edges: number[][];
	/** Provider station keys ("TJ-H00093P") drawn on this segment. */
	markers: string[];
}

/** A traced corridor line — `key` is the provider entity id "OP:CODE". */
export interface SchematicLine {
	key: string;
	operator: string;
	code: string;
	name: string;
	/** Provider/artwork color, CSS hex. */
	color: string;
	/** Stroke half-width in world units — also the capsule radius for cuts. */
	r: number;
	segments: SchematicLineSegment[];
}

/** Authored label capsule for one station marker (commute `label-points.json`);
 *  `station` is the provider key matching SchematicPoint.id. */
export interface SchematicLabelPoint {
	id: string;
	station: string;
	text: string;
	ax: number;
	ay: number;
	bx: number;
	by: number;
	r: number;
	cr?: number;
}

/** A capsule hole in the dim veil — cr === r degenerates to a line capsule;
 *  ax === bx && ay === by is a circle. */
export interface CutShape {
	ax: number;
	ay: number;
	bx: number;
	by: number;
	r: number;
	cr: number;
}

/** Lines whose traced geometry passes near a world point, nearest first.
 *  Ported from commute's map-line-isolate.ts (MIT, Shiori Labs) — keying by
 *  line collapses a corridor's two drawn directions into one answer. */
export function linesNear(
	lines: readonly SchematicLine[],
	x: number,
	y: number,
	slopWorld: number
): string[] {
	const hits: Array<{ key: string; dist: number }> = [];
	for (const line of lines) {
		let best = Infinity;
		for (const segment of line.segments) {
			for (const [ax, ay, bx, by] of segment.edges) {
				const d = pointToSegmentDistance(x, y, ax, ay, bx, by);
				if (d < best) best = d;
			}
		}
		if (best <= line.r + slopWorld) hits.push({ key: line.key, dist: best });
	}
	hits.sort((a, b) => a.dist - b.dist);
	return hits.map((h) => h.key);
}

/** Every shape held at full strength when a line is isolated: the traced
 *  stroke, each station marker, and each authored label capsule. Ported from
 *  commute's lineCutShapes — the names stay sharp for the same reason they do
 *  there: on a small marker the name carries most of the identity. */
export function lineCutShapes(
	line: SchematicLine,
	points: readonly SchematicPoint[],
	labels: readonly SchematicLabelPoint[]
): CutShape[] {
	const markerByStation = new Map<string, SchematicPoint>();
	for (const p of points) markerByStation.set(p.id, p);
	const labelByStation = new Map<string, SchematicLabelPoint>();
	for (const p of labels) if (!labelByStation.has(p.station)) labelByStation.set(p.station, p);

	const shapes: CutShape[] = [];
	const seen = new Set<string>();
	for (const segment of line.segments) {
		for (const [ax, ay, bx, by] of segment.edges) {
			shapes.push({ ax, ay, bx, by, r: line.r, cr: line.r });
		}
		for (const stationKey of segment.markers) {
			if (seen.has(stationKey)) continue;
			seen.add(stationKey);
			const marker = markerByStation.get(stationKey);
			if (marker) {
				shapes.push({
					ax: marker.ax,
					ay: marker.ay,
					bx: marker.bx,
					by: marker.by,
					r: marker.radius,
					cr: marker.cornerRadius ?? marker.radius
				});
			}
			const label = labelByStation.get(stationKey);
			if (label) {
				shapes.push({
					ax: label.ax,
					ay: label.ay,
					bx: label.bx,
					by: label.by,
					r: label.r,
					cr: label.cr ?? label.r
				});
			}
		}
	}
	return shapes;
}

/** Spotlight shapes for a selected station: its marker plus its authored
 *  label capsule when one exists. */
export function stationCutShapes(
	point: SchematicPoint,
	labels: readonly SchematicLabelPoint[]
): CutShape[] {
	const shapes: CutShape[] = [
		{
			ax: point.ax,
			ay: point.ay,
			bx: point.bx,
			by: point.by,
			r: point.radius,
			cr: point.cornerRadius ?? point.radius
		}
	];
	const label = labels.find((l) => l.station === point.id);
	if (label) {
		shapes.push({
			ax: label.ax,
			ay: label.ay,
			bx: label.bx,
			by: label.by,
			r: label.r,
			cr: label.cr ?? label.r
		});
	}
	return shapes;
}

function pointToSegmentDistance(
	px: number,
	py: number,
	ax: number,
	ay: number,
	bx: number,
	by: number
): number {
	const dx = bx - ax;
	const dy = by - ay;
	const lengthSquared = dx * dx + dy * dy;
	if (lengthSquared === 0) return Math.hypot(px - ax, py - ay);
	const t = Math.max(0, Math.min(1, ((px - ax) * dx + (py - ay) * dy) / lengthSquared));
	return Math.hypot(px - (ax + t * dx), py - (ay + t * dy));
}

export interface Viewport {
	width: number;
	height: number;
}

export interface WorldRect {
	width: number;
	height: number;
}

export function toScreen(camera: IntegrationCamera, wx: number, wy: number) {
	return { x: wx * camera.scale + camera.tx, y: wy * camera.scale + camera.ty };
}

export function toWorld(camera: IntegrationCamera, sx: number, sy: number) {
	return { x: (sx - camera.tx) / camera.scale, y: (sy - camera.ty) / camera.scale };
}

/** Capsule hit test in world coordinates (blueprint §18). */
export function hitTestPoint(
	x: number,
	y: number,
	point: SchematicPoint,
	extraRadius = 0
): boolean {
	const dx = point.bx - point.ax;
	const dy = point.by - point.ay;
	const lengthSquared = dx * dx + dy * dy || 1;
	const t = Math.max(0, Math.min(1, ((x - point.ax) * dx + (y - point.ay) * dy) / lengthSquared));
	const closestX = point.ax + dx * t;
	const closestY = point.ay + dy * t;
	const distanceSquared = (x - closestX) ** 2 + (y - closestY) ** 2;
	const radius = point.radius + extraRadius;
	return distanceSquared <= radius * radius;
}

/** Nearest capsule within radius + slop, so touch targets stay generous
 *  without fattening the visible marker (blueprint §19). `slop` is in world
 *  units — callers pass touchSlopPx / scale. */
export function hitTest(
	x: number,
	y: number,
	points: SchematicPoint[],
	slop = 0
): SchematicPoint | undefined {
	let best: SchematicPoint | undefined;
	let bestDist = Infinity;
	for (const p of points) {
		if (!hitTestPoint(x, y, p, slop)) continue;
		const cx = (p.ax + p.bx) / 2;
		const cy = (p.ay + p.by) / 2;
		const d = (x - cx) ** 2 + (y - cy) ** 2;
		if (d < bestDist) {
			bestDist = d;
			best = p;
		}
	}
	return best;
}

/** Fit the whole world into the viewport, centered, with padding. */
export function fitCamera(world: WorldRect, viewport: Viewport, padding = 24): IntegrationCamera {
	const availW = Math.max(viewport.width - padding * 2, 1);
	const availH = Math.max(viewport.height - padding * 2, 1);
	const scale = Math.min(availW / world.width, availH / world.height);
	return {
		scale,
		tx: (viewport.width - world.width * scale) / 2,
		ty: (viewport.height - world.height * scale) / 2
	};
}

/** Zoom around a screen-space anchor so the point under the cursor stays put. */
export function zoomAt(
	camera: IntegrationCamera,
	sx: number,
	sy: number,
	factor: number,
	world: WorldRect,
	viewport: Viewport
): IntegrationCamera {
	const min = Math.min(viewport.width / world.width, viewport.height / world.height) * 0.9;
	const scale = Math.min(Math.max(camera.scale * factor, min), 8);
	const applied = scale / camera.scale;
	return clampCamera(
		{ scale, tx: sx - (sx - camera.tx) * applied, ty: sy - (sy - camera.ty) * applied },
		world,
		viewport
	);
}

/** True when a world point is comfortably inside the viewport (margin in
 *  screen px) — used to skip camera moves that would be pointless. */
export function pointOnScreen(
	camera: IntegrationCamera,
	wx: number,
	wy: number,
	viewport: Viewport,
	margin = 40
): boolean {
	const s = toScreen(camera, wx, wy);
	return (
		s.x >= margin &&
		s.y >= margin &&
		s.x <= viewport.width - margin &&
		s.y <= viewport.height - margin
	);
}

/** Camera centered on a world point at the given scale, clamped so the map
 *  never shows only empty space. */
export function focusCamera(
	wx: number,
	wy: number,
	scale: number,
	world: WorldRect,
	viewport: Viewport
): IntegrationCamera {
	return clampCamera(
		{
			scale,
			tx: viewport.width / 2 - wx * scale,
			ty: viewport.height / 2 - wy * scale
		},
		world,
		viewport
	);
}

/** Keep at least `margin` px of world on screen; recenter when the world is
 *  smaller than the viewport (blueprint §21 — never pan to empty). */
export function clampCamera(
	camera: IntegrationCamera,
	world: WorldRect,
	viewport: Viewport,
	margin = 80
): IntegrationCamera {
	const clamp = (t: number, span: number, view: number) => {
		const lo = view - margin - span;
		const hi = margin;
		if (lo > hi) return (view - span) / 2;
		return Math.min(Math.max(t, lo), hi);
	};
	return {
		scale: camera.scale,
		tx: clamp(camera.tx, world.width * camera.scale, viewport.width),
		ty: clamp(camera.ty, world.height * camera.scale, viewport.height)
	};
}
