<script lang="ts">
	import type { PageProps } from './$types';
	import { resolve } from '$app/paths';
	import { browser } from '$app/environment';
	import BackNav from '$lib/components/app-shell/BackNav.svelte';
	import DepartureBoard from '$lib/components/station/DepartureBoard.svelte';
	import { basemapStyleUrl } from '$lib/basemap';
	import { facilityLabel } from '$lib/facilities';
	import { SectionHeading, StateBlock } from '@singgah/ui';
	import { TransitMap, stationsToGeoJSON } from '@singgah/map';

	const STYLE_URL = basemapStyleUrl();

	let { data }: PageProps = $props();
	const station = $derived(data.station);
	const departures = $derived(data.departures);
</script>

<svelte:head><title>{station?.name ?? 'Stasiun'} · Singgah</title></svelte:head>

{#if station === null}
	<StateBlock kind="error">{data.error}</StateBlock>
	<BackNav href="/" label="Kembali ke beranda" />
{:else}
	<BackNav href="/" label="Beranda" />

	<h1 class="sg-page-title">{station.name}</h1>
	{#if station.officialName && station.officialName !== station.name}
		<p class="sg-meta">{station.officialName}</p>
	{/if}
	<p class="sg-meta">
		{#if station.code}<span>{station.code}</span> ·
		{/if}
		<span>{station.kind}</span> ·
		<span>{station.lat.toFixed(5)}, {station.lon.toFixed(5)}</span>
	</p>

	{#if browser}
		<div class="mini-map">
			<TransitMap
				styleUrl={STYLE_URL}
				data={stationsToGeoJSON([station])}
				center={[station.lon, station.lat]}
				zoom={14}
				interactive={false}
				fitToData
			/>
		</div>
	{/if}

	<section>
		<SectionHeading>Rute yang melayani</SectionHeading>
		{#if station.lines.length === 0}
			<StateBlock kind="empty">Belum ada rute tercatat untuk stasiun ini.</StateBlock>
		{:else}
			<ul class="sg-list">
				{#each station.lines as line (line.id)}
					<li>
						<a href={resolve('/routes/[id]', { id: line.id })}>
							{#if line.color}
								<span class="sg-dot" style:background-color={'#' + line.color}></span>
							{/if}
							{line.shortName || line.longName}
							{#if line.agencyName}<span class="muted">· {line.agencyName}</span>{/if}
						</a>
					</li>
				{/each}
			</ul>
		{/if}
	</section>

	<section>
		<SectionHeading>Jadwal keberangkatan</SectionHeading>
		{#if departures === null}
			<StateBlock kind="error">Jadwal keberangkatan sedang tidak tersedia.</StateBlock>
		{:else if departures.lines.length === 0}
			<StateBlock kind="empty">Belum ada jadwal tercatat untuk stasiun ini.</StateBlock>
		{:else}
			<DepartureBoard lines={departures.lines} />
			<p class="sg-meta">
				Jadwal statis {departures.source.provider} — bukan posisi live.
			</p>
		{/if}
	</section>

	<section>
		<SectionHeading>Transit</SectionHeading>
		{#if station.transfers.length === 0}
			<StateBlock kind="empty">Tidak ada transfer tercatat.</StateBlock>
		{:else}
			<ul class="sg-list">
				{#each station.transfers as transfer (transfer.toStop.id)}
					<li>
						<a href={resolve('/stations/[id]', { id: transfer.toStop.id })}>
							{transfer.toStop.name}
							{#if transfer.walkDistanceM}
								<span class="muted">· jalan {transfer.walkDistanceM} m</span>
							{/if}
							{#if transfer.notes}<span class="muted">· {transfer.notes}</span>{/if}
						</a>
					</li>
				{/each}
			</ul>
		{/if}
	</section>

	{#if station.facilities.length > 0}
		<section>
			<SectionHeading>Fasilitas</SectionHeading>
			<ul class="sg-list">
				{#each station.facilities as f (f.type + f.text)}
					<li>
						{facilityLabel(f.type)}
						{#if f.text}<span class="muted">· {f.text}</span>{/if}
						{#if f.accessibilityRelevant}<span class="muted">· relevan aksesibilitas</span>{/if}
					</li>
				{/each}
			</ul>
			<p class="sg-meta">
				Keberadaan fasilitas dilaporkan penyedia data — bukan jaminan berfungsi.
			</p>
		</section>
	{/if}

	<p class="sg-meta">
		Sumber: {station.source.provider}
		{#if station.source.fetchedAt}
			· diambil {new Date(station.source.fetchedAt).toLocaleString('id-ID')}
		{/if}
	</p>
{/if}

<style>
	.muted {
		color: var(--sg-text-muted);
		font-size: var(--sg-text-secondary);
	}
	.mini-map {
		height: 14rem;
		max-width: 32rem;
		border: 1px solid var(--sg-border);
		border-radius: var(--sg-radius-card);
		overflow: hidden;
	}
</style>
