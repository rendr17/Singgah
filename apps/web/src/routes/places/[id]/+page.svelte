<script lang="ts">
	import { onMount } from 'svelte';
	import { resolve } from '$app/paths';
	import { authedApi, clearSession, readSession } from '$lib/session';
	import { Button, SectionHeading, StateBlock } from '@singgah/ui';
	import type { components } from '@singgah/api-client';
	import type { PageProps } from './$types';

	type Personal = components['schemas']['PlacePersonalResponse']['personal'];
	let { data }: PageProps = $props();
	let personal = $state<Personal | null>(null);
	let personalLoading = $state(false);
	let busy = $state<'save' | 'visit' | null>(null);
	let actionMessage = $state('');
	let actionError = $state('');

	onMount(() => {
		// Do not mint an anonymous session just because someone opened a public POI page.
		if (readSession(localStorage)) void loadPersonal();
	});

	async function loadPersonal() {
		if (!data.place) return;
		personalLoading = true;
		try {
			const {
				data: result,
				error,
				response
			} = await authedApi.GET('/api/v1/places/{id}/personal', {
				params: { path: { id: data.place.id } }
			});
			if (error || !result) {
				if (response.status === 401) clearSession();
				return;
			}
			personal = result.personal;
		} catch {
			// Public place detail remains useful when the private session endpoint is offline.
		} finally {
			personalLoading = false;
		}
	}

	async function toggleSaved() {
		if (!data.place || busy) return;
		busy = 'save';
		actionError = '';
		actionMessage = '';
		try {
			if (personal?.saved) {
				const { error, response } = await authedApi.DELETE('/api/v1/places/{id}/saved', {
					params: { path: { id: data.place.id } }
				});
				if (error) {
					if (response.status === 401) clearSession();
					throw new Error('Tempat gagal dihapus dari daftar tersimpan.');
				}
				personal = { saved: false, visitedAt: personal.visitedAt };
				actionMessage = 'Tempat dihapus dari daftar tersimpan.';
			} else {
				const { error, response } = await authedApi.PUT('/api/v1/places/{id}/saved', {
					params: { path: { id: data.place.id } }
				});
				if (error) {
					if (response.status === 401) clearSession();
					throw new Error('Tempat gagal disimpan.');
				}
				personal = { saved: true, visitedAt: personal?.visitedAt ?? null };
				actionMessage = 'Tempat tersimpan di akun sesi perangkat ini.';
			}
		} catch (error) {
			actionError = error instanceof Error ? error.message : 'Aksi simpan gagal.';
		} finally {
			busy = null;
		}
	}

	async function recordVisit() {
		if (!data.place || busy) return;
		busy = 'visit';
		actionError = '';
		actionMessage = '';
		const observedAt = new Date().toISOString();
		try {
			const {
				data: result,
				error,
				response
			} = await authedApi.POST('/api/v1/places/{id}/visits', {
				params: { path: { id: data.place.id } },
				body: { clientMutationId: crypto.randomUUID(), observedAt }
			});
			if (error || !result) {
				if (response.status === 401) clearSession();
				throw new Error('Kunjungan belum tersimpan. Coba lagi saat koneksi tersedia.');
			}
			personal = { saved: personal?.saved ?? false, visitedAt: result.visit.observedAt };
			actionMessage = result.replayed
				? 'Kunjungan sebelumnya sudah tercatat.'
				: 'Kunjungan dicatat secara manual.';
		} catch (error) {
			actionError = error instanceof Error ? error.message : 'Kunjungan belum tersimpan.';
		} finally {
			busy = null;
		}
	}
</script>

<svelte:head><title>{data.place?.name ?? 'Detail tempat'} · Singgah</title></svelte:head>

{#if data.error}
	<StateBlock kind="error">{data.error}</StateBlock>
{:else if data.place}
	{@const place = data.place}
	<a class="back-link" href={resolve('/explore')}>← Jelajah</a>
	<h1 class="sg-page-title">{place.name}</h1>
	<p class="sg-meta">
		{place.category}{#if place.curated}
			· Kurasi editorial{/if}{#if place.priceBand != null}
			· kisaran harga {place.priceBand}{/if}
	</p>
	<p class="coordinates">{place.lat.toFixed(5)}, {place.lon.toFixed(5)}</p>

	<section aria-label="Daftar pribadi">
		<SectionHeading>Daftar pribadi</SectionHeading>
		<div class="actions">
			<Button variant="secondary" disabled={busy !== null || personalLoading} onclick={toggleSaved}>
				{busy === 'save'
					? 'Menyimpan…'
					: personal?.saved
						? 'Hapus dari tersimpan'
						: 'Simpan tempat'}
			</Button>
			<Button disabled={busy !== null} onclick={recordVisit}>
				{busy === 'visit' ? 'Mencatat…' : 'Tandai sudah dikunjungi'}
			</Button>
		</div>
		<p class="sg-meta">
			Kunjungan dicatat hanya setelah kamu menekan tombol; aplikasi tidak meminta atau menyimpan
			lokasi perangkat.
		</p>
		{#if personal?.visitedAt}<p class="sg-meta">
				Kunjungan terakhir: {new Date(personal.visitedAt).toLocaleString('id-ID')}
			</p>{/if}
		{#if actionMessage}<p class="feedback" role="status">{actionMessage}</p>{/if}
		{#if actionError}<p class="error" role="alert">{actionError}</p>{/if}
	</section>

	<section aria-label="Akses transit terdekat">
		<SectionHeading>Akses transit terdekat</SectionHeading>
		{#if place.transitAccess.length === 0}
			<StateBlock kind="empty">Belum ada akses transit terverifikasi untuk tempat ini.</StateBlock>
		{:else}
			<ul class="access-list">
				{#each place.transitAccess as access (access.stopId)}
					<li>
						<a href={resolve('/stations/[id]', { id: access.stopId })}>{access.stopName}</a>
						<span
							>{access.walkDistanceM} m{#if access.walkSeconds !== null}
								· ±{Math.max(1, Math.round(access.walkSeconds / 60))} menit{/if}</span
						>
					</li>
				{/each}
			</ul>
			<p class="sg-meta">
				Jarak garis lurus × faktor detour — estimasi, bukan rute pedestrian. Dihitung {new Date(
					place.transitAccess[0].computedAt
				).toLocaleDateString('id-ID')}.
			</p>
		{/if}
	</section>

	<section aria-label="Aksesibilitas">
		<SectionHeading>Aksesibilitas</SectionHeading>
		{#if Object.keys(place.accessibility).length}
			<pre class="metadata">{JSON.stringify(place.accessibility, null, 2)}</pre>
		{:else}
			<StateBlock kind="empty">Belum ada metadata aksesibilitas dari sumber.</StateBlock>
		{/if}
	</section>

	<section aria-label="Sumber data">
		<SectionHeading>Sumber data</SectionHeading>
		<p>
			{place.source.name}{#if place.source.licenseName}
				· {place.source.licenseName}{/if}
		</p>
		{#if place.source.attributionText}<p class="sg-meta">{place.source.attributionText}</p>{/if}
		{#if place.sourceUpdatedAt}<p class="sg-meta">
				Pembaruan sumber: {new Date(place.sourceUpdatedAt).toLocaleDateString('id-ID')}
			</p>{/if}
		{#if place.source.url}
			<!-- External provider URLs are intentionally not SvelteKit routes. -->
			<!-- eslint-disable-next-line svelte/no-navigation-without-resolve -->
			<a href={place.source.url} target="_blank" rel="noopener noreferrer">Lihat sumber ↗</a>
		{/if}
		{#if place.source.termsUrl}
			<!-- External provider URLs are intentionally not SvelteKit routes. -->
			<!-- eslint-disable-next-line svelte/no-navigation-without-resolve -->
			<a class="terms-link" href={place.source.termsUrl} target="_blank" rel="noopener noreferrer"
				>Ketentuan sumber ↗</a
			>
		{/if}
	</section>
{:else}
	<StateBlock kind="error">Tempat tidak tersedia.</StateBlock>
{/if}

<style>
	.back-link {
		color: var(--sg-brand);
		font-size: var(--sg-text-secondary);
	}
	.coordinates {
		color: var(--sg-text-muted);
		font-size: var(--sg-text-meta);
	}
	.actions {
		display: flex;
		flex-wrap: wrap;
		gap: var(--sg-space-2);
	}
	.feedback {
		color: var(--sg-text);
	}
	.error {
		color: var(--sg-danger);
	}
	.access-list {
		list-style: none;
		margin: 0;
		padding: 0;
		max-width: 40rem;
	}
	.access-list li {
		display: flex;
		justify-content: space-between;
		gap: var(--sg-space-3);
		padding: var(--sg-space-3) 0;
		border-block-start: 1px solid var(--sg-border);
	}
	.access-list li:last-child {
		border-block-end: 1px solid var(--sg-border);
	}
	.metadata {
		max-width: 42rem;
		overflow-x: auto;
		padding: var(--sg-space-3);
		border: 1px solid var(--sg-border);
		border-radius: var(--sg-radius-card);
	}
	.terms-link {
		margin-inline-start: var(--sg-space-3);
	}
</style>
