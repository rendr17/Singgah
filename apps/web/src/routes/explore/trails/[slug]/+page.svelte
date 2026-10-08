<script lang="ts">
	import { resolve } from '$app/paths';
	import { Button, SectionHeading, StateBlock } from '@singgah/ui';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();
	let skipped = $state<number[]>([]);

	function toggleSkipped(sequence: number) {
		skipped = skipped.includes(sequence)
			? skipped.filter((value) => value !== sequence)
			: [...skipped, sequence];
	}
</script>

<svelte:head><title>{data.trail?.title ?? 'Detail jalur'} · Singgah</title></svelte:head>

<a class="back-link" href={resolve('/explore/trails')}>← Semua jalur</a>
{#if data.error}
	<StateBlock kind="error">{data.error}</StateBlock>
{:else if data.trail}
	{@const trail = data.trail}
	<h1 class="sg-page-title">{trail.title}</h1>
	<p class="sg-meta">{trail.description}</p>

	<section aria-label="Mulai dan akhiri dengan transit">
		<SectionHeading>Mulai dan akhiri dengan transit</SectionHeading>
		<div class="transit-pair">
			<a href={resolve('/stations/[id]', { id: trail.startTransit.id })}>
				<strong>Mulai</strong><span>{trail.startTransit.name}</span>
			</a>
			<span aria-hidden="true">→</span>
			<a href={resolve('/stations/[id]', { id: trail.endTransit.id })}>
				<strong>Akhiri</strong><span>{trail.endTransit.name}</span>
			</a>
		</div>
	</section>

	<section aria-label="Estimasi untuk seluruh jalur">
		<SectionHeading>Estimasi untuk seluruh jalur</SectionHeading>
		{#if trail.canStart && trail.walkDistanceM !== undefined && trail.estimatedDurationMinutes !== undefined}
			<p class="estimate">
				<strong>{trail.walkDistanceM} m</strong> total jalan ·
				<strong>±{trail.estimatedDurationMinutes} menit</strong>
			</p>
			<p class="sg-meta">{trail.walkingEstimateBasis}. {trail.durationIncludes}.</p>
		{:else}
			<StateBlock kind="offline"
				>Estimasi penuh belum tersedia karena akses transit pada jalur belum lengkap.</StateBlock
			>
		{/if}
		{#if trail.budgetMinIDR !== undefined || trail.budgetMaxIDR !== undefined}
			<p class="sg-meta">
				Perkiraan biaya: {trail.budgetMinIDR ?? '—'}–{trail.budgetMaxIDR ?? '—'} IDR; cek harga dari pengelola
				tempat.
			</p>
		{/if}
	</section>

	<section aria-label="Urutan tempat">
		<SectionHeading
			>Urutan tempat · {trail.stops?.length ?? trail.availableStopCount}</SectionHeading
		>
		{#if !trail.canStart || !trail.stops?.length}
			<StateBlock kind="empty"
				>Stop kurasi belum lengkap di katalog, jadi jalur belum bisa dimulai.</StateBlock
			>
		{:else}
			<ol class="stops">
				{#each trail.stops as stop (stop.sequence)}
					<li class:stop-skipped={skipped.includes(stop.sequence)}>
						<div class="stop-heading">
							<span class="sequence">{stop.sequence}</span>
							<div>
								<a class="place-name" href={resolve('/places/[id]', { id: stop.place.id })}
									>{stop.place.name}</a
								>
								<p class="sg-meta">
									{stop.place.category} · rencana singgah {stop.stayMinutes} menit · {stop.walkDistanceFromPreviousM}
									m dari titik sebelumnya
								</p>
							</div>
						</div>
						{#if stop.notes}<p class="stop-notes">{stop.notes}</p>{/if}
						<p class="sg-meta">
							Jam operasional tidak tersedia di data jalur ini; cek ke pengelola sebelum berangkat.
						</p>
						{#if stop.place.accessibility && Object.keys(stop.place.accessibility).length > 0}
							<p class="sg-meta">Ada catatan aksesibilitas dari sumber; lihat detail tempat.</p>
						{/if}
						<p class="source-note">
							{#if stop.place.source.attribution}{stop.place.source.attribution}{:else}{stop.place
									.source.name}{/if}
							{#if stop.place.source.licenseName}
								· {stop.place.source.licenseName}{/if}
						</p>
						<Button
							variant="secondary"
							aria-pressed={skipped.includes(stop.sequence)}
							onclick={() => toggleSkipped(stop.sequence)}
						>
							{skipped.includes(stop.sequence) ? 'Kembalikan stop' : 'Lewati stop ini'}
						</Button>
					</li>
				{/each}
			</ol>
			<p class="sg-meta" role="status">
				{skipped.length
					? `${skipped.length} stop dilewati untuk tampilan ini; estimasi tetap untuk jalur penuh.`
					: 'Semua stop tampil. Kamu bisa melewati stop tanpa mengubah urutan kurasi.'}
			</p>
		{/if}
	</section>
{:else}
	<StateBlock kind="error">Jalur tidak tersedia.</StateBlock>
{/if}

<style>
	.back-link {
		color: var(--sg-brand);
		font-size: var(--sg-text-secondary);
	}
	.transit-pair {
		display: flex;
		align-items: center;
		flex-wrap: wrap;
		gap: var(--sg-space-3);
	}
	.transit-pair a {
		display: grid;
		gap: var(--sg-space-1);
		color: inherit;
	}
	.transit-pair strong {
		color: var(--sg-text-muted);
		font-size: var(--sg-text-meta);
	}
	.estimate {
		font-size: var(--sg-text-body);
	}
	.stops {
		list-style: none;
		margin: 0;
		padding: 0;
		max-width: 46rem;
	}
	.stops > li {
		border-block-start: 1px solid var(--sg-border);
		padding-block: var(--sg-space-4);
	}
	.stops > li:last-child {
		border-block-end: 1px solid var(--sg-border);
	}
	.stop-heading {
		display: flex;
		align-items: flex-start;
		gap: var(--sg-space-3);
	}
	.sequence {
		display: grid;
		place-items: center;
		width: 2rem;
		height: 2rem;
		flex: 0 0 2rem;
		border: 1px solid var(--sg-border);
		border-radius: 50%;
		font-weight: var(--sg-weight-semibold);
	}
	.place-name {
		font-weight: var(--sg-weight-semibold);
		color: var(--sg-text);
	}
	.stop-heading p,
	.stop-notes {
		margin-block: var(--sg-space-1);
	}
	.stop-notes {
		padding-inline-start: 2.75rem;
	}
	.source-note {
		color: var(--sg-text-muted);
		font-size: var(--sg-text-meta);
	}
	.stop-skipped {
		opacity: 0.58;
	}
</style>
