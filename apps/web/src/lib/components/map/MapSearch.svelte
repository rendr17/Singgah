<script lang="ts">
	import { api } from '$lib/api';
	import { unwrap } from '@singgah/api-client';
	import type { components } from '@singgah/api-client';
	import { SearchField } from '@singgah/ui';

	type Station = components['schemas']['StationSummary'];
	type Line = components['schemas']['RouteSummary'];

	// Map-mode-independent search (blueprint §32/§78). Results resolve to
	// canonical IDs — what a pick means is the caller's decision.
	interface Props {
		onStation?: (station: Station) => void;
		onLine?: (line: Line) => void;
	}

	let { onStation, onLine }: Props = $props();

	const listboxId = $props.id();

	type Item = { kind: 'station'; station: Station } | { kind: 'line'; line: Line };

	let value = $state('');
	let items = $state<Item[]>([]);
	let stationCount = $state(0);
	let open = $state(false);
	let active = $state(-1);
	let timer: ReturnType<typeof setTimeout>;
	// pick() writes `value` programmatically — that write must not trigger a
	// fresh fetch, or the dropdown would reopen right after a selection.
	let suppressNext = false;

	$effect(() => {
		const q = value.trim();
		clearTimeout(timer);
		if (suppressNext) {
			suppressNext = false;
			return;
		}
		if (q === '') {
			items = [];
			open = false;
			return;
		}
		timer = setTimeout(async () => {
			try {
				const [s, r] = await Promise.all([
					unwrap(api.GET('/api/v1/stations', { params: { query: { query: q, limit: 8 } } })),
					unwrap(api.GET('/api/v1/routes', { params: { query: { query: q, limit: 6 } } }))
				]);
				if (value.trim() !== q) return; // a newer query already owns the list
				stationCount = s.stations.length;
				items = [
					...s.stations.map((station) => ({ kind: 'station', station }) as Item),
					...r.routes.map((line) => ({ kind: 'line', line }) as Item)
				];
				active = -1;
				open = items.length > 0;
			} catch {
				if (value.trim() !== q) return;
				items = [];
				open = false;
			}
		}, 300);
		return () => clearTimeout(timer);
	});

	function pick(item: Item) {
		open = false;
		active = -1;
		const next =
			item.kind === 'station'
				? item.station.name
				: (item.line.shortName ?? item.line.longName ?? '');
		// Only a changed value re-triggers the effect — an unchanged write
		// would leave suppressNext set and swallow the next real keystroke.
		if (next !== value) suppressNext = true;
		value = next;
		if (item.kind === 'station') onStation?.(item.station);
		else onLine?.(item.line);
	}

	function onkeydown(e: KeyboardEvent) {
		if (!open) return;
		if (e.key === 'ArrowDown') {
			e.preventDefault();
			active = (active + 1) % items.length;
		} else if (e.key === 'ArrowUp') {
			e.preventDefault();
			active = (active - 1 + items.length) % items.length;
		} else if (e.key === 'Enter') {
			e.preventDefault();
			if (active >= 0) pick(items[active]);
		} else if (e.key === 'Escape') {
			e.stopPropagation();
			open = false;
			active = -1;
		}
	}

	function onfocusout(e: FocusEvent) {
		if (!(e.currentTarget as HTMLElement).contains(e.relatedTarget as Node)) {
			open = false;
		}
	}
</script>

<div class="sg-map-search" {onfocusout}>
	<SearchField
		label="Cari stasiun atau lin"
		placeholder="Cari stasiun atau lin…"
		bind:value
		role="combobox"
		aria-autocomplete="list"
		aria-haspopup="listbox"
		aria-expanded={open}
		aria-controls="{listboxId}-listbox"
		aria-activedescendant={active >= 0 ? `${listboxId}-option-${active}` : undefined}
		autocomplete="off"
		{onkeydown}
	/>
	{#if open}
		<ul
			class="sg-map-search__options"
			role="listbox"
			id="{listboxId}-listbox"
			aria-label="Hasil pencarian"
		>
			{#each items as item, i (item.kind === 'station' ? item.station.id : item.line.id)}
				{#if i === 0 || (i === stationCount && stationCount > 0)}
					<li class="sg-map-search__group" role="presentation">
						{item.kind === 'station' ? 'Stasiun' : 'Lin'}
					</li>
				{/if}
				<li role="presentation">
					<button
						type="button"
						role="option"
						id="{listboxId}-option-{i}"
						aria-selected={i === active}
						tabindex="-1"
						onmousemove={() => (active = i)}
						onclick={() => pick(item)}
					>
						{#if item.kind === 'station'}
							<span class="sg-map-search__main">{item.station.name}</span>
							<span class="sg-map-search__meta">{item.station.code || item.station.operator}</span>
						{:else}
							<span
								class="sg-map-search__chip"
								style:background-color={item.line.color ? `#${item.line.color}` : null}
								style:color={item.line.color ? '#fff' : null}>{item.line.shortName ?? '—'}</span
							>
							<span class="sg-map-search__main">{item.line.longName ?? item.line.agencyName}</span>
						{/if}
					</button>
				</li>
			{/each}
		</ul>
	{/if}
</div>

<style>
	.sg-map-search {
		position: relative;
	}
	.sg-map-search__options {
		position: absolute;
		inset-inline: 0;
		top: calc(100% + var(--sg-space-1));
		list-style: none;
		margin: 0;
		padding: 0;
		background-color: var(--sg-surface);
		border: 1px solid var(--sg-border);
		border-radius: var(--sg-radius-input);
		box-shadow: var(--sg-shadow-card);
		overflow: hidden;
		z-index: var(--sg-z-overlay);
	}
	.sg-map-search__group {
		padding: var(--sg-space-2) var(--sg-space-3) var(--sg-space-1);
		font-size: var(--sg-text-caption);
		font-weight: var(--sg-weight-bold);
		color: var(--sg-text-muted);
	}
	.sg-map-search__options button {
		display: flex;
		align-items: center;
		gap: var(--sg-space-2);
		width: 100%;
		min-height: var(--sg-target-min);
		padding: var(--sg-space-2) var(--sg-space-3);
		background: none;
		border: none;
		font: inherit;
		color: inherit;
		text-align: left;
		cursor: pointer;
	}
	.sg-map-search__options button[aria-selected='true'] {
		background-color: var(--sg-surface-muted);
	}
	.sg-map-search__main {
		flex: 1;
		min-width: 0;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	.sg-map-search__meta {
		flex: none;
		font-size: var(--sg-text-caption);
		color: var(--sg-text-muted);
	}
	.sg-map-search__chip {
		flex: none;
		min-width: 2.25rem;
		padding: 0 var(--sg-space-1);
		border-radius: var(--sg-radius-button);
		background-color: var(--sg-surface-muted);
		color: var(--sg-text);
		font-size: var(--sg-text-caption);
		font-weight: var(--sg-weight-bold);
		text-align: center;
	}
</style>
