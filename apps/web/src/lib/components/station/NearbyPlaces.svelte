<script lang="ts">
	import { resolve } from '$app/paths';
	import { StateBlock } from '@singgah/ui';
	import type { components } from '@singgah/api-client';

	type PlaceSummary = components['schemas']['PlaceSummary'];
	type Category = NonNullable<PlaceSummary['category']>;

	let { places }: { places: PlaceSummary[] | null } = $props();

	// Canonical doc/14 launch categories → display labels (Indonesian-first).
	const CATEGORY_LABEL: Record<Category, string> = {
		makan: 'Makan enak',
		ngopi: 'Ngopi & nongkrong',
		hiburan: 'Main & hiburan',
		taman: 'Taman & santai',
		budaya: 'Budaya & cerita',
		belanja: 'Belanja & jajan',
		other: 'Lainnya'
	};

	let active = $state<Category | 'all'>('all');

	const present = $derived(
		places ? ([...new Set(places.map((p) => p.category))] as Category[]) : []
	);
	const shown = $derived(
		places === null ? null : active === 'all' ? places : places.filter((p) => p.category === active)
	);

	// walkSeconds is a straight-line × detour estimate (ADR-011) — display it
	// with an explicit "±" so it never reads as a routed walking time.
	function walkMinutes(p: PlaceSummary): number | null {
		return p.walkSeconds === null ? null : Math.max(1, Math.round(p.walkSeconds / 60));
	}
</script>

{#if shown === null}
	<StateBlock kind="error">Data tempat sekitar sedang tidak tersedia.</StateBlock>
{:else if places === null || places.length === 0}
	<StateBlock kind="empty">Belum ada tempat tercatat di sekitar stasiun ini.</StateBlock>
{:else}
	{#if present.length > 1}
		<div class="chips" role="group" aria-label="Filter kategori">
			<button
				type="button"
				class="chip"
				class:chip--active={active === 'all'}
				aria-pressed={active === 'all'}
				onclick={() => (active = 'all')}
			>
				Semua
			</button>
			{#each present as cat (cat)}
				<button
					type="button"
					class="chip"
					class:chip--active={active === cat}
					aria-pressed={active === cat}
					onclick={() => (active = active === cat ? 'all' : cat)}
				>
					{CATEGORY_LABEL[cat] ?? cat}
				</button>
			{/each}
		</div>
	{/if}

	{#if shown.length === 0}
		<StateBlock kind="empty">Tidak ada tempat kategori ini di sekitar sini.</StateBlock>
	{:else}
		<ul class="nearby-place-list">
			{#each shown as place (place.id)}
				<li class="place-row">
					<span class="name">
						{place.name}
						{#if place.curated}<span class="curated">Kurasi</span>{/if}
					</span>
					<span class="muted">
						{CATEGORY_LABEL[place.category] ?? place.category}
						· {place.walkDistanceM} m{#if walkMinutes(place) !== null}
							· ±{walkMinutes(place)} mnt{/if}
					</span>
					<a class="place-detail-link" href={resolve('/places/[id]', { id: place.id })}>Detail →</a>
					<a
						class="place-map-link"
						href={`https://www.google.com/maps/search/?api=1&query=${place.lat},${place.lon}`}
						target="_blank"
						rel="noopener noreferrer"
						aria-label={`Lihat ${place.name} di Google Maps`}>Lihat di peta ↗</a
					>
				</li>
			{/each}
		</ul>
		<p class="sg-meta">
			Jarak garis lurus dari stasiun — estimasi, bukan rute jalan kaki. Data © kontributor
			OpenStreetMap.
		</p>
	{/if}
{/if}

<style>
	.chips {
		display: flex;
		flex-wrap: wrap;
		gap: var(--sg-space-1);
		margin-block-end: var(--sg-space-3);
	}
	.chip {
		border: 1px solid var(--sg-border);
		border-radius: var(--sg-radius-pill);
		background: transparent;
		color: var(--sg-text);
		font: inherit;
		font-size: var(--sg-text-secondary);
		padding: var(--sg-space-1) var(--sg-space-3);
		cursor: pointer;
	}
	.chip--active {
		background: var(--sg-surface-raised, var(--sg-border));
		font-weight: var(--sg-weight-semibold);
	}
	.name {
		font-weight: var(--sg-weight-medium);
	}
	.curated {
		display: inline-block;
		margin-inline-start: var(--sg-space-1);
		padding: 0 var(--sg-space-2);
		border-radius: var(--sg-radius-pill);
		border: 1px solid var(--sg-border);
		font-size: var(--sg-text-meta);
		color: var(--sg-text-muted);
	}
	.place-row {
		display: grid;
		grid-template-columns: minmax(0, 1fr) auto;
		align-items: center;
		gap: var(--sg-space-1) var(--sg-space-3);
		padding-block: var(--sg-space-3);
		border-block-start: 1px solid var(--sg-border);
	}
	.nearby-place-list {
		list-style: none;
		max-width: 40rem;
		margin: 0;
		padding: 0;
	}
	.nearby-place-list > li:last-child {
		border-block-end: 1px solid var(--sg-border);
	}
	.name,
	.muted {
		grid-column: 1;
	}
	.place-detail-link {
		grid-column: 1;
		color: var(--sg-brand);
		font-size: var(--sg-text-secondary);
	}
	.place-map-link {
		grid-column: 2;
		grid-row: 1 / span 2;
		color: var(--sg-brand);
		font-size: var(--sg-text-secondary);
		white-space: nowrap;
	}
	.muted {
		display: block;
		color: var(--sg-text-muted);
		font-size: var(--sg-text-secondary);
	}
</style>
