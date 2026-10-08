<script lang="ts">
	import { mapStore } from '$lib/stores/map.svelte';
</script>

<div class="sg-mode-switch" role="group" aria-label="Mode peta">
	<button
		aria-pressed={mapStore.mode === 'geographic'}
		onclick={() => mapStore.setMode('geographic')}
	>
		Peta
	</button>
	<button
		aria-pressed={mapStore.mode === 'integration'}
		onclick={() => mapStore.setMode('integration')}
	>
		Integrasi
	</button>
</div>

<style>
	/* Segmented overlay — same visual family as the lines toggle on /map. */
	.sg-mode-switch {
		position: absolute;
		/* Keep the switch above the floating bottom navigation. */
		bottom: calc(var(--nav-h) + var(--sg-space-6) + env(safe-area-inset-bottom, 0px));
		left: 50%;
		translate: -50% 0;
		display: flex;
		gap: var(--sg-space-1);
		padding: var(--sg-space-1);
		background-color: var(--sg-surface);
		border: 1px solid var(--sg-border);
		border-radius: var(--sg-radius-button);
		box-shadow: var(--sg-shadow-overlay);
		z-index: 1;
	}
	.sg-mode-switch button {
		min-height: var(--sg-target-min);
		padding: 0 var(--sg-space-4);
		border: 0;
		border-radius: calc(var(--sg-radius-button) - var(--sg-space-1));
		background-color: transparent;
		color: var(--sg-text-muted);
		font-family: var(--sg-font-body);
		font-size: var(--sg-text-secondary);
		font-weight: var(--sg-weight-bold);
		cursor: pointer;
	}
	.sg-mode-switch button[aria-pressed='true'] {
		background-color: var(--sg-brand);
		color: var(--sg-brand-contrast);
	}
</style>
