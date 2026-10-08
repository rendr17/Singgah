<script lang="ts">
	import { resolve } from '$app/paths';
	import { api } from '$lib/api';
	import StationCombobox from '$lib/components/station/StationCombobox.svelte';
	import NearbyPlaces from '$lib/components/station/NearbyPlaces.svelte';
	import { unwrap } from '@singgah/api-client';
	import type { components } from '@singgah/api-client';
	import type { PageProps } from './$types';
	import { Button, SectionHeading, StateBlock } from '@singgah/ui';

	let { data }: PageProps = $props();
	type Station = components['schemas']['StationSummary'];
	type Place = components['schemas']['PlaceSummary'];

	let stationQuery = $state('');
	let selectedStation = $state<Station | null>(null);
	let places = $state<Place[] | null>(null);
	let placesLoading = $state(false);
	let placesError = $state('');

	async function loadPlaces(station: Station) {
		selectedStation = station;
		places = null;
		placesError = '';
		placesLoading = true;
		try {
			const result = await unwrap(
				api.GET('/api/v1/places', {
					params: { query: { near_stop_id: station.id, limit: 50 } }
				})
			);
			if (selectedStation?.id === station.id) places = result.places;
		} catch {
			if (selectedStation?.id === station.id) {
				placesError = 'Tempat sekitar gagal dimuat. Periksa koneksi atau pilih stasiun lain.';
			}
		} finally {
			if (selectedStation?.id === station.id) placesLoading = false;
		}
	}

	function clearStation() {
		places = null;
		placesLoading = false;
		placesError = '';
	}
</script>

<h1 class="sg-page-title">Jelajah</h1>
<p class="sg-meta">
	Koleksi stasiun pilihan untuk mengenal jaringan transit. Progresmu tersimpan di Paspor.
</p>
<nav class="explore-links" aria-label="Jelajah kota">
	<a href={resolve('/explore/trails')}>Jalur jalan kaki →</a>
	<a href={resolve('/explore/saved')}>Tempat tersimpan →</a>
</nav>

<section>
	<SectionHeading>Cari tempat dari stasiun</SectionHeading>
	<p class="sg-meta">Pilih stasiun sebagai titik awal untuk melihat tempat di sekitarnya.</p>
	<div class="station-search">
		<StationCombobox
			label="Cari stasiun atau halte"
			placeholder="Ketik nama stasiun atau halte"
			bind:value={stationQuery}
			bind:selected={selectedStation}
			onpick={loadPlaces}
			onclear={clearStation}
		/>
	</div>

	{#if selectedStation}
		<div class="anchor-row">
			<p class="anchor-name">Sekitar {selectedStation.name}</p>
			<a href={resolve('/stations/[id]', { id: selectedStation.id })}>Lihat info stasiun</a>
		</div>
	{/if}

	{#if placesLoading}
		<StateBlock kind="loading">Memuat tempat sekitar…</StateBlock>
	{:else if placesError}
		<StateBlock kind="error">{placesError}</StateBlock>
		{#if selectedStation}
			<Button variant="secondary" onclick={() => loadPlaces(selectedStation!)}>Coba lagi</Button>
		{/if}
	{:else if places}
		<NearbyPlaces {places} />
	{:else}
		<StateBlock kind="empty" illustration="/illustrations/empty-transit-world.webp">
			Pilih stasiun atau halte untuk melihat tempat di sekitarnya.
		</StateBlock>
	{/if}
</section>

<section>
	<SectionHeading>Koleksi stasiun</SectionHeading>
	{#if data.error}
		<StateBlock kind="error">{data.error}</StateBlock>
	{:else if data.collections.length === 0}
		<StateBlock kind="empty" illustration="/illustrations/empty-transit-world.webp">
			Belum ada koleksi — koleksi stasiun bertema sedang disiapkan.
		</StateBlock>
	{:else}
		<ul class="sg-list">
			{#each data.collections as c (c.id)}
				<li>
					<div class="coll-row">
						<span class="coll-title">{c.title}</span>
						<span class="muted coll-count">{c.itemCount} stasiun</span>
					</div>
					{#if c.description}<p class="muted desc">{c.description}</p>{/if}
					<p class="muted provenance">
						{c.kind === 'curated' ? 'Kurasi editorial' : 'Dihitung dari katalog stasiun'}
					</p>
				</li>
			{/each}
		</ul>
		<p class="sg-meta">
			Buka Paspor untuk melihat stasiun yang sudah kamu kunjungi di setiap koleksi.
		</p>
	{/if}
</section>

<style>
	.coll-row {
		display: flex;
		justify-content: space-between;
		align-items: baseline;
		gap: var(--sg-space-2);
	}
	.station-search {
		max-width: 32rem;
		margin-block-start: var(--sg-space-3);
	}
	.explore-links {
		display: flex;
		flex-wrap: wrap;
		gap: var(--sg-space-3);
		margin-block: var(--sg-space-3);
	}
	.explore-links a {
		color: var(--sg-brand);
	}
	.anchor-row {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: var(--sg-space-3);
		margin-block-start: var(--sg-space-3);
	}
	.anchor-name {
		margin: 0;
		font-weight: var(--sg-weight-medium);
	}
	.anchor-row a {
		color: var(--sg-brand);
		font-size: var(--sg-text-secondary);
		white-space: nowrap;
	}
	.coll-title {
		font-weight: var(--sg-weight-semibold);
	}
	.muted {
		color: var(--sg-text-muted);
		font-size: var(--sg-text-secondary);
	}
	.desc {
		margin-block: 0.125rem 0;
	}
	.coll-count {
		white-space: nowrap;
	}
	.provenance {
		margin-block: var(--sg-space-1) 0;
		font-size: var(--sg-text-meta);
	}
</style>
