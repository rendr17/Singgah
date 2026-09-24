<script lang="ts">
	import { resolve } from '$app/paths';
	import { api } from '$lib/api';
	import BackNav from '$lib/components/app-shell/BackNav.svelte';
	import { unwrap } from '@singgah/api-client';
	import type { components } from '@singgah/api-client';
	import { SearchField, StatusBadge } from '@singgah/ui';

	type Station = components['schemas']['StationSummary'];
	type Plan = components['schemas']['JourneyPlan'];

	let fromStation = $state<Station | null>(null);
	let toStation = $state<Station | null>(null);
	let plan = $state<Plan | null>(null);
	let planning = $state(false);
	let planError = $state('');

	function useStationSearch() {
		let query = $state('');
		let results = $state<Station[]>([]);
		let timer: ReturnType<typeof setTimeout>;
		$effect(() => {
			const q = query.trim();
			clearTimeout(timer);
			if (q === '') {
				results = [];
				return;
			}
			timer = setTimeout(async () => {
				try {
					const data = await unwrap(
						api.GET('/api/v1/stations', { params: { query: { query: q } } })
					);
					results = data.stations;
				} catch {
					results = [];
				}
			}, 300);
			return () => clearTimeout(timer);
		});
		return {
			get query() {
				return query;
			},
			set query(v) {
				query = v;
			},
			get results() {
				return results;
			},
			set results(v) {
				results = v;
			}
		};
	}

	const from = useStationSearch();
	const to = useStationSearch();

	function pick(which: 'from' | 'to', s: Station) {
		if (which === 'from') {
			fromStation = s;
			from.query = s.name;
			from.results = [];
		} else {
			toStation = s;
			to.query = s.name;
			to.results = [];
		}
	}

	async function submit() {
		if (!fromStation || !toStation) return;
		planning = true;
		planError = '';
		plan = null;
		try {
			plan = await unwrap(
				api.GET('/api/v1/journeys', {
					params: { query: { from: fromStation.id, to: toStation.id } }
				})
			);
		} catch (e) {
			planError = e instanceof Error ? e.message : 'Pencarian rute gagal.';
		} finally {
			planning = false;
		}
	}
</script>

<BackNav href="/" label="Pencarian" />
<h1>Perjalanan</h1>
<p class="muted">Rencana stasiun ke stasiun — data jadwal statis.</p>

<div class="fields">
	<div class="field">
		<label for="from">Dari</label>
		<SearchField
			id="from"
			label="Stasiun asal"
			placeholder="Nama stasiun…"
			bind:value={from.query}
		/>
		{#if from.results.length > 0}
			<ul class="options">
				{#each from.results as s (s.id)}
					<li><button type="button" onclick={() => pick('from', s)}>{s.name}</button></li>
				{/each}
			</ul>
		{/if}
	</div>
	<div class="field">
		<label for="to">Ke</label>
		<SearchField id="to" label="Stasiun tujuan" placeholder="Nama stasiun…" bind:value={to.query} />
		{#if to.results.length > 0}
			<ul class="options">
				{#each to.results as s (s.id)}
					<li><button type="button" onclick={() => pick('to', s)}>{s.name}</button></li>
				{/each}
			</ul>
		{/if}
	</div>
	<button
		type="button"
		class="go"
		disabled={!fromStation || !toStation || planning}
		onclick={submit}
	>
		{planning ? 'Mencari…' : 'Cari rute'}
	</button>
</div>

{#if planError}
	<p role="alert">{planError}</p>
{/if}

{#if plan}
	{#if plan.itinerary === null}
		<p>Provider tidak menemukan rute antara {plan.from.name} dan {plan.to.name}.</p>
	{:else}
		{@const itin = plan.itinerary}
		<section class="result">
			<header class="result-head">
				<StatusBadge status={itin.status} />
				{#if itin.fare?.total != null}
					<span class="fare">Rp{itin.fare.total.toLocaleString('id-ID')}</span>
				{/if}
				<span class="muted">
					{(itin.totalDistanceM / 1000).toFixed(1)} km · {itin.rideLegs} naik
					{#if itin.walkTransfers > 0}· {itin.walkTransfers} transit jalan{/if}
				</span>
			</header>

			<ol class="legs">
				{#each itin.legs as leg, i (i)}
					<li class={['leg', `leg--${leg.type}`]}>
						{#if leg.type === 'walk'}
							<span class="leg-icon" aria-hidden="true">↔</span>
							<span>
								Jalan ke {leg.to.name}
								{#if leg.distanceM}<span class="muted"> · {Math.round(leg.distanceM)} m</span>{/if}
							</span>
						{:else}
							<span class="leg-icon" aria-hidden="true">●</span>
							<span>
								<strong>{leg.line}</strong> arah {leg.headsign || leg.to.name}
								<span class="muted">
									· {leg.stationCount} perhentian
									{#if leg.distanceM}· {(leg.distanceM / 1000).toFixed(1)} km{/if}
								</span>
								{#if leg.stops && leg.stops.length > 2}
									<details>
										<summary>{leg.stops.length} stasiun dilewati</summary>
										<ul>
											{#each leg.stops as stop, j (j)}
												<li>
													{#if stop.id}
														<a href={resolve('/stations/[id]', { id: stop.id })}>{stop.name}</a>
													{:else}
														{stop.name}
													{/if}
												</li>
											{/each}
										</ul>
									</details>
								{/if}
							</span>
						{/if}
					</li>
				{/each}
			</ol>

			{#if itin.fare && itin.fare.segments.length > 1}
				<details class="fare-detail">
					<summary>Rincian tarif</summary>
					<ul>
						{#each itin.fare.segments as seg, i (i)}
							<li>
								{seg.operator}: {seg.from.name} → {seg.to.name} — Rp{seg.amount.toLocaleString(
									'id-ID'
								)}
							</li>
						{/each}
					</ul>
				</details>
			{/if}
		</section>
	{/if}
{/if}

<style>
	.muted {
		color: var(--sg-text-muted);
		font-size: 0.875rem;
	}
	.fields {
		display: flex;
		flex-direction: column;
		gap: var(--sg-space-3);
		max-width: 28rem;
		margin-block: var(--sg-space-4);
	}
	.field label {
		display: block;
		font-weight: 600;
		margin-bottom: var(--sg-space-1);
	}
	.options {
		list-style: none;
		margin: var(--sg-space-1) 0 0;
		padding: 0;
		border: 1px solid var(--sg-border);
		border-radius: var(--sg-radius-input);
	}
	.options button {
		display: block;
		width: 100%;
		text-align: left;
		padding: var(--sg-space-2) var(--sg-space-3);
		background: none;
		border: none;
		cursor: pointer;
	}
	.options button:hover {
		background-color: var(--sg-surface-muted);
	}
	.go {
		min-height: var(--sg-target-min, 44px);
		padding: 0 var(--sg-space-4);
		border: none;
		border-radius: var(--sg-radius-input);
		background-color: var(--sg-brand, #0f6b4f);
		color: #fff;
		font-weight: 600;
		cursor: pointer;
	}
	.go:disabled {
		opacity: 0.5;
		cursor: default;
	}
	.result {
		max-width: 36rem;
		border: 1px solid var(--sg-border);
		border-radius: var(--sg-radius-card);
		padding: var(--sg-space-3) var(--sg-space-4);
	}
	.result-head {
		display: flex;
		align-items: center;
		gap: var(--sg-space-3);
		flex-wrap: wrap;
	}
	.fare {
		font-weight: 700;
		font-variant-numeric: tabular-nums;
	}
	.legs {
		list-style: none;
		padding: 0;
		margin: var(--sg-space-3) 0 0;
		display: flex;
		flex-direction: column;
		gap: var(--sg-space-3);
	}
	.leg {
		display: flex;
		gap: var(--sg-space-2);
	}
	.leg-icon {
		color: var(--sg-brand, #0f6b4f);
	}
	details ul {
		margin: var(--sg-space-1) 0 0;
		padding-left: var(--sg-space-4);
	}
</style>
