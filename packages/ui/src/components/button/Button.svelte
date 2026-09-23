<script lang="ts">
	import type { Snippet } from 'svelte';
	import type { HTMLButtonAttributes } from 'svelte/elements';

	interface Props extends HTMLButtonAttributes {
		variant?: 'primary' | 'secondary' | 'ghost';
		children: Snippet;
	}

	let {
		variant = 'primary',
		type = 'button',
		class: className,
		children,
		...rest
	}: Props = $props();
</script>

<button {type} class={['sg-button', `sg-button--${variant}`, className]} {...rest}>
	{@render children()}
</button>

<style>
	.sg-button {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		gap: var(--sg-space-2);
		min-height: var(--sg-target-min);
		padding: 0 var(--sg-space-5);
		border: 1px solid transparent;
		border-radius: var(--sg-radius-button);
		font-family: var(--sg-font-body);
		font-size: var(--sg-text-body);
		font-weight: var(--sg-weight-bold);
		cursor: pointer;
		transition:
			background-color var(--sg-motion-fast) var(--sg-ease-standard),
			scale var(--sg-motion-instant) var(--sg-ease-standard);
	}

	.sg-button:active:not(:disabled) {
		scale: 0.98;
	}

	.sg-button:disabled {
		opacity: 0.55;
		cursor: not-allowed;
	}

	.sg-button--primary {
		background-color: var(--sg-brand);
		color: var(--sg-brand-contrast);
	}
	.sg-button--primary:hover:not(:disabled) {
		background-color: var(--sg-brand-strong);
	}

	.sg-button--secondary {
		background-color: var(--sg-surface);
		border-color: var(--sg-border);
		color: var(--sg-text);
	}
	.sg-button--secondary:hover:not(:disabled) {
		background-color: var(--sg-surface-muted);
	}

	.sg-button--ghost {
		background-color: transparent;
		color: var(--sg-brand);
		padding-inline: var(--sg-space-3);
	}
	.sg-button--ghost:hover:not(:disabled) {
		background-color: var(--sg-surface-muted);
	}
</style>
