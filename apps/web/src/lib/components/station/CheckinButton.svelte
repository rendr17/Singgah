<script lang="ts">
	import { authedApi, clearSession } from '$lib/session';
	import { Button } from '@singgah/ui';

	let { stopId }: { stopId: string } = $props();

	type Phase = 'idle' | 'locating' | 'sending' | 'done' | 'error';
	let phase = $state<Phase>('idle');
	let visitStatus = $state<'confirmed' | 'low_confidence' | null>(null);
	let mutationId = $state('');

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

	async function checkin() {
		// Same key on retry → the server replays the recorded visit, never a
		// duplicate (idempotency contract).
		if (!mutationId) mutationId = crypto.randomUUID();
		phase = 'locating';
		const fix = await locate();
		phase = 'sending';
		const { data, error, response } = await authedApi.POST('/api/v1/visits', {
			body: {
				stopId,
				clientMutationId: mutationId,
				observedAt: new Date().toISOString(),
				...(fix ? { lat: fix.lat, lon: fix.lon } : {})
			}
		});
		if (error || !data) {
			// A 401 means the stored token is dead — drop it so the next tap
			// re-mints instead of failing forever.
			if (response.status === 401) clearSession();
			phase = 'error';
			return;
		}
		visitStatus = data.visit.status;
		phase = 'done';
	}
</script>

<div class="checkin">
	{#if phase === 'done' && visitStatus}
		<p class="checkin-done" role="status">
			Ditandai{visitStatus === 'confirmed'
				? ' — lokasi terverifikasi.'
				: ' — tanpa verifikasi lokasi.'}
		</p>
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
