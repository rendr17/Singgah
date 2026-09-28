<script lang="ts">
	import { resolve } from '$app/paths';
	import { browser } from '$app/environment';
	import { onMount, tick } from 'svelte';
	import { SvelteMap } from 'svelte/reactivity';
	import { api } from '$lib/api';
	import { basemapStyleUrl } from '$lib/basemap';
	import { facilityLabel } from '$lib/facilities';
	import { mapStore } from '$lib/stores/map.svelte';
	import BackNav from '$lib/components/app-shell/BackNav.svelte';
	import DepartureBoard from '$lib/components/station/DepartureBoard.svelte';
	import MapModeSwitcher from '$lib/components/map/MapModeSwitcher.svelte';
	import MapSearch from '$lib/components/map/MapSearch.svelte';
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

	type Station = components['schemas']['StationSummary'];
	type StationDetail = components['schemas']['StationDetail'];
	type Board = components['schemas']['StationDepartures'];
	type RouteLines = components['schemas']['RouteLineCollection'];
	type RouteSummary = components['schemas']['RouteSummary'];
	type RouteDetail = components['schemas']['RouteDetail'];
	type Plan = components['schemas']['JourneyPlan'];

	const STYLE_URL = basemapStyleUrl();

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

	// Station sheet — the two fetches degrade independently: a timetable
	// outage must not hide the station's catalog detail.
	let detail = $state<StationDetail | null>(null);
	let detailError = $state('');
	let board = $state<Board | null>(null);
	let boardError = $state('');
	let boardLoading = $state(false);
	let sheetEl = $state<HTMLElement | undefined>();
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

	$effect(() => {
		const from = mapStore.fromStationId;
		const to = mapStore.toStationId;
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
				const p = await unwrap(api.GET('/api/v1/journeys', { params: { query: { from, to } } }));
				if (cancelled || mapStore.fromStationId !== from || mapStore.toStationId !== to) return;
				journey = p;
				mapStore.selectJourney(p.itineraries.length > 0 ? `${from}:${to}` : undefined);
			} catch (e) {
				if (cancelled || mapStore.fromStationId !== from || mapStore.toStationId !== to) return;
				journeyError = e instanceof Error ? e.message : 'Rencana rute gagal dimuat.';
			} finally {
				if (!cancelled) journeyLoading = false;
			}
		})();
		return () => {
			cancelled = true;
		};
	});

	const journeyRoute = $derived(
		journey?.itineraries[0] ? journeyToGeoJSON(journey.itineraries[0].legs) : undefined
	);

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
				for (const r of data.routes) {
					if (!r.agencyCode || !r.shortName) continue;
					const key = `${r.agencyCode}:${r.shortName}`;
					routesByKey.set(key, r);
					keyByRouteId.set(r.id, key);
				}
			} catch {
				/* isolation links degrade — taps still open stations */
			}
		})();
	});

	let timer: ReturnType<typeof setTimeout>;
	function onViewportChange(bbox: [number, number, number, number]) {
		clearTimeout(timer);
		timer = setTimeout(() => {
			const bboxStr = bbox.map((n) => n.toFixed(5)).join(',');
			void (async () => {
				try {
					const data = await unwrap(
						api.GET('/api/v1/stations', { params: { query: { bbox: bboxStr, limit: 500 } } })
					);
					stations = data.stations;
					error = '';
				} catch (e) {
					error = e instanceof Error ? e.message : 'Data stasiun gagal dimuat.';
				} finally {
					firstLoad = false;
				}
			})();
			// Route lines degrade independently — a lines outage keeps the last
			// frame's geometry rather than hiding the station dots.
			void (async () => {
				try {
					const data = await unwrap(
						api.GET('/api/v1/map/lines', { params: { query: { bbox: bboxStr } } })
					);
					lines = data.lines;
				} catch {
					/* decorative layer — degrade silently */
				}
			})();
		}, 250);
	}

	function openStation(id: string) {
		mapStore.selectLine(); // one detail surface at a time
		if (mapStore.selectedStationId === id) return;
		mapStore.selectStation(id);
		detail = null;
		board = null;
		detailError = '';
		boardError = '';
		boardLoading = true;
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
					detailError = e instanceof Error ? e.message : 'Detail stasiun gagal dimuat.';
			}
		})();
		void (async () => {
			try {
				const d = await unwrap(
					api.GET('/api/v1/stations/{id}/departures', { params: { path: { id } } })
				);
				if (mapStore.selectedStationId === id) board = d.departures;
			} catch (e) {
				if (mapStore.selectedStationId === id)
					boardError = e instanceof Error ? e.message : 'Jadwal sedang tidak tersedia.';
			} finally {
				if (mapStore.selectedStationId === id) boardLoading = false;
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
		if (mapStore.selectedLineId === id) return;
		openLineId(id);
	}

	// A corridor tap resolves a traced artwork key ("TJ:7F") to its route.
	function openLineByKey(key: string) {
		const r = routesByKey.get(key);
		if (r) openLineId(r.id);
	}

	function openLineId(id: string) {
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
				if (mapStore.selectedLineId === id)
					lineError = e instanceof Error ? e.message : 'Detail lin gagal dimuat.';
			}
		})();
	}

	function setFromHere() {
		if (!detail) return;
		mapStore.setFrom(detail.id);
		fromName = detail.name;
	}
	function setToHere() {
		if (!detail) return;
		mapStore.setTo(detail.id);
		toName = detail.name;
	}
	function clearRouteChips() {
		mapStore.clearRoute();
		fromName = undefined;
		toName = undefined;
	}
</script>

<svelte:head><title>Peta jaringan · Singgah</title></svelte:head>

<svelte:window
	onkeydown={(e) => {
		if (e.key !== 'Escape') return;
		if (mapStore.selectedStationId || mapStore.selectedLineId) closeSheet();
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
		<BackNav href="/" label="Beranda" />
		<div class="map-chrome__search">
			<MapSearch onStation={(s) => openStation(s.id)} onLine={(l) => openLine(l.id)} />
		</div>
		{#if mapStore.fromStationId || mapStore.toStationId}
			<div class="routebar" aria-label="Asal dan tujuan">
				<span class="routebar__leg">Dari: <strong>{fromName ?? '—'}</strong></span>
				<span class="routebar__leg">Ke: <strong>{toName ?? '—'}</strong></span>
				{#if mapStore.fromStationId && mapStore.toStationId}
					<a
						class="routebar__go"
						href={resolve(`/plan?from=${mapStore.fromStationId}&to=${mapStore.toStationId}`)}
						>Rencanakan</a
					>
				{/if}
				<IconButton label="Hapus asal dan tujuan" onclick={clearRouteChips}>
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
	</div>

	{#if !mapFailed && firstLoad}
		<div class="map-note">
			<StateBlock kind="loading">Memuat stasiun…</StateBlock>
		</div>
	{/if}
	{#if !mapFailed && error}
		<div class="map-note">
			<StateBlock kind="error">{error}</StateBlock>
		</div>
	{/if}

	{#if browser}
		<!-- Both panes stay mounted across mode switches — hiding, not
		     recreating, keeps each map's camera and loaded tiles. -->
		<div class="map-pane" hidden={mapStore.mode !== 'geographic'}>
			<TransitMap
				styleUrl={STYLE_URL}
				data={stationsToGeoJSON(stations)}
				{lines}
				{linesVisible}
				{onViewportChange}
				focus={focusPoint}
				route={journeyRoute}
				onSelect={openStation}
				onSelectLine={openLine}
				onFailed={() => (mapFailed = true)}
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

	{#if mapStore.selectedStationId}
		<aside class="sheet" aria-label="Detail stasiun" tabindex="-1" bind:this={sheetEl}>
			<header class="sheet-head">
				{#if detail}
					{#if detail.code}<span class="code-badge">{detail.code}</span>{/if}
					<h2>{detail.name}</h2>
				{:else}
					{@const s = stations.find((x) => x.id === mapStore.selectedStationId)}
					<h2>{s?.name ?? 'Stasiun'}</h2>
				{/if}
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

			{#if detailError}
				<StateBlock kind="error">{detailError}</StateBlock>
			{:else if !detail}
				<StateBlock kind="loading">Memuat detail stasiun…</StateBlock>
			{:else}
				<div class="sheet-actions">
					<a
						class="action action--primary"
						href={resolve(
							mapStore.fromStationId
								? `/plan?from=${mapStore.fromStationId}&to=${detail.id}`
								: `/plan?to=${detail.id}`
						)}>Rute ke sini</a
					>
					<a class="action" href={resolve('/stations/[id]', { id: detail.id })}>Jadwal lengkap</a>
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
						<DepartureBoard lines={board.lines} compact />
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
		<aside class="sheet" aria-label="Detail lin" tabindex="-1" bind:this={sheetEl}>
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
	{#if (mapStore.fromStationId || mapStore.toStationId) && !mapStore.selectedStationId && !mapStore.selectedLineId}
		<aside class="sheet" aria-label="Rencana perjalanan" tabindex="-1">
			<header class="sheet-head">
				<h2>Rencana perjalanan</h2>
				<IconButton label="Hapus rencana" onclick={clearRouteChips}>
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

			{#if journeyLoading}
				<StateBlock kind="loading">Mencari rute…</StateBlock>
			{:else if journeyError}
				<StateBlock kind="error">{journeyError}</StateBlock>
			{:else if journey?.itineraries[0]}
				{@const itin = journey.itineraries[0]}
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
				<p class="journey-link">
					<a href={resolve(`/journey?from=${mapStore.fromStationId}&to=${mapStore.toStationId}`)}
						>Linimasa lengkap →</a
					>
				</p>
				<p class="sg-meta board-note">
					Jadwal statis {journey.source.provider} — bukan posisi live.
				</p>
			{:else if journey}
				<StateBlock kind="empty"
					>Provider tidak menemukan rute antara {journey.from.name} dan {journey.to
						.name}.</StateBlock
				>
			{:else}
				<p class="sg-meta">
					Pilih stasiun lalu tandai “Dari sini” dan “Ke sini” untuk merencanakan perjalanan.
				</p>
			{/if}
		</aside>
	{/if}
</div>

<style>
	/* Fullscreen map surface: fixed to the viewport under the shell chrome —
	   sidebar on wide, bottom nav on compact both stay visible and usable. */
	.map-wrap {
		position: fixed;
		inset: 0;
		bottom: calc(var(--nav-h, 3.5rem) + env(safe-area-inset-bottom, 0px));
		overflow: hidden;
		z-index: 1;
	}
	@media (min-width: 48rem) {
		.map-wrap {
			inset-inline-start: var(--nav-w, 10.5rem);
			bottom: 0;
		}
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
	.map-note {
		position: absolute;
		top: calc(var(--sg-space-3) + var(--sg-target-min) + var(--sg-space-3));
		inset-inline: var(--sg-space-3);
		display: flex;
		justify-content: center;
		pointer-events: none;
		z-index: 2;
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

	/* Overlay chip — below the chrome bar, above the sheet layer. */
	.lines-toggle {
		position: absolute;
		top: calc(var(--sg-space-3) + var(--sg-target-min) + var(--sg-space-2));
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

	/* Compact: bottom sheet over the map (docs/10). */
	.sheet {
		position: absolute;
		inset-inline: 0;
		bottom: 0;
		max-height: 70%;
		overflow-y: auto;
		padding: var(--sg-space-3) var(--sg-space-4) var(--sg-space-4);
		background-color: var(--sg-surface);
		border-top: 1px solid var(--sg-border);
		border-radius: var(--sg-radius-sheet) var(--sg-radius-sheet) 0 0;
		box-shadow: var(--sg-shadow-sheet);
		z-index: var(--sg-z-sheet);
	}
	.sheet-head {
		display: flex;
		align-items: center;
		gap: var(--sg-space-2);
	}
	.sheet-head h2 {
		flex: 1;
		margin: 0;
		font-size: var(--sg-text-screen);
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
	.journey-link {
		margin-block: var(--sg-space-3) 0;
	}
	.journey-link a {
		display: inline-flex;
		align-items: center;
		min-height: var(--sg-target-min);
		font-weight: var(--sg-weight-semibold);
	}

	/* Medium and up: side sheet (docs/10). */
	@media (min-width: 48rem) {
		.sheet {
			inset-inline: auto 0;
			top: 0;
			bottom: 0;
			width: min(24rem, 45%);
			max-height: none;
			border-top: 0;
			border-inline-start: 1px solid var(--sg-border);
			border-radius: 0;
			box-shadow: var(--sg-shadow-overlay);
		}
	}
</style>
