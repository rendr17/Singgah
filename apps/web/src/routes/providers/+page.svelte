<script lang="ts">
	import type { PageProps } from './$types';
	import type { components } from '@singgah/api-client';
	import BackNav from '$lib/components/app-shell/BackNav.svelte';
	import { SectionHeading, StateBlock, Surface } from '@singgah/ui';

	type Provider = components['schemas']['Provider'];

	let { data }: PageProps = $props();

	function ageLabel(iso: string | undefined): string {
		if (!iso) return 'belum pernah ingest';
		const days = Math.floor((Date.now() - new Date(iso).getTime()) / 86_400_000);
		const date = new Date(iso).toLocaleString('id-ID');
		if (days <= 0) return `ingest hari ini · ${date}`;
		if (days === 1) return `ingest kemarin · ${date}`;
		return `ingest ${days} hari lalu · ${date}`;
	}

	// An attempt newer than the last success (or no success at all) means the
	// latest ingest failed — say so instead of looking merely "stale".
	function failedAttempt(p: Provider): string {
		if (!p.lastAttemptAt) return '';
		const failed =
			!p.lastSuccessAt || new Date(p.lastAttemptAt).getTime() > new Date(p.lastSuccessAt).getTime();
		if (!failed) return '';
		return `ingest terakhir gagal · dicoba ${new Date(p.lastAttemptAt).toLocaleString('id-ID')}`;
	}
</script>

<BackNav href="/" label="Pencarian" />

<h1 class="sg-page-title">Penyedia data</h1>
<p class="sg-meta">Sumber data transit, lisensi, dan kesegaran ingest.</p>

{#if data.error}
	<StateBlock kind="error">{data.error}</StateBlock>
{:else}
	{#each data.providers as p (p.code)}
		{@const failed = failedAttempt(p)}
		<Surface>
			<SectionHeading>{p.name}</SectionHeading>
			<p class="sg-meta">
				<code>{p.code}</code>
				{#if !p.isActive}<span class="sg-warn">· nonaktif</span>{/if}
			</p>
			<p>{ageLabel(p.lastSuccessAt)}</p>
			{#if failed}<p class="sg-warn">{failed}</p>{/if}
			{#if p.licenseName}<p>Lisensi: {p.licenseName}</p>{/if}
			{#if p.attributionText}<p>{p.attributionText}</p>{/if}
			{#if p.allowedUse}<p class="sg-meta">{p.allowedUse}</p>{/if}
			{#if p.refreshCadence}<p class="sg-meta">Jadwal refresh: {p.refreshCadence}</p>{/if}
			{#if p.knownLimitations}
				<p class="sg-meta">Keterbatasan: {p.knownLimitations}</p>
			{/if}
		</Surface>
	{/each}
{/if}
