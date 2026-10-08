<script lang="ts">
	import { onMount } from 'svelte';
	import { fade } from 'svelte/transition';
	import { SvelteMap } from 'svelte/reactivity';
	import {
		clampCamera,
		fitCamera,
		focusCamera,
		hitTest,
		introSequence,
		lineCutShapes,
		linesNear,
		markerLineStops,
		pickArtworkTier,
		pointOnScreen,
		stationCutShapes,
		toWorld,
		zoomAt,
		type CutShape,
		type IntegrationCamera,
		type IntroSequence,
		type SchematicLabelPoint,
		type SchematicLine,
		type SchematicPoint,
		type SchematicTile
	} from './schematic';

	interface Props {
		/** Artwork tiles placed in schematic world coordinates. */
		tiles: SchematicTile[];
		/** Whole-map backdrop image shown while tiles load. */
		previewUrl?: string;
		worldWidth: number;
		worldHeight: number;
		points: SchematicPoint[];
		/**
		 * 'overlay' draws per-point dots/labels; 'baked' means the artwork
		 * already contains them and only the hit targets + selection halo
		 * render on top.
		 */
		markers?: 'overlay' | 'baked';
		/** Traced corridor geometry for line isolation + corridor taps. */
		lines?: SchematicLine[];
		/** Authored label capsules — punched out of the dim veil with markers. */
		labels?: SchematicLabelPoint[];
		/** Key ("OP:CODE") of the line held sharp while the rest of the map
		 *  dims — commute's isolate treatment (map-line-isolate.ts). */
		lineKey?: string;
		/** Artwork provenance caption rendered in the corner. */
		attribution?: string;
		/** Canonical station id currently selected — shared with the geo map. */
		selectedId?: string;
		/** False while a mode switch hides the pane; becoming true replays the
		 *  network entrance sweep once (skipped under reduced motion). */
		visible?: boolean;
		onSelect?: (stationId: string) => void;
		/** Tap on a corridor stroke — nearest traced line key. */
		onLineSelect?: (lineKey: string) => void;
		/** Screen-space transform; scale <= 0 means "fit on first mount". */
		camera?: IntegrationCamera;
	}

	let {
		tiles,
		previewUrl,
		worldWidth,
		worldHeight,
		points,
		markers = 'overlay',
		lines = [],
		labels = [],
		lineKey,
		attribution,
		selectedId,
		visible = true,
		onSelect,
		onLineSelect,
		camera = $bindable()
	}: Props = $props();

	// Paint-server ids are document-global — prefix with the instance id so
	// two mounted maps never cross-reference each other's gradients.
	const uid = $props.id();
	let el: HTMLDivElement;
	let vw = $state(0);
	let vh = $state(0);
	let hovering = $state(false);
	let animating = $state(false);
	let reducedMotion = $state(false);

	const world = $derived({ width: worldWidth, height: worldHeight });

	// Artwork resolution follows zoom: pre-rasterized webp under the 2x
	// ceiling, vector SVG past it — mirrors commute's pickTier.
	let dpr = $state(1);
	const artTier = $derived(pickArtworkTier(camera?.scale ?? 1, dpr));
	let tileErrors = $state(0);
	let previewFailed = $state(false);

	// Touch slop in screen px, converted to world units at hit time so tap
	// targets stay usable at any zoom (blueprint §19).
	const TOUCH_SLOP_PX = 24;
	const TAP_TOLERANCE_PX = 6;
	// Station focus zoom — close enough to read labels, far enough for context.
	const FOCUS_SCALE = 1.2;

	// Dim treatment (commute's scrim + cutouts, implemented as an SVG mask —
	// the veil punches capsule holes so the artwork itself stays sharp through
	// them). An isolated line outranks a station spotlight.
	const cutShapes = $derived.by((): CutShape[] => {
		const line = lineKey ? lines.find((l) => l.key === lineKey) : undefined;
		if (line) return lineCutShapes(line, points, labels);
		if (!selectedId) return [];
		const p = points.find((pt) => pt.stationId === selectedId);
		return p ? stationCutShapes(p, labels) : [];
	});

	$effect(() => {
		if (vw > 0 && vh > 0 && (!camera || camera.scale <= 0)) {
			camera = fitCamera(world, { width: vw, height: vh }, 32);
		}
	});

	// Selection is shared across map modes. A selection made while this pane
	// is hidden (0-size viewport) is deferred until it becomes visible again;
	// a selection whose marker is already on screen never moves the camera.
	let pendingFocus = $state<string | undefined>();
	let prevSelected: string | undefined;

	$effect(() => {
		if (selectedId !== prevSelected) {
			prevSelected = selectedId;
			if (selectedId) pendingFocus = selectedId;
		}
	});

	$effect(() => {
		const target = pendingFocus;
		if (!target || vw <= 0 || vh <= 0 || !camera || camera.scale <= 0) return;
		pendingFocus = undefined;
		const p = points.find((pt) => pt.stationId === target);
		if (!p) return;
		const cx = (p.ax + p.bx) / 2;
		const cy = (p.ay + p.by) / 2;
		if (pointOnScreen(camera, cx, cy, view())) return;
		animating = true;
		camera = focusCamera(cx, cy, Math.max(camera.scale, FOCUS_SCALE), world, view());
		setTimeout(() => (animating = false), 260);
	});

	// --- entrance sweep -----------------------------------------------------
	// Each traced line draws itself inward and both halves meet at the middle
	// of the map — introSequence() gives every polyline's two pieces an end
	// at the anchor, so dashoffset animation converges there ("bertemu di
	// tengah"). The artwork hides during the draw and fades back over the
	// finished strokes, which sit exactly on the corridor bands.
	const INTRO_STROKE_MS = 560;
	const INTRO_SPAN_MS = 340;
	const INTRO_REVEAL_MS = 380;
	let introPhase = $state<'idle' | 'draw' | 'reveal'>('idle');
	let introData = $state<IntroSequence | undefined>();
	// Bump restarts the CSS animations by remounting the overlay block.
	let introRun = $state(0);
	let introTimers: ReturnType<typeof setTimeout>[] = [];
	let introArmed = true;

	$effect(() => {
		if (!visible) {
			introArmed = true;
			return;
		}
		// Deferred until the pane is actually laid out — a mode-hidden pane
		// still runs effects but reports a 0-size viewport.
		if (!introArmed || lines.length === 0 || vw <= 0) return;
		introArmed = false;
		if (reducedMotion) return;
		const seq = introSequence(lines, points, world, {
			strokeMs: INTRO_STROKE_MS,
			spanMs: INTRO_SPAN_MS
		});
		if (seq.strokes.length === 0) return;
		introData = seq;
		introRun++;
		introPhase = 'draw';
		// A replay while the previous sweep is still scheduled must not let its
		// stale timers land mid-draw.
		for (const t of introTimers) clearTimeout(t);
		introTimers = [];
		const settle = INTRO_SPAN_MS + INTRO_STROKE_MS + 120;
		introTimers.push(setTimeout(() => (introPhase = 'reveal'), settle));
		introTimers.push(
			setTimeout(() => {
				introPhase = 'idle';
				introData = undefined;
			}, settle + INTRO_REVEAL_MS)
		);
	});

	// --- gestures -----------------------------------------------------------

	const active = new SvelteMap<number, { x: number; y: number }>();
	let drag: { x: number; y: number; tx: number; ty: number; moved: boolean } | null = null;
	let pinch: { d: number; cam: IntegrationCamera; x: number; y: number } | null = null;
	let pinched = false;

	function localXY(e: PointerEvent | MouseEvent | WheelEvent) {
		const r = el.getBoundingClientRect();
		return { x: e.clientX - r.left, y: e.clientY - r.top };
	}
	function dist(a: { x: number; y: number }, b: { x: number; y: number }) {
		return Math.hypot(a.x - b.x, a.y - b.y);
	}
	function view() {
		return { width: vw, height: vh };
	}

	function onPointerDown(e: PointerEvent) {
		// Overlay controls (reset) must not start a drag — capturing the
		// pointer here would retarget their click to the container.
		if ((e.target as HTMLElement).closest('button, a, input, select, label')) return;
		if (e.pointerType === 'mouse' && e.button !== 0) return;
		animating = false; // gestures must not be smoothed by the focus transition
		el.setPointerCapture(e.pointerId);
		const p = localXY(e);
		active.set(e.pointerId, p);
		if (active.size === 2 && camera) {
			const [a, b] = [...active.values()];
			pinch = {
				d: Math.max(dist(a, b), 1),
				cam: { ...camera },
				x: (a.x + b.x) / 2,
				y: (a.y + b.y) / 2
			};
			pinched = true;
			drag = null;
		} else if (active.size === 1 && camera) {
			drag = { x: p.x, y: p.y, tx: camera.tx, ty: camera.ty, moved: false };
		}
	}

	function onPointerMove(e: PointerEvent) {
		if (!active.has(e.pointerId)) {
			// hover affordance only when not dragging
			if (camera && camera.scale > 0) {
				const p = localXY(e);
				const w = toWorld(camera, p.x, p.y);
				const slop = TOUCH_SLOP_PX / camera.scale;
				hovering = !!hitTest(w.x, w.y, points, slop) || linesNear(lines, w.x, w.y, slop).length > 0;
			}
			return;
		}
		const p = localXY(e);
		active.set(e.pointerId, p);
		if (!camera) return;
		if (active.size >= 2 && pinch) {
			const [a, b] = [...active.values()];
			camera = zoomAt(pinch.cam, pinch.x, pinch.y, dist(a, b) / pinch.d, world, view());
			return;
		}
		if (drag) {
			const dx = p.x - drag.x;
			const dy = p.y - drag.y;
			if (!drag.moved && Math.hypot(dx, dy) > TAP_TOLERANCE_PX) drag.moved = true;
			camera = clampCamera({ ...camera, tx: drag.tx + dx, ty: drag.ty + dy }, world, view());
		}
	}

	function onPointerUp(e: PointerEvent) {
		active.delete(e.pointerId);
		if (pinch && active.size < 2) {
			pinch = null;
		}
		if (active.size === 1) {
			// one finger remains after a pinch — rebase the drag, no jump
			const [p] = [...active.values()];
			if (camera) drag = { x: p.x, y: p.y, tx: camera.tx, ty: camera.ty, moved: true };
		}
		if (active.size === 0) {
			if (drag && !drag.moved && !pinched && camera && camera.scale > 0) {
				const p = localXY(e);
				const w = toWorld(camera, p.x, p.y);
				const slop = TOUCH_SLOP_PX / camera.scale;
				const hit = hitTest(w.x, w.y, points, slop);
				if (hit) onSelect?.(hit.stationId);
				else {
					// A tap on a corridor stroke resolves to the nearest traced
					// line — markers win first, so stations stay selectable.
					const keys = linesNear(lines, w.x, w.y, slop);
					if (keys[0]) onLineSelect?.(keys[0]);
				}
			}
			drag = null;
			pinch = null;
			pinched = false;
		}
	}

	// A cancelled pointer (browser took over, touch became a scroll) is never
	// a tap — drop gesture state without hit-testing.
	function onPointerCancel(e: PointerEvent) {
		active.delete(e.pointerId);
		if (active.size < 2) pinch = null;
		if (active.size === 0) {
			drag = null;
			pinched = false;
		}
	}

	function onWheel(e: WheelEvent) {
		e.preventDefault();
		animating = false;
		if (!camera || camera.scale <= 0) return;
		const p = localXY(e);
		camera = zoomAt(camera, p.x, p.y, Math.exp(-e.deltaY * 0.0018), world, view());
	}

	function onDblClick(e: MouseEvent) {
		e.preventDefault();
		if (!camera || camera.scale <= 0) return;
		const p = localXY(e);
		camera = zoomAt(camera, p.x, p.y, 1.6, world, view());
	}

	function reset() {
		animating = true;
		camera = fitCamera(world, view(), 32);
		setTimeout(() => (animating = false), 260);
	}

	onMount(() => {
		dpr = window.devicePixelRatio || 1;
		const motionPreference = window.matchMedia('(prefers-reduced-motion: reduce)');
		reducedMotion = motionPreference.matches;
		const updateMotionPreference = () => (reducedMotion = motionPreference.matches);
		motionPreference.addEventListener('change', updateMotionPreference);
		el.addEventListener('wheel', onWheel, { passive: false });
		return () => {
			motionPreference.removeEventListener('change', updateMotionPreference);
			el.removeEventListener('wheel', onWheel);
			for (const t of introTimers) clearTimeout(t);
		};
	});

	function labelPos(p: SchematicPoint) {
		const cx = (p.ax + p.bx) / 2;
		const cy = (p.ay + p.by) / 2;
		switch (p.labelSide) {
			case 'left':
				return { x: cx - p.radius - 16, y: cy, anchor: 'end', baseline: 'central' };
			case 'top':
				return { x: cx, y: cy - p.radius - 14, anchor: 'middle', baseline: 'auto' };
			case 'bottom':
				return { x: cx, y: cy + p.radius + 14, anchor: 'middle', baseline: 'hanging' };
			default:
				return { x: cx + p.radius + 16, y: cy, anchor: 'start', baseline: 'central' };
		}
	}
</script>

<div
	class="sg-im"
	bind:this={el}
	bind:clientWidth={vw}
	bind:clientHeight={vh}
	class:sg-im--hover={hovering}
	class:sg-im--anim={animating}
	role="region"
	aria-label="Peta integrasi jaringan"
	onpointerdown={onPointerDown}
	onpointermove={onPointerMove}
	onpointerup={onPointerUp}
	onpointercancel={onPointerCancel}
	ondblclick={onDblClick}
>
	{#if tiles.length > 0 && tileErrors >= tiles.length}
		<p class="sg-im__state" role="alert">Peta integrasi tidak dapat dimuat.</p>
	{:else if camera && camera.scale > 0}
		<div
			class="sg-im__world"
			style:width="{worldWidth}px"
			style:height="{worldHeight}px"
			style:transform="translate({camera.tx}px, {camera.ty}px) scale({camera.scale})"
		>
			<div class="sg-im__artwork" class:sg-im__artwork--off={introPhase === 'draw'}>
				{#if previewUrl && !previewFailed}
					<img
						class="sg-im__art"
						src={previewUrl}
						width={worldWidth}
						height={worldHeight}
						alt=""
						draggable="false"
						onerror={() => (previewFailed = true)}
					/>
				{/if}
				{#each tiles as t (t.svg)}
					<img
						class="sg-im__tile"
						src={artTier === 'svg' ? t.svg : (t.rasters[artTier] ?? t.svg)}
						style:left="{t.x}px"
						style:top="{t.y}px"
						width={t.w}
						height={t.h}
						alt=""
						draggable="false"
						onerror={() => (tileErrors += 1)}
					/>
				{/each}
			</div>
			<svg
				class="sg-im__overlay"
				viewBox="0 0 {worldWidth} {worldHeight}"
				width={worldWidth}
				height={worldHeight}
				aria-hidden="true"
			>
				{#if cutShapes.length > 0}
					<!-- Veil with capsule cutouts: the artwork stays sharp through
					     the holes — same treatment as commute's canvas scrim. The
					     shapes blur inside the mask so the dim edge ramps back in
					     over ~26wu (commute's SPOTLIGHT_FEATHER_WORLD). -->
					<defs>
						<filter
							id="sg-im-feather"
							filterUnits="userSpaceOnUse"
							x="0"
							y="0"
							width={worldWidth}
							height={worldHeight}
						>
							<feGaussianBlur stdDeviation="13" />
						</filter>
						<mask
							id="sg-im-veil-mask"
							maskUnits="userSpaceOnUse"
							x="0"
							y="0"
							width={worldWidth}
							height={worldHeight}
						>
							<rect width={worldWidth} height={worldHeight} fill="#fff" />
							<g filter="url(#sg-im-feather)">
								{#each cutShapes as s, i (i)}
									{#if s.ax === s.bx && s.ay === s.by}
										<circle cx={s.ax} cy={s.ay} r={s.r} fill="#000" />
									{:else if (s.ax === s.bx || s.ay === s.by) && s.cr < s.r}
										<!-- axis-aligned capsule with small corner radius = the
										     authored rounded-rect shape (pills, label boxes) -->
										<rect
											x={Math.min(s.ax, s.bx) - s.r}
											y={Math.min(s.ay, s.by) - s.r}
											width={Math.abs(s.bx - s.ax) + s.r * 2}
											height={Math.abs(s.by - s.ay) + s.r * 2}
											rx={s.cr}
											fill="#000"
										/>
									{:else}
										<line
											x1={s.ax}
											y1={s.ay}
											x2={s.bx}
											y2={s.by}
											stroke="#000"
											stroke-width={s.r * 2}
											stroke-linecap="round"
										/>
									{/if}
								{/each}
							</g>
						</mask>
					</defs>
					<rect
						class="sg-im__veil"
						width={worldWidth}
						height={worldHeight}
						mask="url(#sg-im-veil-mask)"
						transition:fade={{ duration: reducedMotion ? 0 : 220 }}
					/>
				{/if}
				{#each points as p (p.id)}
					{@const sel = p.stationId === selectedId}
					{@const lp = labelPos(p)}
					<g class="sg-im__pt" class:sg-im__pt--sel={sel}>
						{#if sel && !p.noRing}
							<!-- The ring hugs the whole marker capsule — every roundel
							     of an interchange sits inside it — and runs through
							     the serving lines' colours, ordered by where
							     their strokes cross the spine. -->
							{@const stops = markerLineStops(p, lines)}
							{@const hcx = (p.ax + p.bx) / 2}
							{@const hcy = (p.ay + p.by) / 2}
							{@const hdx = p.bx - p.ax}
							{@const hdy = p.by - p.ay}
							{@const hlen = Math.hypot(hdx, hdy)}
							{@const pad = p.radius + 10}
							{@const gid = `sg-im-hg-${uid}-${p.id}`}
							{#if stops.length > 1}
								<linearGradient id={gid} x1="0" y1="0.5" x2="1" y2="0.5">
									{#each stops as s (s.line.key)}
										<stop
											offset={hlen > 1 ? (s.t * hlen + pad) / (hlen + pad * 2) : s.t}
											stop-color={s.line.color}
										/>
									{/each}
								</linearGradient>
							{/if}
							<g
								transform="translate({hcx} {hcy}) rotate({(Math.atan2(hdy, hdx) * 180) / Math.PI})"
								transition:fade={{ duration: reducedMotion ? 0 : 180 }}
							>
								<rect
									x={-hlen / 2 - pad}
									y={-pad}
									width={hlen + pad * 2}
									height={pad * 2}
									rx={Math.min((p.cornerRadius ?? p.radius) + 10, pad)}
									class="sg-im__halo"
									style:stroke={stops.length > 1
										? `url(#${gid})`
										: stops.length === 1
											? stops[0].line.color
											: undefined}
								/>
							</g>
						{/if}
						{#if markers !== 'baked'}
							{#if p.ax === p.bx && p.ay === p.by}
								<circle cx={p.ax} cy={p.ay} r={p.radius} class="sg-im__dot" />
							{:else}
								<line
									x1={p.ax}
									y1={p.ay}
									x2={p.bx}
									y2={p.by}
									class="sg-im__dot sg-im__dot--capsule"
									style:stroke-width={p.radius * 2}
									stroke-linecap="round"
								/>
							{/if}
							{#if p.label}
								<text
									x={lp.x}
									y={lp.y}
									text-anchor={lp.anchor}
									dominant-baseline={lp.baseline}
									class="sg-im__label">{p.label}</text
								>
							{/if}
						{/if}
					</g>
				{/each}
			</svg>
			{#if introPhase !== 'idle' && introData}
				{#key introRun}
					<svg
						class="sg-im__intro"
						viewBox="0 0 {worldWidth} {worldHeight}"
						width={worldWidth}
						height={worldHeight}
						aria-hidden="true"
					>
						{#each introData.strokes as s, i (i)}
							<path
								class="sg-im__ipath"
								d={s.d}
								pathLength="1"
								stroke={s.color}
								stroke-width={s.width}
								style:animation-delay="{s.delayMs}ms"
							/>
						{/each}
						{#each introData.markers as m, i (i)}
							<g class="sg-im__imark" style:animation-delay="{m.delayMs}ms">
								<circle cx={m.x} cy={m.y} r={m.r} fill="#fff" />
								<circle cx={m.x} cy={m.y} r={m.r * 0.72} fill={m.color} />
							</g>
						{/each}
					</svg>
				{/key}
			{/if}
		</div>
	{:else}
		<p class="sg-im__state" role="status">Memuat peta integrasi…</p>
	{/if}

	<button type="button" class="sg-im__reset" aria-label="Reset tampilan peta" onclick={reset}>
		<svg
			viewBox="0 0 24 24"
			width="18"
			height="18"
			fill="none"
			stroke="currentColor"
			stroke-width="2"
			stroke-linecap="round"
			aria-hidden="true"><path d="M3 12a9 9 0 1 0 3-6.7" /><path d="M3 4v5h5" /></svg
		>
	</button>
	{#if attribution}
		<p class="sg-im__attr">{attribution}</p>
	{/if}
</div>

<style>
	.sg-im {
		position: relative;
		width: 100%;
		height: 100%;
		overflow: hidden;
		background-color: var(--sg-surface-muted, #eef1f2);
		touch-action: none;
		user-select: none;
		cursor: grab;
	}
	.sg-im--hover {
		cursor: pointer;
	}
	.sg-im__world {
		position: absolute;
		top: 0;
		left: 0;
		transform-origin: 0 0;
	}
	/* Only programmatic focus/reset moves animate — gestures set
	   .sg-im--anim off before writing the camera. */
	.sg-im--anim .sg-im__world {
		transition: transform 0.22s ease-out;
	}
	@media (prefers-reduced-motion: reduce) {
		.sg-im--anim .sg-im__world {
			transition: none;
		}
	}
	.sg-im__art,
	.sg-im__tile,
	.sg-im__overlay,
	.sg-im__intro {
		position: absolute;
		top: 0;
		left: 0;
		display: block;
	}
	.sg-im__artwork {
		position: absolute;
		inset: 0;
		transition: opacity 0.38s ease-out;
	}
	.sg-im__artwork--off {
		opacity: 0;
		transition-duration: 0.1s;
	}
	.sg-im__intro {
		pointer-events: none;
	}
	.sg-im__ipath {
		fill: none;
		stroke-linecap: round;
		stroke-linejoin: round;
		stroke-dasharray: 1;
		stroke-dashoffset: 1;
		animation: sg-im-ipath 0.56s cubic-bezier(0.22, 0.61, 0.36, 1) both;
	}
	@keyframes sg-im-ipath {
		to {
			stroke-dashoffset: 0;
		}
	}
	.sg-im__imark {
		transform-box: fill-box;
		transform-origin: center;
		opacity: 0;
		animation: sg-im-imark 0.2s ease-out both;
	}
	@keyframes sg-im-imark {
		from {
			opacity: 0;
			transform: scale(0.3);
		}
		to {
			opacity: 1;
			transform: scale(1);
		}
	}
	@media (prefers-reduced-motion: reduce) {
		.sg-im__ipath {
			animation: none;
			stroke-dashoffset: 0;
		}
		.sg-im__imark {
			animation: none;
			opacity: 1;
			transform: none;
		}
		.sg-im__artwork {
			transition-duration: 0.1s;
		}
	}
	.sg-im__attr {
		position: absolute;
		bottom: var(--sg-space-2, 0.5rem);
		inset-inline-end: var(--sg-space-3, 0.75rem);
		margin: 0;
		padding: 0.15rem 0.4rem;
		border-radius: var(--sg-radius-button, 0.5rem);
		background-color: color-mix(in srgb, var(--sg-surface, #ffffff) 75%, transparent);
		color: var(--sg-text-muted, #626a70);
		font-size: 0.625rem;
		pointer-events: none;
		z-index: 1;
	}
	.sg-im__overlay {
		overflow: visible;
	}
	.sg-im__veil {
		fill: var(--sg-surface, #ffffff);
		opacity: 0.65;
	}
	.sg-im__state {
		position: absolute;
		inset: 0;
		display: grid;
		place-items: center;
		margin: 0;
		color: var(--sg-text-muted, #626a70);
		font-size: 0.875rem;
	}
	.sg-im__dot {
		fill: #ffffff;
		stroke: var(--sg-text, #141719);
		stroke-width: 5;
	}
	.sg-im__dot--capsule {
		stroke: var(--sg-text, #141719);
	}
	.sg-im__pt--sel .sg-im__dot {
		stroke: var(--sg-brand, #0f6b4f);
	}
	.sg-im__halo {
		fill: none;
		stroke: var(--sg-brand, #0f6b4f);
		stroke-width: 6;
	}
	.sg-im__label {
		font-family: var(--sg-font-body, sans-serif);
		font-size: 30px;
		fill: var(--sg-text, #141719);
		paint-order: stroke;
		stroke: var(--sg-surface, #ffffff);
		stroke-width: 7;
	}
	.sg-im__pt--sel .sg-im__label {
		font-weight: 700;
	}
	.sg-im__reset {
		position: absolute;
		bottom: var(--sg-space-3, 0.75rem);
		inset-inline-start: var(--sg-space-3, 0.75rem);
		display: grid;
		place-items: center;
		width: var(--sg-target-min, 2.75rem);
		height: var(--sg-target-min, 2.75rem);
		border: 1px solid var(--sg-border, #d7dee2);
		border-radius: var(--sg-radius-button, 0.5rem);
		background-color: var(--sg-surface, #ffffff);
		color: var(--sg-text, #141719);
		box-shadow: var(--sg-shadow-overlay, 0 2px 8px rgba(20, 23, 25, 0.12));
		cursor: pointer;
		z-index: 1;
	}
</style>
