<script lang="ts">
	import type { PageProps } from './$types';
	import { resolve } from '$app/paths';
	import BackNav from '$lib/components/app-shell/BackNav.svelte';

	let { data }: PageProps = $props();
</script>

<BackNav href="/" label="Pencarian" />
<h1 class="sg-page-title">Rute</h1>

{#if data.error}
	<p class="sg-error" role="alert">{data.error}</p>
{:else if data.routes.length === 0}
	<p class="sg-meta">Belum ada rute yang tercatat.</p>
{:else}
	<ul class="sg-list">
		{#each data.routes as route (route.id)}
			<li>
				<a href={resolve('/routes/[id]', { id: route.id })}>
					{#if route.color}
						<span class="sg-dot" style:background-color={'#' + route.color}></span>
					{/if}
					<span class="name">{route.shortName || route.longName}</span>
					{#if route.longName && route.shortName}<span class="muted">{route.longName}</span>{/if}
					<span class="muted">· {route.agencyName || route.agencyCode}</span>
				</a>
			</li>
		{/each}
	</ul>
{/if}

<style>
	.name {
		font-weight: var(--sg-weight-semibold);
	}
	.muted {
		color: var(--sg-text-muted);
		font-size: var(--sg-text-secondary);
	}
</style>
