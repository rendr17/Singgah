<script lang="ts">
	import type { Snippet } from 'svelte';
	import type { HTMLButtonAttributes } from 'svelte/elements';

	interface Props extends HTMLButtonAttributes {
		/** Icon-only buttons have no visible text, so the label is the
		    accessible name — it is required, not optional. */
		label: string;
		variant?: 'ghost' | 'surface';
		children: Snippet;
	}

	let {
		label,
		variant = 'ghost',
		type = 'button',
		class: className,
		children,
		...rest
	}: Props = $props();
</script>

<button
	{type}
	aria-label={label}
	class={['sg-icon-button', `sg-icon-button--${variant}`, className]}
	{...rest}
>
	{@render children()}
</button>

<style>
	.sg-icon-button {
		display: inline-grid;
		place-items: center;
		inline-size: var(--sg-target-min);
		block-size: var(--sg-target-min);
		border: 1px solid transparent;
		border-radius: var(--sg-radius-button);
		color: var(--sg-text);
		cursor: pointer;
		transition:
			background-color var(--sg-motion-fast) var(--sg-ease-standard),
			scale var(--sg-motion-instant) var(--sg-ease-standard);
	}

	.sg-icon-button:active:not(:disabled) {
		scale: 0.98;
	}

	.sg-icon-button:disabled {
		opacity: 0.55;
		cursor: not-allowed;
	}

	.sg-icon-button--ghost {
		background-color: transparent;
	}
	.sg-icon-button--ghost:hover:not(:disabled) {
		background-color: var(--sg-surface-muted);
	}

	.sg-icon-button--surface {
		background-color: var(--sg-surface);
		border-color: var(--sg-border);
	}
	.sg-icon-button--surface:hover:not(:disabled) {
		background-color: var(--sg-surface-muted);
	}
</style>
