<script lang="ts">
	import type { PageProps } from './$types';
	import { resolve } from '$app/paths';
	import JourneyTimeline from '$lib/components/journey/JourneyTimeline.svelte';
	import { SectionHeading, StateBlock, StatusBadge } from '@singgah/ui';

	let { data }: PageProps = $props();
	const plan = $derived(data.plan);
	const itin = $derived(plan?.itinerary ?? null);

	// Back lands on the prefilled form — BackNav only takes bare pathnames, so
	// the query is rebuilt here.
	const back = $derived.by(() => {
		if (!plan?.from.id || !plan.to.id) return null;
		return {
			from: plan.from.id,
			to: plan.to.id,
			at: plan.at ? `&at=${encodeURIComponent(plan.at)}` : ''
		};
	});

	const fmt = (iso: string) =>
		new Date(iso).toLocaleString('id-ID', { dateStyle: 'medium', timeStyle: 'short' });
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
		{#if back}
			<a href={resolve(`/plan?from=${back.from}&to=${back.to}${back.at}`)}>← Rencana</a>
		{:else}
			<a href={resolve('/plan')}>← Rencana</a>
		{/if}
	</nav>

	<h1 class="sg-page-title">{plan.from.name} → {plan.to.name}</h1>
	{#if plan.at}
		<p class="sg-meta">Untuk berangkat {fmt(plan.at)}</p>
	{/if}
	{#if data.fromCache}
		<StateBlock kind="offline">Rute tersimpan — data per {fmt(plan.source.requestedAt)}</StateBlock>
	{/if}

	{#if itin === null}
		<StateBlock kind="empty">
			Provider tidak menemukan rute antara {plan.from.name} dan {plan.to.name}.
		</StateBlock>
	{:else}
		<header class="summary-head">
			<StatusBadge status={itin.status} />
			{#if itin.fare?.total != null}
				<span class="fare">Rp{itin.fare.total.toLocaleString('id-ID')}</span>
			{/if}
			<span class="sg-meta">
				{(itin.totalDistanceM / 1000).toFixed(1)} km · {itin.rideLegs} naik
				{#if itin.walkTransfers > 0}· {itin.walkTransfers} transit jalan{/if}
			</span>
		</header>

		<section>
			<SectionHeading>Linimasa perjalanan</SectionHeading>
			<JourneyTimeline from={plan.from} legs={itin.legs} />
		</section>

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
	{/if}

	<p class="sg-meta source">
		Sumber: {plan.source.provider} · diminta {fmt(plan.source.requestedAt)}
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

	.fare {
		font-weight: var(--sg-weight-bold);
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

	.source {
		margin-block-start: var(--sg-space-5);
	}
</style>
