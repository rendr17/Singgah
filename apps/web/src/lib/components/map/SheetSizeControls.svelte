<script lang="ts">
	export type MapSheetSize = 'peek' | 'medium' | 'expanded';

	interface Props {
		size: MapSheetSize;
		onChange: (size: MapSheetSize) => void;
	}

	let { size, onChange }: Props = $props();
	const sizes: MapSheetSize[] = ['peek', 'medium', 'expanded'];

	function adjust(delta: -1 | 1) {
		const next = sizes[sizes.indexOf(size) + delta];
		if (next) onChange(next);
	}
</script>

<div class="sheet-size-controls" role="group" aria-label="Ukuran panel">
	<button
		type="button"
		aria-label="Perkecil panel"
		title="Perkecil panel"
		disabled={size === 'peek'}
		onclick={() => adjust(-1)}
	>
		<svg
			viewBox="0 0 24 24"
			width="18"
			height="18"
			fill="none"
			stroke="currentColor"
			stroke-width="2"
			aria-hidden="true"
		>
			<path d="m6 9 6 6 6-6" />
		</svg>
	</button>
	<button
		type="button"
		aria-label="Perbesar panel"
		title="Perbesar panel"
		disabled={size === 'expanded'}
		onclick={() => adjust(1)}
	>
		<svg
			viewBox="0 0 24 24"
			width="18"
			height="18"
			fill="none"
			stroke="currentColor"
			stroke-width="2"
			aria-hidden="true"
		>
			<path d="m6 15 6-6 6 6" />
		</svg>
	</button>
</div>

<style>
	.sheet-size-controls {
		display: none;
		flex: none;
		gap: var(--sg-space-1);
	}
	.sheet-size-controls button {
		display: grid;
		place-items: center;
		width: var(--sg-target-min);
		height: var(--sg-target-min);
		padding: 0;
		border: 1px solid var(--sg-border);
		border-radius: var(--sg-radius-button);
		background: var(--sg-surface);
		color: var(--sg-text);
		cursor: pointer;
	}
	.sheet-size-controls button:disabled {
		color: var(--sg-text-muted);
		opacity: 0.5;
		cursor: default;
	}
	@media (max-width: 47.999rem) {
		.sheet-size-controls {
			display: flex;
		}
	}
</style>
