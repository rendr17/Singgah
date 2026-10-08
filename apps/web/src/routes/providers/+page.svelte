<script lang="ts">
	import { invalidateAll } from '$app/navigation';
	import type { PageProps } from './$types';
	import type { components } from '@singgah/api-client';
	import BackNav from '$lib/components/app-shell/BackNav.svelte';
	import { StateBlock } from '@singgah/ui';

	type Provider = components['schemas']['Provider'];

	let { data }: PageProps = $props();

	function timestamp(iso: string | undefined): string {
		if (!iso) return 'belum tercatat';
		return `${new Date(iso).toLocaleString('id-ID', {
			timeZone: 'Asia/Jakarta',
			dateStyle: 'medium',
			timeStyle: 'short'
		})} WIB`;
	}

	// A newer failed attempt must remain visible beside the last successful update.
	function failedAttempt(p: Provider): boolean {
		if (!p.lastAttemptAt) return false;
		return (
			!p.lastSuccessAt || new Date(p.lastAttemptAt).getTime() > new Date(p.lastSuccessAt).getTime()
		);
	}
</script>

<svelte:head>
	<title>Sumber data &amp; atribusi · Singgah</title>
	<meta
		name="description"
		content="Lihat penyedia data Singgah, lisensi, atribusi, dan waktu pembaruan terakhir."
	/>
</svelte:head>

<BackNav href="/" label="Beranda" />

<h1 class="sg-page-title">Sumber data &amp; atribusi</h1>
<p class="sg-meta intro">
	Setiap sumber memiliki waktu pembaruan dan batas penggunaan sendiri. Waktu di bawah menunjukkan
	pembaruan data sumber, bukan posisi kendaraan secara langsung.
</p>

{#if data.error}
	<StateBlock kind="error">{data.error}</StateBlock>
	<button class="retry" type="button" onclick={() => invalidateAll()}>Coba lagi</button>
{:else if data.providers.length === 0}
	<StateBlock kind="empty">Belum ada sumber data yang terdaftar saat ini.</StateBlock>
{:else}
	<ul class="provider-list">
		{#each data.providers as p (p.code)}
			<li>
				<div class="provider-head">
					<h2>{p.name}</h2>
					<span class:inactive={!p.isActive}>{p.isActive ? 'Aktif' : 'Tidak aktif'}</span>
				</div>
				<p class="provider-code">{p.code}</p>
				<dl>
					<div>
						<dt>Pembaruan berhasil</dt>
						<dd>{timestamp(p.lastSuccessAt)}</dd>
					</div>
					{#if failedAttempt(p)}
						<div class="failed">
							<dt>Percobaan terakhir gagal</dt>
							<dd>{timestamp(p.lastAttemptAt)}</dd>
						</div>
					{/if}
					{#if p.licenseName}<div>
							<dt>Lisensi</dt>
							<dd>{p.licenseName}</dd>
						</div>{/if}
					{#if p.refreshCadence}<div>
							<dt>Jadwal pembaruan</dt>
							<dd>{p.refreshCadence}</dd>
						</div>{/if}
				</dl>
				{#if p.attributionText}<p class="attribution">{p.attributionText}</p>{/if}
				{#if p.allowedUse}<p class="sg-meta">{p.allowedUse}</p>{/if}
				{#if p.knownLimitations}
					<p class="sg-meta">Batas data: {p.knownLimitations}</p>
				{/if}
			</li>
		{/each}
	</ul>
{/if}

<style>
	.intro {
		max-width: 42rem;
		margin-bottom: var(--sg-space-8);
	}
	.provider-list {
		max-width: 48rem;
		margin: 0;
		padding: 0;
		list-style: none;
	}
	.provider-list li {
		padding-block: var(--sg-space-6);
		border-top: 1px solid var(--sg-border);
	}
	.provider-head {
		display: flex;
		align-items: baseline;
		justify-content: space-between;
		gap: var(--sg-space-3);
	}
	.provider-head h2 {
		margin: 0;
		font-size: var(--sg-text-section);
	}
	.provider-head span {
		color: var(--sg-success);
		font-size: var(--sg-text-meta);
		font-weight: var(--sg-weight-semibold);
	}
	.provider-head .inactive,
	.failed {
		color: var(--sg-warning);
	}
	.provider-code {
		margin: var(--sg-space-1) 0 var(--sg-space-4);
		color: var(--sg-text-muted);
		font-family: var(--sg-font-mono);
		font-size: var(--sg-text-meta);
	}
	dl {
		display: grid;
		gap: var(--sg-space-2);
		margin: 0;
	}
	dl div {
		display: grid;
		grid-template-columns: minmax(9rem, 0.7fr) minmax(0, 1.3fr);
		gap: var(--sg-space-3);
	}
	dt {
		color: var(--sg-text-muted);
	}
	dd {
		margin: 0;
	}
	.attribution {
		margin-block: var(--sg-space-4) 0;
		font-weight: var(--sg-weight-medium);
	}
	.retry {
		min-height: var(--sg-target-min);
		padding-inline: var(--sg-space-4);
		border: 1px solid var(--sg-border);
		border-radius: var(--sg-radius-button);
		background: var(--sg-surface);
		color: var(--sg-text);
		font: inherit;
		cursor: pointer;
	}
	@media (max-width: 32rem) {
		dl div {
			grid-template-columns: 1fr;
			gap: 0;
		}
	}
</style>
