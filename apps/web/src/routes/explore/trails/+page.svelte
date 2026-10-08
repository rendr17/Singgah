<script lang="ts">
	import { resolve } from '$app/paths';
	import { SectionHeading, StateBlock } from '@singgah/ui';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();
</script>

<svelte:head><title>Jalur jalan kaki · Singgah</title></svelte:head>

<a class="back-link" href={resolve('/explore')}>← Jelajah</a>
<h1 class="sg-page-title">Jalur jalan kaki pilihan</h1>
<p class="sg-meta">
	Rute singkat yang dikurasi dengan konteks stasiun awal dan akhir. Jarak adalah estimasi garis
	lurus, bukan navigasi jalan kaki.
</p>

<section aria-label="Jalur kurasi">
	<SectionHeading>Jalur kurasi</SectionHeading>
	{#if data.error}
		<StateBlock kind="error">{data.error}</StateBlock>
	{:else if data.trails.length === 0}
		<StateBlock kind="empty">Belum ada jalur yang tersedia.</StateBlock>
	{:else}
		<ul class="trail-list">
			{#each data.trails as trail (trail.slug)}
				<li>
					{#if trail.canStart}
						<a class="trail-row" href={resolve('/explore/trails/[slug]', { slug: trail.slug })}>
							<span class="trail-title">{trail.title}</span>
							<span class="trail-theme">{trail.theme}</span>
							<span class="transit-context">
								{trail.startTransit.name} → {trail.endTransit.name}
							</span>
							<span class="trail-count">{trail.availableStopCount} tempat</span>
						</a>
					{:else}
						<div
							class="trail-row trail-row--unavailable"
							aria-describedby={'unavailable-' + trail.slug}
						>
							<span class="trail-title">{trail.title}</span>
							<span class="trail-theme">{trail.theme}</span>
							<span class="transit-context">
								{trail.startTransit.name} → {trail.endTransit.name}
							</span>
							<span class="trail-count">Belum tersedia</span>
							<p id={'unavailable-' + trail.slug} class="unavailable-note">
								Sebagian tempat atau akses transit belum terverifikasi di katalog.
							</p>
						</div>
					{/if}
				</li>
			{/each}
		</ul>
	{/if}
</section>

<style>
	.back-link {
		color: var(--sg-brand);
		font-size: var(--sg-text-secondary);
	}
	.trail-list {
		list-style: none;
		margin: 0;
		padding: 0;
		max-width: 44rem;
	}
	.trail-list > li {
		border-block-start: 1px solid var(--sg-border);
	}
	.trail-list > li:last-child {
		border-block-end: 1px solid var(--sg-border);
	}
	.trail-row {
		display: grid;
		grid-template-columns: minmax(0, 1fr) auto;
		gap: var(--sg-space-1) var(--sg-space-3);
		padding: var(--sg-space-4) 0;
		color: inherit;
		text-decoration: none;
	}
	a.trail-row:hover .trail-title {
		color: var(--sg-brand);
	}
	.trail-title {
		font-weight: var(--sg-weight-semibold);
	}
	.trail-theme,
	.transit-context,
	.trail-count,
	.unavailable-note {
		color: var(--sg-text-muted);
		font-size: var(--sg-text-secondary);
	}
	.transit-context,
	.unavailable-note {
		grid-column: 1;
	}
	.trail-count {
		grid-column: 2;
		grid-row: 1 / span 2;
		white-space: nowrap;
	}
	.unavailable-note {
		margin: 0;
	}
	.trail-row--unavailable {
		opacity: 0.78;
	}
</style>
