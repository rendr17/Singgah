<script lang="ts">
	import { resolve } from '$app/paths';
	import { replaceState } from '$app/navigation';
	import { page } from '$app/state';
	import { onMount } from 'svelte';
	import { api } from '$lib/api';
	import { loadCachedPlan, saveCachedPlan } from '$lib/plan-cache';
	import BackNav from '$lib/components/app-shell/BackNav.svelte';
	import StationCombobox from '$lib/components/station/StationCombobox.svelte';
	import { unwrap } from '@singgah/api-client';
	import type { components } from '@singgah/api-client';
	import { Button, StateBlock, StatusBadge, Surface, TextField } from '@singgah/ui';

	type Station = components['schemas']['StationSummary'];
	type Plan = components['schemas']['JourneyPlan'];

	let fromStation = $state<Station | null>(null);
	let toStation = $state<Station | null>(null);
	let plan = $state<Plan | null>(null);
	let planning = $state(false);
	let planError = $state('');
	// True when the shown plan came from localStorage after a failed fetch —
	// the banner must say so instead of letting stale data pass as fresh.
	let planFromCache = $state(false);
	// datetime-local value; empty = leave now. Fed into `at` on submit.
	let departAt = $state('');

	const loadCached = (fromId: string, toId: string, at: string) =>
		loadCachedPlan(localStorage, fromId, toId, at);
	const saveCached = (fromId: string, toId: string, at: string, p: Plan) =>
		saveCachedPlan(localStorage, fromId, toId, at, p);

	// toLocalInput renders an ISO instant for <input type="datetime-local"> —
	// the field is device-local by definition, so it gets a local reading.
	function toLocalInput(iso: string): string {
		const d = new Date(iso);
		if (Number.isNaN(d.getTime())) return '';
		const p = (n: number) => String(n).padStart(2, '0');
		return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())}T${p(d.getHours())}:${p(d.getMinutes())}`;
	}

	let fromQuery = $state('');
	let toQuery = $state('');

	function pick(which: 'from' | 'to', s: Station) {
		if (which === 'from') {
			fromStation = s;
			fromQuery = s.name;
		} else {
			toStation = s;
			toQuery = s.name;
		}
	}

	// Timeline screen takes the same params; station ids come from the picked
	// fields first, falling back to the plan's own refs (cached shared links).
	const detail = $derived.by(() => {
		const from = fromStation?.id ?? plan?.from.id;
		const to = toStation?.id ?? plan?.to.id;
		if (!plan || !from || !to) return null;
		return { from, to, at: plan.at ? `&at=${encodeURIComponent(plan.at)}` : '' };
	});

	async function submit() {
		if (!fromStation || !toStation) return;
		planning = true;
		planError = '';
		plan = null;
		planFromCache = false;
		const at = departAt ? new Date(departAt).toISOString() : '';
		try {
			plan = await unwrap(
				api.GET('/api/v1/journeys', {
					params: { query: { from: fromStation.id, to: toStation.id, at: at || undefined } }
				})
			);
			saveCached(fromStation.id, toStation.id, at, plan);
			// Keep the plan shareable: the URL is the snapshot, not app state.
			replaceState(
				resolve(
					`/plan?from=${fromStation.id}&to=${toStation.id}${at ? '&at=' + encodeURIComponent(at) : ''}`
				),
				page.state
			);
		} catch (e) {
			const cached = loadCached(fromStation.id, toStation.id, at);
			if (cached) {
				plan = cached;
				planFromCache = true;
			} else {
				planError = e instanceof Error ? e.message : 'Pencarian rute gagal.';
			}
		} finally {
			planning = false;
		}
	}

	// Shared links arrive as /plan?from=<uuid>&to=<uuid> — resolve each to its
	// station so the fields show real names, then plan automatically.
	onMount(async () => {
		const fromId = page.url.searchParams.get('from');
		const toId = page.url.searchParams.get('to');
		const atId = page.url.searchParams.get('at');
		if (atId) departAt = toLocalInput(atId);
		let ready = true;
		for (const [which, id] of [
			['from', fromId],
			['to', toId]
		] as const) {
			if (!id) {
				ready = false;
				continue;
			}
			try {
				const data = await unwrap(api.GET('/api/v1/stations/{id}', { params: { path: { id } } }));
				pick(which, data.station);
			} catch {
				ready = false; // dead link params degrade to an empty field, not an error page
			}
		}
		if (ready) {
			await submit();
		} else if (fromId && toId) {
			// Offline open of a shared link: station names can't be resolved,
			// but the cached copy for this exact pair still renders.
			const cached = loadCached(fromId, toId, atId ?? '');
			if (cached) {
				plan = cached;
				planFromCache = true;
			}
		}
	});
</script>

<svelte:head><title>Perjalanan · Singgah</title></svelte:head>

<BackNav href="/" label="Beranda" />
<h1 class="sg-page-title">Perjalanan</h1>
<p class="sg-meta">Rencana stasiun ke stasiun — data jadwal statis.</p>

<form
	class="fields"
	onsubmit={(e) => {
		e.preventDefault();
		submit();
	}}
>
	<div class="field">
		<label for="from">Dari</label>
		<StationCombobox
			id="from"
			label="Stasiun asal"
			placeholder="Nama stasiun…"
			bind:value={fromQuery}
			bind:selected={fromStation}
		/>
	</div>
	<div class="field">
		<label for="to">Ke</label>
		<StationCombobox
			id="to"
			label="Stasiun tujuan"
			placeholder="Nama stasiun…"
			bind:value={toQuery}
			bind:selected={toStation}
		/>
	</div>
	<TextField id="at" type="datetime-local" label="Berangkat" bind:value={departAt} />
	<Button type="submit" disabled={!fromStation || !toStation || planning}>
		{planning ? 'Mencari…' : 'Cari rute'}
	</Button>
</form>

{#if planning}
	<StateBlock kind="loading">Mencari rute…</StateBlock>
{/if}

{#if planError}
	<StateBlock kind="error">{planError}</StateBlock>
{/if}

{#if plan}
	{#if planFromCache}
		<p class="sg-meta" role="status">
			Rute tersimpan — data per {new Date(plan.source.requestedAt).toLocaleString('id-ID')}
		</p>
	{/if}
	{#if plan.itinerary === null}
		<StateBlock kind="empty">
			Provider tidak menemukan rute antara {plan.from.name} dan {plan.to.name}.
		</StateBlock>
	{:else}
		{@const itin = plan.itinerary}
		<Surface class="result">
			{#if plan.at}
				<p class="sg-meta plan-for">
					Untuk berangkat {new Date(plan.at).toLocaleString('id-ID', {
						dateStyle: 'medium',
						timeStyle: 'short'
					})}
				</p>
			{/if}
			<header class="result-head">
				<StatusBadge status={itin.status} />
				{#if itin.fare?.total != null}
					<span class="fare">Rp{itin.fare.total.toLocaleString('id-ID')}</span>
				{/if}
				<span class="sg-meta">
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
								{#if leg.distanceM}<span class="sg-meta">
										· {Math.round(leg.distanceM)} m</span
									>{/if}
							</span>
						{:else}
							<span class="leg-icon" aria-hidden="true">●</span>
							<span>
								<strong>{leg.line}</strong> arah {leg.headsign || leg.to.name}
								<span class="sg-meta">
									· {leg.stationCount} perhentian
									{#if leg.distanceM}· {(leg.distanceM / 1000).toFixed(1)} km{/if}
								</span>
								{#if leg.nextDepartures && leg.nextDepartures.length > 0}
									<span class="departures">
										Berangkat {leg.nextDepartures.map((d) => d.time).join(' · ')}
									</span>
								{/if}
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

			{#if detail}
				<p class="detail-link">
					<a href={resolve(`/journey?from=${detail.from}&to=${detail.to}${detail.at}`)}
						>Rincian perjalanan →</a
					>
				</p>
			{/if}
		</Surface>
	{/if}
{/if}

<style>
	.fields {
		display: flex;
		flex-direction: column;
		gap: var(--sg-space-3);
		max-width: 28rem;
		margin-block: var(--sg-space-4);
	}
	.field label {
		display: block;
		font-size: var(--sg-text-secondary);
		font-weight: var(--sg-weight-bold);
		margin-bottom: var(--sg-space-1);
	}
	.plan-for {
		margin: 0 0 var(--sg-space-2);
	}
	.result {
		max-width: 36rem;
	}
	.result-head {
		display: flex;
		align-items: center;
		gap: var(--sg-space-3);
		flex-wrap: wrap;
	}
	.fare {
		font-weight: var(--sg-weight-bold);
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
		color: var(--sg-brand);
	}
	.departures {
		display: block;
		margin-top: var(--sg-space-1);
		font-weight: var(--sg-weight-semibold);
		font-variant-numeric: tabular-nums;
	}
	summary {
		cursor: pointer;
		font-size: var(--sg-text-secondary);
		color: var(--sg-text-muted);
	}
	details ul {
		margin: var(--sg-space-1) 0 0;
		padding-left: var(--sg-space-4);
	}
</style>
