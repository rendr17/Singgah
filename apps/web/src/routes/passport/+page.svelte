<script lang="ts">
	import { onMount } from 'svelte';
	import { authedApi, clearSession } from '$lib/session';
	import { MODE_LABELS, type RouteMode } from '$lib/route-groups';
	import { SectionHeading, StateBlock } from '@singgah/ui';
	import type { components } from '@singgah/api-client';

	type Progress = components['schemas']['PassportProgress'];

	let progress = $state<Progress | null>(null);
	let error = $state('');

	// Routes with any recorded visit, most-complete first — the empty tail of
	// untouched corridors would bury the signal.
	const visitedRoutes = $derived(
		(progress?.byRoute ?? [])
			.filter((r) => r.visitedStops > 0)
			.sort((a, b) => b.visitedStops / b.totalStops - a.visitedStops / a.totalStops)
	);

	function modeLabel(mode: string): string {
		return MODE_LABELS[mode as RouteMode] ?? mode;
	}

	onMount(async () => {
		const { data, error: e, response } = await authedApi.GET('/api/v1/passport/progress');
		if (e || !data) {
			if (response.status === 401) clearSession();
			error = 'Progres paspor gagal dimuat — periksa koneksi lalu muat ulang.';
			return;
		}
		progress = data;
	});
</script>

<svelte:head><title>Paspor · Singgah</title></svelte:head>

<h1 class="sg-page-title">Paspor</h1>
<p class="sg-meta">
	Catatan stasiun dan rute yang sudah kamu lewati — tersimpan privat, hanya di
	akses lewat token perangkat ini.
</p>

{#if error}
	<StateBlock kind="error">{error}</StateBlock>
{:else if progress === null}
	<div class="sg-skeleton" style:height="6rem"></div>
{:else}
	<section>
		<SectionHeading>Jelajah jaringan</SectionHeading>
		<p class="total">
			<strong>{progress.visitedStops}</strong>
			dari {progress.totalStops} stasiun terlayani rute
		</p>
		<ul class="sg-list modes">
			{#each progress.byMode.filter((m) => m.totalStops > 0) as m (m.mode)}
				<li>
					<div class="bar-row">
						<span class="bar-label">{modeLabel(m.mode)}</span>
						<span class="bar-count">{m.visitedStops}/{m.totalStops}</span>
					</div>
					<div
						class="bar"
						role="progressbar"
						aria-valuenow={m.visitedStops}
						aria-valuemax={m.totalStops}
						aria-label="Progres {modeLabel(m.mode)}"
					>
						<div
							class="bar-fill"
							style:width={(m.totalStops ? (m.visitedStops / m.totalStops) * 100 : 0) + '%'}
						></div>
					</div>
				</li>
			{/each}
		</ul>
	</section>

	<section>
		<SectionHeading>Koridor yang sudah dilewati</SectionHeading>
		{#if visitedRoutes.length === 0}
			<StateBlock kind="empty">
				Belum ada kunjungan tercatat — tandai stasiun dari halaman detailnya.
			</StateBlock>
		{:else}
			<ul class="sg-list">
				{#each visitedRoutes as r (r.routeId)}
					<li class="route-row">
						{#if r.color}
							<span class="sg-dot" style:background-color={'#' + r.color}></span>
						{/if}
						<span>{r.name}</span>
						<span class="muted">{r.visitedStops}/{r.totalStops} stasiun</span>
					</li>
				{/each}
			</ul>
		{/if}
	</section>
{/if}

<style>
	.total {
		margin-block: 0 var(--sg-space-3);
	}
	.total strong {
		font-size: var(--sg-text-screen);
	}
	.modes li {
		margin-block-end: var(--sg-space-3);
	}
	.bar-row {
		display: flex;
		justify-content: space-between;
		font-size: var(--sg-text-secondary);
	}
	.bar-count {
		color: var(--sg-text-muted);
	}
	.bar {
		height: 0.375rem;
		border-radius: 999px;
		background-color: var(--sg-surface-muted);
		overflow: hidden;
	}
	.bar-fill {
		height: 100%;
		background-color: var(--sg-brand);
	}
	.route-row {
		display: flex;
		align-items: baseline;
		gap: var(--sg-space-2);
	}
	.muted {
		color: var(--sg-text-muted);
		font-size: var(--sg-text-secondary);
	}
</style>
