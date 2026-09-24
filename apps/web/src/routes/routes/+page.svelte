<script lang="ts">
	import type { PageProps } from './$types';
	import { resolve } from '$app/paths';

	let { data }: PageProps = $props();
</script>

<nav><a href={resolve('/')}>← Pencarian</a></nav>
<h1>Rute</h1>

{#if data.error}
	<p role="alert">{data.error}</p>
{:else if data.routes.length === 0}
	<p>Belum ada rute yang tercatat.</p>
{:else}
	<ul>
		{#each data.routes as route (route.id)}
			<li>
				<a href={resolve('/routes/[id]', { id: route.id })}>
					{#if route.color}
						<span class="dot" style:background-color={'#' + route.color}></span>
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
	ul {
		list-style: none;
		padding: 0;
		display: flex;
		flex-direction: column;
		gap: var(--sg-space-1);
		max-width: 40rem;
	}
	a {
		display: flex;
		align-items: baseline;
		gap: var(--sg-space-2);
		color: var(--sg-text);
		text-decoration: none;
	}
	.name {
		font-weight: 600;
	}
	.muted {
		color: var(--sg-text-muted);
		font-size: 0.875rem;
	}
	.dot {
		display: inline-block;
		width: 0.75rem;
		height: 0.75rem;
		border-radius: 50%;
		align-self: center;
	}
</style>
