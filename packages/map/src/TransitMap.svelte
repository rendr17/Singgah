<script lang="ts">
	import { onMount } from 'svelte';
	import type { FeatureCollection, Point } from 'geojson';
	import type { Map as MLMap, GeoJSONSource } from 'maplibre-gl';
	import 'maplibre-gl/dist/maplibre-gl.css';

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
		/** Viewport bounds after each settled move — drives bbox-scoped fetches. */
		onViewportChange?: (bbox: [number, number, number, number]) => void;
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
		onViewportChange,
		class: className
	}: Props = $props();

	let container: HTMLDivElement;
	let map: MLMap | undefined;
	let ready = $state(false);
	let failed = $state(false);

	const SOURCE = 'singgah-stations';

	// Canvas layers can't read CSS custom properties — mirror the token values
	// (09_DESIGN_SYSTEM.md) instead of introducing a theme plumbing layer.
	const BRAND = '#0F6B4F';
	const MUTED = '#626A70';
	const TEXT = '#141719';

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
				map = m;
				m.on('load', () => {
					if (cancelled) return;
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
							'circle-color': ['match', ['get', 'kind'], 'station', BRAND, MUTED],
							'circle-radius': ['interpolate', ['linear'], ['zoom'], 10, 4, 14, 7],
							'circle-stroke-width': 1.5,
							'circle-stroke-color': '#FFFFFF'
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
					m!.on('click', 'stations', (e) => {
						const id = e.features?.[0]?.properties?.id;
						if (typeof id === 'string') onSelect?.(id);
					});
					m!.on('mouseenter', 'stations', () => {
						m!.getCanvas().style.cursor = 'pointer';
					});
					m!.on('mouseleave', 'stations', () => {
						m!.getCanvas().style.cursor = '';
					});
					if (onViewportChange) {
						const emit = () => {
							const b = m!.getBounds();
							onViewportChange([b.getWest(), b.getSouth(), b.getEast(), b.getNorth()]);
						};
						m!.on('moveend', emit);
						emit();
					}
					if (fitToData) fitMapToData(m!, data);
					ready = true;
				});
			} catch {
				if (!cancelled) failed = true;
			}
		})();
		return () => {
			cancelled = true;
			m?.remove();
		};
	});

	function fitMapToData(m: MLMap, fc: FeatureCollection<Point>) {
		const pts = fc.features.map((f) => f.geometry.coordinates as [number, number]);
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
		if (!ready || !map) return;
		const src = map.getSource<GeoJSONSource>(SOURCE);
		src?.setData(data);
		if (fitToData) fitMapToData(map, data);
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
