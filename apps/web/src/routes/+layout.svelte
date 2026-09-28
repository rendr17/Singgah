<script lang="ts">
	import './layout.css';
	import favicon from '$lib/assets/favicon.svg';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import { installQueueDriver } from '$lib/mutation-queue';
	import { onMount } from 'svelte';

	let { children } = $props();

	// navigator.onLine only exists client-side; start optimistic and sync on
	// mount so SSR markup never mismatches.
	let online = $state(true);
	onMount(() => {
		online = navigator.onLine;
		return installQueueDriver();
	});

	// Primary nav per docs/05: compact gets a bottom bar, wider layouts a rail.
	// Icons are 24x24 stroke glyphs; operator colors stay out of chrome.
	const NAV = [
		{
			href: '/',
			label: 'Beranda',
			icon: ['M3 9l9-7 9 7v11a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z', 'M9 22V12h6v10']
		},
		{ href: '/plan', label: 'Perjalanan', icon: ['M3 11l19-9-9 19-2-8-8-2z'] },
		{
			href: '/map',
			label: 'Peta',
			icon: ['M1 6v16l7-4 8 4 7-4V2l-7 4-8-4-7 4z', 'M8 2v16', 'M16 6v16']
		},
		{
			href: '/explore',
			label: 'Jelajah',
			icon: [
				'M12 22a10 10 0 1 0 0-20 10 10 0 0 0 0 20z',
				'M16.24 7.76l-2.12 6.36-6.36 2.12 2.12-6.36z'
			]
		},
		{
			href: '/passport',
			label: 'Paspor',
			icon: [
				'M4 19.5A2.5 2.5 0 0 1 6.5 17H20',
				'M6.5 2H20v20H6.5A2.5 2.5 0 0 1 4 19.5v-15A2.5 2.5 0 0 1 6.5 2z'
			]
		}
	] as const;
</script>

<svelte:window ononline={() => (online = true)} onoffline={() => (online = false)} />

<svelte:head><link rel="icon" href={favicon} /></svelte:head>

<a class="skip-link" href="#content">Langsung ke konten</a>

<nav class="app-nav" aria-label="Navigasi utama">
	<a class="brand brand--rail" href={resolve('/')} aria-label="Singgah — beranda">Singgah</a>
	{#each NAV as item (item.href)}
		{@const current =
			item.href === '/' ? page.url.pathname === '/' : page.url.pathname.startsWith(item.href)}
		<a href={resolve(item.href)} aria-current={current ? 'page' : undefined}>
			<svg
				viewBox="0 0 24 24"
				fill="none"
				stroke="currentColor"
				stroke-width="2"
				stroke-linecap="round"
				stroke-linejoin="round"
				aria-hidden="true"
			>
				{#each item.icon as d (d)}
					<path {d} />
				{/each}
			</svg>
			<span>{item.label}</span>
		</a>
	{/each}
</nav>

<div class="page">
	<header class="app-header">
		<a class="brand" href={resolve('/')} aria-label="Singgah — beranda">Singgah</a>
	</header>

	<main id="content">
		{@render children()}
	</main>
</div>

{#if !online}
	<div class="offline-banner" role="status">
		Tidak ada koneksi — data yang tampil mungkin tersimpan lama.
	</div>
{/if}

<style>
	/* Shared between the fixed nav, main's clearance and the banner offset. */
	:global(:root) {
		--nav-h: 3.5rem;
		--nav-w: 10.5rem;
	}

	.app-header {
		display: flex;
		align-items: center;
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

	/* Compact shows the brand in the top header; the rail carries its own. */
	.brand--rail {
		display: none;
	}

	/* Compact (<48rem): bottom bar — docs/05, docs/10. */
	.app-nav {
		position: fixed;
		inset-inline: 0;
		bottom: 0;
		display: flex;
		min-height: calc(var(--nav-h) + env(safe-area-inset-bottom, 0px));
		padding-bottom: env(safe-area-inset-bottom, 0px);
		background-color: var(--sg-surface);
		border-top: 1px solid var(--sg-border);
		z-index: var(--sg-z-sticky);
	}

	.app-nav a {
		flex: 1;
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		gap: 2px;
		min-height: var(--sg-target-min);
		padding: var(--sg-space-1);
		color: var(--sg-text-muted);
		font-size: var(--sg-text-meta);
		line-height: var(--sg-leading-tight);
		text-decoration: none;
	}

	.app-nav svg {
		inline-size: 1.375rem;
		block-size: 1.375rem;
	}

	.app-nav a[aria-current='page'] {
		color: var(--sg-brand);
		font-weight: var(--sg-weight-semibold);
	}

	main {
		margin-inline: auto;
		max-width: 72rem;
		width: 100%;
		padding: var(--sg-space-4) var(--sg-space-3);
		/* clear the fixed bottom bar */
		padding-bottom: calc(var(--nav-h) + var(--sg-space-4));
	}

	/* Wide (>=48rem): left sidebar — docs/05, docs/10. The rail is fixed, so
	   .page shifts right and main keeps centering in the remaining space. */
	@media (min-width: 48rem) {
		.app-header {
			display: none;
		}

		.app-nav {
			inset-block: 0;
			inset-inline-start: 0;
			inset-inline-end: auto;
			width: var(--nav-w);
			min-height: 0;
			flex-direction: column;
			align-items: stretch;
			border-top: 0;
			border-inline-end: 1px solid var(--sg-border);
			padding: var(--sg-space-4) var(--sg-space-2) var(--sg-space-2);
			gap: var(--sg-space-1);
		}

		.app-nav .brand--rail {
			display: block;
			padding: var(--sg-space-1) var(--sg-space-2) var(--sg-space-4);
			font-size: var(--sg-text-section);
			font-weight: var(--sg-weight-bold);
			color: var(--sg-text);
		}

		.app-nav a {
			flex: none;
			flex-direction: row;
			justify-content: flex-start;
			gap: var(--sg-space-2);
			padding: var(--sg-space-2);
			border-radius: var(--sg-radius-button);
			font-size: var(--sg-text-secondary);
		}

		.app-nav a:hover {
			background-color: var(--sg-surface-muted);
		}

		.page {
			margin-inline-start: var(--nav-w);
		}

		main {
			padding-bottom: var(--sg-space-4);
		}

		.offline-banner {
			inset-inline-start: var(--nav-w);
			bottom: var(--sg-space-4);
		}
	}

	/* Workspace pages (docs/10 wide) need more canvas than the text column. */
	@media (min-width: 75rem) {
		main {
			max-width: 96rem;
		}
	}

	.offline-banner {
		position: fixed;
		inset-inline: 0;
		/* sits above the bottom bar on compact */
		bottom: calc(var(--nav-h) + env(safe-area-inset-bottom, 0px) + var(--sg-space-2));
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
