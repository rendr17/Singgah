<script lang="ts">
	import { onMount } from 'svelte';
	import type { FeatureCollection, LineString, MultiLineString, Point } from 'geojson';
	import type {
		ExpressionSpecification,
		FilterSpecification,
		Map as MLMap,
		GeoJSONSource
	} from 'maplibre-gl';
	import 'maplibre-gl/dist/maplibre-gl.css';
	import { OPERATOR_ICON_URLS, operatorIcon } from './operators';

	interface Props {
		/** MapLibre style URL — tiles/glyphs come from the style, not this component. */
		styleUrl: string;
		data: FeatureCollection<Point>;
		center?: [number, number];
		zoom?: number;
		interactive?: boolean;
		/** Fit the camera to the data whenever it changes (mini-map mode). */
		fitToData?: boolean;
		/** Click on an unclustered station — receives its canonical id. */
		onSelect?: (id: string) => void;
		/** Click on a network route line — receives its canonical route id. */
		onSelectLine?: (routeId: string) => void;
		/** Viewport bounds after each settled move — drives bbox-scoped fetches. */
		onViewportChange?: (bbox: [number, number, number, number]) => void;
		/** Journey overlay — LineStrings get `dashed` for walk legs; Points mark
		 *  board/alight endpoints (`endpoint: true`, solid) and stops ridden
		 *  through (hollow). */
		route?: FeatureCollection;
		/** Network route lines drawn under the station dots (integrated-map style).
		 *  Each feature's `properties.color` is hex without '#'; empty falls back to muted. */
		lines?: FeatureCollection<LineString | MultiLineString>;
		/** Visibility switch for the route-lines overlay — data still loads either way. */
		linesVisible?: boolean;
		/** Imperative camera target: ease to this lon/lat only when it falls
		 *  outside the current view — in-view selections never move the map. */
		focus?: [number, number];
		/** Pixels obscured at the bottom of the mobile viewport by the nav and sheet. */
		bottomPadding?: number;
		/** Called once when the map can never become ready (init or style load
		 *  failure) — lets parents drop loading affordances that would otherwise
		 *  spin forever. */
		onFailed?: () => void;
		class?: string;
	}

	let {
		styleUrl,
		data,
		center = [106.8229, -6.1945],
		zoom = 10.5,
		interactive = true,
		fitToData = false,
		onSelect,
		onSelectLine,
		onViewportChange,
		route,
		lines,
		linesVisible = true,
		focus,
		bottomPadding = 0,
		onFailed,
		class: className
	}: Props = $props();

	let container: HTMLDivElement;
	let map: MLMap | undefined;
	let ready = $state(false);
	let failed = $state(false);
	let initialTileErrors = 0;

	const SOURCE = 'singgah-stations';
	const ROUTE = 'singgah-route';
	const LINES = 'singgah-lines';
	const EMPTY_FC: FeatureCollection = { type: 'FeatureCollection', features: [] };

	// Canvas layers can't read CSS custom properties — mirror the token values
	// (09_DESIGN_SYSTEM.md) instead of introducing a theme plumbing layer.
	const BRAND = '#0F6B4F';
	const MUTED = '#626A70';
	const TEXT = '#141719';

	// Points whose `icon` names a registered operator mark get the official
	// logo on a white badge; the rest keep the plain dot.
	const HAS_ICON: ExpressionSpecification = ['!=', ['coalesce', ['get', 'icon'], ''], ''];

	// A feature may carry `color` (hex without '#') — the drawn route then
	// wears its corridor's catalog color instead of the flat brand line.
	const HAS_COLOR: ExpressionSpecification = ['!=', ['coalesce', ['get', 'color'], ''], ''];
	const FEATURE_COLOR: ExpressionSpecification = [
		'case',
		HAS_COLOR,
		['concat', '#', ['get', 'color']],
		BRAND
	];

	onMount(() => {
		let cancelled = false;
		let m: MLMap | undefined;
		(async () => {
			try {
				const ml = await import('maplibre-gl');
				if (cancelled) return;
				m = new ml.Map({
					container,
					style: styleUrl,
					center,
					zoom,
					interactive,
					attributionControl: { compact: true }
				});
				if (interactive) {
					m.addControl(new ml.NavigationControl({ showCompass: false }), 'bottom-right');
					m.addControl(new ml.ScaleControl({ unit: 'metric' }), 'bottom-left');
				}
				map = m;
				// A failed style or repeated initial tile fetch never reaches a
				// usable first frame. Ignore a few transient tile failures, then
				// expose the fallback instead of leaving a blank canvas.
				m.on('error', (e) => {
					const ev = e as { sourceId?: string; tile?: unknown };
					if (cancelled || failed) return;
					if (ev.sourceId || ev.tile) {
						if (!ready && ++initialTileErrors >= 4) {
							failed = true;
							onFailed?.();
						}
						return;
					}
					failed = true;
					onFailed?.();
				});
				m.on('load', () => {
					if (cancelled || failed) return;
					// Singgah paints its own stop/station marks — drop the basemap's
					// transit POIs (station, bus_stop, ferry_terminal) so icons and
					// labels don't double up (71_ICONS_MAP_STYLE.md). Wraps the
					// style's existing filter instead of re-declaring its kind list.
					if (m!.getLayer('pois')) {
						const notTransit: ExpressionSpecification = [
							'!',
							['in', ['get', 'kind'], ['literal', ['station', 'bus_stop', 'ferry_terminal']]]
						];
						const base = m!.getFilter('pois');
						m!.setFilter(
							'pois',
							(base ? ['all', base, notTransit] : notTransit) as FilterSpecification
						);
					}
					m!.addSource(LINES, { type: 'geojson', data: lines ?? EMPTY_FC });
					// Network lines sit under the station dots — registered first so
					// clusters/stations paint on top of them.
					m!.addLayer({
						id: 'route-lines',
						type: 'line',
						source: LINES,
						filter: ['!=', ['get', 'source'], 'stops'],
						layout: { visibility: linesVisible ? 'visible' : 'none' },
						paint: {
							'line-color': [
								'case',
								['!=', ['coalesce', ['get', 'color'], ''], ''],
								['concat', '#', ['get', 'color']],
								MUTED
							],
							'line-width': ['interpolate', ['linear'], ['zoom'], 9, 1.5, 14, 3.5],
							'line-opacity': 0.8
						}
					});
					// Corridors without ingested shape geometry draw as straight
					// stop-to-stop polylines — dashed so the approximation reads as
					// approximate (transit truth: never present a fallback as a real path).
					m!.addLayer({
						id: 'route-lines-approx',
						type: 'line',
						source: LINES,
						filter: ['==', ['get', 'source'], 'stops'],
						layout: { visibility: linesVisible ? 'visible' : 'none' },
						paint: {
							'line-color': [
								'case',
								['!=', ['coalesce', ['get', 'color'], ''], ''],
								['concat', '#', ['get', 'color']],
								MUTED
							],
							'line-width': ['interpolate', ['linear'], ['zoom'], 9, 1.5, 14, 3],
							'line-dasharray': [2, 1.6],
							'line-opacity': 0.55
						}
					});
					m!.addSource(SOURCE, {
						type: 'geojson',
						data,
						cluster: true,
						clusterMaxZoom: 13,
						clusterRadius: 42
					});
					m!.addLayer({
						id: 'clusters',
						type: 'circle',
						source: SOURCE,
						filter: ['has', 'point_count'],
						paint: {
							'circle-color': BRAND,
							'circle-opacity': 0.85,
							'circle-radius': ['step', ['get', 'point_count'], 14, 20, 18, 80, 24]
						}
					});
					m!.addLayer({
						id: 'cluster-count',
						type: 'symbol',
						source: SOURCE,
						filter: ['has', 'point_count'],
						layout: {
							'text-field': '{point_count_abbreviated}',
							'text-size': 12
						},
						paint: { 'text-color': '#FFFFFF' }
					});
					m!.addLayer({
						id: 'stations',
						type: 'circle',
						source: SOURCE,
						filter: ['!', ['has', 'point_count']],
						paint: {
							'circle-color': [
								'case',
								HAS_ICON,
								'#FFFFFF',
								['match', ['get', 'kind'], 'station', BRAND, MUTED]
							],
							'circle-radius': [
								'interpolate',
								['linear'],
								['zoom'],
								10,
								['case', HAS_ICON, 7, 4],
								14,
								['case', HAS_ICON, 11, 7]
							],
							'circle-stroke-width': 1.5,
							'circle-stroke-color': [
								'case',
								HAS_ICON,
								['match', ['get', 'kind'], 'station', BRAND, MUTED],
								'#FFFFFF'
							]
						}
					});
					m!.addLayer({
						id: 'station-labels',
						type: 'symbol',
						source: SOURCE,
						filter: ['!', ['has', 'point_count']],
						minzoom: 13,
						layout: {
							'text-field': ['get', 'name'],
							'text-size': 12,
							'text-offset': [0, 1.1],
							'text-anchor': 'top',
							'text-optional': true
						},
						paint: {
							'text-color': TEXT,
							'text-halo-color': '#FFFFFF',
							'text-halo-width': 1.2
						}
					});
					m!.addSource(ROUTE, { type: 'geojson', data: route ?? EMPTY_FC });
					m!.addLayer({
						id: 'route-walk',
						type: 'line',
						source: ROUTE,
						filter: [
							'all',
							['==', ['geometry-type'], 'LineString'],
							['==', ['get', 'dashed'], true]
						],
						paint: {
							'line-color': MUTED,
							'line-width': 3,
							'line-dasharray': [1.5, 1.5]
						}
					});
					m!.addLayer({
						id: 'route-ride',
						type: 'line',
						source: ROUTE,
						filter: [
							'all',
							['==', ['geometry-type'], 'LineString'],
							['!=', ['get', 'dashed'], true]
						],
						paint: {
							'line-color': FEATURE_COLOR,
							'line-width': 4,
							'line-opacity': 0.9
						}
					});
					m!.addLayer({
						id: 'route-stops',
						type: 'circle',
						source: ROUTE,
						filter: [
							'all',
							['==', ['geometry-type'], 'Point'],
							['!=', ['get', 'endpoint'], true],
							['!', HAS_ICON]
						],
						paint: {
							'circle-color': '#FFFFFF',
							'circle-radius': ['interpolate', ['linear'], ['zoom'], 10, 3, 14, 5],
							'circle-stroke-width': 2,
							'circle-stroke-color': FEATURE_COLOR
						}
					});
					m!.addLayer({
						id: 'route-endpoints',
						type: 'circle',
						source: ROUTE,
						filter: ['all', ['==', ['geometry-type'], 'Point'], ['==', ['get', 'endpoint'], true]],
						paint: {
							'circle-color': [
								'match',
								['get', 'endpointRole'],
								'origin',
								'#176B4A',
								'destination',
								'#B44343',
								FEATURE_COLOR
							],
							'circle-radius': 7,
							'circle-stroke-width': 2.5,
							'circle-stroke-color': '#FFFFFF'
						}
					});
					m!.addLayer({
						id: 'route-endpoint-labels',
						type: 'symbol',
						source: ROUTE,
						filter: ['all', ['==', ['geometry-type'], 'Point'], ['==', ['get', 'endpoint'], true]],
						layout: {
							'text-field': [
								'match',
								['get', 'endpointRole'],
								'origin',
								'A',
								'destination',
								'B',
								''
							],
							'text-size': 10,
							'text-allow-overlap': true
						},
						paint: { 'text-color': '#FFFFFF' }
					});
					// Official operator marks ride on top of the badge circles —
					// decorative, so a failed decode leaves the badges in place.
					void (async () => {
						try {
							await Promise.all(
								Object.entries(OPERATOR_ICON_URLS).map(async ([op, url]) => {
									const img = await m!.loadImage(url);
									if (!cancelled) m!.addImage(operatorIcon(op), img.data);
								})
							);
						} catch {
							return;
						}
						if (cancelled) return;
						m!.addLayer(
							{
								id: 'station-icons',
								type: 'symbol',
								source: SOURCE,
								filter: ['all', ['!', ['has', 'point_count']], HAS_ICON],
								layout: {
									'icon-image': ['get', 'icon'],
									'icon-size': ['interpolate', ['linear'], ['zoom'], 10, 0.16, 14, 0.28]
								}
							},
							'station-labels'
						);
						m!.addLayer({
							id: 'route-stop-badges',
							type: 'circle',
							source: ROUTE,
							filter: [
								'all',
								['==', ['geometry-type'], 'Point'],
								['!=', ['get', 'endpoint'], true],
								HAS_ICON
							],
							paint: {
								'circle-color': '#FFFFFF',
								'circle-radius': ['interpolate', ['linear'], ['zoom'], 10, 6, 14, 9],
								'circle-stroke-width': 1.5,
								'circle-stroke-color': BRAND
							}
						});
						m!.addLayer({
							id: 'route-stop-icons',
							type: 'symbol',
							source: ROUTE,
							filter: [
								'all',
								['==', ['geometry-type'], 'Point'],
								['!=', ['get', 'endpoint'], true],
								HAS_ICON
							],
							layout: {
								'icon-image': ['get', 'icon'],
								'icon-size': ['interpolate', ['linear'], ['zoom'], 10, 0.13, 14, 0.22]
							}
						});
					})();
					// Logo marks sit over the dots — both layers select the same way.
					for (const layer of ['stations', 'station-icons']) {
						m!.on('click', layer, (e) => {
							const id = e.features?.[0]?.properties?.id;
							if (typeof id === 'string') onSelect?.(id);
						});
						m!.on('mouseenter', layer, () => {
							m!.getCanvas().style.cursor = 'pointer';
						});
						m!.on('mouseleave', layer, () => {
							m!.getCanvas().style.cursor = '';
						});
					}
					// Corridor strokes select their route — except where a
					// station/cluster sits on the line: markers draw on top, so
					// they win the tap too.
					for (const layer of ['route-lines', 'route-lines-approx']) {
						m!.on('click', layer, (e) => {
							const id = e.features?.[0]?.properties?.routeId;
							if (typeof id !== 'string') return;
							const covered = ['stations', 'station-icons', 'clusters']
								.filter((l) => m!.getLayer(l))
								.some((l) => m!.queryRenderedFeatures(e.point, { layers: [l] }).length > 0);
							if (!covered) onSelectLine?.(id);
						});
						m!.on('mouseenter', layer, () => {
							m!.getCanvas().style.cursor = 'pointer';
						});
						m!.on('mouseleave', layer, () => {
							m!.getCanvas().style.cursor = '';
						});
					}
					if (onViewportChange) {
						const emit = () => {
							// A hidden pane (other map mode) has a 0-size container — its
							// bounds are a degenerate point and must never drive a fetch.
							if (container.clientWidth === 0 || container.clientHeight === 0) return;
							const b = m!.getBounds();
							onViewportChange([b.getWest(), b.getSouth(), b.getEast(), b.getNorth()]);
						};
						m!.on('moveend', emit);
						// Re-emit when the pane resizes back into view — an easeTo done
						// while hidden ends in a moveend that the guard above drops.
						m!.on('resize', emit);
						emit();
					}
					if (fitToData) fitGeo(m!, data);
					ready = true;
				});
			} catch {
				if (!cancelled) {
					failed = true;
					onFailed?.();
				}
			}
		})();
		return () => {
			cancelled = true;
			m?.remove();
		};
	});

	function fitGeo(m: MLMap, fc: FeatureCollection) {
		const pts: [number, number][] = [];
		for (const f of fc.features) {
			const g = f.geometry;
			if (g.type === 'Point') pts.push(g.coordinates as [number, number]);
			else if (g.type === 'LineString') pts.push(...(g.coordinates as [number, number][]));
			else if (g.type === 'MultiLineString')
				for (const seg of g.coordinates as [number, number][][]) pts.push(...seg);
		}
		if (pts.length === 0) return;
		if (pts.length === 1) {
			m.jumpTo({ center: pts[0], zoom: Math.max(m.getZoom(), 14) });
			return;
		}
		const lons = pts.map((p) => p[0]);
		const lats = pts.map((p) => p[1]);
		m.fitBounds(
			[
				[Math.min(...lons), Math.min(...lats)],
				[Math.max(...lons), Math.max(...lats)]
			],
			{ padding: 48, maxZoom: 14 }
		);
	}

	$effect(() => {
		if (!ready || !map || !focus) return;
		if (map.getBounds().contains(focus)) return;
		const reduced = matchMedia('(prefers-reduced-motion: reduce)').matches;
		map.easeTo({
			center: focus,
			zoom: Math.max(map.getZoom(), 13.5),
			duration: reduced ? 0 : 600
		});
	});

	// Keep the camera's visual center in the portion of the map above the mobile
	// navigation and sheet. This only follows panel geometry, never data updates.
	$effect(() => {
		if (!ready || !map) return;
		const bottom = Math.max(0, Math.round(bottomPadding));
		const current = map.getPadding();
		if (current.top === 0 && current.right === 0 && current.bottom === bottom && current.left === 0)
			return;
		map.setPadding({ top: 0, right: 0, bottom, left: 0 });
	});

	$effect(() => {
		if (!ready || !map) return;
		const src = map.getSource<GeoJSONSource>(SOURCE);
		src?.setData(data);
		map.getSource<GeoJSONSource>(LINES)?.setData(lines ?? EMPTY_FC);
		for (const id of ['route-lines', 'route-lines-approx']) {
			if (map.getLayer(id)) {
				map.setLayoutProperty(id, 'visibility', linesVisible ? 'visible' : 'none');
			}
		}
		map.getSource<GeoJSONSource>(ROUTE)?.setData(route ?? EMPTY_FC);
	});

	// Camera fitting tracks `route` only. If it shared the sync effect above,
	// every viewport-scoped stations/lines refresh (each pan's moveend →
	// fetch → new `data`) would refit to the route and lock pan/zoom.
	$effect(() => {
		if (!ready || !map) return;
		const r = route;
		if (r && r.features.length > 0) fitGeo(map, r);
	});

	$effect(() => {
		if (!ready || !map || !fitToData) return;
		const d = data;
		if (route?.features.length) return;
		fitGeo(map, d);
	});
</script>

<div class={['sg-map', className]} bind:this={container}>
	{#if failed}
		<p class="sg-map__state" role="alert">Peta tidak bisa dimuat.</p>
	{:else if !ready}
		<p class="sg-map__state" role="status">Memuat peta…</p>
	{/if}
</div>

<style>
	.sg-map {
		position: relative;
		width: 100%;
		height: 100%;
		min-height: 16rem;
		background-color: var(--sg-surface-muted, #eef1f2);
	}
	.sg-map__state {
		position: absolute;
		inset: 0;
		display: grid;
		place-items: center;
		margin: 0;
		color: var(--sg-text-muted, #626a70);
		font-size: 0.875rem;
	}
</style>
