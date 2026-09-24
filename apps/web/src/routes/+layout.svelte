<script lang="ts">
	import './layout.css';
	import favicon from '$lib/assets/favicon.svg';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import { onMount } from 'svelte';

	let { children } = $props();

	// navigator.onLine only exists client-side; start optimistic and sync on
	// mount so SSR markup never mismatches.
	let online = $state(true);
	onMount(() => {
		online = navigator.onLine;
	});

	const NAV = [
		{ href: '/plan', label: 'Perjalanan' },
		{ href: '/map', label: 'Peta' },
		{ href: '/routes', label: 'Rute' },
		{ href: '/providers', label: 'Penyedia' }
	] as const;
</script>

<svelte:window ononline={() => (online = true)} onoffline={() => (online = false)} />

<svelte:head><link rel="icon" href={favicon} /></svelte:head>

<a class="skip-link" href="#content">Langsung ke konten</a>

<header class="app-header">
	<a class="brand" href={resolve('/')} aria-label="Singgah — beranda">Singgah</a>
	<nav aria-label="Navigasi utama">
		{#each NAV as item (item.href)}
			<a
				href={resolve(item.href)}
				aria-current={page.url.pathname.startsWith(item.href) ? 'page' : undefined}
			>
				{item.label}
			</a>
		{/each}
	</nav>
</header>

<main id="content">
	{@render children()}
</main>

{#if !online}
	<div class="offline-banner" role="status">
		Tidak ada koneksi — data yang tampil mungkin tersimpan lama.
	</div>
{/if}

<style>
	.app-header {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: var(--sg-space-3);
		padding: var(--sg-space-2) var(--sg-space-3);
		background-color: var(--sg-surface);
		border-bottom: 1px solid var(--sg-border);
	}

	.brand {
		font-family: var(--sg-font-display);
		font-weight: var(--sg-weight-bold);
		color: var(--sg-text);
		text-decoration: none;
	}

	nav {
		display: flex;
		gap: var(--sg-space-1);
		flex-wrap: wrap;
	}

	nav a {
		padding: var(--sg-space-2) var(--sg-space-2);
		border-radius: var(--sg-radius-button);
		color: var(--sg-text-muted);
		font-size: var(--sg-text-secondary);
		text-decoration: none;
	}

	nav a:hover {
		background-color: var(--sg-surface-muted);
		color: var(--sg-text);
	}

	nav a[aria-current='page'] {
		color: var(--sg-brand);
		font-weight: var(--sg-weight-semibold);
	}

	main {
		margin-inline: auto;
		max-width: 72rem;
		width: 100%;
		padding: var(--sg-space-4) var(--sg-space-3);
	}

	.offline-banner {
		position: fixed;
		inset-inline: 0;
		bottom: var(--sg-space-4);
		margin-inline: auto;
		width: fit-content;
		max-width: calc(100vw - var(--sg-space-6));
		padding: var(--sg-space-2) var(--sg-space-4);
		border: 1px solid var(--sg-border);
		border-radius: var(--sg-radius-pill);
		background-color: var(--sg-surface);
		box-shadow: var(--sg-shadow-overlay);
		color: var(--sg-warning);
		font-size: var(--sg-text-secondary);
		z-index: var(--sg-z-toast);
	}

	.skip-link {
		position: absolute;
		left: var(--sg-space-3);
		top: -100%;
		z-index: 10;
		padding: var(--sg-space-2) var(--sg-space-3);
		border-radius: var(--sg-radius-button);
		background: var(--sg-brand);
		color: #fff;
	}

	.skip-link:focus {
		top: var(--sg-space-2);
	}
</style>
