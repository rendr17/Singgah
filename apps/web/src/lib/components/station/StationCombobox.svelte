<script lang="ts">
	import { api } from '$lib/api';
	import { unwrap } from '@singgah/api-client';
	import type { components } from '@singgah/api-client';
	import { SearchField } from '@singgah/ui';

	type Station = components['schemas']['StationSummary'];

	// Combobox pattern: focus stays on the input, the list is announced via
	// aria-activedescendant. `selected` only stays valid while the text still
	// equals the picked name — editing the text clears the pick, which is what
	// keeps /plan from submitting a station the user no longer sees.
	interface Props {
		id?: string;
		label: string;
		placeholder?: string;
		value?: string;
		selected?: Station | null;
		onpick?: (station: Station) => void;
	}

	let {
		id,
		label,
		placeholder,
		value = $bindable(''),
		selected = $bindable(null),
		onpick
	}: Props = $props();

	const listboxId = $props.id();

	let results = $state<Station[]>([]);
	let open = $state(false);
	let active = $state(-1);
	let timer: ReturnType<typeof setTimeout>;

	$effect(() => {
		const q = value.trim();
		clearTimeout(timer);
		// Programmatic picks (shared /plan links) write name + selected together;
		// a matching pair must not re-open the dropdown.
		if (selected && q === selected.name) {
			results = [];
			open = false;
			return;
		}
		if (selected) selected = null;
		if (q === '') {
			results = [];
			open = false;
			return;
		}
		timer = setTimeout(async () => {
			try {
				const data = await unwrap(api.GET('/api/v1/stations', { params: { query: { query: q } } }));
				results = data.stations;
				active = -1;
				open = results.length > 0;
			} catch {
				results = [];
				open = false;
			}
		}, 300);
		return () => clearTimeout(timer);
	});

	function pick(station: Station) {
		selected = station;
		value = station.name;
		results = [];
		open = false;
		onpick?.(station);
	}

	function onkeydown(e: KeyboardEvent) {
		if (!open) return;
		if (e.key === 'ArrowDown') {
			e.preventDefault();
			active = (active + 1) % results.length;
		} else if (e.key === 'ArrowUp') {
			e.preventDefault();
			active = (active - 1 + results.length) % results.length;
		} else if (e.key === 'Enter') {
			// The listbox is open — Enter belongs to it, not the parent form.
			e.preventDefault();
			if (active >= 0) pick(results[active]);
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

<div class="sg-combobox" {onfocusout}>
	<SearchField
		{id}
		{label}
		{placeholder}
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
		<ul class="sg-combobox__options" role="listbox" id="{listboxId}-listbox" aria-label={label}>
			{#each results as station, i (station.id)}
				<li role="presentation">
					<button
						type="button"
						role="option"
						id="{listboxId}-option-{i}"
						aria-selected={i === active}
						tabindex="-1"
						onmousemove={() => (active = i)}
						onclick={() => pick(station)}
					>
						{station.name}
					</button>
				</li>
			{/each}
		</ul>
	{/if}
</div>

<style>
	.sg-combobox {
		position: relative;
	}

	.sg-combobox__options {
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

	.sg-combobox__options button {
		display: flex;
		align-items: center;
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

	.sg-combobox__options button[aria-selected='true'] {
		background-color: var(--sg-surface-muted);
	}
</style>
