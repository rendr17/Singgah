<script lang="ts">
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { env } from '$env/dynamic/public';
	import { browser } from '$app/environment';
	import { api } from '$lib/api';
	import BackNav from '$lib/components/app-shell/BackNav.svelte';
	import { unwrap } from '@singgah/api-client';
	import type { components } from '@singgah/api-client';
	import { TransitMap, stationsToGeoJSON } from '@singgah/map';
	import { StateBlock } from '@singgah/ui';

	type Station = components['schemas']['StationSummary'];

	// OpenFreeMap: free public OpenMapTiles hosting, no key — fine for dev and
	// early production; self-hosting a style is a later infra decision.
	const STYLE_URL = env.PUBLIC_MAP_STYLE_URL ?? 'https://tiles.openfreemap.org/styles/positron';

	let stations = $state<Station[]>([]);
	let error = $state('');

	let timer: ReturnType<typeof setTimeout>;
	function onViewportChange(bbox: [number, number, number, number]) {
		clearTimeout(timer);
		timer = setTimeout(async () => {
			try {
				const data = await unwrap(
					api.GET('/api/v1/stations', {
						params: { query: { bbox: bbox.map((n) => n.toFixed(5)).join(','), limit: 500 } }
					})
				);
				stations = data.stations;
				error = '';
			} catch (e) {
				error = e instanceof Error ? e.message : 'Data stasiun gagal dimuat.';
			}
		}, 250);
	}
</script>

<BackNav href="/" label="Pencarian" />
<h1 class="sg-page-title">Peta jaringan</h1>

{#if error}
	<StateBlock kind="error">{error}</StateBlock>
{/if}

<div class="map-wrap">
	{#if browser}
		<TransitMap
			styleUrl={STYLE_URL}
			data={stationsToGeoJSON(stations)}
			{onViewportChange}
			onSelect={(id) => goto(resolve('/stations/[id]', { id }))}
		/>
	{/if}
</div>

<p class="sg-meta">
	Stasiun dimuat mengikuti area yang terlihat. Klik titik untuk membuka detail stasiun.
</p>

<style>
	.map-wrap {
		height: min(70vh, 36rem);
		border: 1px solid var(--sg-border);
		border-radius: var(--sg-radius-card);
		overflow: hidden;
	}
</style>
