<script lang="ts">
	import { onMount } from 'svelte';
	import { resolve } from '$app/paths';
	import { authedApi, clearSession } from '$lib/session';
	import { Button, SectionHeading, StateBlock } from '@singgah/ui';
	import type { components } from '@singgah/api-client';

	type SavedPlace = components['schemas']['SavedPlace'];
	let places = $state<SavedPlace[] | null>(null);
	let loading = $state(true);
	let error = $state('');
	let removing = $state('');

	onMount(() => {
		void loadSaved();
	});

	async function loadSaved() {
		loading = true;
		error = '';
		try {
			const {
				data,
				error: apiError,
				response
			} = await authedApi.GET('/api/v1/places/saved', {
				params: { query: { limit: 50 } }
			});
			if (apiError || !data) {
				if (response.status === 401) clearSession();
				error = 'Daftar tempat tersimpan gagal dimuat.';
				return;
			}
			places = data.places;
		} catch {
			error = 'Koneksi terputus. Daftar tersimpan belum dapat dimuat.';
		} finally {
			loading = false;
		}
	}

	async function removePlace(id: string) {
		removing = id;
		error = '';
		try {
			const { error: apiError, response } = await authedApi.DELETE('/api/v1/places/{id}/saved', {
				params: { path: { id } }
			});
			if (apiError) {
				if (response.status === 401) clearSession();
				throw new Error('Tempat gagal dihapus. Coba lagi.');
			}
			places = (places ?? []).filter((place) => place.id !== id);
		} catch (caught) {
			error = caught instanceof Error ? caught.message : 'Tempat gagal dihapus.';
		} finally {
			removing = '';
		}
	}
</script>

<svelte:head><title>Tempat tersimpan · Singgah</title></svelte:head>

<a class="back-link" href={resolve('/explore')}>← Jelajah</a>
<h1 class="sg-page-title">Tempat tersimpan</h1>
<p class="sg-meta">Daftar ini privat dan terikat ke sesi anonim di perangkat ini.</p>

<section aria-label="Daftar tersimpan">
	<SectionHeading>Daftar pribadi</SectionHeading>
	{#if loading}
		<StateBlock kind="loading">Memuat tempat tersimpan…</StateBlock>
	{:else if error}
		<StateBlock kind="error">{error}</StateBlock>
		<Button variant="secondary" onclick={loadSaved}>Coba lagi</Button>
	{:else if !places?.length}
		<StateBlock kind="empty"
			>Belum ada tempat tersimpan. Buka detail tempat lalu pilih “Simpan tempat”.</StateBlock
		>
	{:else}
		<ul class="saved-list">
			{#each places as place (place.id)}
				<li>
					<div>
						<a class="place-name" href={resolve('/places/[id]', { id: place.id })}>{place.name}</a>
						<p class="sg-meta">
							{place.category} · dekat {place.transitStopName} · {place.walkDistanceM} m dari halte/stasiun
						</p>
						<p class="source-note">
							{place.source.attributionText ?? place.source.name}{#if place.source.licenseName}
								· {place.source.licenseName}{/if}
						</p>
						{#if place.visitedAt}<p class="sg-meta">
								Dikunjungi {new Date(place.visitedAt).toLocaleDateString('id-ID')}
							</p>{/if}
					</div>
					<Button
						variant="ghost"
						disabled={removing === place.id}
						onclick={() => removePlace(place.id)}
					>
						{removing === place.id ? 'Menghapus…' : 'Hapus'}
					</Button>
				</li>
			{/each}
		</ul>
		<p class="sg-meta">
			Jarak transit yang ditampilkan adalah estimasi garis lurus, bukan rute berjalan.
		</p>
	{/if}
</section>

<style>
	.back-link {
		color: var(--sg-brand);
		font-size: var(--sg-text-secondary);
	}
	.saved-list {
		list-style: none;
		margin: 0;
		padding: 0;
		max-width: 44rem;
	}
	.saved-list li {
		display: flex;
		justify-content: space-between;
		align-items: flex-start;
		gap: var(--sg-space-3);
		padding-block: var(--sg-space-4);
		border-block-start: 1px solid var(--sg-border);
	}
	.saved-list li:last-child {
		border-block-end: 1px solid var(--sg-border);
	}
	.place-name {
		color: var(--sg-text);
		font-weight: var(--sg-weight-semibold);
	}
	.saved-list p {
		margin-block: var(--sg-space-1) 0;
	}
	.source-note {
		color: var(--sg-text-muted);
		font-size: var(--sg-text-meta);
	}
</style>
