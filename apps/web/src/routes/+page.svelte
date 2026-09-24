<script lang="ts">
	import { dev } from '$app/environment';
	import { resolve } from '$app/paths';
	import UiPreview from '$lib/dev/UiPreview.svelte';
	import { api } from '$lib/api';
	import { unwrap } from '@singgah/api-client';
	import type { components } from '@singgah/api-client';
	import { SearchField } from '@singgah/ui';

	type Station = components['schemas']['StationSummary'];

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
</script>

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
	<p class="sg-meta" role="status">Mencari…</p>
{:else if error}
	<p class="sg-error" role="alert">{error}</p>
{:else if query.trim() !== '' && stations.length === 0}
	<p class="sg-meta">Tidak ada stasiun yang cocok dengan “{query.trim()}”.</p>
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
</style>
