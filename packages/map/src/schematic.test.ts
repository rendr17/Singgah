import { describe, expect, it } from 'vitest';
import {
	clampCamera,
	fitCamera,
	focusCamera,
	hitTest,
	hitTestPoint,
	lineCutShapes,
	linesNear,
	pickArtworkTier,
	pointOnScreen,
	stationCutShapes,
	toScreen,
	toWorld,
	zoomAt,
	type SchematicLabelPoint,
	type SchematicLine,
	type SchematicPoint
} from './schematic';

const world = { width: 1600, height: 1260 };
const viewport = { width: 800, height: 600 };

const circle: SchematicPoint = {
	id: 'a',
	stationId: 'sa',
	ax: 100,
	ay: 100,
	bx: 100,
	by: 100,
	radius: 10
};
const capsule: SchematicPoint = {
	id: 'b',
	stationId: 'sb',
	ax: 200,
	ay: 200,
	bx: 300,
	by: 200,
	radius: 10
};

describe('coordinate transform', () => {
	it('world -> screen -> world round-trips', () => {
		const cam = { tx: 40, ty: -20, scale: 1.5 };
		const s = toScreen(cam, 123, 456);
		const w = toWorld(cam, s.x, s.y);
		expect(w.x).toBeCloseTo(123);
		expect(w.y).toBeCloseTo(456);
	});
});

describe('hitTestPoint', () => {
	it('hits inside a circle, misses outside', () => {
		expect(hitTestPoint(105, 105, circle)).toBe(true);
		expect(hitTestPoint(120, 100, circle)).toBe(false);
	});

	it('hits along a capsule spine and its round caps', () => {
		expect(hitTestPoint(250, 205, capsule)).toBe(true); // middle
		expect(hitTestPoint(305, 200, capsule)).toBe(true); // beyond bx inside cap
		expect(hitTestPoint(250, 225, capsule)).toBe(false); // off the side
	});

	it('extraRadius widens the hit area (touch slop)', () => {
		expect(hitTestPoint(125, 100, circle)).toBe(false);
		expect(hitTestPoint(125, 100, circle, 20)).toBe(true);
	});
});

describe('hitTest', () => {
	it('returns the nearest point when capsules overlap', () => {
		const near: SchematicPoint = { ...circle, id: 'n', stationId: 'sn', ax: 108, bx: 108 };
		expect(hitTest(105, 100, [circle, near])?.id).toBe('n');
	});

	it('returns undefined on a miss', () => {
		expect(hitTest(500, 500, [circle, capsule])).toBeUndefined();
	});
});

describe('fitCamera', () => {
	it('centers the world and respects padding', () => {
		const cam = fitCamera(world, viewport, 0);
		expect(cam.scale).toBeCloseTo(600 / 1260); // height-bound
		expect(cam.tx).toBeCloseTo((800 - 1600 * (600 / 1260)) / 2);
		expect(cam.ty).toBe(0);
	});
});

describe('zoomAt', () => {
	it('keeps the anchor point fixed on screen', () => {
		const cam = { tx: 100, ty: 50, scale: 1 };
		const before = toWorld(cam, 400, 300);
		const zoomed = zoomAt(cam, 400, 300, 2, world, viewport);
		const after = toScreen(zoomed, before.x, before.y);
		expect(after.x).toBeCloseTo(400);
		expect(after.y).toBeCloseTo(300);
		expect(zoomed.scale).toBeCloseTo(2);
	});

	it('does not zoom out below the fit bound', () => {
		const cam = { tx: 0, ty: 0, scale: 0.5 };
		const zoomed = zoomAt(cam, 400, 300, 0.01, world, viewport);
		expect(zoomed.scale).toBeGreaterThan(0.4);
	});
});

describe('focusCamera + pointOnScreen', () => {
	it('centers the world point in the viewport', () => {
		const cam = focusCamera(800, 600, 1.5, world, viewport);
		const s = toScreen(cam, 800, 600);
		expect(s.x).toBeCloseTo(400);
		expect(s.y).toBeCloseTo(300);
	});

	it('reports visibility with a screen margin', () => {
		const cam = { tx: 0, ty: 0, scale: 1 };
		expect(pointOnScreen(cam, 400, 300, viewport)).toBe(true);
		expect(pointOnScreen(cam, 10, 300, viewport)).toBe(false); // inside, within margin
		expect(pointOnScreen(cam, -50, 300, viewport)).toBe(false); // off-screen
	});
});

describe('clampCamera', () => {
	it('prevents panning the world fully off-screen', () => {
		const cam = clampCamera({ tx: -5000, ty: 0, scale: 1 }, world, viewport);
		expect(cam.tx).toBe(800 - 80 - 1600); // right edge keeps `margin` on screen
	});

	it('recenters a world smaller than the viewport', () => {
		const cam = clampCamera({ tx: 999, ty: 0, scale: 0.1 }, world, viewport);
		expect(cam.tx).toBe((800 - 160) / 2);
	});
});

const lineA: SchematicLine = {
	key: 'TJ:1',
	operator: 'TJ',
	code: '1',
	name: 'Blok M - Kota',
	color: '#D62126',
	r: 7.5,
	segments: [
		{
			kind: 'TRUNK',
			edges: [
				[100, 500, 300, 500],
				[300, 500, 300, 700]
			],
			markers: ['TJ-A', 'TJ-B', 'TJ-A']
		},
		{
			kind: 'BRANCH',
			edges: [[300, 700, 450, 700]],
			markers: ['TJ-C']
		}
	]
};

const isoPoints: SchematicPoint[] = [
	{ id: 'TJ-A', stationId: 's1', ax: 100, ay: 500, bx: 100, by: 500, radius: 20 },
	{ id: 'TJ-B', stationId: 's2', ax: 300, ay: 500, bx: 340, by: 500, radius: 12, cornerRadius: 5 },
	{ id: 'TJ-C', stationId: 's3', ax: 450, ay: 700, bx: 450, by: 700, radius: 15 },
	{ id: 'TJ-X', stationId: 's4', ax: 900, ay: 900, bx: 900, by: 900, radius: 10 }
];

const isoLabels: SchematicLabelPoint[] = [
	{ id: 'LBL-A', station: 'TJ-A', text: 'A', ax: 110, ay: 480, bx: 200, by: 480, r: 18, cr: 6 },
	{ id: 'LBL-B', station: 'TJ-B', text: 'B', ax: 310, ay: 470, bx: 380, by: 470, r: 18 }
];

describe('linesNear', () => {
	it('hits a corridor stroke within radius + slop, nearest first', () => {
		expect(linesNear([lineA], 200, 510, 10)).toEqual(['TJ:1']);
		expect(linesNear([lineA], 200, 520, 10)).toEqual([]); // 20 > r 7.5 + slop 10
	});

	it('returns multiple keys when corridors share a trunk', () => {
		const lineB = { ...lineA, key: 'TJ:2' };
		expect(linesNear([lineA, lineB], 200, 500, 0)).toEqual(['TJ:1', 'TJ:2']);
	});

	it('tolerates empty input', () => {
		expect(linesNear([], 200, 500, 10)).toEqual([]);
	});
});

describe('lineCutShapes', () => {
	it('emits stroke capsules + deduped markers + label capsules', () => {
		const shapes = lineCutShapes(lineA, isoPoints, isoLabels);
		// 3 stroke capsules (2 trunk edges + 1 branch edge)
		expect(shapes.filter((s) => s.r === lineA.r && s.cr === lineA.r)).toHaveLength(3);
		// TJ-A appears twice in markers — one hole only
		const markerShapes = shapes.filter((s) => s.ax === 100 && s.ay === 500 && s.r === 20);
		expect(markerShapes).toHaveLength(1);
		// label capsules carried through
		expect(shapes.some((s) => s.ax === 110 && s.ay === 480 && s.cr === 6)).toBe(true);
		// station not on the line gets no shape
		expect(shapes.every((s) => !(s.ax === 900 && s.ay === 900))).toBe(true);
	});

	it('skips markers with no authored point or label', () => {
		const orphan: SchematicLine = {
			...lineA,
			segments: [{ kind: 'TRUNK', edges: [[0, 0, 10, 0]], markers: ['TJ-ZZZ'] }]
		};
		expect(lineCutShapes(orphan, isoPoints, isoLabels)).toHaveLength(1);
	});
});

describe('stationCutShapes', () => {
	it('cuts the marker plus its authored label capsule', () => {
		const shapes = stationCutShapes(isoPoints[0], isoLabels);
		expect(shapes).toHaveLength(2);
		expect(shapes[0]).toMatchObject({ ax: 100, ay: 500, r: 20 });
		expect(shapes[1]).toMatchObject({ ax: 110, ay: 480, cr: 6 });
	});

	it('falls back to the marker alone when no label exists', () => {
		expect(stationCutShapes(isoPoints[2], isoLabels)).toHaveLength(1);
	});
});

describe('pickArtworkTier', () => {
	it('climbs pow2 raster tiers with scale*dpr, then falls back to SVG', () => {
		expect(pickArtworkTier(0.2, 1)).toBe('0.5');
		expect(pickArtworkTier(0.6, 1)).toBe('1');
		expect(pickArtworkTier(1.5, 1)).toBe('2');
		expect(pickArtworkTier(0.3, 2)).toBe('1');
		expect(pickArtworkTier(3, 1)).toBe('svg');
		expect(pickArtworkTier(1.5, 2)).toBe('svg');
	});

	it('clamps tiny/degenerate inputs to the coarsest tier', () => {
		expect(pickArtworkTier(0.01, 1)).toBe('0.5');
		expect(pickArtworkTier(0.8, 0)).toBe('1');
	});
});
