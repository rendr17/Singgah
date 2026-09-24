<script lang="ts">
	import type { PageProps } from './$types';
	import { resolve } from '$app/paths';
	import { browser } from '$app/environment';
	import { env } from '$env/dynamic/public';
	import BackNav from '$lib/components/app-shell/BackNav.svelte';
	import { SectionHeading } from '@singgah/ui';
	import { TransitMap, stationsToGeoJSON } from '@singgah/map';

	const STYLE_URL = env.PUBLIC_MAP_STYLE_URL ?? 'https://tiles.openfreemap.org/styles/positron';

	let { data }: PageProps = $props();
	const station = $derived(data.station);

	// Human labels for the provider-declared amenity types (contract: lowercase
	// snake). Unknown future types fall back to the raw type string.
	const FACILITY_LABELS: Record<string, string> = {
		toilet: 'Toilet',
		toilet_accessible: 'Toilet aksesibel',
		praying_room: 'Musala',
		nursing_room: 'Ruang menyusui',
		elevator_paid: 'Lift (area berbayar)',
		elevator_unpaid: 'Lift (area gratis)',
		escalator_paid: 'Eskalator (area berbayar)',
		escalator_unpaid: 'Eskalator (area gratis)',
		parking: 'Parkir',
		bike_parking: 'Parkir sepeda',
		lockers: 'Loker',
		charging_station: 'Stasiun pengisian daya'
	};
</script>

{#if station === null}
	<p role="alert">{data.error}</p>
	<BackNav href="/" label="Kembali ke pencarian" />
{:else}
	<BackNav href="/" label="Pencarian" />

	<h1>{station.name}</h1>
	{#if station.officialName && station.officialName !== station.name}
		<p class="official">{station.officialName}</p>
	{/if}
	<p class="meta">
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
			<p>Belum ada rute tercatat untuk stasiun ini.</p>
		{:else}
			<ul>
				{#each station.lines as line (line.id)}
					<li>
						<a href={resolve('/routes/[id]', { id: line.id })}>
							{#if line.color}
								<span class="dot" style:background-color={'#' + line.color}></span>
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
		<SectionHeading>Transit</SectionHeading>
		{#if station.transfers.length === 0}
			<p>Tidak ada transfer tercatat.</p>
		{:else}
			<ul>
				{#each station.transfers as transfer (transfer.toStop.id)}
					<li>
						<a href={resolve('/stations/[id]', { id: transfer.toStop.id })}
							>{transfer.toStop.name}</a
						>
						{#if transfer.walkDistanceM}
							<span class="muted">· jalan {transfer.walkDistanceM} m</span>
						{/if}
						{#if transfer.notes}<span class="muted">· {transfer.notes}</span>{/if}
					</li>
				{/each}
			</ul>
		{/if}
	</section>

	{#if station.facilities.length > 0}
		<section>
			<SectionHeading>Fasilitas</SectionHeading>
			<ul>
				{#each station.facilities as f (f.type + f.text)}
					<li>
						{FACILITY_LABELS[f.type] ?? f.type}
						{#if f.text}<span class="muted">· {f.text}</span>{/if}
						{#if f.accessibilityRelevant}<span class="muted">· relevan aksesibilitas</span>{/if}
					</li>
				{/each}
			</ul>
			<p class="muted">Keberadaan fasilitas dilaporkan penyedia data — bukan jaminan berfungsi.</p>
		</section>
	{/if}

	<p class="source">
		Sumber: {station.source.provider}
		{#if station.source.fetchedAt}
			· diambil {new Date(station.source.fetchedAt).toLocaleString('id-ID')}
		{/if}
	</p>
{/if}

<style>
	.official {
		color: var(--sg-text-muted);
	}
	.meta,
	.source,
	.muted {
		color: var(--sg-text-muted);
		font-size: 0.875rem;
	}
	ul {
		list-style: none;
		padding: 0;
		display: flex;
		flex-direction: column;
		gap: var(--sg-space-1);
	}
	a {
		color: var(--sg-text);
	}
	.mini-map {
		height: 14rem;
		max-width: 32rem;
		border: 1px solid var(--sg-border);
		border-radius: var(--sg-radius-card, 12px);
		overflow: hidden;
	}
	.dot {
		display: inline-block;
		width: 0.75rem;
		height: 0.75rem;
		border-radius: 50%;
		margin-inline-end: var(--sg-space-1);
	}
</style>
