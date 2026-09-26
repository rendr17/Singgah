<script lang="ts">
	import type { PageProps } from './$types';
	import { resolve } from '$app/paths';
	import BackNav from '$lib/components/app-shell/BackNav.svelte';
	import { SectionHeading, StateBlock } from '@singgah/ui';

	let { data }: PageProps = $props();
	const route = $derived(data.route);
</script>

<svelte:head
	><title>{route ? `${route.shortName || route.longName} · Singgah` : 'Rute · Singgah'}</title
	></svelte:head
>

{#if route === null}
	<StateBlock kind="error">{data.error}</StateBlock>
	<BackNav href="/routes" label="Semua rute" />
{:else}
	<BackNav href="/routes" label="Semua rute" />

	<h1 class="sg-page-title">
		{#if route.color}
			<span class="sg-dot" style:background-color={'#' + route.color}></span>
		{/if}
		{route.shortName || route.longName}
	</h1>
	<p class="sg-meta">
		{#if route.longName && route.shortName}{route.longName} ·
		{/if}
		{route.mode}{#if route.agencyName}
			· {route.agencyName}{/if}
	</p>

	<section>
		<SectionHeading>Stasiun yang dilayani</SectionHeading>
		{#if route.stops.length === 0}
			<StateBlock kind="empty">Belum ada stasiun tercatat untuk rute ini.</StateBlock>
		{:else}
			<ul class="sg-list">
				{#each route.stops as stop (stop.seq)}
					<li>
						<a href={resolve('/stations/[id]', { id: stop.id })}>
							{#if stop.stationNumber}<span class="num muted">{stop.stationNumber}</span>{/if}
							{stop.name}
							{#if stop.code}<span class="muted">· {stop.code}</span>{/if}
						</a>
					</li>
				{/each}
			</ul>
		{/if}
	</section>

	<p class="sg-meta source">
		Sumber: {route.source.provider}
		{#if route.source.fetchedAt}
			· diambil {new Date(route.source.fetchedAt).toLocaleString('id-ID')}
		{/if}
	</p>
{/if}

<style>
	.muted {
		color: var(--sg-text-muted);
		font-size: var(--sg-text-secondary);
	}
	.num {
		display: inline-block;
		min-width: 2.25rem;
		font-variant-numeric: tabular-nums;
	}
</style>
