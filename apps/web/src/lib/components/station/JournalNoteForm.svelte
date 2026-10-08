<script lang="ts">
	import { authedApi, clearSession } from '$lib/session';
	import { Button } from '@singgah/ui';

	let { stopId }: { stopId: string } = $props();

	type Phase = 'idle' | 'saving' | 'saved' | 'error';
	let phase = $state<Phase>('idle');
	let body = $state('');

	async function save() {
		const text = body.trim();
		if (!text || phase === 'saving') return;
		phase = 'saving';
		const { error, response } = await authedApi.POST('/api/v1/journal', {
			body: { stopId, body: text }
		});
		if (error) {
			if (response?.status === 401) clearSession();
			phase = 'error';
			return;
		}
		body = '';
		phase = 'saved';
	}
</script>

<div class="journal-form">
	<label class="sr" for="journal-body">Catatan</label>
	<textarea
		id="journal-body"
		bind:value={body}
		rows="3"
		maxlength="2000"
		placeholder="Catatan tentang stasiun ini…"
		disabled={phase === 'saving'}></textarea>
	<div class="journal-actions">
		<Button
			variant="secondary"
			disabled={!body.trim() || phase === 'saving' || phase === 'saved'}
			onclick={save}
		>
			{phase === 'saving' ? 'Menyimpan…' : 'Simpan catatan'}
		</Button>
		{#if phase === 'saved'}
			<span class="note-ok" role="status">Tersimpan.</span>
		{:else if phase === 'error'}
			<span class="note-err" role="alert">Gagal menyimpan — coba lagi.</span>
		{/if}
	</div>
	<p class="note-meta">Catatan privat — hanya untuk akun perangkat ini.</p>
</div>

<style>
	.journal-form {
		display: flex;
		flex-direction: column;
		gap: var(--sg-space-2);
	}
	.sr {
		position: absolute;
		width: 1px;
		height: 1px;
		overflow: hidden;
		clip: rect(0 0 0 0);
	}
	textarea {
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
	.journal-actions {
		display: flex;
		align-items: center;
		gap: var(--sg-space-3);
	}
	.note-ok {
		color: var(--sg-text-muted);
		font-size: var(--sg-text-secondary);
	}
	.note-err {
		color: var(--sg-danger);
		font-size: var(--sg-text-secondary);
	}
	.note-meta {
		margin: 0;
		color: var(--sg-text-muted);
		font-size: var(--sg-text-secondary);
	}
</style>
