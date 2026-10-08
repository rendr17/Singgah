<script lang="ts">
	import { api } from '$lib/api';
	import { unwrap } from '@singgah/api-client';
	import type { components } from '@singgah/api-client';
	import { SearchField } from '@singgah/ui';
	import { badgeTextColor } from '$lib/route-groups';

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
		onclear?: () => void;
	}

	let {
		id,
		label,
		placeholder,
		value = $bindable(''),
		selected = $bindable(null),
		onpick,
		onclear
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
		if (selected) {
			selected = null;
			onclear?.();
		}
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

	// Search hits carry `lines` — the corridors serving the stop. The agency
	// line and the a11y label are derived from them, never from a guess.
	function agenciesOf(station: Station) {
		const names = (station.lines ?? []).map((l) => l.agencyName ?? '').filter((n) => n !== '');
		return [...new Set(names)].join(' · ');
	}

	function optionLabel(station: Station) {
		const codes = (station.lines ?? [])
			.map((l) => l.shortName ?? '')
			.filter((c) => c !== '')
			.join(' ');
		return [station.name, agenciesOf(station), codes && `lin ${codes}`].filter(Boolean).join(', ');
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
						aria-label={optionLabel(station)}
						tabindex="-1"
						onmousemove={() => (active = i)}
						onclick={() => pick(station)}
					>
						<span class="opt-main">
							<span class="opt-name">{station.name}</span>
							{#if agenciesOf(station)}
								<span class="opt-sub">{agenciesOf(station)}</span>
							{/if}
						</span>
						{#if station.lines && station.lines.length > 0}
							<span class="opt-lines" aria-hidden="true">
								{#each station.lines as line (line.id)}
									<i
										class="opt-badge"
										style:background-color={line.color
											? `#${line.color}`
											: 'var(--sg-line-default)'}
										style:color={line.color
											? badgeTextColor(line.color)
											: 'var(--sg-brand-contrast)'}>{line.shortName ?? '—'}</i
									>
								{/each}
							</span>
						{/if}
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

	.sg-combobox__options button[aria-selected='true'] {
		background-color: var(--sg-surface-muted);
	}

	.opt-main {
		display: flex;
		flex: 1;
		flex-direction: column;
		min-width: 0;
	}

	.opt-name {
		font-weight: var(--sg-weight-medium);
	}

	.opt-sub {
		color: var(--sg-text-muted);
		font-size: var(--sg-text-meta);
	}

	/* Serving-corridor badges — line colors are data, not decoration. */
	.opt-lines {
		display: flex;
		flex-wrap: wrap;
		justify-content: flex-end;
		gap: var(--sg-space-1);
		max-width: 45%;
	}

	.opt-badge {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		box-sizing: border-box;
		min-width: 1.5rem;
		height: 1.375rem;
		padding: 0 var(--sg-space-1);
		border-radius: var(--sg-radius-button);
		font-size: var(--sg-text-meta);
		font-style: normal;
		font-weight: var(--sg-weight-bold);
		font-variant-numeric: tabular-nums;
	}
</style>
