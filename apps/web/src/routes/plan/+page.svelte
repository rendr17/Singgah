<script lang="ts">
	import { resolve } from '$app/paths';
	import { replaceState } from '$app/navigation';
	import { page } from '$app/state';
	import { onMount } from 'svelte';
	import { SvelteSet, SvelteURLSearchParams } from 'svelte/reactivity';
	import { api } from '$lib/api';
	import { loadCachedPlan, saveCachedPlan } from '$lib/plan-cache';
	import BackNav from '$lib/components/app-shell/BackNav.svelte';
	import StationCombobox from '$lib/components/station/StationCombobox.svelte';
	import { unwrap } from '@singgah/api-client';
	import type { components } from '@singgah/api-client';
	import { Button, StateBlock, StatusBadge, TextField } from '@singgah/ui';

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
	// Serialized query of the displayed plan — itinerary detail links and
	// the cache key share it.
	let planKey = $state('');

	// anchor toggles between depart-at and arrive-by on one datetime field.
	let anchor = $state<'depart' | 'arrive'>('depart');
	// datetime-local value; empty = leave now (depart) / ASAP (arrive is ignored empty).
	let departAt = $state('');
	let modes = $state<Set<string>>(new Set());
	let maxWalk = $state('');
	let maxTransfers = $state('');
	let stepFree = $state(false);

	const MODE_OPTIONS = [
		['rail', 'KRL'],
		['subway', 'MRT'],
		['tram', 'LRT'],
		['bus', 'Bus']
	] as const;

	const LABELS: Record<string, string> = {
		fastest: 'Tercepat',
		fewest_transfers: 'Transit tersedikit',
		least_walking: 'Jalan tersedikit',
		alternative: 'Alternatif'
	};

	const loadCached = (key: string) => loadCachedPlan(localStorage, key);
	const saveCached = (key: string, p: Plan) => saveCachedPlan(localStorage, key, p);

	// toLocalInput renders an ISO instant for <input type="datetime-local"> —
	// the field is device-local by definition, so it gets a local reading.
	function toLocalInput(iso: string): string {
		const d = new Date(iso);
		if (Number.isNaN(d.getTime())) return '';
		const p = (n: number) => String(n).padStart(2, '0');
		return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())}T${p(d.getHours())}:${p(d.getMinutes())}`;
	}

	function fmtTime(iso: string) {
		return new Date(iso).toLocaleTimeString('id-ID', {
			hour: '2-digit',
			minute: '2-digit',
			timeZone: 'Asia/Jakarta'
		});
	}
	function fmtDur(sec: number) {
		const h = Math.floor(sec / 3600);
		const m = Math.round((sec % 3600) / 60);
		return h > 0 ? `${h} j ${m} mnt` : `${m} mnt`;
	}

	// buildParams is the single serializer for the request, the shareable
	// URL, and the cache key — all three must stay identical.
	function buildParams(): URLSearchParams {
		const p = new SvelteURLSearchParams();
		p.set('from', fromStation!.id);
		p.set('to', toStation!.id);
		const t = departAt ? new Date(departAt).toISOString() : '';
		if (t) p.set(anchor === 'arrive' ? 'arriveBy' : 'at', t);
		const mm = [...modes].sort().join(',');
		if (mm) p.set('modes', mm);
		if (maxWalk) p.set('maxWalkM', maxWalk);
		if (maxTransfers) p.set('maxTransfers', maxTransfers);
		if (stepFree) p.set('stepFree', 'true');
		return p;
	}

	// Timeline takes the same params; the itinerary index picks which one.
	function detailHref(i: number, key: string): string {
		return `/journey?${key}&i=${i}`;
	}

	function toggleMode(m: string) {
		const next = new SvelteSet(modes);
		if (next.has(m)) next.delete(m);
		else next.add(m);
		modes = next;
	}

	async function submit() {
		if (!fromStation || !toStation) return;
		planning = true;
		planError = '';
		plan = null;
		planFromCache = false;
		const params = buildParams();
		const key = params.toString();
		const q = Object.fromEntries(params) as Record<string, string>;
		try {
			plan = await unwrap(
				api.GET('/api/v1/journeys', {
					params: {
						query: {
							from: q.from,
							to: q.to,
							at: q.at,
							arriveBy: q.arriveBy,
							modes: q.modes,
							maxWalkM: q.maxWalkM ? Number(q.maxWalkM) : undefined,
							maxTransfers: q.maxTransfers ? Number(q.maxTransfers) : undefined,
							stepFree: q.stepFree ? true : undefined
						}
					}
				})
			);
			saveCached(key, plan);
			planKey = key;
			// Keep the plan shareable: the URL is the snapshot, not app state.
			replaceState(resolve(`/plan?${key}` as `/plan?${string}`), page.state);
		} catch (e) {
			const cached = loadCached(key);
			if (cached) {
				plan = cached;
				planFromCache = true;
				planKey = key;
			} else {
				planError = e instanceof Error ? e.message : 'Pencarian rute gagal.';
			}
		} finally {
			planning = false;
		}
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

	// Shared links carry the full query — restore every input, then plan.
	onMount(async () => {
		const sp = page.url.searchParams;
		const fromId = sp.get('from');
		const toId = sp.get('to');
		if (sp.get('arriveBy')) {
			anchor = 'arrive';
			departAt = toLocalInput(sp.get('arriveBy')!);
		} else if (sp.get('at')) {
			departAt = toLocalInput(sp.get('at')!);
		}
		if (sp.get('modes')) modes = new Set(sp.get('modes')!.split(','));
		maxWalk = sp.get('maxWalkM') ?? '';
		maxTransfers = sp.get('maxTransfers') ?? '';
		stepFree = sp.get('stepFree') === 'true' || sp.get('stepFree') === '1';
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
			// but the cached copy for this exact query still renders.
			const params = new SvelteURLSearchParams();
			for (const [k, v] of sp) params.set(k, v);
			const cached = loadCached(params.toString());
			if (cached) {
				plan = cached;
				planFromCache = true;
				planKey = params.toString();
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

	<div class="anchor" role="group" aria-label="Patokan waktu">
		<button
			type="button"
			class="anchor-btn"
			class:anchor-btn--on={anchor === 'depart'}
			aria-pressed={anchor === 'depart'}
			onclick={() => (anchor = 'depart')}>Berangkat</button
		>
		<button
			type="button"
			class="anchor-btn"
			class:anchor-btn--on={anchor === 'arrive'}
			aria-pressed={anchor === 'arrive'}
			onclick={() => (anchor = 'arrive')}>Tiba sebelum</button
		>
	</div>
	<TextField
		id="at"
		type="datetime-local"
		label={anchor === 'depart' ? 'Berangkat' : 'Tiba sebelum'}
		bind:value={departAt}
	/>

	<fieldset class="opts">
		<legend>Mode</legend>
		<div class="chips">
			{#each MODE_OPTIONS as [value, label] (value)}
				<button
					type="button"
					class="chip"
					class:chip--on={modes.has(value)}
					aria-pressed={modes.has(value)}
					onclick={() => toggleMode(value)}>{label}</button
				>
			{/each}
		</div>
	</fieldset>

	<div class="opts-row">
		<label>
			Jalan maks.
			<select bind:value={maxWalk}>
				<option value="">Bebas</option>
				<option value="200">200 m</option>
				<option value="500">500 m</option>
				<option value="1000">1 km</option>
			</select>
		</label>
		<label>
			Transit maks.
			<select bind:value={maxTransfers}>
				<option value="">Bebas</option>
				<option value="0">Tanpa transit</option>
				<option value="1">1</option>
				<option value="2">2</option>
				<option value="3">3</option>
			</select>
		</label>
	</div>
	<label class="check">
		<input type="checkbox" bind:checked={stepFree} />
		Bebas tangga (lift)
	</label>

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
	{#if plan.itineraries.length === 0}
		<StateBlock kind="empty" illustration="/illustrations/empty-transit-world.webp">
			Tidak ada rute terjadwal antara {plan.from.name} dan {plan.to.name}
			{#if plan.query.stepFree || (plan.query.modes?.length ?? 0) > 0}
				dengan filter ini
			{/if}.
		</StateBlock>
	{:else}
		<ol class="itins">
			{#each plan.itineraries as itin, i (i)}
				{@const key = planKey}
				<li>
					<a class="itin" href={resolve(detailHref(i, key) as `/journey?${string}`)}>
						<header class="itin-head">
							<span class="label">{LABELS[itin.label] ?? itin.label}</span>
							<StatusBadge status={itin.status} />
							<span class="sg-tabular itin-time">
								{fmtTime(itin.departAt)} → {fmtTime(itin.arriveAt)}
							</span>
						</header>
						<p class="sg-meta itin-meta">
							{fmtDur(itin.durationSec)}
							{#if itin.transfers > 0}· {itin.transfers} transit{/if}
							{#if itin.walkM > 0}· jalan {itin.walkM} m{/if}
						</p>
					</a>
				</li>
			{/each}
		</ol>
		{#if plan.fareReference?.total != null}
			<p class="sg-meta fare-ref">
				Estimasi tarif koridor: Rp{plan.fareReference.total.toLocaleString('id-ID')} (referensi)
			</p>
		{/if}
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

	.anchor {
		display: inline-flex;
		border: 1px solid var(--sg-border);
		border-radius: var(--sg-radius-button);
		overflow: hidden;
	}
	.anchor-btn {
		min-height: var(--sg-target-min);
		padding: 0 var(--sg-space-3);
		border: 0;
		background: var(--sg-surface);
		color: var(--sg-text);
		font-weight: var(--sg-weight-bold);
		cursor: pointer;
	}
	.anchor-btn--on {
		background: var(--sg-brand);
		color: var(--sg-brand-contrast);
	}

	.opts {
		border: 0;
		padding: 0;
		margin: 0;
	}
	.opts legend {
		font-size: var(--sg-text-secondary);
		font-weight: var(--sg-weight-bold);
		padding: 0;
		margin-bottom: var(--sg-space-1);
	}
	.chips {
		display: flex;
		flex-wrap: wrap;
		gap: var(--sg-space-1);
	}
	.chip {
		min-height: var(--sg-target-min);
		padding: 0 var(--sg-space-3);
		border: 1px solid var(--sg-border);
		border-radius: var(--sg-radius-pill);
		background: var(--sg-surface);
		color: var(--sg-text);
		font-weight: var(--sg-weight-bold);
		cursor: pointer;
	}
	.chip--on {
		background: var(--sg-brand);
		border-color: transparent;
		color: var(--sg-brand-contrast);
	}

	.opts-row {
		display: flex;
		gap: var(--sg-space-3);
		flex-wrap: wrap;
	}
	.opts-row label {
		display: flex;
		flex-direction: column;
		gap: var(--sg-space-1);
		font-size: var(--sg-text-secondary);
		font-weight: var(--sg-weight-bold);
	}
	.opts-row select {
		min-height: var(--sg-target-min);
		padding: 0 var(--sg-space-2);
		border: 1px solid var(--sg-border);
		border-radius: var(--sg-radius-button);
		background: var(--sg-surface);
		color: var(--sg-text);
	}
	.check {
		display: flex;
		align-items: center;
		gap: var(--sg-space-2);
		font-size: var(--sg-text-secondary);
		min-height: var(--sg-target-min);
	}

	.itins {
		list-style: none;
		padding: 0;
		margin: var(--sg-space-3) 0 0;
		display: flex;
		flex-direction: column;
		gap: var(--sg-space-2);
		max-width: 36rem;
	}
	.itin {
		display: block;
		padding: var(--sg-space-3);
		border: 1px solid var(--sg-border);
		border-radius: var(--sg-radius-card);
		background: var(--sg-surface);
		text-decoration: none;
		color: var(--sg-text);
	}
	@media (prefers-reduced-motion: no-preference) {
		.itins > li {
			animation: sg-row-enter var(--sg-motion-base) var(--sg-ease-enter) both;
		}
		.itins > li:nth-child(2) {
			animation-delay: 25ms;
		}
		.itins > li:nth-child(3) {
			animation-delay: 50ms;
		}
		.itins > li:nth-child(4) {
			animation-delay: 75ms;
		}
	}
	.itin-head {
		display: flex;
		align-items: center;
		gap: var(--sg-space-2);
		flex-wrap: wrap;
	}
	.label {
		font-weight: var(--sg-weight-bold);
	}
	.itin-time {
		font-variant-numeric: tabular-nums;
	}
	.itin-meta {
		margin: var(--sg-space-1) 0 0;
	}
	.fare-ref {
		margin-block-start: var(--sg-space-3);
	}
</style>
