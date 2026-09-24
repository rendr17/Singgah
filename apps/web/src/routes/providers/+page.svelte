<script lang="ts">
	import type { PageProps } from './$types';
	import { resolve } from '$app/paths';
	import { SectionHeading, Surface } from '@singgah/ui';

	let { data }: PageProps = $props();

	function ageLabel(iso: string | undefined): string {
		if (!iso) return 'belum pernah ingest';
		const days = Math.floor((Date.now() - new Date(iso).getTime()) / 86_400_000);
		const date = new Date(iso).toLocaleString('id-ID');
		if (days <= 0) return `ingest hari ini · ${date}`;
		if (days === 1) return `ingest kemarin · ${date}`;
		return `ingest ${days} hari lalu · ${date}`;
	}
</script>

<nav><a href={resolve('/')}>← Pencarian</a></nav>

<h1>Penyedia data</h1>
<p class="muted">Sumber data transit, lisensi, dan kesegaran ingest.</p>

{#if data.error}
	<p role="alert">{data.error}</p>
{:else}
	{#each data.providers as p (p.code)}
		<Surface>
			<SectionHeading>{p.name}</SectionHeading>
			<p class="meta">
				<code>{p.code}</code>
				{#if !p.isActive}<span class="stale">· nonaktif</span>{/if}
			</p>
			<p>{ageLabel(p.lastSuccessAt)}</p>
			{#if p.licenseName}<p>Lisensi: {p.licenseName}</p>{/if}
			{#if p.attributionText}<p>{p.attributionText}</p>{/if}
			{#if p.allowedUse}<p class="muted">{p.allowedUse}</p>{/if}
			{#if p.refreshCadence}<p class="muted">Jadwal refresh: {p.refreshCadence}</p>{/if}
			{#if p.knownLimitations}
				<p class="muted">Keterbatasan: {p.knownLimitations}</p>
			{/if}
		</Surface>
	{/each}
{/if}

<style>
	.meta,
	.muted {
		color: var(--sg-text-muted);
		font-size: 0.875rem;
	}
	.stale {
		color: var(--sg-status-estimated, #a66a16);
	}
</style>
