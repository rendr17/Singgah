<script lang="ts">
	import type { Snippet } from 'svelte';

	interface Props {
		variant?: 'default' | 'muted';
		children: Snippet;
		class?: string;
	}

	let { variant = 'default', children, class: className }: Props = $props();
</script>

<div class={['sg-surface', variant === 'muted' && 'sg-surface--muted', className]}>
	{@render children()}
</div>

<style>
	.sg-surface {
		background-color: var(--sg-surface);
		border: 1px solid var(--sg-border);
		border-radius: var(--sg-radius-card);
		padding: var(--sg-space-4);
	}

	.sg-surface--muted {
		background-color: var(--sg-surface-muted);
		border-color: transparent;
	}
</style>
