<script lang="ts">
	import { resolve } from '$app/paths';
	import { browser } from '$app/environment';
	import { onMount, tick, untrack } from 'svelte';
	import { page } from '$app/state';
	import { replaceState } from '$app/navigation';
	import { SvelteMap, SvelteSet, SvelteURLSearchParams } from 'svelte/reactivity';
	import { api } from '$lib/api';
	import { basemapStyleUrl } from '$lib/basemap';
	import { facilityLabel } from '$lib/facilities';
	import { mapStore } from '$lib/stores/map.svelte';
	import DepartureBoard from '$lib/components/station/DepartureBoard.svelte';
	import CheckinButton from '$lib/components/station/CheckinButton.svelte';
	import JournalNoteForm from '$lib/components/station/JournalNoteForm.svelte';
	import NearbyPlaces from '$lib/components/station/NearbyPlaces.svelte';
	import StationCombobox from '$lib/components/station/StationCombobox.svelte';
	import MapModeSwitcher from '$lib/components/map/MapModeSwitcher.svelte';
	import MapSearch from '$lib/components/map/MapSearch.svelte';
	import SheetSizeControls, {
		type MapSheetSize
	} from '$lib/components/map/SheetSizeControls.svelte';
	import {
		buildIntegrationTiles,
		loadIntegrationMap,
		type IntegrationManifest
	} from '$lib/integration-map';
	import { unwrap } from '@singgah/api-client';
	import type { components } from '@singgah/api-client';
	import { TransitMap, IntegrationMap, stationsToGeoJSON, journeyToGeoJSON } from '@singgah/map';
	import type { SchematicLabelPoint, SchematicLine, SchematicPoint } from '@singgah/map';
	import { IconButton, StateBlock, StatusBadge } from '@singgah/ui';

	interface Props {
		/** Mobile Explore/Passport panel height, in viewport-height units. */
		overlayHeightVh?: number;
	}

	let { overlayHeightVh = 0 }: Props = $props();

	type Station = components['schemas']['StationSummary'];
	type StationDetail = components['schemas']['StationDetail'];
	type Board = components['schemas']['StationDepartures'];
	type RouteLines = components['schemas']['RouteLineCollection'];
	type RouteSummary = components['schemas']['RouteSummary'];
	type RouteDetail = components['schemas']['RouteDetail'];
	type Plan = components['schemas']['JourneyPlan'];
	type Place = components['schemas']['PlaceSummary'];

	const STYLE_URL = basemapStyleUrl();

	function displayError(error: unknown, fallback: string): string {
		const message = error instanceof Error ? error.message.trim() : '';
		// Fetch implementations expose browser-specific transport strings. They
		// are not useful to travellers and should not leak into the surface.
		if (!message || /failed to fetch|networkerror|load failed/i.test(message)) return fallback;
		return message;
	}

	let stations = $state<Station[]>([]);
	let lines = $state<RouteLines | undefined>();
	let linesVisible = $state(false);
	let error = $state('');
	// First fetch only — later pans keep the last markers and don't flash a
	// loading note over a usable map.
	let firstLoad = $state(true);
	// A dead map (style load failure) never emits a viewport, so firstLoad
	// would spin "Memuat…" forever — the note is gated on this instead.
	let mapFailed = $state(false);
	let mapAttempt = $state(0);
	let plannerOpen = $state(false);
	let plannerAnchor = $state<'depart' | 'arrive'>('depart');
	let plannerTime = $state('');
	let plannerModes = new SvelteSet<string>();
	let plannerMaxWalk = $state('');
	let plannerMaxTransfers = $state('');
	let plannerStepFree = $state(false);
	let fromStation = $state<Station | null>(null);
	let toStation = $state<Station | null>(null);
	let fromQuery = $state('');
	let toQuery = $state('');

	// Station sheet — the two fetches degrade independently: a timetable
	// outage must not hide the station's catalog detail.
	let detail = $state<StationDetail | null>(null);
	let detailError = $state('');
	let board = $state<Board | null>(null);
	let boardError = $state('');
	let boardLoading = $state(false);
	let places = $state<Place[] | null>(null);
	let placesLoading = $state(false);
	let boardExpanded = $state(false);
	let sheetEl = $state<HTMLElement | undefined>();
	let sheetSize = $state<MapSheetSize>('medium');
	let mapBottomPadding = $state(0);

	$effect(() => {
		const panel = sheetEl;
		const externalPanelHeight = overlayHeightVh;
		if (!browser) return;

		const compactViewport = window.matchMedia('(max-width: 47.999rem)');
		const updatePadding = () => {
			if (!compactViewport.matches) {
				mapBottomPadding = 0;
				return;
			}
			if (panel?.isConnected) {
				mapBottomPadding = Math.max(
					0,
					Math.round(window.innerHeight - panel.getBoundingClientRect().top)
				);
				return;
			}
			const nav = document.querySelector<HTMLElement>('.app-nav');
			const navOcclusion = nav
				? Math.max(0, window.innerHeight - nav.getBoundingClientRect().top)
				: 0;
			mapBottomPadding = Math.max(
				0,
				Math.round((window.innerHeight * externalPanelHeight) / 100 + navOcclusion)
			);
		};

		updatePadding();
		const observer = panel ? new ResizeObserver(updatePadding) : undefined;
		if (panel) observer?.observe(panel);
		window.addEventListener('resize', updatePadding);
		compactViewport.addEventListener('change', updatePadding);
		return () => {
			observer?.disconnect();
			window.removeEventListener('resize', updatePadding);
			compactViewport.removeEventListener('change', updatePadding);
		};
	});
	// Camera target for the geographic map — set when the selected station's
	// detail (with lat/lon) arrives, from either map mode.
	let focusPoint = $state<[number, number] | undefined>();

	// Line detail — shares the sheet surface with the station detail
	// (single-detail-surface, blueprint §30); the two selections are
	// mutually exclusive through openStation/openLine.
	let lineDetail = $state<RouteDetail | null>(null);
	let lineError = $state('');

	// Display names for the origin/destination chips — the store keeps
	// canonical IDs only; names live here so mode switches stay free of
	// refetches.
	let fromName = $state<string | undefined>();
	let toName = $state<string | undefined>();

	// Journey — plan data is domain state kept out of the map components
	// (§79); the store only carries selectedJourneyId. The fetch re-runs on
	// the from/to pair and stale responses are dropped.
	let journey = $state<Plan | null>(null);
	let journeyLoading = $state(false);
	let journeyError = $state('');
	let journeyRequest = 0;
	let itineraryIndex = $state(0);

	$effect(() => {
		const from = mapStore.fromStationId;
		const to = mapStore.toStationId;
		const at = asInstant(plannerTime);
		const anchor = plannerAnchor;
		const modes = [...plannerModes].sort().join(',') || undefined;
		const maxWalkM = plannerMaxWalk ? Number(plannerMaxWalk) : undefined;
		const maxTransfers = plannerMaxTransfers ? Number(plannerMaxTransfers) : undefined;
		const stepFree = plannerStepFree || undefined;
		const requestId = ++journeyRequest;
		if (!from || !to) {
			journey = null;
			journeyError = '';
			journeyLoading = false;
			return;
		}
		let cancelled = false;
		journey = null;
		journeyError = '';
		journeyLoading = true;
		void (async () => {
			try {
				const p = await unwrap(
					api.GET('/api/v1/journeys', {
						params: {
							query: {
								from,
								to,
								at: anchor === 'depart' ? at : undefined,
								arriveBy: anchor === 'arrive' ? at : undefined,
								modes,
								maxWalkM,
								maxTransfers,
								stepFree
							}
						}
					})
				);
				if (cancelled || requestId !== journeyRequest) return;
				journey = p;
				itineraryIndex = Math.min(itineraryIndex, Math.max(0, p.itineraries.length - 1));
				mapStore.selectJourney(
					p.itineraries.length > 0 ? `${from}:${to}:${itineraryIndex}` : undefined
				);
			} catch (e) {
				if (cancelled || requestId !== journeyRequest) return;
				journeyError = displayError(e, 'Rencana rute gagal dimuat.');
			} finally {
				if (!cancelled && requestId === journeyRequest) journeyLoading = false;
			}
		})();
		return () => {
			cancelled = true;
		};
	});

	const selectedItinerary = $derived(
		journey?.itineraries[itineraryIndex] ?? journey?.itineraries[0]
	);
	const journeyRoute = $derived(
		selectedItinerary ? journeyToGeoJSON(selectedItinerary.legs) : undefined
	);

	const MODE_OPTIONS = [
		['rail', 'KRL'],
		['subway', 'MRT'],
		['tram', 'LRT'],
		['bus', 'Bus']
	] as const;

	function toggleMode(mode: string) {
		if (plannerModes.has(mode)) plannerModes.delete(mode);
		else plannerModes.add(mode);
	}

	function asInstant(value: string): string | undefined {
		if (!value) return undefined;
		const date = new Date(value);
		return Number.isNaN(date.getTime()) ? undefined : date.toISOString();
	}

	function swapStations() {
		[fromStation, toStation] = [toStation, fromStation];
		[fromQuery, toQuery] = [toQuery, fromQuery];
		mapStore.setFrom(fromStation?.id);
		mapStore.setTo(toStation?.id);
		[fromName, toName] = [fromStation?.name, toStation?.name];
	}

	function selectItinerary(index: number) {
		itineraryIndex = index;
		const params = new SvelteURLSearchParams(page.url.search);
		params.set('i', String(index));
		replaceState(resolve(`/app?${params}` as `/app?${string}`), page.state);
	}

	function planToHere() {
		sheetSize = 'medium';
		setToHere();
		closeSheet();
		savePlannerUrl();
	}

	function savePlannerUrl() {
		const params = new SvelteURLSearchParams({ panel: 'plan' });
		if (mapStore.fromStationId) params.set('from', mapStore.fromStationId);
		if (mapStore.toStationId) params.set('to', mapStore.toStationId);
		const at = asInstant(plannerTime);
		if (at) {
			params.set(plannerAnchor === 'arrive' ? 'arriveBy' : 'at', at);
		}
		if (plannerModes.size) params.set('modes', [...plannerModes].sort().join(','));
		if (plannerMaxWalk) params.set('maxWalkM', plannerMaxWalk);
		if (plannerMaxTransfers) params.set('maxTransfers', plannerMaxTransfers);
		if (plannerStepFree) params.set('stepFree', 'true');
		replaceState(resolve(`/app?${params}` as `/app?${string}`), page.state);
	}

	function toggleRouteCatalog() {
		const params = new SvelteURLSearchParams(page.url.search);
		if (routeCatalogOpen) {
			params.delete('panel');
			routeCatalogOpen = false;
		} else {
			params.set('panel', 'routes');
			routeCatalogOpen = true;
			plannerOpen = false;
			sheetSize = 'medium';
		}
		replaceState(
			params.size ? resolve(`/app?${params}` as `/app?${string}`) : resolve('/app'),
			page.state
		);
	}

	function closePlanner() {
		clearRouteChips(true);
		plannerOpen = false;
	}

	function retryMap() {
		mapFailed = false;
		firstLoad = true;
		error = '';
		mapAttempt += 1;
	}

	// Integration-map assets are authored static files — loaded once, not
	// viewport-scoped like the geographic station fetch.
	let integrationManifest = $state<IntegrationManifest | undefined>();
	let integrationPoints = $state<SchematicPoint[]>([]);
	let integrationLines = $state<SchematicLine[]>([]);
	let integrationLabels = $state<SchematicLabelPoint[]>([]);
	let integrationFailed = $state(false);

	const integrationTiles = $derived(
		integrationManifest ? buildIntegrationTiles(integrationManifest) : []
	);

	// Route catalog maps canonical UUIDs to the traced artwork keys
	// ("TJ:7F") — needed in both directions: opening a line isolates the map,
	// tapping a corridor stroke opens the line.
	const routesByKey = new SvelteMap<string, RouteSummary>();
	const keyByRouteId = new SvelteMap<string, string>();
	let routeCatalog = $state<RouteSummary[]>([]);
	let routeCatalogQuery = $state('');
	let routeCatalogLoading = $state(true);
	let routeCatalogFailed = $state(false);
	let routeCatalogOpen = $state(false);
	const filteredRoutes = $derived(
		routeCatalog.filter((route) =>
			`${route.shortName ?? ''} ${route.longName ?? ''} ${route.agencyName ?? ''}`
				.toLowerCase()
				.includes(routeCatalogQuery.trim().toLowerCase())
		)
	);

	// Station provider keys ("TJ-H00093P") -> the traced lines passing through
	// them, for the per-stop transfer chips on the line sheet.
	const linesByStationKey = $derived.by(() => {
		const m = new SvelteMap<string, SchematicLine[]>();
		for (const l of integrationLines) {
			for (const seg of l.segments) {
				for (const mk of seg.markers) {
					const arr = m.get(mk);
					if (arr) arr.push(l);
					else m.set(mk, [l]);
				}
			}
		}
		return m;
	});

	const selectedLineKey = $derived(
		mapStore.selectedLineId ? keyByRouteId.get(mapStore.selectedLineId) : undefined
	);

	function resolvePlannerStation(which: 'from' | 'to', id: string) {
		void (async () => {
			try {
				const d = await unwrap(api.GET('/api/v1/stations/{id}', { params: { path: { id } } }));
				if (which === 'from' && mapStore.fromStationId === id) {
					fromStation = d.station;
					fromQuery = fromName = d.station.name;
				} else if (which === 'to' && mapStore.toStationId === id) {
					toStation = d.station;
					toQuery = toName = d.station.name;
				}
			} catch {
				/* A missing API leaves the shared ID intact and the field editable. */
			}
		})();
	}

	$effect(() => {
		const params = new SvelteURLSearchParams(page.url.search);
		const panel = params.get('panel');
		const fromId = params.get('from');
		const toId = params.get('to');
		plannerOpen = panel === 'plan' || panel === 'journey' || (!panel && (!!fromId || !!toId));
		routeCatalogOpen = panel === 'routes';
		plannerAnchor = params.has('arriveBy') ? 'arrive' : 'depart';
		const at = params.get(plannerAnchor === 'arrive' ? 'arriveBy' : 'at');
		if (at) {
			const date = new Date(at);
			if (!Number.isNaN(date.getTime())) {
				const pad = (n: number) => String(n).padStart(2, '0');
				plannerTime = `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}T${pad(date.getHours())}:${pad(date.getMinutes())}`;
			}
		} else plannerTime = '';
		const modes = (params.get('modes') ?? '').split(',').filter(Boolean);
		const currentModes = untrack(() => [...plannerModes]);
		if (
			modes.length !== currentModes.length ||
			modes.some((mode) => !currentModes.includes(mode))
		) {
			plannerModes.clear();
			for (const mode of modes) plannerModes.add(mode);
		}
		plannerMaxWalk = params.get('maxWalkM') ?? '';
		plannerMaxTransfers = params.get('maxTransfers') ?? '';
		plannerStepFree = params.get('stepFree') === 'true' || params.get('stepFree') === '1';
		itineraryIndex = Math.max(0, Number(params.get('i') ?? '0') || 0);
		if (fromId) {
			if (fromId !== untrack(() => mapStore.fromStationId)) mapStore.setFrom(fromId);
			if (fromId !== untrack(() => fromStation?.id)) resolvePlannerStation('from', fromId);
		}
		if (toId) {
			if (toId !== untrack(() => mapStore.toStationId)) mapStore.setTo(toId);
			if (toId !== untrack(() => toStation?.id)) resolvePlannerStation('to', toId);
		}
		const stationId = params.get('station');
		const lineId = params.get('route') ?? params.get('line');
		if (stationId && stationId !== untrack(() => mapStore.selectedStationId)) {
			untrack(() => openStation(stationId));
		} else if (lineId && lineId !== untrack(() => mapStore.selectedLineId)) {
			untrack(() => openLineId(lineId));
		}
	});

	onMount(() => {
		void (async () => {
			try {
				const { manifest, points, lines: traced, labels } = await loadIntegrationMap();
				integrationPoints = points;
				integrationLines = traced;
				integrationLabels = labels;
				integrationManifest = manifest;
			} catch {
				integrationFailed = true;
			}
		})();
		void (async () => {
			try {
				const data = await unwrap(api.GET('/api/v1/routes', { params: { query: { limit: 500 } } }));
				routeCatalog = data.routes;
				for (const r of data.routes) {
					if (!r.agencyCode || !r.shortName) continue;
					const key = `${r.agencyCode}:${r.shortName}`;
					routesByKey.set(key, r);
					keyByRouteId.set(r.id, key);
				}
			} catch {
				routeCatalogFailed = true;
			} finally {
				routeCatalogLoading = false;
			}
		})();
	});

	let timer: ReturnType<typeof setTimeout>;
	let viewportRequest = 0;
	function onViewportChange(bbox: [number, number, number, number]) {
		const requestId = ++viewportRequest;
		clearTimeout(timer);
		timer = setTimeout(() => {
			const bboxStr = bbox.map((n) => n.toFixed(5)).join(',');
			void (async () => {
				try {
					const data = await unwrap(
						api.GET('/api/v1/stations', { params: { query: { bbox: bboxStr, limit: 500 } } })
					);
					if (requestId !== viewportRequest) return;
					stations = data.stations;
					error = '';
				} catch (e) {
					if (requestId === viewportRequest)
						error = displayError(e, 'Data stasiun gagal dimuat. Periksa koneksi lalu coba lagi.');
				} finally {
					if (requestId === viewportRequest) firstLoad = false;
				}
			})();
			// Route lines degrade independently — a lines outage keeps the last
			// frame's geometry rather than hiding the station dots.
			void (async () => {
				try {
					const data = await unwrap(
						api.GET('/api/v1/map/lines', { params: { query: { bbox: bboxStr } } })
					);
					if (requestId === viewportRequest) lines = data.lines;
				} catch {
					/* decorative layer — degrade silently */
				}
			})();
		}, 250);
	}

	function openStation(id: string) {
		routeCatalogOpen = false;
		mapStore.selectLine(); // one detail surface at a time
		if (mapStore.selectedStationId === id) return;
		sheetSize = 'medium';
		mapStore.selectStation(id);
		detail = null;
		board = null;
		detailError = '';
		boardError = '';
		places = null;
		boardExpanded = false;
		boardLoading = true;
		placesLoading = true;
		void tick().then(() => sheetEl?.focus());
		void (async () => {
			try {
				const d = await unwrap(api.GET('/api/v1/stations/{id}', { params: { path: { id } } }));
				if (mapStore.selectedStationId === id) {
					detail = d.station;
					focusPoint = [d.station.lon, d.station.lat];
				}
			} catch (e) {
				if (mapStore.selectedStationId === id)
					detailError = displayError(e, 'Detail stasiun gagal dimuat.');
			}
		})();
		void (async () => {
			try {
				const d = await unwrap(
					api.GET('/api/v1/stations/{id}/departures', {
						params: { path: { id }, query: { window: 1440 } }
					})
				);
				if (mapStore.selectedStationId === id) board = d.departures;
			} catch (e) {
				if (mapStore.selectedStationId === id)
					boardError = displayError(e, 'Jadwal sedang tidak tersedia.');
			} finally {
				if (mapStore.selectedStationId === id) boardLoading = false;
			}
		})();
		void (async () => {
			try {
				const d = await unwrap(
					api.GET('/api/v1/places', { params: { query: { near_stop_id: id, limit: 50 } } })
				);
				if (mapStore.selectedStationId === id) places = d.places;
			} catch {
				if (mapStore.selectedStationId === id) places = null;
			} finally {
				if (mapStore.selectedStationId === id) placesLoading = false;
			}
		})();
	}

	function closeSheet() {
		mapStore.selectStation();
		mapStore.selectLine();
	}

	// Line selection reuses the sheet surface — the route's own stop list is
	// its detail; tapping a stop jumps to that station.
	function openLine(id: string) {
		routeCatalogOpen = false;
		if (mapStore.selectedLineId === id) return;
		openLineId(id);
	}

	// A corridor tap resolves a traced artwork key ("TJ:7F") to its route.
	function openLineByKey(key: string) {
		const r = routesByKey.get(key);
		if (r) openLineId(r.id);
	}

	function openLineId(id: string) {
		routeCatalogOpen = false;
		sheetSize = 'medium';
		mapStore.selectStation();
		mapStore.selectLine(id);
		lineDetail = null;
		lineError = '';
		void tick().then(() => sheetEl?.focus());
		void (async () => {
			try {
				const d = await unwrap(api.GET('/api/v1/routes/{id}', { params: { path: { id } } }));
				if (mapStore.selectedLineId === id) lineDetail = d.route;
			} catch (e) {
				if (mapStore.selectedLineId === id) lineError = displayError(e, 'Detail lin gagal dimuat.');
			}
		})();
	}

	function setFromHere() {
		if (!detail) return;
		mapStore.setFrom(detail.id);
		fromName = detail.name;
		fromQuery = detail.name;
		fromStation = detail;
		savePlannerUrl();
		closeSheet();
	}
	function setToHere() {
		if (!detail) return;
		mapStore.setTo(detail.id);
		toName = detail.name;
		toQuery = detail.name;
		toStation = detail;
		savePlannerUrl();
		closeSheet();
	}
	function clearRouteChips(closePlanner = false) {
		mapStore.clearRoute();
		fromName = undefined;
		toName = undefined;
		fromStation = null;
		toStation = null;
		fromQuery = '';
		toQuery = '';
		const params = new SvelteURLSearchParams(page.url.search);
		for (const key of [
			'from',
			'to',
			'at',
			'arriveBy',
			'modes',
			'maxWalkM',
			'maxTransfers',
			'stepFree',
			'i'
		]) {
			params.delete(key);
		}
		if (closePlanner) params.delete('panel');
		replaceState(
			params.size ? resolve(`/app?${params}` as `/app?${string}`) : resolve('/app'),
			page.state
		);
	}
</script>

<svelte:window
	onkeydown={(e) => {
		if (e.key !== 'Escape') return;
		const panel = page.url.searchParams.get('panel');
		if (panel === 'explore' || panel === 'passport') {
			const params = new SvelteURLSearchParams(page.url.search);
			params.delete('panel');
			replaceState(
				params.size ? resolve(`/app?${params}` as `/app?${string}`) : resolve('/app'),
				page.state
			);
		} else if (mapStore.selectedStationId || mapStore.selectedLineId) closeSheet();
		else if (routeCatalogOpen) toggleRouteCatalog();
		else if (plannerOpen) closePlanner();
		else if (mapStore.fromStationId || mapStore.toStationId) clearRouteChips();
	}}
/>

<h1 class="sg-sr-only">Peta jaringan</h1>

<p class="sg-meta sg-sr-only">
	Stasiun dimuat mengikuti area yang terlihat. Klik titik untuk melihat jadwal stasiun.
</p>

<div class="map-wrap">
	<!-- Chrome overlays the fullscreen map (blueprint §30): back link,
	     mode-independent search, and the from/to chip bar. -->
	<div class="map-chrome">
		<div class="map-chrome__search">
			<MapSearch onStation={(s) => openStation(s.id)} onLine={(l) => openLine(l.id)} />
		</div>
		{#if !plannerOpen}
			<button
				class="chrome-action"
				type="button"
				aria-expanded={routeCatalogOpen}
				onclick={toggleRouteCatalog}
			>
				{routeCatalogOpen ? 'Tutup rute' : 'Semua rute'}
			</button>
		{/if}
		{#if mapStore.fromStationId || mapStore.toStationId}
			<div class="routebar" aria-label="Asal dan tujuan">
				<span class="routebar__leg">Dari: <strong>{fromName ?? '—'}</strong></span>
				<span class="routebar__leg">Ke: <strong>{toName ?? '—'}</strong></span>
				{#if mapStore.fromStationId && mapStore.toStationId}
					<button class="routebar__go" type="button" onclick={savePlannerUrl}>Rencanakan</button>
				{/if}
				<IconButton label="Hapus asal dan tujuan" onclick={() => clearRouteChips()}>
					<svg
						viewBox="0 0 24 24"
						width="18"
						height="18"
						fill="none"
						stroke="currentColor"
						stroke-width="2"
						stroke-linecap="round"
						aria-hidden="true"><path d="M18 6L6 18M6 6l12 12" /></svg
					>
				</IconButton>
			</div>
		{/if}
		{#if browser && mapStore.mode === 'geographic'}
			<label class="lines-toggle">
				<input type="checkbox" bind:checked={linesVisible} />
				Garis rute
			</label>
		{/if}
		{#if !mapFailed && firstLoad}
			<div class="map-note">
				<StateBlock kind="loading">Memuat stasiun…</StateBlock>
			</div>
		{/if}
		{#if !mapFailed && error}
			<div class="map-note map-note--interactive">
				<StateBlock kind="error">{error}</StateBlock>
				<button class="action" type="button" onclick={retryMap}>Coba muat ulang</button>
			</div>
		{/if}
		{#if mapFailed && mapStore.mode === 'geographic'}
			<div class="map-note map-note--interactive" role="alert">
				<StateBlock kind="error"
					>Peta jalan belum tersedia. Periksa koneksi lalu coba lagi.</StateBlock
				>
				<div class="map-recovery">
					<button class="action action--primary" type="button" onclick={retryMap}>Coba lagi</button>
					<button class="action" type="button" onclick={() => mapStore.setMode('integration')}
						>Buka peta integrasi</button
					>
				</div>
			</div>
		{/if}
	</div>

	{#if browser}
		<!-- Both panes stay mounted across mode switches — hiding, not
		     recreating, keeps each map's camera and loaded tiles. -->
		<div class="map-pane" hidden={mapStore.mode !== 'geographic'}>
			{#key mapAttempt}
				<TransitMap
					styleUrl={STYLE_URL}
					data={stationsToGeoJSON(stations)}
					{lines}
					{linesVisible}
					bottomPadding={mapBottomPadding}
					{onViewportChange}
					focus={focusPoint}
					route={journeyRoute}
					onSelect={openStation}
					onSelectLine={openLine}
					onFailed={() => (mapFailed = true)}
				/>
			{/key}
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
					lineKey={selectedLineKey}
					markers={integrationManifest.markers ?? 'overlay'}
					attribution={integrationManifest.attribution}
					selectedId={mapStore.selectedStationId}
					visible={mapStore.mode === 'integration'}
					onSelect={openStation}
					onLineSelect={openLineByKey}
					bind:camera={mapStore.integrationCamera}
				/>
			{:else}
				<div class="im-fallback">
					<StateBlock kind="loading">Memuat peta integrasi…</StateBlock>
				</div>
			{/if}
		</div>
		<MapModeSwitcher />
	{/if}

	{#if routeCatalogOpen && !mapStore.selectedStationId && !mapStore.selectedLineId}
		<aside
			class="sheet"
			class:sheet--peek={sheetSize === 'peek'}
			class:sheet--expanded={sheetSize === 'expanded'}
			aria-label="Daftar rute"
			bind:this={sheetEl}
		>
			<header class="sheet-head">
				<h2>Semua rute</h2>
				<SheetSizeControls size={sheetSize} onChange={(size) => (sheetSize = size)} />
				<IconButton label="Tutup daftar rute" onclick={toggleRouteCatalog}>
					<svg
						viewBox="0 0 24 24"
						width="20"
						height="20"
						fill="none"
						stroke="currentColor"
						stroke-width="2"
						stroke-linecap="round"
						aria-hidden="true"><path d="M18 6L6 18M6 6l12 12" /></svg
					>
				</IconButton>
			</header>
			<label class="catalog-search-label" for="route-filter">Cari rute atau operator</label>
			<input
				class="catalog-search"
				id="route-filter"
				type="search"
				placeholder="Contoh: KRL, MRT…"
				bind:value={routeCatalogQuery}
			/>
			{#if routeCatalogLoading}
				<StateBlock kind="loading">Memuat daftar rute…</StateBlock>
			{:else if routeCatalogFailed}
				<StateBlock kind="error">Daftar rute belum tersedia. Periksa koneksi API.</StateBlock>
			{:else if filteredRoutes.length === 0}
				<StateBlock kind="empty">Tidak ada rute yang cocok.</StateBlock>
			{:else}
				<ul class="catalog-list">
					{#each filteredRoutes as route (route.id)}
						<li>
							<button class="catalog-route" type="button" onclick={() => openLine(route.id)}>
								<span
									class="catalog-badge"
									style:background-color={route.color ? `#${route.color}` : null}
									style:color={route.color ? '#fff' : null}>{route.shortName ?? '—'}</span
								>
								<span class="catalog-copy"
									><strong>{route.longName ?? route.shortName}</strong><span class="sg-meta"
										>{route.agencyName ?? route.agencyCode}</span
									></span
								>
							</button>
						</li>
					{/each}
				</ul>
			{/if}
		</aside>
	{/if}

	{#if mapStore.selectedStationId}
		<aside
			class="sheet"
			class:sheet--peek={sheetSize === 'peek'}
			class:sheet--expanded={sheetSize === 'expanded'}
			aria-label="Detail stasiun"
			tabindex="-1"
			bind:this={sheetEl}
		>
			<header class="sheet-head">
				{#if detail}
					{#if detail.code}<span class="code-badge">{detail.code}</span>{/if}
					<h2>{detail.name}</h2>
				{:else}
					{@const s = stations.find((x) => x.id === mapStore.selectedStationId)}
					<h2>{s?.name ?? 'Stasiun'}</h2>
				{/if}
				<SheetSizeControls size={sheetSize} onChange={(size) => (sheetSize = size)} />
				<IconButton label="Tutup panel stasiun" onclick={closeSheet}>
					<svg
						viewBox="0 0 24 24"
						width="20"
						height="20"
						fill="none"
						stroke="currentColor"
						stroke-width="2"
						stroke-linecap="round"
						aria-hidden="true"><path d="M18 6L6 18M6 6l12 12" /></svg
					>
				</IconButton>
			</header>
			{#if detail}
				{#if detail.officialName && detail.officialName !== detail.name}
					<p class="sg-meta station-official-name">{detail.officialName}</p>
				{/if}
				<p class="sg-meta station-coordinates">
					{#if detail.code}{detail.code} ·
					{/if}{detail.kind} · {detail.lat.toFixed(5)}, {detail.lon.toFixed(5)}
				</p>
			{/if}

			{#if detailError}
				<StateBlock kind="error">{detailError}</StateBlock>
			{:else if !detail}
				<StateBlock kind="loading">Memuat detail stasiun…</StateBlock>
			{:else}
				<div class="sheet-actions">
					<a
						class="action action--primary"
						onclick={(e) => {
							e.preventDefault();
							planToHere();
						}}
						href={resolve(
							mapStore.fromStationId
								? `/plan?from=${mapStore.fromStationId}&to=${detail.id}`
								: `/plan?to=${detail.id}`
						)}>Rute ke sini</a
					>
					<button class="action" type="button" onclick={() => (boardExpanded = !boardExpanded)}>
						{boardExpanded ? 'Jadwal ringkas' : 'Jadwal lengkap'}
					</button>
				</div>
				<div class="sheet-actions sheet-actions--origin">
					<button
						class="action"
						type="button"
						aria-pressed={mapStore.fromStationId === detail.id}
						onclick={setFromHere}>Dari sini</button
					>
					<button
						class="action"
						type="button"
						aria-pressed={mapStore.toStationId === detail.id}
						onclick={setToHere}>Ke sini</button
					>
				</div>
			{/if}

			<section class="sheet-section" aria-label="Keberangkatan">
				{#if boardError}
					<StateBlock kind="error">{boardError}</StateBlock>
				{:else if boardLoading}
					<StateBlock kind="loading">Memuat jadwal…</StateBlock>
				{:else if board}
					{#if board.lines.length === 0}
						<StateBlock kind="empty"
							>Tidak ada keberangkatan terjadwal dalam {Math.round(board.windowMinutes / 60)} jam ke depan.</StateBlock
						>
					{:else}
						<DepartureBoard lines={board.lines} compact={!boardExpanded} />
					{/if}
					<p class="sg-meta board-note">
						Jadwal statis {board.source.provider} — bukan posisi live.
					</p>
				{/if}
			</section>

			{#if detail}
				<section class="sheet-section" aria-label="Fasilitas">
					<h3 class="sheet-heading">Fasilitas</h3>
					{#if detail.facilities.length === 0}
						<p class="sg-meta">Tidak ada data fasilitas untuk stasiun ini.</p>
					{:else}
						<ul class="sheet-list">
							{#each detail.facilities as f (f.type + f.text)}
								<li>
									{facilityLabel(f.type)}{#if f.text}
										· {f.text}{/if}
								</li>
							{/each}
						</ul>
					{/if}
				</section>

				<section class="sheet-section" aria-label="Koridor">
					<h3 class="sheet-heading">Koridor</h3>
					<p class="sg-meta">
						{detail.lines.length} rute melayani · {detail.transfers.length} transfer tercatat
					</p>
					{#if detail.lines.length > 0}
						<ul class="corridor-list">
							{#each detail.lines as l (l.id)}
								<li>
									<button type="button" class="corridor" onclick={() => openLineId(l.id)}>
										<span
											class="line-badge"
											style:background-color={l.color ? `#${l.color}` : null}
											style:color={l.color ? '#fff' : null}>{l.shortName ?? '—'}</span
										>
										{l.longName ?? l.agencyName ?? ''}
									</button>
								</li>
							{/each}
						</ul>
					{/if}
				</section>

				<section class="sheet-section" aria-label="Transfer">
					<h3 class="sheet-heading">Transfer</h3>
					{#if detail.transfers.length === 0}
						<p class="sg-meta">Tidak ada transfer tercatat.</p>
					{:else}
						<ul class="sheet-list">
							{#each detail.transfers as transfer (transfer.toStop.id)}
								<li>
									<button
										class="text-action"
										type="button"
										onclick={() => openStation(transfer.toStop.id)}
									>
										{transfer.toStop.name}{#if transfer.walkDistanceM}
											· jalan {transfer.walkDistanceM} m{/if}
									</button>
									{#if transfer.notes}<span class="sg-meta"> · {transfer.notes}</span>{/if}
								</li>
							{/each}
						</ul>
					{/if}
				</section>

				<section class="sheet-section" aria-label="Tempat di sekitar">
					<h3 class="sheet-heading">Tempat di sekitar</h3>
					{#if placesLoading}
						<StateBlock kind="loading">Memuat tempat sekitar…</StateBlock>
					{:else}
						<NearbyPlaces {places} />
					{/if}
				</section>

				<section class="sheet-section" aria-label="Catatan perjalanan">
					<h3 class="sheet-heading">Catatan perjalanan</h3>
					<CheckinButton stopId={detail.id} />
					<JournalNoteForm stopId={detail.id} />
				</section>
				<p class="sg-meta board-note">
					Sumber: {detail.source.provider}{#if detail.source.fetchedAt}
						· diambil {new Date(detail.source.fetchedAt).toLocaleString('id-ID')}{/if}
				</p>

				<a
					class="gmaps sg-meta"
					href="https://www.google.com/maps/search/?api=1&query={detail.lat},{detail.lon}"
					target="_blank"
					rel="noopener noreferrer">Buka di Google Maps ↗</a
				>
			{/if}
		</aside>
	{/if}

	{#if mapStore.selectedLineId}
		<aside
			class="sheet"
			class:sheet--peek={sheetSize === 'peek'}
			class:sheet--expanded={sheetSize === 'expanded'}
			aria-label="Detail lin"
			tabindex="-1"
			bind:this={sheetEl}
		>
			<header class="sheet-head">
				{#if lineDetail}
					<span
						class="line-badge"
						style:background-color={lineDetail.color ? `#${lineDetail.color}` : null}
						style:color={lineDetail.color ? '#fff' : null}>{lineDetail.shortName ?? '—'}</span
					>
					<h2>{lineDetail.longName ?? lineDetail.agencyName ?? 'Lin'}</h2>
				{:else}
					<h2>Lin</h2>
				{/if}
				<SheetSizeControls size={sheetSize} onChange={(size) => (sheetSize = size)} />
				<IconButton label="Tutup panel lin" onclick={closeSheet}>
					<svg
						viewBox="0 0 24 24"
						width="20"
						height="20"
						fill="none"
						stroke="currentColor"
						stroke-width="2"
						stroke-linecap="round"
						aria-hidden="true"><path d="M18 6L6 18M6 6l12 12" /></svg
					>
				</IconButton>
			</header>

			{#if lineError}
				<StateBlock kind="error">{lineError}</StateBlock>
			{:else if !lineDetail}
				<StateBlock kind="loading">Memuat detail lin…</StateBlock>
			{:else}
				<p class="sg-meta">{lineDetail.agencyName}</p>
				<section class="sheet-section" aria-label="Urutan stasiun">
					{#if lineDetail.stops.length === 0}
						<p class="sg-meta">Urutan stasiun lin ini belum tersedia.</p>
					{:else}
						<ol class="sheet-list line-stops">
							{#each lineDetail.stops as stop (stop.id + stop.seq)}
								{@const otherLines = stop.code
									? (linesByStationKey
											.get(`${lineDetail.agencyCode}-${stop.code}`)
											?.filter((l) => l.key !== selectedLineKey) ?? [])
									: []}
								<li>
									<button type="button" class="line-stop" onclick={() => openStation(stop.id)}>
										<span class="line-stop__seq">{stop.stationNumber ?? stop.seq}</span>
										{stop.name}
									</button>
									{#if otherLines.length > 0}
										<span class="line-xfers">
											{#each otherLines as l (l.key)}
												{@const r = routesByKey.get(l.key)}
												<button
													type="button"
													class="xfer-chip"
													style:background-color={l.color}
													style:color="#fff"
													title={l.name}
													aria-label="Lin {l.code}: {l.name}"
													onclick={() => r && openLineId(r.id)}>{l.code}</button
												>
											{/each}
										</span>
									{/if}
								</li>
							{/each}
						</ol>
					{/if}
				</section>
				<p class="sg-meta board-note">
					Data {lineDetail.source.provider} — urutan mengikuti data provider.
				</p>
			{/if}
		</aside>
	{/if}

	<!-- Journey surface shares the sheet slot — it yields to a station/line
	     detail and comes back when that sheet closes. -->
	{#if plannerOpen && !mapStore.selectedStationId && !mapStore.selectedLineId}
		<aside
			class="sheet sheet--planner"
			class:sheet--peek={sheetSize === 'peek'}
			class:sheet--expanded={sheetSize === 'expanded'}
			aria-label="Rencana perjalanan"
			tabindex="-1"
			bind:this={sheetEl}
		>
			<header class="sheet-head">
				<h2>Rencanakan perjalanan</h2>
				<SheetSizeControls size={sheetSize} onChange={(size) => (sheetSize = size)} />
				<IconButton label="Hapus rencana" onclick={closePlanner}>
					<svg
						viewBox="0 0 24 24"
						width="20"
						height="20"
						fill="none"
						stroke="currentColor"
						stroke-width="2"
						stroke-linecap="round"
						aria-hidden="true"><path d="M18 6L6 18M6 6l12 12" /></svg
					>
				</IconButton>
			</header>
			<button class="text-action planner-routes-action" type="button" onclick={toggleRouteCatalog}>
				Lihat semua rute
			</button>

			<form
				class="planner"
				onsubmit={(e) => {
					e.preventDefault();
					savePlannerUrl();
				}}
			>
				<div class="planner-stations">
					<div class="planner-field">
						<label for="map-from">Dari</label>
						<StationCombobox
							id="map-from"
							label="Stasiun asal"
							placeholder="Nama stasiun…"
							bind:value={fromQuery}
							bind:selected={fromStation}
							onpick={(s) => {
								mapStore.setFrom(s.id);
								fromName = s.name;
							}}
							onclear={() => {
								mapStore.setFrom();
								fromName = undefined;
							}}
						/>
					</div>
					<button
						class="swap"
						type="button"
						aria-label="Tukar stasiun asal dan tujuan"
						onclick={swapStations}
					>
						<svg
							viewBox="0 0 24 24"
							width="18"
							height="18"
							fill="none"
							stroke="currentColor"
							stroke-width="1.8"
							stroke-linecap="round"
							stroke-linejoin="round"
							aria-hidden="true"
						>
							<path d="M8 4v14m0-14-3 3m3-3 3 3M16 20V6m0 14 3-3m-3 3-3-3" />
						</svg>
					</button>
					<div class="planner-field">
						<label for="map-to">Ke</label>
						<StationCombobox
							id="map-to"
							label="Stasiun tujuan"
							placeholder="Nama stasiun…"
							bind:value={toQuery}
							bind:selected={toStation}
							onpick={(s) => {
								mapStore.setTo(s.id);
								toName = s.name;
							}}
							onclear={() => {
								mapStore.setTo();
								toName = undefined;
							}}
						/>
					</div>
				</div>
				<div class="planner-time-field">
					<div class="planner-anchor" role="group" aria-label="Patokan waktu">
						<button
							type="button"
							class:planner-anchor--on={plannerAnchor === 'depart'}
							aria-pressed={plannerAnchor === 'depart'}
							onclick={() => (plannerAnchor = 'depart')}>Berangkat</button
						>
						<button
							type="button"
							class:planner-anchor--on={plannerAnchor === 'arrive'}
							aria-pressed={plannerAnchor === 'arrive'}
							onclick={() => (plannerAnchor = 'arrive')}>Tiba sebelum</button
						>
					</div>
					<label class="planner-label" for="map-time"
						>{plannerAnchor === 'depart' ? 'Waktu berangkat' : 'Batas waktu tiba'}</label
					>
					<input
						class="planner-time"
						id="map-time"
						type="datetime-local"
						bind:value={plannerTime}
					/>
				</div>
				<section class="planner-preferences" aria-labelledby="planner-preferences-title">
					<h3 class="planner-section-title" id="planner-preferences-title">
						Preferensi perjalanan
					</h3>
					<fieldset class="planner-modes">
						<legend>Moda transportasi</legend>
						<div class="mode-chips">
							{#each MODE_OPTIONS as [value, label] (value)}
								<button
									type="button"
									class:mode-chip--on={plannerModes.has(value)}
									aria-pressed={plannerModes.has(value)}
									onclick={() => toggleMode(value)}>{label}</button
								>
							{/each}
						</div>
					</fieldset>
					<div class="planner-limits">
						<label
							>Jalan maks.<select bind:value={plannerMaxWalk}
								><option value="">Bebas</option><option value="200">200 m</option><option
									value="500">500 m</option
								><option value="1000">1 km</option></select
							></label
						>
						<label
							>Transit maks.<select bind:value={plannerMaxTransfers}
								><option value="">Bebas</option><option value="0">Tanpa transit</option><option
									value="1">1</option
								><option value="2">2</option><option value="3">3</option></select
							></label
						>
					</div>
					<label class="step-free"
						><input type="checkbox" bind:checked={plannerStepFree} /> Bebas tangga (lift)</label
					>
				</section>
				<button
					class="action action--primary planner-submit"
					type="submit"
					disabled={!fromStation || !toStation || journeyLoading}
				>
					{journeyLoading ? 'Mencari rute…' : 'Cari rute'}
				</button>
			</form>

			{#if journeyLoading}
				<StateBlock kind="loading">Mencari rute…</StateBlock>
			{:else if journeyError}
				<StateBlock kind="error">{journeyError}</StateBlock>
			{:else if journey && selectedItinerary}
				{@const itin = selectedItinerary}
				{#if journey && journey.itineraries.length > 1}
					<div class="itinerary-switcher" role="group" aria-label="Pilihan rute">
						{#each journey.itineraries as other, i (i)}
							<button
								type="button"
								aria-pressed={i === itineraryIndex}
								class:itinerary-switcher--on={i === itineraryIndex}
								onclick={() => selectItinerary(i)}
							>
								{other.label.replaceAll('_', ' ')} · {Math.round(other.durationSec / 60)} mnt
							</button>
						{/each}
					</div>
				{/if}
				<header class="journey-head">
					<StatusBadge status={itin.status} />
					{#if journey.fareReference?.total != null}
						<span class="fare">Rp{journey.fareReference.total.toLocaleString('id-ID')}</span>
					{/if}
					<span class="sg-meta">
						{Math.floor(itin.durationSec / 60)} mnt · {itin.rideLegs} naik
						{#if itin.transfers > 0}· {itin.transfers} transit{/if}
					</span>
				</header>

				<ol class="journey-legs">
					{#each itin.legs as leg, i (i)}
						<li class="jleg">
							{#if leg.type === 'walk'}
								<span class="jleg-icon" aria-hidden="true">↔</span>
								<span>
									Jalan ke {leg.to.name}
									{#if leg.distanceM}<span class="sg-meta">
											· {Math.round(leg.distanceM)} m</span
										>{/if}
								</span>
							{:else}
								<span class="jleg-icon" aria-hidden="true">●</span>
								<span class="jleg-body">
									<strong>{leg.line}</strong> arah {leg.headsign || leg.to.name}
									<span class="sg-meta">
										· {leg.stationCount} perhentian
										{#if leg.distanceM}· {(leg.distanceM / 1000).toFixed(1)} km{/if}
									</span>
									{#if leg.nextDepartures && leg.nextDepartures.length > 0}
										<span class="jleg-deps">
											Berangkat {leg.nextDepartures.map((d) => d.time).join(' · ')}
										</span>
									{/if}
									{#if leg.stops && leg.stops.length > 2}
										<details>
											<summary>{leg.stops.length} stasiun dilewati</summary>
											<ul>
												{#each leg.stops as stop, j (j)}
													<li>
														{#if stop.id}
															{@const sid = stop.id}
															<button
																type="button"
																class="stop-link"
																onclick={() => openStation(sid)}>{stop.name}</button
															>
														{:else}
															{stop.name}
														{/if}
													</li>
												{/each}
											</ul>
										</details>
									{/if}
									{#if leg.alternatives && leg.alternatives.length > 0}
										<ul class="jleg-alts">
											{#each leg.alternatives as alt (alt.routeId)}
												<li>
													Bisa juga naik
													<strong>{alt.shortName ?? alt.line}</strong>{#if alt.name}
														— {alt.name}{/if}
													<span class="sg-meta"> · {alt.stationCount} perhentian</span>
												</li>
											{/each}
										</ul>
									{/if}
								</span>
							{/if}
						</li>
					{/each}
				</ol>

				{#if journey.fareReference && journey.fareReference.segments.length > 0}
					<details class="fare-detail">
						<summary>Estimasi tarif koridor (referensi)</summary>
						<ul>
							{#each journey.fareReference.segments as seg, i (i)}
								<li>
									{seg.operator}: {seg.from.name} → {seg.to.name} — Rp{seg.amount.toLocaleString(
										'id-ID'
									)}
								</li>
							{/each}
						</ul>
					</details>
				{/if}
				<p class="sg-meta board-note">
					Jadwal statis {journey.source.provider} — bukan posisi live.
				</p>
			{:else if journey}
				<StateBlock kind="empty"
					>Provider tidak menemukan rute antara {journey.from.name} dan {journey.to
						.name}.</StateBlock
				>
			{:else}
				<p class="planner-hint">
					Pilih stasiun asal dan tujuan untuk mencari rute. Hasil perjalanan akan muncul di sini.
				</p>
			{/if}
		</aside>
	{/if}
</div>

<style>
	/* Beranda owns the fullscreen map; sheets stop above its bottom navigation. */
	.map-wrap {
		position: fixed;
		inset: 0;
		overflow: hidden;
		z-index: 1;
	}
	.map-chrome {
		position: absolute;
		top: var(--sg-space-3);
		inset-inline: var(--sg-space-3);
		display: flex;
		flex-wrap: wrap;
		align-items: flex-start;
		gap: var(--sg-space-2);
		z-index: 2;
	}
	/* BackNav renders a flow element — restyle it as a floating chip. */
	.map-chrome :global(.back-nav) {
		margin: 0;
	}
	.map-chrome :global(.back-nav a) {
		padding: 0 var(--sg-space-3);
		background-color: var(--sg-surface);
		border: 1px solid var(--sg-border);
		border-radius: var(--sg-radius-button);
		box-shadow: var(--sg-shadow-overlay);
	}
	.map-chrome__search {
		flex: 1;
		min-width: 12rem;
		max-width: 26rem;
	}
	.chrome-action {
		min-height: var(--sg-target-min);
		padding: 0 var(--sg-space-3);
		border: 1px solid var(--sg-border);
		border-radius: var(--sg-radius-button);
		background: var(--sg-surface);
		box-shadow: var(--sg-shadow-overlay);
		color: var(--sg-text);
		font: inherit;
		font-size: var(--sg-text-secondary);
		font-weight: var(--sg-weight-semibold);
		cursor: pointer;
	}
	.map-note {
		flex: 0 0 100%;
		display: flex;
		justify-content: center;
		pointer-events: none;
	}
	.map-note--interactive {
		pointer-events: auto;
		flex-direction: column;
		align-items: center;
		gap: var(--sg-space-2);
		padding: var(--sg-space-2);
		background: color-mix(in srgb, var(--sg-canvas) 92%, transparent);
		border: 1px solid var(--sg-border);
		border-radius: var(--sg-radius-card);
		box-shadow: var(--sg-shadow-overlay);
	}
	.map-recovery {
		display: flex;
		gap: var(--sg-space-2);
	}
	.routebar {
		flex: 0 1 auto;
		min-width: 0;
		display: flex;
		align-items: center;
		gap: var(--sg-space-2);
		min-height: var(--sg-target-min);
		padding: 0 var(--sg-space-2) 0 var(--sg-space-3);
		background-color: var(--sg-surface);
		border: 1px solid var(--sg-border);
		border-radius: var(--sg-radius-input);
		font-size: var(--sg-text-secondary);
	}
	.routebar__leg {
		min-width: 0;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	.routebar__go {
		flex: none;
		align-self: stretch;
		align-content: center;
		padding-inline: var(--sg-space-1);
		font-weight: var(--sg-weight-bold);
		color: var(--sg-brand);
		text-decoration: none;
		white-space: nowrap;
		border: 0;
		background: transparent;
		font: inherit;
		cursor: pointer;
	}
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
	.sheet:focus {
		outline: none;
	}
	@media (prefers-reduced-motion: no-preference) {
		.sheet {
			animation: sg-panel-enter var(--sg-motion-panel) var(--sg-ease-enter) both;
		}
	}
	@media (min-width: 48rem) and (prefers-reduced-motion: no-preference) {
		.sheet {
			animation-name: sg-panel-enter-side;
		}
	}

	/* Keep map controls in the wrapping chrome row so compact screens do not overlap. */
	.lines-toggle {
		flex: none;
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
	}

	/* Compact: bottom sheet over the map (docs/10). */
	.sheet {
		position: absolute;
		inset-inline: 0;
		bottom: calc(var(--nav-h) + env(safe-area-inset-bottom, 0px));
		height: 56dvh;
		max-height: calc(100dvh - var(--nav-h) - env(safe-area-inset-bottom, 0px) - var(--sg-space-3));
		overflow-y: auto;
		box-sizing: border-box;
		padding: var(--sg-space-3) var(--sg-space-4) var(--sg-space-4);
		background-color: var(--sg-surface);
		border-top: 1px solid var(--sg-border);
		border-radius: var(--sg-radius-sheet) var(--sg-radius-sheet) 0 0;
		box-shadow: var(--sg-shadow-sheet);
		z-index: var(--sg-z-sheet);
		transition: height 180ms ease-out;
	}
	.sheet--peek {
		height: 38dvh;
	}
	.sheet--expanded {
		height: 86dvh;
	}
	.sheet-head {
		position: sticky;
		top: 0;
		z-index: 1;
		display: flex;
		align-items: center;
		gap: var(--sg-space-2);
		min-height: 3.5rem;
		margin: calc(0px - var(--sg-space-3)) calc(0px - var(--sg-space-4)) var(--sg-space-2);
		padding: var(--sg-space-3) var(--sg-space-4);
		background: var(--sg-surface);
		border-bottom: 1px solid var(--sg-border);
	}
	.sheet-head h2 {
		flex: 1;
		min-width: 0;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
		margin: 0;
		font-size: var(--sg-text-screen);
		font-weight: var(--sg-weight-regular);
		letter-spacing: -0.03em;
	}
	.sheet--planner .sheet-head {
		padding-block-end: var(--sg-space-2);
		border-bottom: 1px solid var(--sg-border);
	}
	.code-badge {
		flex: none;
		padding: var(--sg-space-1) var(--sg-space-2);
		border-radius: var(--sg-radius-button);
		background-color: var(--sg-surface-muted);
		font-weight: var(--sg-weight-bold);
		font-size: var(--sg-text-secondary);
	}
	.sheet-actions {
		display: flex;
		gap: var(--sg-space-2);
		margin-block: var(--sg-space-3);
	}
	.action {
		flex: 1;
		display: inline-flex;
		align-items: center;
		justify-content: center;
		min-height: var(--sg-target-min);
		padding: 0 var(--sg-space-4);
		border: 1px solid var(--sg-border);
		border-radius: var(--sg-radius-button);
		background-color: var(--sg-surface);
		color: var(--sg-text);
		font: inherit;
		font-weight: var(--sg-weight-bold);
		text-decoration: none;
		cursor: pointer;
	}
	.action[aria-pressed='true'] {
		border-color: var(--sg-brand);
		color: var(--sg-brand);
	}
	.sheet-actions--origin {
		margin-top: calc(-1 * var(--sg-space-2));
	}
	.action--primary {
		background-color: var(--sg-brand);
		border-color: transparent;
		color: var(--sg-brand-contrast);
	}
	.sheet-section {
		margin-top: var(--sg-space-4);
	}
	.planner {
		display: flex;
		flex-direction: column;
		gap: var(--sg-space-3);
		margin-block: var(--sg-space-3);
	}
	.planner-time-field {
		display: grid;
		gap: var(--sg-space-2);
	}
	.planner-time-field .planner-label {
		margin-bottom: 0;
	}
	.planner-stations {
		display: grid;
		grid-template-columns: minmax(0, 1fr) auto;
		align-items: end;
		gap: var(--sg-space-2);
	}
	.planner-field:first-child {
		grid-column: 1;
	}
	.planner-field:last-child {
		grid-column: 1;
	}
	.planner-field label,
	.planner-label {
		display: block;
		margin-bottom: var(--sg-space-1);
		font-size: var(--sg-text-secondary);
		font-weight: var(--sg-weight-semibold);
	}
	.swap {
		grid-column: 2;
		grid-row: 1 / span 2;
		align-self: center;
		width: var(--sg-target-min);
		min-height: var(--sg-target-min);
		border: 1px solid var(--sg-border);
		border-radius: var(--sg-radius-pill);
		background: var(--sg-surface);
		color: var(--sg-brand);
		font-size: var(--sg-text-section);
		cursor: pointer;
	}
	.swap svg {
		display: block;
		margin: auto;
	}
	.swap:hover {
		background: var(--sg-surface-muted);
		border-color: var(--sg-brand);
	}
	.planner-anchor {
		display: inline-flex;
		align-self: flex-start;
		border: 1px solid var(--sg-border);
		border-radius: var(--sg-radius-button);
		overflow: hidden;
	}
	.planner-anchor button {
		min-height: var(--sg-target-min);
		padding-inline: var(--sg-space-3);
		border: 0;
		background: var(--sg-surface);
		color: var(--sg-text);
		font: inherit;
		font-weight: var(--sg-weight-semibold);
		cursor: pointer;
	}
	.planner-anchor .planner-anchor--on {
		background: var(--sg-brand);
		color: var(--sg-brand-contrast);
	}
	.planner-time,
	.planner-limits select {
		min-height: var(--sg-target-min);
		padding-inline: var(--sg-space-2);
		border: 1px solid var(--sg-border);
		border-radius: var(--sg-radius-button);
		background: var(--sg-surface);
		color: var(--sg-text);
		font: inherit;
		box-sizing: border-box;
	}
	.planner-time {
		width: 100%;
	}
	.planner-preferences {
		display: grid;
		gap: var(--sg-space-3);
		padding: var(--sg-space-3);
		border: 1px solid var(--sg-border);
		border-radius: var(--sg-radius-card);
		background: var(--sg-surface-muted);
	}
	.planner-section-title {
		margin: 0;
		font-size: var(--sg-text-body);
		font-weight: var(--sg-weight-semibold);
	}
	.planner-modes {
		margin: 0;
		padding: 0;
		border: 0;
	}
	.planner-modes legend {
		margin-bottom: var(--sg-space-1);
		font-size: var(--sg-text-secondary);
		font-weight: var(--sg-weight-semibold);
	}
	.mode-chips,
	.itinerary-switcher {
		display: flex;
		flex-wrap: wrap;
		gap: var(--sg-space-2);
	}
	.planner-limits {
		display: grid;
		grid-template-columns: repeat(2, minmax(0, 1fr));
		gap: var(--sg-space-2);
	}
	.mode-chips button,
	.itinerary-switcher button {
		min-height: var(--sg-target-min);
		padding-inline: var(--sg-space-3);
		border: 1px solid var(--sg-border);
		border-radius: var(--sg-radius-pill);
		background: var(--sg-surface);
		color: var(--sg-text);
		font: inherit;
		font-size: var(--sg-text-secondary);
		cursor: pointer;
	}
	.mode-chips .mode-chip--on,
	.itinerary-switcher .itinerary-switcher--on {
		border-color: var(--sg-brand);
		background: var(--sg-brand);
		color: var(--sg-brand-contrast);
	}
	.planner-limits label {
		display: flex;
		min-width: 0;
		flex-direction: column;
		gap: var(--sg-space-1);
		font-size: var(--sg-text-secondary);
		font-weight: var(--sg-weight-semibold);
	}
	.planner-limits select {
		width: 100%;
		min-width: 0;
	}
	.step-free {
		display: flex;
		align-items: center;
		gap: var(--sg-space-2);
		min-height: var(--sg-target-min);
		font-size: var(--sg-text-secondary);
	}
	.step-free input {
		width: 1.125rem;
		height: 1.125rem;
		margin: 0;
		accent-color: var(--sg-brand);
	}
	.planner-submit {
		width: 100%;
	}
	.planner-hint {
		margin: var(--sg-space-3) 0 0;
		padding: var(--sg-space-3);
		border-radius: var(--sg-radius-button);
		background: var(--sg-surface-muted);
		color: var(--sg-text-muted);
		font-size: var(--sg-text-secondary);
		line-height: 1.5;
	}
	.planner-submit:disabled {
		opacity: 0.55;
		cursor: not-allowed;
	}
	.itinerary-switcher {
		margin-top: var(--sg-space-3);
	}
	.text-action {
		padding: 0;
		border: 0;
		background: none;
		color: var(--sg-brand);
		font: inherit;
		text-align: start;
		cursor: pointer;
	}
	.planner-routes-action {
		min-height: var(--sg-target-min);
	}
	.catalog-search-label {
		display: block;
		margin-block: var(--sg-space-3) var(--sg-space-1);
		font-size: var(--sg-text-secondary);
		font-weight: var(--sg-weight-semibold);
	}
	.catalog-search {
		width: 100%;
		min-height: var(--sg-target-min);
		padding-inline: var(--sg-space-3);
		border: 1px solid var(--sg-border);
		border-radius: var(--sg-radius-input);
		background: var(--sg-surface);
		color: var(--sg-text);
		font: inherit;
	}
	.catalog-list {
		list-style: none;
		margin: var(--sg-space-3) 0 0;
		padding: 0;
	}
	.catalog-route {
		display: flex;
		align-items: center;
		gap: var(--sg-space-3);
		width: 100%;
		min-height: var(--sg-target-min);
		padding: var(--sg-space-2);
		border: 0;
		border-radius: var(--sg-radius-button);
		background: transparent;
		color: var(--sg-text);
		font: inherit;
		text-align: start;
		cursor: pointer;
	}
	.catalog-route:hover {
		background: var(--sg-surface-muted);
	}
	.catalog-badge {
		flex: none;
		min-width: 2.5rem;
		padding: var(--sg-space-1) var(--sg-space-2);
		border-radius: var(--sg-radius-button);
		background: var(--sg-surface-muted);
		font-weight: var(--sg-weight-bold);
		text-align: center;
	}
	.catalog-copy {
		display: flex;
		flex-direction: column;
		min-width: 0;
	}
	.catalog-copy strong {
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	.sheet-heading {
		margin: 0 0 var(--sg-space-1);
		font-size: var(--sg-text-body);
	}
	.sheet-list {
		margin: 0;
		padding: 0;
		list-style: none;
		font-size: var(--sg-text-secondary);
	}
	.sheet-list li {
		padding-block: var(--sg-space-1);
	}
	.board-note {
		margin-block: var(--sg-space-2) 0;
	}
	.gmaps {
		display: inline-block;
		margin-top: var(--sg-space-4);
		min-height: var(--sg-target-min);
		align-content: center;
	}
	.line-badge {
		flex: none;
		min-width: 2.5rem;
		padding: var(--sg-space-1) var(--sg-space-2);
		border-radius: var(--sg-radius-button);
		background-color: var(--sg-surface-muted);
		font-weight: var(--sg-weight-bold);
		font-size: var(--sg-text-secondary);
		text-align: center;
	}
	.corridor-list {
		margin: var(--sg-space-1) 0 0;
		padding: 0;
		list-style: none;
	}
	.corridor {
		display: flex;
		align-items: center;
		gap: var(--sg-space-2);
		width: 100%;
		min-height: var(--sg-target-min);
		padding: var(--sg-space-1) 0;
		background: none;
		border: none;
		font: inherit;
		font-size: var(--sg-text-secondary);
		color: inherit;
		text-align: left;
		cursor: pointer;
	}
	.corridor:hover {
		color: var(--sg-brand);
	}
	.line-stops {
		max-height: 16rem;
		overflow-y: auto;
	}
	.line-stops li {
		display: flex;
		align-items: baseline;
		gap: var(--sg-space-2);
	}
	.line-xfers {
		flex: none;
		display: inline-flex;
		flex-wrap: wrap;
		gap: 0.25rem;
		justify-content: flex-end;
		margin-inline-start: auto;
	}
	.xfer-chip {
		padding: 0.1rem 0.4rem;
		border: none;
		border-radius: var(--sg-radius-button);
		font: inherit;
		font-size: var(--sg-text-caption);
		font-weight: var(--sg-weight-bold);
		cursor: pointer;
	}
	.line-stop {
		display: flex;
		align-items: center;
		gap: var(--sg-space-2);
		width: 100%;
		min-height: var(--sg-target-min);
		padding: var(--sg-space-1) 0;
		background: none;
		border: none;
		font: inherit;
		color: inherit;
		text-align: left;
		cursor: pointer;
	}
	.line-stop:hover {
		color: var(--sg-brand);
	}
	.line-stop__seq {
		flex: none;
		min-width: 2rem;
		font-size: var(--sg-text-caption);
		color: var(--sg-text-muted);
		text-align: end;
	}
	.journey-head {
		display: flex;
		align-items: center;
		gap: var(--sg-space-3);
		flex-wrap: wrap;
		margin-block: var(--sg-space-2) 0;
	}
	.fare {
		font-weight: var(--sg-weight-bold);
		font-variant-numeric: tabular-nums;
	}
	.journey-legs {
		list-style: none;
		padding: 0;
		margin: var(--sg-space-3) 0 0;
		display: flex;
		flex-direction: column;
		gap: var(--sg-space-3);
	}
	.jleg {
		display: flex;
		gap: var(--sg-space-2);
	}
	.jleg-icon {
		color: var(--sg-brand);
	}
	.jleg-body {
		min-width: 0;
	}
	.jleg-deps {
		display: block;
		margin-top: var(--sg-space-1);
		font-weight: var(--sg-weight-semibold);
		font-variant-numeric: tabular-nums;
	}
	.jleg-alts {
		list-style: none;
		margin: var(--sg-space-1) 0 0;
		padding: var(--sg-space-1) 0 0;
		border-top: 1px dashed var(--sg-border);
		font-size: var(--sg-text-secondary);
	}
	.jleg-alts li {
		padding-block: var(--sg-space-1);
	}
	.fare-detail,
	.jleg details {
		margin-top: var(--sg-space-2);
	}
	.fare-detail summary,
	.jleg details summary {
		cursor: pointer;
		font-size: var(--sg-text-secondary);
		color: var(--sg-text-muted);
	}
	.fare-detail ul,
	.jleg details ul {
		margin: var(--sg-space-1) 0 0;
		padding-left: var(--sg-space-4);
	}
	.stop-link {
		background: none;
		border: none;
		padding: 0;
		font: inherit;
		color: var(--sg-brand);
		cursor: pointer;
		text-align: left;
	}
	.stop-link:hover {
		text-decoration: underline;
	}
	/* Medium and up: side sheet (docs/10). */
	@media (min-width: 48rem) {
		.sheet {
			inset-inline: auto 0;
			top: 0;
			bottom: calc(var(--nav-h) + var(--sg-space-3) + env(safe-area-inset-bottom, 0px));
			width: min(27rem, 45%);
			height: auto;
			max-height: none;
			border-top: 0;
			border-inline-start: 1px solid var(--sg-border);
			border-radius: 0;
			box-shadow: var(--sg-shadow-overlay);
		}
		.sheet--peek,
		.sheet--expanded {
			height: auto;
		}
	}
	@media (prefers-reduced-motion: reduce) {
		.sheet {
			transition: none;
		}
	}
</style>
