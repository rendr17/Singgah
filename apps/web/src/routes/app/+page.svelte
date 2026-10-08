<script lang="ts">
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import { onMount } from 'svelte';
	import type { PageProps } from './$types';
	import SheetSizeControls, {
		type MapSheetSize
	} from '$lib/components/map/SheetSizeControls.svelte';

	let { data }: PageProps = $props();
	let mapError = $state(false);
	let MapPage = $state<(typeof import('./MapWorkspace.svelte'))['default'] | null>(null);
	let ExplorePage = $state<(typeof import('../explore/+page.svelte'))['default'] | null>(null);
	let PassportPage = $state<(typeof import('../passport/+page.svelte'))['default'] | null>(null);
	let panelSize = $state<MapSheetSize>('medium');
	const activePanel = $derived(page.url.searchParams.get('panel') ?? '');
	const overlayHeightVh = $derived(
		activePanel === 'explore' || activePanel === 'passport'
			? { peek: 38, medium: 56, expanded: 86 }[panelSize]
			: 0
	);

	onMount(() => {
		void import('./MapWorkspace.svelte')
			.then((module) => (MapPage = module.default))
			.catch(() => (mapError = true));
	});

	$effect(() => {
		if (activePanel === 'explore' && !ExplorePage) {
			void import('../explore/+page.svelte').then((module) => {
				if (page.url.searchParams.get('panel') === 'explore') ExplorePage = module.default;
			});
		}
		if (activePanel === 'passport' && !PassportPage) {
			void import('../passport/+page.svelte').then((module) => {
				if (page.url.searchParams.get('panel') === 'passport') PassportPage = module.default;
			});
		}
	});
</script>

<svelte:head>
	<title>
		{activePanel === 'plan' || activePanel === 'journey'
			? 'Perjalanan'
			: activePanel === 'explore'
				? 'Jelajah'
				: activePanel === 'passport'
					? 'Paspor'
					: 'Beranda'} · Singgah
	</title>
</svelte:head>

<div class="map-home">
	{#if MapPage}
		<MapPage {overlayHeightVh} />
	{:else if mapError}
		<p class="map-load-state" role="alert">
			Peta gagal dimuat. Muat ulang halaman untuk mencoba lagi.
		</p>
	{:else}
		<p class="map-load-state" role="status">Menyiapkan peta…</p>
	{/if}

	{#if activePanel === 'explore' || activePanel === 'passport'}
		<aside
			class="map-panel"
			class:map-panel--explore={activePanel === 'explore'}
			class:map-panel--passport={activePanel === 'passport'}
			class:map-panel--peek={panelSize === 'peek'}
			class:map-panel--expanded={panelSize === 'expanded'}
			aria-label={activePanel === 'explore' ? 'Jelajah' : 'Paspor'}
		>
			<header class="map-panel__head">
				<span class="map-panel__label"
					>SINGGAH / {activePanel === 'explore' ? 'JELAJAH' : 'PASPOR'}</span
				>
				<SheetSizeControls size={panelSize} onChange={(size) => (panelSize = size)} />
				<a href={resolve('/app')} aria-label="Tutup panel">
					<svg
						viewBox="0 0 24 24"
						width="20"
						height="20"
						fill="none"
						stroke="currentColor"
						stroke-width="2"
						stroke-linecap="round"
						aria-hidden="true"
					>
						<path d="M18 6 6 18M6 6l12 12" />
					</svg>
				</a>
			</header>
			<div class="map-panel__content">
				{#if activePanel === 'explore'}
					{#if ExplorePage}
						<ExplorePage {data} params={{}} form={null} />
						<a class="map-panel__next" href={resolve('/app?panel=passport' as `/app?${string}`)}>
							Lihat progres di Paspor <span aria-hidden="true">↗</span>
						</a>
					{:else}<p role="status">Memuat Jelajah…</p>{/if}
				{:else if PassportPage}
					<PassportPage />
					<a class="map-panel__next" href={resolve('/app?panel=explore' as `/app?${string}`)}>
						Jelajahi koleksi stasiun <span aria-hidden="true">↗</span>
					</a>
				{:else}
					<p role="status">Memuat Paspor…</p>
				{/if}
			</div>
		</aside>
	{/if}
</div>

<style>
	.map-home {
		min-height: 100dvh;
	}

	.map-load-state {
		position: fixed;
		inset: 0;
		display: grid;
		place-items: center;
		margin: 0;
		background: var(--sg-surface-muted);
		color: var(--sg-text-muted);
		z-index: 1;
	}

	.map-panel {
		position: fixed;
		inset: auto 0 calc(var(--nav-h) + env(safe-area-inset-bottom, 0px));
		z-index: var(--sg-z-overlay);
		display: flex;
		flex-direction: column;
		min-height: 0;
		height: 56dvh;
		max-height: calc(100dvh - var(--nav-h) - env(safe-area-inset-bottom, 0px));
		background: var(--sg-surface);
		border: 1px solid var(--sg-border);
		border-bottom: 0;
		border-radius: var(--sg-radius-sheet) var(--sg-radius-sheet) 0 0;
		box-shadow: var(--sg-shadow-sheet);
		overflow: hidden;
		transition: height var(--sg-motion-panel) var(--sg-ease-standard);
	}

	.map-panel--peek {
		height: 38dvh;
	}

	.map-panel--expanded {
		height: 86dvh;
	}

	@media (prefers-reduced-motion: no-preference) {
		.map-panel {
			animation: sg-panel-enter var(--sg-motion-panel) var(--sg-ease-enter) both;
		}
	}

	.map-panel__head {
		flex: none;
		display: flex;
		align-items: center;
		justify-content: space-between;
		min-height: 3.5rem;
		padding-inline: var(--sg-space-4);
		border-bottom: 1px solid var(--sg-border);
	}

	.map-panel__label {
		color: var(--sg-text-muted);
		font-family: var(--sg-font-mono);
		font-size: var(--sg-text-meta);
		letter-spacing: 0.12em;
	}

	.map-panel__head a {
		display: grid;
		place-items: center;
		min-width: var(--sg-target-min);
		min-height: var(--sg-target-min);
		margin-inline-end: calc(-1 * var(--sg-space-2));
		color: var(--sg-text-muted);
	}

	.map-panel__content {
		min-height: 0;
		padding: clamp(1.25rem, 3vw, 2rem);
		overflow: auto;
	}

	.map-panel__content :global(.sg-page-title) {
		margin: 0 0 var(--sg-space-2);
		font-size: clamp(2.125rem, 4vw, 2.75rem);
		line-height: 1.08;
	}

	.map-panel__content :global(.sg-page-title + .sg-meta) {
		max-width: 34ch;
		margin: 0;
		line-height: 1.5;
	}

	.map-panel__content :global(section) {
		margin-block-start: var(--sg-space-8);
	}

	.map-panel__content :global(.sg-section-heading) {
		margin-block-end: var(--sg-space-3);
		padding-block-end: var(--sg-space-3);
		border-bottom: 1px solid var(--sg-border);
	}

	.map-panel__content :global(.sg-section-heading h2) {
		font-size: var(--sg-text-row);
		font-weight: var(--sg-weight-medium);
	}

	.map-panel--explore .map-panel__content :global(.sg-list) {
		counter-reset: collection;
	}

	.map-panel--explore .map-panel__content :global(.sg-list > li) {
		counter-increment: collection;
		display: grid;
		grid-template-columns: 1.75rem minmax(0, 1fr);
		align-items: start;
		gap: var(--sg-space-1) var(--sg-space-3);
		padding: var(--sg-space-4) 0;
		border-top: 0;
		border-bottom: 1px solid var(--sg-border);
	}

	.map-panel--explore .map-panel__content :global(.sg-list > li::before) {
		content: counter(collection, decimal-leading-zero);
		grid-row: 1 / span 2;
		margin-top: var(--sg-space-1);
		color: var(--sg-brand);
		font-family: var(--sg-font-mono);
		font-size: var(--sg-text-meta);
	}

	.map-panel--explore .map-panel__content :global(.coll-row) {
		grid-column: 2;
		align-items: start;
	}

	.map-panel--explore .map-panel__content :global(.coll-title) {
		font-size: var(--sg-text-body);
		font-weight: var(--sg-weight-medium);
		line-height: 1.3;
	}

	.map-panel--explore .map-panel__content :global(.coll-row .muted) {
		flex: none;
		white-space: nowrap;
		font-size: var(--sg-text-meta);
	}

	.map-panel--explore .map-panel__content :global(.desc) {
		grid-column: 2;
		margin: 0;
		line-height: 1.5;
	}

	.map-panel--explore .map-panel__content :global(section > .sg-meta:last-child) {
		margin-block-start: var(--sg-space-3);
	}

	.map-panel--passport .map-panel__content :global(.total) {
		margin: 0 0 var(--sg-space-4);
		padding: var(--sg-space-4);
		border-radius: var(--sg-radius-card);
		background: var(--sg-brand-subtle);
		line-height: 1.3;
	}

	.map-panel--passport .map-panel__content :global(.total strong) {
		font-size: 2.75rem;
		font-weight: var(--sg-weight-regular);
		letter-spacing: -0.06em;
	}

	.map-panel--passport .map-panel__content :global(.modes > li) {
		display: grid;
		gap: var(--sg-space-2);
		padding: var(--sg-space-4) 0;
	}

	.map-panel--passport .map-panel__content :global(.sg-list > li.entry) {
		display: block;
		padding-block: var(--sg-space-4);
	}

	.map-panel__next {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: var(--sg-space-3);
		min-height: var(--sg-target-min);
		margin-block-start: var(--sg-space-6);
		padding-block-start: var(--sg-space-3);
		border-top: 1px solid var(--sg-border);
		color: var(--sg-brand);
		font-weight: var(--sg-weight-medium);
		text-decoration: none;
	}

	.map-panel__next:hover {
		text-decoration: underline;
		text-underline-offset: 0.25em;
	}

	@media (min-width: 48rem) {
		.map-panel {
			inset: 0 0 calc(var(--nav-h) + var(--sg-space-3) + env(safe-area-inset-bottom, 0px)) auto;
			width: min(27rem, 45%);
			height: auto;
			max-height: none;
			border-block: 0;
			border-inline-start: 1px solid var(--sg-border);
			border-inline-end: 0;
			border-radius: 0;
			box-shadow: var(--sg-shadow-overlay);
		}

		.map-panel--explore {
			bottom: auto;
			max-height: calc(100dvh - var(--nav-h) - var(--sg-space-3));
			border-bottom: 1px solid var(--sg-border);
			border-radius: 0 0 0 var(--sg-radius-card);
		}
	}

	@media (min-width: 48rem) and (prefers-reduced-motion: no-preference) {
		.map-panel {
			animation-name: sg-panel-enter-side;
		}
	}
</style>
