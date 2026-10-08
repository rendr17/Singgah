<script lang="ts">
	import { onMount } from 'svelte';
	import {
		discardFailed,
		idbMutationStore,
		listFailed,
		replayQueue,
		retryFailed,
		sendCheckin,
		type QueuedMutation
	} from '$lib/mutation-queue';
	import { Button } from '@singgah/ui';

	let failed = $state<QueuedMutation[] | null>(null);

	async function refresh() {
		failed = await listFailed(idbMutationStore());
	}

	async function retry(id: string) {
		const store = idbMutationStore();
		await retryFailed(store, id);
		// Try to deliver right away — a success removes the item from the list.
		await replayQueue(store, sendCheckin);
		await refresh();
	}

	async function discard(id: string) {
		await discardFailed(idbMutationStore(), id);
		await refresh();
	}

	onMount(refresh);
</script>

{#if failed && failed.length > 0}
	<div class="failed-queue">
		<p class="fq-note">
			{failed.length} kunjungan gagal terkirim — server menolak. Kirim ulang atau buang.
		</p>
		<ul>
			{#each failed as m (m.mutationId)}
				<li>
					<span class="fq-when">
						{new Date(m.body.observedAt).toLocaleString('id-ID')}
					</span>
					<span class="fq-stop muted">{m.body.stopId.slice(0, 8)}…</span>
					<Button variant="secondary" onclick={() => retry(m.mutationId)}>Kirim ulang</Button>
					<Button variant="ghost" onclick={() => discard(m.mutationId)}>Buang</Button>
				</li>
			{/each}
		</ul>
	</div>
{/if}

<style>
	.failed-queue {
		border: 1px solid var(--sg-border);
		border-radius: var(--sg-radius-card);
		padding: var(--sg-space-3);
	}
	.fq-note {
		margin-block: 0 var(--sg-space-2);
		font-size: var(--sg-text-secondary);
	}
	ul {
		list-style: none;
		margin: 0;
		padding: 0;
		display: flex;
		flex-direction: column;
		gap: var(--sg-space-2);
	}
	li {
		display: flex;
		align-items: center;
		gap: var(--sg-space-3);
		flex-wrap: wrap;
	}
	.fq-when {
		font-variant-numeric: tabular-nums;
	}
	.muted {
		color: var(--sg-text-muted);
		font-size: var(--sg-text-secondary);
		font-family: var(--sg-font-mono);
	}
</style>
