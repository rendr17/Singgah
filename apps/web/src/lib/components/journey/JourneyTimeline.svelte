<script lang="ts">
	import { resolve } from '$app/paths';
	import type { components } from '@singgah/api-client';

	type StopRef = components['schemas']['JourneyStopRef'];
	type Leg = components['schemas']['JourneyLeg'];
	type Alt = components['schemas']['JourneyLegAlternative'];

	interface Props {
		from: StopRef;
		legs: Leg[];
		/** Called when the rider picks a corridor alternative — the parent
		 *  can redraw the map with that corridor's own stops/shape. */
		onSelectAlternative?: (legIndex: number, alt: Alt | null) => void;
	}

	let { from, legs, onSelectAlternative }: Props = $props();

	// Corridor choice is per-leg; absent index = the provider's pick.
	let selected = $state<Record<number, number>>({});
	// A new plan's legs aren't the old plan's — index keys would cross-apply.
	$effect(() => {
		void legs;
		selected = {};
	});

	function shortCode(line?: string) {
		return line?.split(':').pop() ?? '';
	}

	function pick(i: number, j: number | null, leg: Leg) {
		if (j === null) delete selected[i];
		else selected[i] = j;
		onSelectAlternative?.(i, j === null ? null : (leg.alternatives?.[j] ?? null));
	}
</script>

<ol class="timeline">
	<li class="leg">
		<span class="stop endpoint"><strong>{from.name}</strong></span>
	</li>
	{#each legs as leg, i (i)}
		{#if leg.type === 'walk'}
			<li class="leg leg--walk">
				<p class="seg">
					Jalan ke {leg.to.name}{#if leg.distanceM}
						· {Math.round(leg.distanceM)} m{/if}
				</p>
				<span class="stop endpoint"><strong>{leg.to.name}</strong></span>
			</li>
		{:else}
			{@const alt = selected[i] !== undefined ? leg.alternatives?.[selected[i]] : undefined}
			<li class="leg leg--ride">
				<p class="seg">
					Naik <strong>{alt?.line ?? leg.line}</strong> arah {alt
						? leg.to.name
						: leg.headsign || leg.to.name}{#if alt?.stationCount ?? leg.stationCount}
						· {alt?.stationCount ?? leg.stationCount} perhentian{/if}
				</p>
				{#if leg.alternatives && leg.alternatives.length > 0}
					<p class="alts">
						<span class="alts-label">Koridor</span>
						<button
							type="button"
							class="alt"
							class:alt--on={alt === undefined}
							onclick={() => pick(i, null, leg)}>{shortCode(leg.line)}</button
						>
						{#each leg.alternatives as a, j (j)}
							<button
								type="button"
								class="alt"
								class:alt--on={selected[i] === j}
								title={a.name ?? ''}
								onclick={() => pick(i, j, leg)}>{a.shortName || shortCode(a.line)}</button
							>
						{/each}
					</p>
				{/if}
				{#if !alt && leg.nextDepartures && leg.nextDepartures.length > 0}
					<p class="departures sg-tabular">
						Berangkat {leg.nextDepartures.map((d) => d.time).join(' · ')}
					</p>
				{/if}
				{#each (alt?.stops ?? leg.stops ?? []).filter((s) => s.name !== leg.from.name && s.name !== leg.to.name) as stop, j (j)}
					<span class="stop">
						{#if stop.id}
							<a href={resolve('/stations/[id]', { id: stop.id })}>{stop.name}</a>
						{:else}
							{stop.name}
						{/if}
					</span>
				{/each}
				<span class="stop endpoint"><strong>{leg.to.name}</strong></span>
			</li>
		{/if}
	{/each}
</ol>

<style>
	/* Vertical leg timeline: a rail per leg, dots on station nodes. Ride legs
	   brand the rail, walk legs stay dashed — scannable operator semantics. */
	.timeline {
		list-style: none;
		margin: var(--sg-space-3) 0 0;
		padding: 0;
		max-width: 40rem;
	}

	.leg {
		position: relative;
		margin-inline-start: 0.625rem;
		padding-inline-start: var(--sg-space-4);
		padding-bottom: var(--sg-space-4);
	}

	.leg::before {
		content: '';
		position: absolute;
		inset-inline-start: 0;
		top: 0.7em;
		bottom: 0;
		width: 2px;
		background: var(--sg-border);
	}

	.leg--ride::before {
		background: var(--sg-brand);
	}

	.leg--walk::before {
		background: repeating-linear-gradient(to bottom, var(--sg-border) 0 3px, transparent 3px 6px);
	}

	.leg:last-child {
		padding-bottom: 0;
	}

	.leg:last-child::before {
		bottom: auto;
		height: 0.9em;
	}

	.stop {
		display: block;
		position: relative;
		padding-block: var(--sg-space-1);
	}

	.stop::before {
		content: '';
		position: absolute;
		inset-inline-start: calc(-1 * var(--sg-space-4) - 4px);
		top: 0.65em;
		width: 0.625rem;
		height: 0.625rem;
		border-radius: 50%;
		background: var(--sg-canvas);
		border: 2px solid var(--sg-text-muted);
		transform: translateY(-50%);
	}

	.endpoint::before {
		border-color: var(--sg-text);
	}

	.leg--ride .endpoint::before {
		border-color: var(--sg-brand);
	}

	.seg {
		margin: 0;
		padding-block: var(--sg-space-1);
		color: var(--sg-text-muted);
		font-size: var(--sg-text-secondary);
	}

	/* Corridor alternatives — chips keyed by corridor code, provider's pick
	   first. Selected chip brands like the ride rail. */
	.alts {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: var(--sg-space-1);
		margin: 0 0 var(--sg-space-1);
	}
	.alts-label {
		color: var(--sg-text-muted);
		font-size: var(--sg-text-secondary);
		margin-inline-end: var(--sg-space-1);
	}
	.alt {
		min-height: var(--sg-target-min);
		padding: 0 var(--sg-space-3);
		border: 1px solid var(--sg-border);
		border-radius: var(--sg-radius-button);
		background: var(--sg-surface);
		color: var(--sg-text);
		font-weight: var(--sg-weight-bold);
		font-variant-numeric: tabular-nums;
		cursor: pointer;
	}
	.alt--on {
		background: var(--sg-brand);
		border-color: transparent;
		color: var(--sg-brand-contrast);
	}

	.departures {
		margin: 0 0 var(--sg-space-1);
		font-weight: var(--sg-weight-semibold);
		font-variant-numeric: tabular-nums;
	}
</style>
