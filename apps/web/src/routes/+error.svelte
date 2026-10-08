<script lang="ts">
	import { page } from '$app/state';
	import { resolve } from '$app/paths';

	const missing = $derived(page.status === 404);
</script>

<svelte:head>
	<title>{missing ? 'Halaman tidak ditemukan' : 'Halaman bermasalah'} · Singgah</title>
	<meta
		name="description"
		content={missing
			? 'Alamat halaman ini tidak ditemukan di Singgah.'
			: 'Halaman Singgah belum dapat dibuka. Coba lagi atau kembali ke peta.'}
	/>
</svelte:head>

<div class="error-page">
	<div class="route-art" aria-hidden="true">
		<span class="route-art__start">A</span>
		<span class="route-art__line"></span>
		<span class="route-art__end">?</span>
	</div>
	<p class="eyebrow">SINGGAH / {page.status}</p>
	<h1 class="sg-page-title">
		{missing ? 'Jalurnya belum ketemu.' : 'Perjalanan terhenti sebentar.'}
	</h1>
	<p class="sg-meta" role={missing ? undefined : 'alert'}>
		{missing
			? 'Alamat ini tidak mengarah ke halaman Singgah. Kamu bisa kembali ke peta dan mulai dari sana.'
			: 'Halaman ini belum bisa dimuat. Coba lagi; kalau masih gagal, buka peta untuk melanjutkan.'}
	</p>
	<div class="actions">
		{#if !missing}<button type="button" onclick={() => location.reload()}>Coba lagi</button>{/if}
		<a href={resolve('/app')}>Buka peta <span aria-hidden="true">↗</span></a>
		<a class="home-link" href={resolve('/')}>Kembali ke beranda</a>
	</div>
</div>

<style>
	.error-page {
		max-width: 38rem;
		padding-block: clamp(3rem, 10vh, 7rem);
	}
	.route-art {
		display: flex;
		align-items: center;
		width: min(100%, 19rem);
		margin-bottom: var(--sg-space-8);
	}
	.route-art__start,
	.route-art__end {
		display: grid;
		place-items: center;
		flex: none;
		width: 2.25rem;
		height: 2.25rem;
		border: 2px solid var(--sg-brand);
		border-radius: 50%;
		background: var(--sg-surface);
		color: var(--sg-brand);
		font-weight: var(--sg-weight-bold);
	}
	.route-art__end {
		border-color: var(--sg-border-strong);
		color: var(--sg-text-muted);
	}
	.route-art__line {
		flex: 1;
		border-top: 2px dashed var(--sg-border-strong);
	}
	.eyebrow {
		margin: 0 0 var(--sg-space-2);
		color: var(--sg-brand);
		font-family: var(--sg-font-mono);
		font-size: var(--sg-text-meta);
		letter-spacing: 0.1em;
	}
	.sg-page-title {
		font-size: clamp(2rem, 5vw, 3.25rem);
	}
	.sg-meta {
		max-width: 34rem;
		margin-block: var(--sg-space-3) var(--sg-space-6);
	}
	.actions {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: var(--sg-space-2);
	}
	.actions a,
	.actions button {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		min-height: var(--sg-target-min);
		padding-inline: var(--sg-space-4);
		border: 1px solid var(--sg-border);
		border-radius: var(--sg-radius-button);
		background: var(--sg-surface);
		color: var(--sg-text);
		font: inherit;
		text-decoration: none;
		cursor: pointer;
	}
	.actions a:not(.home-link) {
		border-color: var(--sg-brand);
		background: var(--sg-brand);
		color: var(--sg-brand-contrast);
		gap: var(--sg-space-2);
	}
	.actions a:hover,
	.actions button:hover {
		filter: brightness(0.95);
	}
	.actions .home-link {
		border: 0;
	}
</style>
