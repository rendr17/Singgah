<script lang="ts">
	import type { PageProps } from './$types';
	import { resolve } from '$app/paths';
	import BackNav from '$lib/components/app-shell/BackNav.svelte';
	import { SectionHeading } from '@singgah/ui';

	let { data }: PageProps = $props();
	const route = $derived(data.route);
</script>

{#if route === null}
	<p role="alert">{data.error}</p>
	<BackNav href="/routes" label="Semua rute" />
{:else}
	<BackNav href="/routes" label="Semua rute" />

	<h1>
		{#if route.color}
			<span class="dot" style:background-color={'#' + route.color}></span>
		{/if}
		{route.shortName || route.longName}
	</h1>
	<p class="meta">
		{#if route.longName && route.shortName}{route.longName} ·
		{/if}
		{route.mode}{#if route.agencyName}
			· {route.agencyName}{/if}
	</p>

	<section>
		<SectionHeading>Stasiun yang dilayani</SectionHeading>
		{#if route.stops.length === 0}
			<p>Belum ada stasiun tercatat untuk rute ini.</p>
		{:else}
			<ul>
				{#each route.stops as stop (stop.id)}
					<li>
						{#if stop.stationNumber}<span class="num muted">{stop.stationNumber}</span>{/if}
						<a href={resolve('/stations/[id]', { id: stop.id })}>{stop.name}</a>
						{#if stop.code}<span class="muted">· {stop.code}</span>{/if}
					</li>
				{/each}
			</ul>
		{/if}
	</section>

	<p class="source">
		Sumber: {route.source.provider}
		{#if route.source.fetchedAt}
			· diambil {new Date(route.source.fetchedAt).toLocaleString('id-ID')}
		{/if}
	</p>
{/if}

<style>
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
	.num {
		display: inline-block;
		min-width: 2.25rem;
		font-variant-numeric: tabular-nums;
	}
	.dot {
		display: inline-block;
		width: 0.875rem;
		height: 0.875rem;
		border-radius: 50%;
		margin-inline-end: var(--sg-space-1);
	}
</style>
