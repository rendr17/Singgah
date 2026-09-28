<script lang="ts">
	import type { PageProps } from './$types';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import JourneyTimeline from '$lib/components/journey/JourneyTimeline.svelte';
	import { SectionHeading, StateBlock, StatusBadge } from '@singgah/ui';

	let { data }: PageProps = $props();
	const plan = $derived(data.plan);
	const itin = $derived(plan?.itineraries[data.index] ?? plan?.itineraries[0] ?? null);

	const LABELS: Record<string, string> = {
		fastest: 'Tercepat',
		fewest_transfers: 'Transit tersedikit',
		least_walking: 'Jalan tersedikit',
		alternative: 'Alternatif'
	};

	// Back lands on the prefilled form — the full query is rebuilt from the
	// plan's own echo so shared links keep their filters.
	const back = $derived.by(() => {
		if (!plan?.from.id || !plan.to.id) return '/plan';
		const p = new URLSearchParams({ from: plan.from.id, to: plan.to.id });
		if (plan.query.departAt) p.set('at', plan.query.departAt);
		if (plan.query.arriveBy) p.set('arriveBy', plan.query.arriveBy);
		if (plan.query.modes?.length) p.set('modes', plan.query.modes.join(','));
		if (plan.query.maxWalkM != null) p.set('maxWalkM', String(plan.query.maxWalkM));
		if (plan.query.maxTransfers != null) p.set('maxTransfers', String(plan.query.maxTransfers));
		if (plan.query.stepFree) p.set('stepFree', 'true');
		return `/plan?${p.toString()}`;
	});

	const fmt = (iso: string) =>
		new Date(iso).toLocaleString('id-ID', { dateStyle: 'medium', timeStyle: 'short' });
	const fmtTime = (iso: string) =>
		new Date(iso).toLocaleTimeString('id-ID', { hour: '2-digit', minute: '2-digit' });
	const fmtDur = (sec: number) => {
		const h = Math.floor(sec / 3600);
		const m = Math.round((sec % 3600) / 60);
		return h > 0 ? `${h} j ${m} mnt` : `${m} mnt`;
	};
</script>

<svelte:head
	><title>{plan ? `${plan.from.name} → ${plan.to.name} · Singgah` : 'Perjalanan · Singgah'}</title
	></svelte:head
>

{#if plan === null}
	<StateBlock kind="error">{data.error}</StateBlock>
	<p><a href={resolve('/plan')}>← Rencana</a></p>
{:else}
	<nav class="back" aria-label="Navigasi balik">
		<a href={resolve(back as `/plan?${string}`)}>← Rencana</a>
	</nav>

	<h1 class="sg-page-title">{plan.from.name} → {plan.to.name}</h1>
	<p class="sg-meta">
		{#if plan.query.departAt}
			Berangkat {fmt(plan.query.departAt)}
		{:else if plan.query.arriveBy}
			Tiba sebelum {fmt(plan.query.arriveBy)}
		{/if}
	</p>
	{#if data.fromCache}
		<StateBlock kind="offline">Rute tersimpan — data per {fmt(plan.source.requestedAt)}</StateBlock>
	{/if}

	{#if itin === null}
		<StateBlock kind="empty">
			Tidak ada rute terjadwal antara {plan.from.name} dan {plan.to.name} dengan filter ini.
		</StateBlock>
	{:else}
		<header class="summary-head">
			<span class="label">{LABELS[itin.label] ?? itin.label}</span>
			<StatusBadge status={itin.status} />
			<span class="sg-tabular">{fmtTime(itin.departAt)} → {fmtTime(itin.arriveAt)}</span>
			<span class="sg-meta">
				{fmtDur(itin.durationSec)}
				{#if itin.transfers > 0}· {itin.transfers} transit{/if}
				{#if itin.walkM > 0}· jalan {itin.walkM} m{/if}
			</span>
		</header>
		{#if itin.reason}
			<p class="sg-meta">{itin.reason}</p>
		{/if}

		<section>
			<SectionHeading>Linimasa perjalanan</SectionHeading>
			<JourneyTimeline from={plan.from} legs={itin.legs} />
		</section>

		{#if plan.itineraries.length > 1}
			<section class="alts">
				<SectionHeading>Alternatif lain</SectionHeading>
				<ol class="alt-list">
					{#each plan.itineraries as other, i (i)}
						{#if other !== itin}
							<li>
								<a
									class="alt-card"
									href={resolve(`/journey?${page.url.searchParams.toString().replace(/&?i=\d+/, '')}&i=${i}` as `/journey?${string}`)}
								>
									<span class="label">{LABELS[other.label] ?? other.label}</span>
									<span class="sg-tabular"
										>{fmtTime(other.departAt)} → {fmtTime(other.arriveAt)}</span
									>
									<span class="sg-meta"
										>{fmtDur(other.durationSec)}{#if other.transfers > 0}
											· {other.transfers} transit{/if}{#if other.walkM > 0}
											· jalan {other.walkM} m{/if}</span
									>
								</a>
							</li>
						{/if}
					{/each}
				</ol>
			</section>
		{/if}

		{#if plan.fareReference && plan.fareReference.segments.length > 0}
			<details class="fare-detail">
				<summary>Estimasi tarif koridor (referensi)</summary>
				<ul>
					{#each plan.fareReference.segments as seg, i (i)}
						<li>
							{seg.operator}: {seg.from.name} → {seg.to.name} — Rp{seg.amount.toLocaleString(
								'id-ID'
							)}
						</li>
					{/each}
				</ul>
				{#if plan.fareReference.total != null}
					<p class="sg-meta">Total referensi: Rp{plan.fareReference.total.toLocaleString('id-ID')}</p>
				{/if}
			</details>
		{/if}
	{/if}

	<p class="sg-meta source">
		Sumber: jadwal ter-ingest ({plan.source.provider}) · snapshot {fmt(plan.source.snapshotAt)} ·
		diminta {fmt(plan.source.requestedAt)}
	</p>
{/if}

<style>
	.back {
		margin-bottom: var(--sg-space-4);
		font-size: var(--sg-text-secondary);
	}

	.back a {
		display: inline-flex;
		align-items: center;
		min-height: var(--sg-target-min);
	}

	.summary-head {
		display: flex;
		align-items: center;
		gap: var(--sg-space-3);
		flex-wrap: wrap;
		max-width: 40rem;
	}

	.label {
		font-weight: var(--sg-weight-bold);
	}

	.alts {
		margin-block-start: var(--sg-space-4);
	}
	.alt-list {
		list-style: none;
		padding: 0;
		margin: var(--sg-space-2) 0 0;
		display: flex;
		flex-direction: column;
		gap: var(--sg-space-2);
		max-width: 40rem;
	}
	.alt-card {
		display: flex;
		align-items: baseline;
		gap: var(--sg-space-3);
		flex-wrap: wrap;
		padding: var(--sg-space-2) var(--sg-space-3);
		border: 1px solid var(--sg-border);
		border-radius: var(--sg-radius-card);
		background: var(--sg-surface);
		text-decoration: none;
		color: var(--sg-text);
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

	.fare-detail {
		margin-block-start: var(--sg-space-4);
	}

	.source {
		margin-block-start: var(--sg-space-5);
	}
</style>
