<script lang="ts">
	import { onMount } from 'svelte';
	import { browser } from '$app/environment';
	import { authedApi, clearSession } from '$lib/session';
	import { resolve } from '$app/paths';
	import { MODE_LABELS, type RouteMode } from '$lib/route-groups';
	import FailedQueue from '$lib/components/passport/FailedQueue.svelte';
	import { Button, SectionHeading, StateBlock } from '@singgah/ui';
	import type { components } from '@singgah/api-client';

	type Progress = components['schemas']['PassportProgress'];
	type JournalEntry = components['schemas']['JournalEntry'];

	let progress = $state<Progress | null>(null);
	let entries = $state<JournalEntry[] | null>(null);
	let error = $state('');
	// Per-entry editor state: id → draft text; absent id = not editing.
	let editing = $state<Record<string, string>>({});
	let entryError = $state('');

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
		const [{ data, error: e, response }, journal] = await Promise.all([
			authedApi.GET('/api/v1/passport/progress'),
			authedApi.GET('/api/v1/journal')
		]);
		if (e || !data) {
			if (response.status === 401) clearSession();
			error = 'Progres paspor gagal dimuat — periksa koneksi lalu muat ulang.';
			return;
		}
		progress = data;
		if (journal.error) {
			if (journal.response.status === 401) clearSession();
			entryError = 'Catatan gagal dimuat.';
		} else {
			entries = journal.data?.entries ?? [];
		}
	});

	async function saveEntry(entry: JournalEntry) {
		const text = (editing[entry.id] ?? '').trim();
		if (!text) return;
		entryError = '';
		// baseUpdatedAt carries the optimistic version check — a 409 means the
		// row changed elsewhere and overwriting it would silently lose data.
		const { data, error: e, response } = await authedApi.PATCH('/api/v1/journal/{id}', {
			params: { path: { id: entry.id } },
			body: { body: text, baseUpdatedAt: entry.updatedAt }
		});
		if (e || !data) {
			if (response.status === 401) clearSession();
			entryError =
				response.status === 409
					? 'Catatan berubah di tempat lain — muat ulang.'
					: 'Gagal menyimpan catatan.';
			return;
		}
		entries = (entries ?? []).map((x) => (x.id === entry.id ? data.entry : x));
		delete editing[entry.id];
	}

	async function deleteEntry(id: string) {
		entryError = '';
		const { error: e, response } = await authedApi.DELETE('/api/v1/journal/{id}', {
			params: { path: { id } }
		});
		if (e) {
			if (response.status === 401) clearSession();
			entryError = 'Gagal menghapus catatan.';
			return;
		}
		entries = (entries ?? []).filter((x) => x.id !== id);
		delete editing[id];
	}
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

	{#if progress.byCollection.length > 0}
		<section>
			<SectionHeading>Koleksi</SectionHeading>
			<ul class="sg-list modes">
				{#each progress.byCollection as c (c.collectionId)}
					<li>
						<div class="bar-row">
							<span class="bar-label">{c.title}</span>
							<span class="bar-count">{c.visitedStops}/{c.totalStops}</span>
						</div>
						<div
							class="bar"
							role="progressbar"
							aria-valuenow={c.visitedStops}
							aria-valuemax={c.totalStops}
							aria-label="Progres {c.title}"
						>
							<div
								class="bar-fill"
								style:width={(c.totalStops
									? (c.visitedStops / c.totalStops) * 100
									: 0) + '%'}
							></div>
						</div>
					</li>
				{/each}
			</ul>
		</section>
	{/if}

	{#if browser}
		<FailedQueue />
	{/if}

	<section>
		<SectionHeading>Catatan</SectionHeading>
		{#if entryError}
			<StateBlock kind="error">{entryError}</StateBlock>
		{:else if entries === null}
			<div class="sg-skeleton" style:height="3rem"></div>
		{:else if entries.length === 0}
			<StateBlock kind="empty">
				Belum ada catatan — tulis dari halaman detail stasiun.
			</StateBlock>
		{:else}
			<ul class="sg-list">
				{#each entries as entry (entry.id)}
					<li class="entry">
						{#if editing[entry.id] !== undefined}
							<textarea
								bind:value={editing[entry.id]}
								rows="3"
								maxlength="2000"
								aria-label="Ubah catatan"
							></textarea>
							<div class="entry-actions">
								<Button variant="secondary" onclick={() => saveEntry(entry)}
									>Simpan</Button
								>
								<Button variant="ghost" onclick={() => delete editing[entry.id]}
									>Batal</Button
								>
							</div>
						{:else}
							<p class="entry-body">{entry.body}</p>
							<p class="entry-meta muted">
								{new Date(entry.updatedAt).toLocaleString('id-ID')}
								{#if entry.stopId}
									·
									<a href={resolve('/stations/[id]', { id: entry.stopId })}
										>lihat stasiun</a
									>
								{/if}
								·
								<button
									type="button"
									class="entry-edit"
									onclick={() => (editing[entry.id] = entry.body)}>ubah</button
								>
								·
								<button
									type="button"
									class="entry-edit entry-edit--danger"
									onclick={() => deleteEntry(entry.id)}>hapus</button
								>
							</p>
						{/if}
					</li>
				{/each}
			</ul>
		{/if}
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
	.entry {
		border-block-end: 1px solid var(--sg-border);
		padding-block-end: var(--sg-space-3);
	}
	.entry-body {
		margin-block: 0 var(--sg-space-1);
		white-space: pre-wrap;
	}
	.entry-meta {
		margin: 0;
	}
	.entry-edit {
		border: 0;
		padding: 0;
		background: none;
		color: var(--sg-brand);
		font: inherit;
		cursor: pointer;
	}
	.entry-edit--danger {
		color: var(--sg-danger);
	}
	.entry textarea {
		width: 100%;
		max-width: 32rem;
		padding: var(--sg-space-2) var(--sg-space-3);
		border: 1px solid var(--sg-border);
		border-radius: var(--sg-radius-button);
		font: inherit;
		background: var(--sg-surface);
		color: var(--sg-text);
		resize: vertical;
	}
	.entry-actions {
		display: flex;
		gap: var(--sg-space-2);
		margin-block-start: var(--sg-space-2);
	}
</style>
