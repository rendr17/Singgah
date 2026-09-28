<script lang="ts">
	import { dev } from '$app/environment';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { onMount } from 'svelte';
	import { SvelteMap } from 'svelte/reactivity';
	import UiPreview from '$lib/dev/UiPreview.svelte';
	import MapModeSwitcher from '$lib/components/map/MapModeSwitcher.svelte';
	import MapSearch from '$lib/components/map/MapSearch.svelte';
	import ModeIcon from '$lib/components/map/ModeIcon.svelte';
	import StationCombobox from '$lib/components/station/StationCombobox.svelte';
	import DepartureBoard from '$lib/components/station/DepartureBoard.svelte';
	import JourneyTimeline from '$lib/components/journey/JourneyTimeline.svelte';
	import { basemapStyleUrl } from '$lib/basemap';
	import { facilityLabel } from '$lib/facilities';
	import {
		buildIntegrationTiles,
		loadIntegrationMap,
		type IntegrationManifest
	} from '$lib/integration-map';
	import { loadCachedPlan, saveCachedPlan } from '$lib/plan-cache';
	import { api } from '$lib/api';
	import { mapStore } from '$lib/stores/map.svelte';
	import { unwrap } from '@singgah/api-client';
	import type { components } from '@singgah/api-client';
	import type {
		Feature,
		FeatureCollection,
		SchematicLabelPoint,
		SchematicLine,
		SchematicPoint
	} from '@singgah/map';
	import { Button, IconButton, SearchField, StateBlock, StatusBadge } from '@singgah/ui';

	type Station = components['schemas']['StationSummary'];
	type StationDetail = components['schemas']['StationDetail'];
	type Board = components['schemas']['StationDepartures'];
	type Plan = components['schemas']['JourneyPlan'];
	type RouteLines = components['schemas']['RouteLineCollection'];
	type RouteDetail = components['schemas']['RouteDetail'];
	type RouteSummary = components['schemas']['RouteSummary'];
	type LegAlternative = components['schemas']['JourneyLegAlternative'];

	const STYLE_URL = basemapStyleUrl();

	let query = $state('');
	let stations = $state<Station[]>([]);
	let loading = $state(false);
	let error = $state('');

	let debounce: ReturnType<typeof setTimeout>;
	$effect(() => {
		const q = query.trim();
		clearTimeout(debounce);
		if (q === '') {
			stations = [];
			error = '';
			loading = false;
			return;
		}
		loading = true;
		debounce = setTimeout(async () => {
			try {
				const data = await unwrap(api.GET('/api/v1/stations', { params: { query: { query: q } } }));
				stations = data.stations;
				error = '';
			} catch (e) {
				error = e instanceof Error ? e.message : 'Pencarian gagal.';
				stations = [];
			} finally {
				loading = false;
			}
		}, 300);
		return () => clearTimeout(debounce);
	});

	// --- Wide workspace (>=75rem): persistent map per docs/10. The module is
	// dynamic-imported only when the media query matches, so compact sessions
	// never download the map stack — Home stays light (docs/08). ---
	let wide = $state(false);
	let mapMod = $state<typeof import('@singgah/map') | null>(null);
	onMount(() => {
		const mq = matchMedia('(min-width: 75rem)');
		const sync = () => {
			wide = mq.matches;
			if (wide && !mapMod) void import('@singgah/map').then((m) => (mapMod = m));
			if (wide) loadIntegration();
		};
		sync();
		mq.addEventListener('change', sync);
		return () => mq.removeEventListener('change', sync);
	});

	// Integration-map assets are authored static files — fetched once when
	// the wide workspace mounts, same as /map. Both panes stay mounted across
	// mode switches so each map keeps its camera and loaded tiles.
	let integrationManifest = $state<IntegrationManifest | undefined>();
	let integrationPoints = $state<SchematicPoint[]>([]);
	let integrationLines = $state<SchematicLine[]>([]);
	let integrationLabels = $state<SchematicLabelPoint[]>([]);
	let integrationFailed = $state(false);
	let integrationRequested = false;
	const integrationTiles = $derived(
		integrationManifest ? buildIntegrationTiles(integrationManifest) : []
	);

	// Traced corridor keys ("KCI:C") join the routes catalog on
	// agencyCode:shortName — a tapped stroke resolves to the canonical route
	// id, and a drawn corridor isolates itself on the artwork (veil).
	const routesByKey = new SvelteMap<string, RouteSummary>();
	const keyByRouteId = new SvelteMap<string, string>();

	function loadIntegration() {
		if (integrationRequested) return;
		integrationRequested = true;
		void (async () => {
			try {
				const { manifest, points, lines, labels } = await loadIntegrationMap();
				integrationPoints = points;
				integrationLines = lines;
				integrationLabels = labels;
				integrationManifest = manifest;
			} catch {
				integrationFailed = true;
			}
		})();
		// Catalog join degrades independently — taps still open stations.
		void (async () => {
			try {
				const data = await unwrap(api.GET('/api/v1/routes', { params: { query: { limit: 500 } } }));
				for (const r of data.routes) {
					if (!r.agencyCode || !r.shortName) continue;
					const key = `${r.agencyCode}:${r.shortName}`;
					routesByKey.set(key, r);
					keyByRouteId.set(r.id, key);
				}
			} catch {
				/* corridor taps degrade — stations still select */
			}
		})();
	}

	let fromQuery = $state('');
	let fromStation = $state<Station | null>(null);
	let toQuery = $state('');
	let toStation = $state<Station | null>(null);

	let plan = $state<Plan | null>(null);
	let planning = $state(false);
	let planError = $state('');
	let planFromCache = $state(false);
	let planRoute = $state<FeatureCollection | undefined>(undefined);
	let corridorRoute = $state<FeatureCollection | undefined>(undefined);
	// A drawn corridor wins over the journey overlay — the two never stack.
	const route = $derived(corridorRoute ?? planRoute);
	const routeActive = $derived((route?.features.length ?? 0) > 0);

	// The workspace draws the itinerary on the map instead of navigating to
	// /plan — JourneyStopRef carries only id+name, so stop coordinates are
	// fetched per unique id, once per submitted plan.
	async function goPlan() {
		if (!fromStation || !toStation) return;
		planning = true;
		planError = '';
		plan = null;
		planFromCache = false;
		planRoute = undefined;
		closeCorridor();
		legAlts = {};
		try {
			plan = await unwrap(
				api.GET('/api/v1/journeys', {
					params: { query: { from: fromStation.id, to: toStation.id } }
				})
			);
			saveCachedPlan(localStorage, `from=${fromStation.id}&to=${toStation.id}`, plan);
			planRoute = await routeGeoJSON(plan);
		} catch (e) {
			const cached = loadCachedPlan(localStorage, `from=${fromStation.id}&to=${toStation.id}`);
			if (cached) {
				plan = cached;
				planFromCache = true;
				planRoute = await routeGeoJSON(cached);
			} else {
				planError = e instanceof Error ? e.message : 'Pencarian rute gagal.';
			}
		} finally {
			planning = false;
		}
	}

	// Corridor choice per leg index — a picked alternative swaps that leg's
	// stops/geometry onto the map with the corridor's own catalog data.
	let legAlts = $state<Record<number, LegAlternative>>({});
	async function onLegAlternative(i: number, alt: LegAlternative | null) {
		if (alt) legAlts[i] = alt;
		else delete legAlts[i];
		if (plan) planRoute = await routeGeoJSON(plan);
	}

	async function routeGeoJSON(p: Plan): Promise<FeatureCollection | undefined> {
		const itin = p.itineraries[0];
		if (!itin) return undefined;
		// Effective legs: a chosen corridor alternative supplies its own line,
		// stop slice and shape — endpoints and walk legs stay the plan's.
		const legs = itin.legs.map((l, i) => {
			const alt = legAlts[i];
			return alt
				? {
						...l,
						routeId: alt.routeId,
						line: alt.line,
						stops: alt.stops,
						stationCount: alt.stationCount,
						geometry: alt.geometry
					}
				: l;
		});
		const ids: string[] = [];
		for (const leg of legs) {
			for (const ref of [leg.from, leg.to, ...(leg.stops ?? [])]) {
				if (ref.id && !ids.includes(ref.id)) ids.push(ref.id);
			}
		}
		const coords: Record<string, [number, number]> = {};
		const icons: Record<string, string> = {};
		const { operatorIcon } = await import('@singgah/map');
		await Promise.all(
			ids.map(async (id) => {
				try {
					const d = await unwrap(api.GET('/api/v1/stations/{id}', { params: { path: { id } } }));
					coords[id] = [d.station.lon, d.station.lat];
					icons[id] = operatorIcon(d.station.operator);
				} catch {
					// unresolved stops drop out of the drawn line, not the plan
				}
			})
		);
		// Corridor colors come from the same route-lines catalog the network
		// layer draws — routeId → hex. Fetch on the plan's own bounds so legs
		// outside the current viewport still get their color.
		const colors: Record<string, string> = {};
		for (const f of netLines?.features ?? []) colors[f.properties.routeId] = f.properties.color;
		const cvals = Object.values(coords);
		if (cvals.length > 0) {
			const lons = cvals.map((c) => c[0]);
			const lats = cvals.map((c) => c[1]);
			const bbox = [
				Math.min(...lons) - 0.01,
				Math.min(...lats) - 0.01,
				Math.max(...lons) + 0.01,
				Math.max(...lats) + 0.01
			]
				.map((n) => n.toFixed(5))
				.join(',');
			try {
				const ld = await unwrap(api.GET('/api/v1/map/lines', { params: { query: { bbox } } }));
				for (const f of ld.lines.features) colors[f.properties.routeId] = f.properties.color;
			} catch {
				// missing colors fall back to the brand line — cosmetic only
			}
		}
		const legColor = (leg: (typeof legs)[number]) =>
			leg.type === 'walk' || !leg.routeId ? '' : (colors[leg.routeId] ?? '');
		const features: Feature[] = [];
		for (const leg of legs) {
			// leg.geometry is a real route shape sliced server-side (GTFS);
			// the stop-to-stop polyline is the fallback for unshaped routes.
			const apiLine =
				leg.geometry?.type === 'LineString'
					? (leg.geometry.coordinates as [number, number][])
					: undefined;
			const seq = [leg.from, ...(leg.stops ?? []), leg.to].filter(
				(s, i, arr) => i === 0 || s.id !== arr[i - 1].id
			);
			const line = apiLine ?? seq.flatMap((s) => (s.id && coords[s.id] ? [coords[s.id]] : []));
			if (line.length > 1) {
				features.push({
					type: 'Feature',
					geometry: { type: 'LineString', coordinates: line },
					properties: { dashed: leg.type === 'walk', color: legColor(leg) }
				});
			}
		}
		// Board/alight points (ends + transfers) get solid dots; stops ridden
		// through get the hollow marker — `endpoint` splits the two layers.
		const marked: string[] = [];
		const pushPoint = (ref: Plan['from'], endpoint: boolean, color = '') => {
			if (!ref.id || marked.includes(ref.id) || !coords[ref.id]) return;
			marked.push(ref.id);
			features.push({
				type: 'Feature',
				geometry: { type: 'Point', coordinates: coords[ref.id] },
				properties: { endpoint, icon: icons[ref.id] ?? '', color }
			});
		};
		for (const ref of [p.from, p.to]) pushPoint(ref, true);
		for (const leg of legs) {
			const c = legColor(leg);
			pushPoint(leg.from, true, c);
			pushPoint(leg.to, true, c);
		}
		for (const leg of legs) {
			const c = legColor(leg);
			for (const s of leg.stops ?? []) pushPoint(s, false, c);
		}
		return { type: 'FeatureCollection', features };
	}

	// Viewport-scoped markers + route lines, same contract as /map.
	let mapStations = $state<Station[]>([]);
	let netLines = $state<RouteLines | undefined>();
	let stationsError = $state('');
	// First fetch only — later pans keep the last markers and don't flash a
	// loading note over a usable map.
	let stationsFirstLoad = $state(true);
	// A dead map (style load failure) never emits a viewport, so the stations
	// state would spin "Memuat…" forever — the note is gated on this instead.
	let mapFailed = $state(false);
	// Manual "Garis rute" switch — a drawn journey/corridor still wins, so
	// effective visibility is `linesVisible && !routeActive` on the map.
	let linesVisible = $state(false);
	let mapTimer: ReturnType<typeof setTimeout>;
	function onViewportChange(bbox: [number, number, number, number]) {
		clearTimeout(mapTimer);
		mapTimer = setTimeout(() => {
			const bboxStr = bbox.map((n) => n.toFixed(5)).join(',');
			void (async () => {
				try {
					const data = await unwrap(
						api.GET('/api/v1/stations', {
							params: { query: { bbox: bboxStr, limit: 500 } }
						})
					);
					mapStations = data.stations;
					stationsError = '';
				} catch (e) {
					// Keep the last good marker set — the map note reports the outage
					// instead of leaving an unexplained empty map.
					stationsError = e instanceof Error ? e.message : 'Data stasiun gagal dimuat.';
				} finally {
					stationsFirstLoad = false;
				}
			})();
			// Route lines degrade independently — decorative layer, same as /map.
			void (async () => {
				try {
					const data = await unwrap(
						api.GET('/api/v1/map/lines', { params: { query: { bbox: bboxStr } } })
					);
					netLines = data.lines;
				} catch {
					/* decorative layer — degrade silently */
				}
			})();
		}, 250);
	}

	// Context inspector — the full station detail surface (same contract as the
	// /map sheet): catalog detail and the departure board fetch in parallel and
	// degrade independently, so a timetable outage still shows the station. The
	// same path serves marker clicks and search picks; coordinates double as
	// the camera target, so off-screen picks ease into view.
	let inspector = $state<StationDetail | null>(null);
	let inspectorState = $state<'idle' | 'loading' | 'error'>('idle');
	// Selection marker for the integration map — set on click so the halo
	// doesn't wait for (or flicker on) the detail fetch.
	let inspectingId = $state<string | undefined>();
	let board = $state<Board | null>(null);
	let boardError = $state('');
	let boardLoading = $state(false);
	let focusPoint = $state<[number, number] | undefined>();
	let inspectSeq = 0;

	function inspect(id: string) {
		const seq = ++inspectSeq;
		closeCorridor();
		inspectingId = id;
		inspector = null;
		inspectorState = 'loading';
		board = null;
		boardError = '';
		boardLoading = true;
		void (async () => {
			try {
				const data = await unwrap(api.GET('/api/v1/stations/{id}', { params: { path: { id } } }));
				if (seq !== inspectSeq) return;
				inspector = data.station;
				inspectorState = 'idle';
				focusPoint = [data.station.lon, data.station.lat];
			} catch {
				if (seq === inspectSeq) inspectorState = 'error';
			}
		})();
		void (async () => {
			try {
				const data = await unwrap(
					api.GET('/api/v1/stations/{id}/departures', { params: { path: { id } } })
				);
				if (seq === inspectSeq) board = data.departures;
			} catch (e) {
				if (seq === inspectSeq)
					boardError = e instanceof Error ? e.message : 'Jadwal sedang tidak tersedia.';
			} finally {
				if (seq === inspectSeq) boardLoading = false;
			}
		})();
	}

	// Swap writes name+selected together on both fields — each combobox sees a
	// programmatic pair and stays closed instead of reopening its dropdown.
	function swapStations() {
		[fromStation, toStation] = [toStation, fromStation];
		[fromQuery, toQuery] = [toQuery, fromQuery];
	}

	function closeInspector() {
		inspectSeq++; // stale in-flight fetches must not repopulate the panel
		closeCorridor();
		inspectingId = undefined;
		inspector = null;
		inspectorState = 'idle';
		board = null;
		boardError = '';
		boardLoading = false;
	}

	// Corridor drill-down — a line picked from the inspector swaps the station
	// view for that corridor's ordered stop list and draws its path plus every
	// halt on the map (the corridor FC replaces the journey overlay while open).
	let corridor = $state<RouteDetail | null>(null);
	let corridorState = $state<'idle' | 'loading' | 'error'>('idle');
	let corridorSeq = 0;

	function closeCorridor() {
		corridorSeq++;
		corridor = null;
		corridorState = 'idle';
		corridorRoute = undefined;
	}

	async function openCorridor(lineId: string) {
		const seq = ++corridorSeq;
		corridor = null;
		corridorState = 'loading';
		corridorRoute = undefined;
		try {
			const d = await unwrap(api.GET('/api/v1/routes/{id}', { params: { path: { id: lineId } } }));
			if (seq !== corridorSeq) return;
			corridor = d.route;
			corridorState = 'idle';
			const fc = await corridorGeoJSON(d.route);
			if (seq === corridorSeq) corridorRoute = fc;
		} catch {
			if (seq === corridorSeq) corridorState = 'error';
		}
	}

	// An open corridor isolates itself on the schematic (dim veil) — the key
	// join is only populated for corridors actually traced on the artwork.
	const corridorLineKey = $derived(corridor ? keyByRouteId.get(corridor.id) : undefined);

	// Line taps: a corridor stroke on the schematic resolves "OP:CODE" via the
	// catalog join; the geo map's route lines already carry canonical ids. An
	// open inspector drills into its corridor view — otherwise the route page
	// carries the detail (same destination as picking a line in search).
	function onLineTap(routeId: string) {
		if (inspector) void openCorridor(routeId);
		else void goto(resolve('/routes/[id]', { id: routeId }));
	}
	function onSchematicLine(key: string) {
		const r = routesByKey.get(key);
		if (r) onLineTap(r.id);
	}

	// Drawable corridor: the catalog's MultiLineString for this route wins
	// (real ingested shape); a polyline through the ordered stops is the
	// documented fallback. Every halt becomes a map dot — termini solid,
	// interior stops hollow — so the drawn route shows each halte it passes.
	async function corridorGeoJSON(r: RouteDetail): Promise<FeatureCollection> {
		const stops = r.stops;
		let geom = netLines?.features.find((f) => f.properties.routeId === r.id)?.geometry
			.coordinates as [number, number][][] | undefined;
		if (!geom && stops.length > 1) {
			const lons = stops.map((s) => s.lon);
			const lats = stops.map((s) => s.lat);
			const bbox = [
				Math.min(...lons) - 0.01,
				Math.min(...lats) - 0.01,
				Math.max(...lons) + 0.01,
				Math.max(...lats) + 0.01
			]
				.map((n) => n.toFixed(5))
				.join(',');
			try {
				const ld = await unwrap(api.GET('/api/v1/map/lines', { params: { query: { bbox } } }));
				geom = ld.lines.features.find((f) => f.properties.routeId === r.id)?.geometry
					.coordinates as [number, number][][] | undefined;
			} catch {
				// falls back to the stop polyline below
			}
		}
		if (!geom && stops.length > 1) geom = [stops.map((s) => [s.lon, s.lat])];
		const features: Feature[] = [];
		for (const seg of geom ?? []) {
			features.push({
				type: 'Feature',
				geometry: { type: 'LineString', coordinates: seg },
				properties: { color: r.color ?? '' }
			});
		}
		stops.forEach((s, i) => {
			features.push({
				type: 'Feature',
				geometry: { type: 'Point', coordinates: [s.lon, s.lat] },
				properties: {
					endpoint: i === 0 || i === stops.length - 1,
					icon: '',
					color: r.color ?? ''
				}
			});
		});
		return { type: 'FeatureCollection', features };
	}

	const agencies = $derived.by(() => {
		if (!inspector) return [];
		const out: { name: string; color?: string }[] = [];
		for (const l of inspector.lines) {
			const name = l.agencyName ?? l.mode;
			if (!out.some((a) => a.name === name)) out.push({ name, color: l.color });
		}
		return out;
	});

	// Entry points that already exist — no placeholders for unbuilt features.
	const STARTS = [
		{
			href: '/routes',
			label: 'Semua rute',
			meta: 'Layanan transit yang tercatat',
			tint: 'color-mix(in srgb, var(--sg-brand) 12%, var(--sg-surface))',
			tone: 'var(--sg-brand)',
			icon: ['M8 6h13', 'M8 12h13', 'M8 18h13', 'M3 6h.01', 'M3 12h.01', 'M3 18h.01']
		},
		{
			href: '/providers',
			label: 'Penyedia data',
			meta: 'Sumber, lisensi, kesegaran',
			tint: 'color-mix(in srgb, var(--sg-warning) 14%, var(--sg-surface))',
			tone: 'var(--sg-warning)',
			icon: [
				'M12 8c4.97 0 9-1.34 9-3s-4.03-3-9-3-9 1.34-9 3 4.03 3 9 3z',
				'M3 5v14c0 1.66 4.03 3 9 3s9-1.34 9-3V5',
				'M3 12c0 1.66 4.03 3 9 3s9-1.34 9-3'
			]
		},
		{
			href: '/explore',
			label: 'Jelajah',
			meta: 'Tempat di sekitar transit',
			tint: 'color-mix(in srgb, var(--sg-warm) 18%, var(--sg-surface))',
			tone: 'var(--sg-warm)',
			icon: [
				'M12 22a10 10 0 1 0 0-20 10 10 0 0 0 0 20z',
				'M16.24 7.76l-2.12 6.36-6.36 2.12 2.12-6.36z'
			]
		},
		{
			href: '/passport',
			label: 'Paspor',
			meta: 'Catatan perjalananmu',
			tint: 'color-mix(in srgb, var(--sg-line-mrt) 12%, var(--sg-surface))',
			tone: 'var(--sg-line-mrt)',
			icon: [
				'M4 19.5A2.5 2.5 0 0 1 6.5 17H20',
				'M6.5 2H20v20H6.5A2.5 2.5 0 0 1 4 19.5v-15A2.5 2.5 0 0 1 6.5 2z'
			]
		}
	] as const;
</script>

<svelte:head><title>Beranda · Singgah</title></svelte:head>

<svelte:window
	onkeydown={(e) => {
		if (e.key === 'Escape' && (inspector || inspectorState !== 'idle')) closeInspector();
	}}
/>

<div class="home-compact">
	<h1 class="sg-page-title">Singgah</h1>
	<p class="sg-meta">Pergi boleh spontan. Rute jangan.</p>
	<p>
		<a href={resolve('/plan')}>Rencana perjalanan →</a> ·
		<a href={resolve('/map')}>Peta jaringan →</a> ·
		<a href={resolve('/routes')}>Semua rute →</a> ·
		<a href={resolve('/providers')}>Penyedia data →</a>
	</p>

	<div class="search">
		<SearchField
			label="Cari stasiun atau halte"
			placeholder="Cari stasiun atau halte…"
			bind:value={query}
		/>
	</div>

	{#if loading}
		<span class="sg-sr-only" role="status">Mencari…</span>
		<ul class="sg-list" aria-hidden="true">
			{#each [0, 1, 2] as i (i)}
				<li><span class="sg-skeleton" style:inline-size="{55 - i * 12}%"></span></li>
			{/each}
		</ul>
	{:else if error}
		<StateBlock kind="error">{error}</StateBlock>
	{:else if query.trim() !== '' && stations.length === 0}
		<StateBlock kind="empty">Tidak ada stasiun yang cocok dengan “{query.trim()}”.</StateBlock>
	{:else if stations.length > 0}
		<ul class="sg-list">
			{#each stations as station (station.id)}
				<li>
					<a href={resolve('/stations/[id]', { id: station.id })}>
						<span class="name">{station.name}</span>
						{#if station.code}<span class="code">{station.code}</span>{/if}
						<span class="kind">{station.kind}</span>
					</a>
				</li>
			{/each}
		</ul>
	{/if}

	{#if dev}
		<UiPreview />
	{/if}
</div>

<div class="workspace">
	<div class="float">
		<div class="search-float">
			<MapSearch
				onStation={(s) => inspect(s.id)}
				onLine={(l) => goto(resolve('/routes/[id]', { id: l.id }))}
			/>
		</div>
		<aside class="panel" aria-label="Rencanakan perjalanan">
			<h2 class="panel-title">Rencanakan Perjalanan</h2>

			<form
				onsubmit={(e) => {
					e.preventDefault();
					goPlan();
				}}
			>
				<div class="fieldgroup">
					<div class="fg-row">
						<i class="fg-dot fg-dot--from" aria-hidden="true"></i>
						<div class="fg-field">
							<label for="w-from">Dari</label>
							<StationCombobox
								id="w-from"
								label="Stasiun asal"
								placeholder="Nama stasiun…"
								bind:value={fromQuery}
								bind:selected={fromStation}
							/>
						</div>
					</div>
					<hr class="fg-sep" />
					<div class="fg-row">
						<i class="fg-dot fg-dot--to" aria-hidden="true"></i>
						<div class="fg-field">
							<label for="w-to">Ke</label>
							<StationCombobox
								id="w-to"
								label="Stasiun tujuan"
								placeholder="Nama stasiun…"
								bind:value={toQuery}
								bind:selected={toStation}
							/>
						</div>
					</div>
					<button
						type="button"
						class="fg-swap"
						aria-label="Tukar asal dan tujuan"
						onclick={swapStations}
					>
						<svg
							viewBox="0 0 24 24"
							fill="none"
							stroke="currentColor"
							stroke-width="2"
							stroke-linecap="round"
							stroke-linejoin="round"
							width="16"
							height="16"
							aria-hidden="true"
						>
							<path d="M8 4v14" />
							<path d="m8 4-3 3" />
							<path d="m8 4 3 3" />
							<path d="M16 20V6" />
							<path d="m16 20 3-3" />
							<path d="m16 20-3-3" />
						</svg>
					</button>
				</div>
				<Button class="cta" type="submit" disabled={!fromStation || !toStation || planning}>
					{planning ? 'Mencari…' : 'Cari Rute'}
				</Button>
			</form>

			{#if planning}
				<StateBlock kind="loading">Mencari rute…</StateBlock>
			{:else if planError}
				<StateBlock kind="error">{planError}</StateBlock>
			{:else if plan}
				{#if planFromCache}
					<StateBlock kind="offline">
						Rute tersimpan — data per {new Date(plan.source.requestedAt).toLocaleString('id-ID')}
					</StateBlock>
				{/if}
				{#if plan.itineraries.length === 0}
					<StateBlock kind="empty">
						Tidak ada rute terjadwal antara {plan.from.name} dan {plan.to.name}.
					</StateBlock>
				{:else}
					{@const itin = plan.itineraries[0]}
					<div class="plan-summary">
						<StatusBadge status={itin.status} />
						<span class="sg-meta">
							{#if plan.fareReference?.total != null}Rp{plan.fareReference.total.toLocaleString(
									'id-ID'
								)} ·
							{/if}
							{Math.floor(itin.durationSec / 60)} mnt · {itin.rideLegs} naik
							{#if itin.transfers > 0}· {itin.transfers} transit{/if}
						</span>
					</div>
					<details class="plan-detail">
						<summary>Rincian perjalanan</summary>
						<JourneyTimeline
							from={plan.from}
							legs={itin.legs}
							onSelectAlternative={onLegAlternative}
						/>
					</details>
				{/if}
			{/if}

			<h3 class="starts-title">Mulai dari sini</h3>
			<ul class="starts">
				{#each STARTS as s (s.href)}
					<li>
						<a href={resolve(s.href)}>
							<span class="tile" style:background-color={s.tint} style:color={s.tone}>
								<svg
									viewBox="0 0 24 24"
									fill="none"
									stroke="currentColor"
									stroke-width="2"
									stroke-linecap="round"
									stroke-linejoin="round"
									aria-hidden="true"
								>
									{#each s.icon as d (d)}
										<path {d} />
									{/each}
								</svg>
							</span>
							<span>
								<strong>{s.label}</strong>
								<span class="sg-meta">{s.meta}</span>
							</span>
						</a>
					</li>
				{/each}
			</ul>
		</aside>
	</div>

	<div class="map-canvas">
		{#if wide && mapMod}
			{@const TransitMap = mapMod.TransitMap}
			{@const IntegrationMap = mapMod.IntegrationMap}
			<!-- Both panes stay mounted across mode switches — hiding, not
			     recreating, keeps each map's camera and loaded tiles. -->
			<div class="map-pane" hidden={mapStore.mode !== 'geographic'}>
				<TransitMap
					styleUrl={STYLE_URL}
					data={mapMod.stationsToGeoJSON(mapStations)}
					zoom={13}
					lines={netLines}
					linesVisible={linesVisible && !routeActive}
					{onViewportChange}
					onSelect={inspect}
					onSelectLine={onLineTap}
					focus={focusPoint}
					onFailed={() => (mapFailed = true)}
					{route}
				/>
				<label class="lines-toggle">
					<input type="checkbox" bind:checked={linesVisible} />
					Garis rute
				</label>
			</div>
			<div class="map-pane" hidden={mapStore.mode !== 'integration'}>
				{#if integrationFailed}
					<div class="im-fallback">
						<StateBlock kind="error">Peta integrasi tidak dapat dimuat.</StateBlock>
					</div>
				{:else if integrationManifest}
					<IntegrationMap
						tiles={integrationTiles}
						previewUrl={integrationManifest.preview
							? `${integrationManifest.assetDir}${integrationManifest.preview.url}${integrationManifest.build ? `?v=${integrationManifest.build}` : ''}`
							: undefined}
						worldWidth={integrationManifest.viewBox[2]}
						worldHeight={integrationManifest.viewBox[3]}
						points={integrationPoints}
						lines={integrationLines}
						labels={integrationLabels}
						lineKey={corridorLineKey}
						markers={integrationManifest.markers ?? 'overlay'}
						attribution={integrationManifest.attribution}
						selectedId={inspectingId}
						visible={mapStore.mode === 'integration'}
						onSelect={inspect}
						onLineSelect={onSchematicLine}
						bind:camera={mapStore.integrationCamera}
					/>
				{:else}
					<div class="im-fallback">
						<StateBlock kind="loading">Memuat peta integrasi…</StateBlock>
					</div>
				{/if}
			</div>
			{#if !mapFailed && stationsFirstLoad}
				<div class="map-note">
					<StateBlock kind="loading">Memuat stasiun…</StateBlock>
				</div>
			{:else if !mapFailed && stationsError}
				<div class="map-note">
					<StateBlock kind="error">{stationsError}</StateBlock>
				</div>
			{/if}
			<MapModeSwitcher />
		{:else}
			<StateBlock kind="loading">Memuat peta…</StateBlock>
		{/if}
		<ul class="legend" aria-label="Legenda operator">
			<li><i style:color="var(--sg-line-krl)"><ModeIcon mode="commuter" /></i>KRL</li>
			<li><i style:color="var(--sg-line-mrt)"><ModeIcon mode="metro" /></i>MRT</li>
			<li><i style:color="var(--sg-line-tj)"><ModeIcon mode="bus" /></i>TransJakarta</li>
			<li><i style:color="var(--sg-line-lrt)"><ModeIcon mode="light-rail" /></i>LRT</li>
		</ul>

		{#if inspectorState !== 'idle' || inspector}
			<aside class="inspector" aria-label="Detail stasiun">
				<IconButton class="close" label="Tutup detail" onclick={closeInspector}>
					<svg
						viewBox="0 0 24 24"
						fill="none"
						stroke="currentColor"
						stroke-width="2"
						stroke-linecap="round"
						width="18"
						height="18"
						aria-hidden="true"
					>
						<path d="M18 6 6 18" />
						<path d="m6 6 12 12" />
					</svg>
				</IconButton>
				{#if inspectorState === 'loading'}
					<StateBlock kind="loading">Memuat stasiun…</StateBlock>
				{:else if inspectorState === 'error'}
					<StateBlock kind="error">Detail stasiun gagal dimuat.</StateBlock>
				{:else if inspector}
					{@const st = inspector}
					{#if corridorState !== 'idle' || corridor}
						<button type="button" class="insp-back" onclick={closeCorridor}>
							<svg
								viewBox="0 0 24 24"
								fill="none"
								stroke="currentColor"
								stroke-width="2"
								stroke-linecap="round"
								stroke-linejoin="round"
								width="14"
								height="14"
								aria-hidden="true"><path d="M19 12H5" /><path d="m12 19-7-7 7-7" /></svg
							>
							{st.name}
						</button>
						{#if corridorState === 'loading'}
							<StateBlock kind="loading">Memuat koridor…</StateBlock>
						{:else if corridorState === 'error'}
							<StateBlock kind="error">Detail koridor gagal dimuat.</StateBlock>
						{:else if corridor}
							<div class="corridor-head">
								<i
									class="cbadge"
									style:background-color={corridor.color
										? `#${corridor.color}`
										: 'var(--sg-line-default)'}>{corridor.shortName ?? '—'}</i
								>
								<h2 class="inspector-name">{corridor.longName ?? corridor.agencyName}</h2>
							</div>
							<p class="sg-meta">{corridor.agencyName} · {corridor.stops.length} perhentian</p>
							{#if corridor.stops.length === 0}
								<StateBlock kind="empty">Urutan halte koridor ini belum tersedia.</StateBlock>
							{:else}
								<ol class="insp-list corridor-stops">
									{#each corridor.stops as stop (stop.id + stop.seq)}
										<li>
											<button
												type="button"
												class="insp-link corridor-stop"
												onclick={() => inspect(stop.id)}
											>
												<span class="corridor-seq">{stop.stationNumber ?? stop.seq}</span>
												{stop.name}
											</button>
										</li>
									{/each}
								</ol>
							{/if}
							<p class="sg-meta insp-note">
								Data {corridor.source.provider} — urutan mengikuti data provider.
							</p>
						{/if}
					{:else}
						<h2 class="inspector-name">{inspector.name}</h2>
						<p class="sg-meta">
							{inspector.kind}{#if inspector.code}
								· {inspector.code}{/if}
						</p>
						{#if agencies.length > 0}
							<ul class="chips">
								{#each agencies as a (a.name)}
									<li>
										<i
											class="badge"
											style:background-color={a.color ? `#${a.color}` : 'var(--sg-line-default)'}
										>
											<svg viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">
												<path
													d="M12 2c-3.4 0-5.8.6-5.8 4v8a3.4 3.4 0 0 0 3.3 3.4L8 20v1h1.6l1.4-2h2l1.4 2H16v-1l-1.5-2.6A3.4 3.4 0 0 0 17.8 14V6c0-3.4-2.4-4-5.8-4zM8 7h8v4H8V7zm1.4 7.5a1.3 1.3 0 1 1 0 2.6 1.3 1.3 0 0 1 0-2.6zm5.2 0a1.3 1.3 0 1 1 0 2.6 1.3 1.3 0 0 1 0-2.6z"
												/>
											</svg>
										</i>{a.name}
									</li>
								{/each}
							</ul>
						{/if}
						<p class="sg-meta">
							{inspector.lines.length} rute melayani · {inspector.transfers.length} transfer tercatat
						</p>
						<div class="actions">
							<a class="btn-ghost" href={resolve('/stations/[id]', { id: inspector.id })}
								>Halaman stasiun</a
							>
							<button
								class="btn-primary"
								type="button"
								onclick={() => {
									fromStation = st;
									fromQuery = st.name;
									closeInspector();
								}}>Rute dari sini</button
							>
						</div>

						<section class="insp-section" aria-label="Keberangkatan">
							<h3 class="insp-heading">Keberangkatan</h3>
							{#if boardError}
								<StateBlock kind="error">{boardError}</StateBlock>
							{:else if boardLoading}
								<StateBlock kind="loading">Memuat jadwal…</StateBlock>
							{:else if board}
								{#if board.lines.length === 0}
									<StateBlock kind="empty"
										>Tidak ada keberangkatan terjadwal dalam {Math.round(board.windowMinutes / 60)} jam
										ke depan.</StateBlock
									>
								{:else}
									<DepartureBoard lines={board.lines} compact />
								{/if}
								<p class="sg-meta insp-note">
									Jadwal statis {board.source.provider} — bukan posisi live.
								</p>
							{/if}
						</section>

						{#if st.lines.length > 0}
							<section class="insp-section" aria-label="Koridor">
								<h3 class="insp-heading">Koridor</h3>
								<ul class="insp-list">
									{#each st.lines as l (l.id)}
										<li>
											<button
												type="button"
												class="insp-link corridor-link"
												onclick={() => openCorridor(l.id)}
											>
												<i
													class="cbadge"
													style:background-color={l.color
														? `#${l.color}`
														: 'var(--sg-line-default)'}>{l.shortName ?? '—'}</i
												>
												<span>{l.longName ?? l.agencyName ?? 'Lin'}</span>
											</button>
										</li>
									{/each}
								</ul>
							</section>
						{/if}

						{#if st.transfers.length > 0}
							<section class="insp-section" aria-label="Transit">
								<h3 class="insp-heading">Transit</h3>
								<ul class="insp-list">
									{#each st.transfers as t (t.toStop.id)}
										{@const tid = t.toStop.id}
										<li>
											{#if tid}
												<button type="button" class="insp-link" onclick={() => inspect(tid)}>
													<span>{t.toStop.name}</span>
													{#if t.walkDistanceM}<span class="sg-meta"
															>· jalan {t.walkDistanceM} m</span
														>{/if}
													{#if t.notes}<span class="sg-meta">· {t.notes}</span>{/if}
												</button>
											{:else}
												{t.toStop.name}
											{/if}
										</li>
									{/each}
								</ul>
							</section>
						{/if}

						{#if st.facilities.length > 0}
							<section class="insp-section" aria-label="Fasilitas">
								<h3 class="insp-heading">Fasilitas</h3>
								<ul class="insp-list">
									{#each st.facilities as f (f.type + f.text)}
										<li>
											{facilityLabel(f.type)}{#if f.text}<span class="sg-meta">
													· {f.text}</span
												>{/if}
										</li>
									{/each}
								</ul>
							</section>
						{/if}
					{/if}
				{/if}
			</aside>
		{/if}
	</div>
</div>

<style>
	.search {
		max-width: 32rem;
		margin-block: var(--sg-space-4);
	}
	.name {
		font-weight: var(--sg-weight-semibold);
	}
	.code,
	.kind {
		color: var(--sg-text-muted);
		font-size: var(--sg-text-secondary);
	}

	/* Workspace — docs/10 wide layout: full-bleed persistent map with the
	   planner floating over it (same overlay idiom as /map's .sheet) plus a
	   context inspector. Compact keeps the lightweight search home (no map
	   boot). */
	.workspace {
		display: none;
	}

	@media (min-width: 75rem) {
		.home-compact {
			display: none;
		}

		/* Full-bleed canvas — the workspace drops main's padding AND its 96rem
		   cap so the map touches the viewport edges right of the nav rail
		   (web-concept). :has keeps other pages' measure intact. */
		:global(main:has(> .workspace)) {
			max-width: none;
			padding: 0;
		}

		.workspace {
			display: block;
			position: relative;
			height: 100dvh;
		}

		/* Floating column over the map: global search on top, the planner card
		   below it. The card scrolls when results grow; the search stays put. */
		.float {
			position: absolute;
			top: var(--sg-space-3);
			inset-inline-start: var(--sg-space-3);
			width: 20rem;
			max-height: calc(100% - var(--sg-space-6));
			display: flex;
			flex-direction: column;
			gap: var(--sg-space-2);
			z-index: var(--sg-z-sticky);
		}

		.search-float {
			flex: none;
			border-radius: var(--sg-radius-input);
			box-shadow: var(--sg-shadow-overlay);
		}

		.panel {
			min-height: 0;
			overflow-y: auto;
			padding: var(--sg-space-4);
			background-color: var(--sg-surface);
			border: 1px solid var(--sg-border);
			border-radius: var(--sg-radius-card);
			box-shadow: var(--sg-shadow-overlay);
		}

		.panel-title {
			font-size: var(--sg-text-section);
			margin-block: var(--sg-space-1) var(--sg-space-3);
		}

		/* Dari/Ke as one grouped card (web-concept reference): dot markers stand
		   in for the search glyph, a divider splits the rows, and the swap
		   button rides the right edge. */
		.fieldgroup {
			position: relative;
			margin-bottom: var(--sg-space-3);
			border: 1px solid var(--sg-border);
			border-radius: var(--sg-radius-input);
			background-color: var(--sg-surface);
		}

		.fieldgroup:focus-within {
			border-color: var(--sg-brand);
		}

		.fg-row {
			display: flex;
			align-items: center;
			gap: var(--sg-space-3);
			padding: var(--sg-space-1) var(--sg-space-12) var(--sg-space-1) var(--sg-space-3);
		}

		.fg-dot {
			inline-size: 0.625rem;
			block-size: 0.625rem;
			border-radius: 50%;
			flex: none;
		}

		.fg-dot--from {
			background-color: var(--sg-brand);
		}

		.fg-dot--to {
			background-color: var(--sg-danger);
		}

		.fg-field {
			flex: 1;
			min-inline-size: 0;
		}

		.fg-field label {
			display: block;
			font-size: var(--sg-text-meta);
			color: var(--sg-text-muted);
		}

		.fieldgroup :global(.sg-search) {
			border: 0;
			padding-inline: 0;
			min-height: 2rem;
		}

		.fieldgroup :global(.sg-search:focus-within) {
			border-color: transparent;
		}

		.fieldgroup :global(.sg-search__icon) {
			display: none;
		}

		.fg-sep {
			border: 0;
			border-top: 1px solid var(--sg-border);
			margin: 0 var(--sg-space-3);
		}

		.fg-swap {
			position: absolute;
			top: 50%;
			inset-inline-end: var(--sg-space-2);
			transform: translateY(-50%);
			display: grid;
			place-items: center;
			inline-size: 2rem;
			block-size: 2rem;
			padding: 0;
			border: 1px solid var(--sg-border);
			border-radius: 50%;
			background-color: var(--sg-surface);
			color: var(--sg-text-muted);
			cursor: pointer;
		}

		.fg-swap:hover {
			color: var(--sg-text);
			border-color: var(--sg-text-muted);
		}

		.workspace :global(.cta) {
			width: 100%;
			justify-content: center;
		}

		.plan-summary {
			display: flex;
			align-items: center;
			flex-wrap: wrap;
			gap: var(--sg-space-2) var(--sg-space-3);
			margin-block-start: var(--sg-space-3);
		}

		.plan-detail {
			margin-block-start: var(--sg-space-2);
		}

		.plan-detail summary {
			cursor: pointer;
			font-size: var(--sg-text-secondary);
			font-weight: var(--sg-weight-semibold);
			color: var(--sg-text-muted);
		}

		.starts-title {
			font-size: var(--sg-text-secondary);
			font-weight: var(--sg-weight-bold);
			color: var(--sg-text);
			margin: var(--sg-space-6) 0 var(--sg-space-1);
		}

		.starts {
			list-style: none;
			margin: 0;
			padding: 0;
		}

		.starts a {
			display: flex;
			align-items: center;
			gap: var(--sg-space-3);
			min-height: var(--sg-target-min);
			padding: var(--sg-space-2);
			border-radius: var(--sg-radius-button);
			color: var(--sg-text);
			text-decoration: none;
		}

		.starts a:hover {
			background-color: var(--sg-surface-muted);
		}

		.starts .tile {
			display: grid;
			place-items: center;
			inline-size: 2.25rem;
			block-size: 2.25rem;
			border-radius: var(--sg-radius-button);
			flex: none;
		}

		.starts .tile svg {
			inline-size: 1.125rem;
			block-size: 1.125rem;
		}

		.starts .sg-meta {
			display: block;
		}

		.map-canvas {
			position: absolute;
			inset: 0;
			display: grid;
			place-items: center;
			overflow: hidden;
			background-color: var(--sg-surface-muted);
		}

		.map-canvas :global(.sg-map) {
			height: 100%;
		}

		/* Dual-mode panes + overlay controls — same idioms as /map: panes fill
		   the canvas and swap via `hidden`; the switcher owns bottom-center so
		   the legend rides above it. */
		.map-pane {
			position: absolute;
			inset: 0;
		}

		.map-pane[hidden] {
			display: none;
		}

		.im-fallback {
			display: grid;
			place-items: center;
			width: 100%;
			height: 100%;
			background-color: var(--sg-surface-muted);
		}

		.lines-toggle {
			position: absolute;
			top: var(--sg-space-3);
			inset-inline-end: var(--sg-space-3);
			display: inline-flex;
			align-items: center;
			gap: var(--sg-space-2);
			min-height: var(--sg-target-min);
			padding: 0 var(--sg-space-3);
			background-color: var(--sg-surface);
			border: 1px solid var(--sg-border);
			border-radius: var(--sg-radius-button);
			box-shadow: var(--sg-shadow-overlay);
			font-size: var(--sg-text-secondary);
			z-index: 1;
		}

		/* Station-layer status — same .map-note idiom as /map: a centered chip
		   under the top edge, inert so it never eats map gestures. */
		.map-note {
			position: absolute;
			top: var(--sg-space-3);
			inset-inline: var(--sg-space-3);
			display: flex;
			justify-content: center;
			pointer-events: none;
			z-index: 1;
		}

		.legend {
			position: absolute;
			bottom: calc(var(--sg-space-3) + var(--sg-target-min) + var(--sg-space-3));
			left: 50%;
			transform: translateX(-50%);
			display: flex;
			gap: var(--sg-space-3);
			list-style: none;
			margin: 0;
			padding: var(--sg-space-1) var(--sg-space-3);
			background-color: var(--sg-surface);
			border: 1px solid var(--sg-border);
			border-radius: var(--sg-radius-pill);
			box-shadow: var(--sg-shadow-card);
			font-size: var(--sg-text-meta);
			color: var(--sg-text-muted);
		}

		.legend li {
			display: flex;
			align-items: center;
			gap: var(--sg-space-1);
		}

		.legend i {
			display: inline-flex;
		}

		.inspector {
			position: absolute;
			top: var(--sg-space-3);
			inset-inline-end: var(--sg-space-3);
			width: 19rem;
			max-height: calc(100% - var(--sg-space-6));
			overflow-y: auto;
			padding: var(--sg-space-4);
			background-color: var(--sg-surface);
			border: 1px solid var(--sg-border);
			border-radius: var(--sg-radius-card);
			box-shadow: var(--sg-shadow-overlay);
			z-index: var(--sg-z-sticky);
		}

		.inspector :global(.close) {
			position: absolute;
			top: var(--sg-space-2);
			inset-inline-end: var(--sg-space-2);
		}

		.inspector-name {
			font-size: var(--sg-text-section);
			margin: 0 var(--sg-space-6) 0 0;
		}

		.chips {
			display: flex;
			flex-wrap: wrap;
			gap: var(--sg-space-2);
			list-style: none;
			margin: var(--sg-space-2) 0;
			padding: 0;
		}

		.chips li {
			display: inline-flex;
			align-items: center;
			gap: var(--sg-space-2);
			padding: var(--sg-space-1) var(--sg-space-2) var(--sg-space-1) var(--sg-space-1);
			border: 1px solid var(--sg-border);
			border-radius: var(--sg-radius-button);
			font-size: var(--sg-text-meta);
			font-weight: var(--sg-weight-semibold);
			min-height: 1.75rem;
		}

		.chips .badge {
			display: grid;
			place-items: center;
			inline-size: 1.375rem;
			block-size: 1.375rem;
			border-radius: calc(var(--sg-radius-input) - 2px);
			color: #fff;
		}

		.chips .badge svg {
			inline-size: 0.875rem;
			block-size: 0.875rem;
		}

		.insp-section {
			margin-block-start: var(--sg-space-3);
			padding-block-start: var(--sg-space-3);
			border-top: 1px solid var(--sg-border);
		}

		.insp-heading {
			margin: 0;
			font-size: var(--sg-text-secondary);
			font-weight: var(--sg-weight-bold);
			color: var(--sg-text-muted);
		}

		.insp-list {
			list-style: none;
			margin: var(--sg-space-1) 0 0;
			padding: 0;
			font-size: var(--sg-text-secondary);
		}

		.insp-list li {
			padding-block: var(--sg-space-1);
		}

		.insp-link {
			display: flex;
			align-items: center;
			flex-wrap: wrap;
			gap: var(--sg-space-1);
			width: 100%;
			min-height: var(--sg-target-min);
			padding: 0;
			background: none;
			border: 0;
			font: inherit;
			color: var(--sg-brand);
			text-align: left;
			cursor: pointer;
		}

		.insp-link:hover {
			text-decoration: underline;
		}

		.insp-note {
			margin-block: var(--sg-space-2) 0;
		}

		.insp-back {
			display: inline-flex;
			align-items: center;
			gap: var(--sg-space-1);
			min-height: var(--sg-target-min);
			margin-inline-start: calc(-1 * var(--sg-space-2));
			margin-block-end: var(--sg-space-1);
			padding: 0 var(--sg-space-2);
			background: none;
			border: 0;
			border-radius: var(--sg-radius-button);
			font: inherit;
			font-size: var(--sg-text-secondary);
			font-weight: var(--sg-weight-semibold);
			color: var(--sg-brand);
			cursor: pointer;
		}

		.insp-back:hover {
			text-decoration: underline;
		}

		.corridor-head {
			display: flex;
			align-items: center;
			gap: var(--sg-space-2);
			margin-block-end: var(--sg-space-1);
		}

		.corridor-head .inspector-name {
			margin: 0;
		}

		.cbadge {
			display: inline-grid;
			place-items: center;
			min-inline-size: 1.75rem;
			block-size: 1.75rem;
			padding-inline: var(--sg-space-1);
			border-radius: calc(var(--sg-radius-input) - 2px);
			color: #fff;
			font-size: var(--sg-text-meta);
			font-weight: var(--sg-weight-bold);
			font-style: normal;
			flex: none;
		}

		.corridor-link {
			color: var(--sg-text);
		}

		.corridor-stops {
			max-height: 16rem;
			overflow-y: auto;
		}

		.corridor-seq {
			display: inline-block;
			min-inline-size: 1.75rem;
			color: var(--sg-text-muted);
			font-size: var(--sg-text-meta);
			font-variant-numeric: tabular-nums;
		}

		.actions {
			display: flex;
			gap: var(--sg-space-2);
			margin-block-start: var(--sg-space-3);
		}

		.actions a,
		.actions button {
			display: inline-flex;
			align-items: center;
			justify-content: center;
			flex: 1;
			min-height: var(--sg-target-min);
			padding: var(--sg-space-2) var(--sg-space-3);
			border-radius: var(--sg-radius-button);
			font-size: var(--sg-text-secondary);
			font-weight: var(--sg-weight-semibold);
			font-family: inherit;
			text-decoration: none;
			cursor: pointer;
		}

		.btn-primary {
			border: 0;
			background-color: var(--sg-brand);
			color: var(--sg-brand-contrast);
		}

		.btn-ghost {
			border: 1px solid var(--sg-border);
			color: var(--sg-text);
		}
	}
</style>
