<script lang="ts">
	import './layout.css';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import { installQueueDriver } from '$lib/mutation-queue';
	import { onMount } from 'svelte';

	let { children } = $props();
	const isLanding = $derived(page.url.pathname === '/');
	const isMapHome = $derived(page.url.pathname === '/app');

	// navigator.onLine only exists client-side; start optimistic and sync on
	// mount so SSR markup never mismatches.
	let online = $state(true);
	onMount(() => {
		online = navigator.onLine;
		return installQueueDriver();
	});

	// The map workspace keeps bottom navigation at every viewport size.
	// Icons are 24x24 stroke glyphs; operator colors stay out of chrome.
	const NAV = [
		{
			href: '/app',
			panel: '',
			label: 'Beranda',
			icon: 'station'
		},
		{ href: '/app?panel=plan', panel: 'plan', label: 'Perjalanan', icon: 'transfer' },
		{
			href: '/app?panel=explore',
			panel: 'explore',
			label: 'Jelajah',
			icon: 'explore'
		},
		{
			href: '/app?panel=passport',
			panel: 'passport',
			label: 'Paspor',
			icon: 'passport'
		}
	] as const;
</script>

<svelte:window ononline={() => (online = true)} onoffline={() => (online = false)} />

<svelte:head><link rel="icon" href="/brand/app-icon-blue.svg" type="image/svg+xml" /></svelte:head>

<a class="skip-link" href="#content">Langsung ke konten</a>

{#if !isLanding}
	<nav class="app-nav" class:app-nav--map-home={isMapHome} aria-label="Navigasi utama">
		<a class="brand brand--rail" href={resolve('/app')} aria-label="Singgah — beranda">
			<img src="/brand/app-icon-blue.svg" alt="" class="brand__icon" width="28" height="28" />
			<span class="brand__word">Singgah</span>
		</a>
		{#each NAV as item (item.href)}
			{@const panel = page.url.searchParams.get('panel') ?? ''}
			{@const current = isMapHome
				? item.panel === 'plan'
					? ['plan', 'journey'].includes(panel)
					: item.panel
						? panel === item.panel
						: !['plan', 'journey', 'explore', 'passport'].includes(panel)
				: item.panel === 'plan'
					? page.url.pathname === '/plan' || page.url.pathname === '/journey'
					: item.panel === 'explore'
						? page.url.pathname === '/explore'
						: item.panel === 'passport'
							? page.url.pathname === '/passport'
							: page.url.pathname === '/app'}
			<a
				href={resolve(item.href as '/app' | `/app?${string}`)}
				aria-current={current ? 'page' : undefined}
			>
				<svg
					viewBox="0 0 64 64"
					fill="none"
					stroke="currentColor"
					stroke-width="4"
					stroke-linecap="round"
					stroke-linejoin="round"
					aria-hidden="true"
				>
					<use href="/icons/transit.svg#{item.icon}" />
				</svg>
				<span>{item.label}</span>
			</a>
		{/each}
	</nav>
{/if}

<div class="page" class:page--landing={isLanding} class:page--map-home={isMapHome}>
	{#if !isLanding}
		<header class="app-header" class:app-header--map-home={isMapHome}>
			<a class="brand" href={resolve('/app')} aria-label="Singgah — beranda">
				<img src="/brand/app-icon-blue.svg" alt="" class="brand__icon" width="26" height="26" />
				<span class="brand__word">Singgah</span>
			</a>
		</header>
	{/if}

	<main id="content" class:main--landing={isLanding} class:main--map-home={isMapHome}>
		{@render children()}
	</main>
</div>

{#if !online && !isLanding}
	<div class="offline-banner" class:offline-banner--map-home={isMapHome} role="status">
		Tidak ada koneksi — data yang tampil mungkin tersimpan lama.
	</div>
{/if}

<style>
	/* Shared between the fixed nav, main's clearance and the banner offset. */
	:global(:root) {
		--nav-h: 3.75rem;
		--nav-w: 11rem;
	}

	.app-header {
		position: sticky;
		top: 0;
		z-index: var(--sg-z-sticky);
		display: flex;
		align-items: center;
		gap: var(--sg-space-3);
		min-height: 4.25rem;
		padding: var(--sg-space-2) clamp(var(--sg-space-3), 4vw, var(--sg-space-6));
		background-color: var(--sg-canvas);
		border-bottom: 1px solid var(--sg-border);
	}

	.brand {
		display: inline-flex;
		align-items: center;
		gap: var(--sg-space-2);
		font-family: var(--sg-font-display);
		font-weight: var(--sg-weight-semibold);
		font-size: var(--sg-text-row);
		letter-spacing: -0.04em;
		color: var(--sg-text);
		text-decoration: none;
	}

	.brand__icon {
		display: block;
		height: 1.625rem;
		width: 1.625rem;
		border-radius: var(--sg-radius-button);
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
		background-color: var(--sg-canvas);
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
		transition:
			background-color var(--sg-motion-fast) var(--sg-ease-standard),
			color var(--sg-motion-fast) var(--sg-ease-standard);
	}

	.app-nav svg {
		inline-size: 1.375rem;
		block-size: 1.375rem;
	}

	.app-nav a[aria-current='page'] {
		color: var(--sg-text);
		font-weight: var(--sg-weight-semibold);
		background: var(--sg-brand-subtle);
		box-shadow: inset 0 2px var(--sg-brand);
	}

	main {
		margin-inline: auto;
		max-width: 58rem;
		width: 100%;
		padding: clamp(var(--sg-space-4), 4vw, var(--sg-space-8))
			clamp(var(--sg-space-3), 4vw, var(--sg-space-8));
		/* clear the fixed bottom bar */
		padding-bottom: calc(var(--nav-h) + var(--sg-space-4));
	}

	main.main--landing {
		max-width: none;
		padding: 0;
	}

	.app-header--map-home {
		display: none;
	}
	main.main--map-home {
		max-width: none;
		margin: 0;
		padding: 0;
	}
	.page--map-home {
		margin-inline-start: 0;
	}
	.app-nav--map-home {
		z-index: var(--sg-z-toast);
	}

	/* Other wide pages use a fixed rail; the map workspace overrides it with
	   its floating bottom navigation. */
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
			font-weight: var(--sg-weight-semibold);
			color: var(--sg-text);
		}

		.brand--rail .brand__icon {
			height: 1.75rem;
			width: 1.75rem;
		}

		.brand--rail .brand__word {
			font-size: var(--sg-text-section);
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

		.app-nav a[aria-current='page'] {
			box-shadow: inset 2px 0 var(--sg-brand);
		}

		.app-nav .brand--rail {
			border-radius: var(--sg-radius-button);
		}

		.page {
			margin-inline-start: var(--nav-w);
		}

		main {
			padding-bottom: var(--sg-space-4);
		}

		.page--landing {
			margin-inline-start: 0;
		}
		.page--map-home {
			margin-inline-start: 0;
		}

		.app-nav--map-home {
			inset-block: auto;
			inset-inline-start: 50%;
			inset-inline-end: auto;
			bottom: calc(var(--sg-space-3) + env(safe-area-inset-bottom, 0px));
			transform: translateX(-50%);
			width: min(34rem, calc(100vw - 2rem));
			min-height: var(--nav-h);
			flex-direction: row;
			align-items: center;
			padding: var(--sg-space-1);
			gap: 0;
			border: 1px solid var(--sg-border);
			border-radius: var(--sg-radius-card);
			box-shadow: var(--sg-shadow-overlay);
		}
		.app-nav--map-home .brand--rail {
			display: none;
		}
		.app-nav--map-home a {
			flex: 1;
			flex-direction: column;
			justify-content: center;
			gap: 2px;
			padding: var(--sg-space-1);
			font-size: var(--sg-text-meta);
		}

		.app-nav--map-home a[aria-current='page'] {
			border-radius: var(--sg-radius-button);
		}
		.offline-banner--map-home {
			inset-inline-start: 0;
		}

		.offline-banner {
			inset-inline-start: var(--nav-w);
			bottom: var(--sg-space-4);
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
		border-radius: var(--sg-radius-card);
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
