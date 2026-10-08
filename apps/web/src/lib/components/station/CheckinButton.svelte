<script lang="ts">
	import { onDestroy } from 'svelte';
	import {
		idbMutationStore,
		onSettled,
		queueCheckin,
		replayQueue,
		sendCheckin,
		type CheckinBody
	} from '$lib/mutation-queue';
	import { Button } from '@singgah/ui';

	let { stopId }: { stopId: string } = $props();

	type Phase = 'idle' | 'locating' | 'sending' | 'queued' | 'done' | 'error';
	let phase = $state<Phase>('idle');
	let visitStatus = $state<'confirmed' | 'low_confidence' | null>(null);
	let mutationId = $state('');
	let unsettle: (() => void) | null = null;

	onDestroy(() => unsettle?.());

	// Location is requested only here — on the explicit tap (docs/16). A denied
	// or unavailable fix does not block the mark; it degrades to manual honestly.
	function locate(): Promise<{ lat: number; lon: number } | null> {
		if (!('geolocation' in navigator)) return Promise.resolve(null);
		return new Promise((resolve) => {
			navigator.geolocation.getCurrentPosition(
				(pos) => resolve({ lat: pos.coords.latitude, lon: pos.coords.longitude }),
				() => resolve(null),
				{ timeout: 8_000, maximumAge: 60_000 }
			);
		});
	}

	// The queue is only acknowledged — never called "accepted". When the
	// background replay resolves this mutationId, flip to the real outcome.
	async function parkOffline(body: CheckinBody) {
		unsettle = onSettled(mutationId, (r) => {
			if (r.kind === 'ok') {
				visitStatus = r.visitStatus ?? null;
				phase = 'done';
			} else if (r.kind === 'rejected') {
				phase = 'error';
			}
			// 'network' during replay: stay queued — still pending, honestly.
		});
		await queueCheckin(idbMutationStore(), body);
		phase = 'queued';
	}

	async function checkin() {
		// Same key on retry → the server replays the recorded visit, never a
		// duplicate (idempotency contract).
		if (!mutationId) mutationId = crypto.randomUUID();
		phase = 'locating';
		const fix = await locate();
		const body: CheckinBody = {
			stopId,
			clientMutationId: mutationId,
			observedAt: new Date().toISOString(),
			...(fix ? { lat: fix.lat, lon: fix.lon } : {})
		};
		if (!navigator.onLine) {
			await parkOffline(body);
			return;
		}
		phase = 'sending';
		const result = await sendCheckin(body);
		if (result.kind === 'ok') {
			visitStatus = result.visitStatus ?? null;
			phase = 'done';
		} else if (result.kind === 'network') {
			await parkOffline(body);
		} else {
			phase = 'error';
		}
	}
</script>

<div class="checkin">
	{#if phase === 'done'}
		<p class="checkin-done" role="status">
			Ditandai{visitStatus === 'confirmed'
				? ' — lokasi terverifikasi.'
				: visitStatus === 'low_confidence'
					? ' — tanpa verifikasi lokasi.'
					: '.'}
		</p>
	{:else if phase === 'queued'}
		<p class="checkin-done" role="status">Tertunda — dikirim saat online.</p>
		<Button variant="ghost" onclick={() => void replayQueue(idbMutationStore(), sendCheckin)}>
			Kirim sekarang
		</Button>
	{:else}
		<Button disabled={phase === 'locating' || phase === 'sending'} onclick={checkin}>
			{#if phase === 'locating'}
				Meminta lokasi…
			{:else if phase === 'sending'}
				Menandai…
			{:else if phase === 'error'}
				Coba lagi
			{:else}
				Tandai dikunjungi
			{/if}
		</Button>
		{#if phase === 'error'}
			<p class="checkin-note checkin-note--err" role="alert">
				Gagal menandai — periksa koneksi lalu coba lagi.
			</p>
		{:else}
			<p class="checkin-note">Lokasi hanya diminta saat kamu menekan tombol ini.</p>
		{/if}
	{/if}
</div>

<style>
	.checkin {
		display: flex;
		flex-direction: column;
		gap: var(--sg-space-1);
		align-items: flex-start;
	}
	.checkin-note {
		margin: 0;
		color: var(--sg-text-muted);
		font-size: var(--sg-text-secondary);
	}
	.checkin-note--err {
		color: var(--sg-danger);
	}
	.checkin-done {
		margin: 0;
		font-weight: var(--sg-weight-semibold);
	}
</style>
