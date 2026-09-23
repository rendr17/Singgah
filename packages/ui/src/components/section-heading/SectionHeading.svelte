<script lang="ts">
	import type { Snippet } from 'svelte';

	interface Props {
		level?: 1 | 2 | 3;
		children: Snippet;
		/** Optional trailing action (link/button) rendered beside the title. */
		action?: Snippet;
		class?: string;
	}

	let { level = 2, children, action, class: className }: Props = $props();
</script>

<header class={['sg-section-heading', className]}>
	<svelte:element this={`h${level}`} class={`sg-section-heading__title--h${level}`}>
		{@render children()}
	</svelte:element>
	{#if action}
		<div class="sg-section-heading__action">{@render action()}</div>
	{/if}
</header>

<style>
	.sg-section-heading {
		display: flex;
		align-items: baseline;
		justify-content: space-between;
		gap: var(--sg-space-3);
	}

	.sg-section-heading :global(h1),
	.sg-section-heading :global(h2),
	.sg-section-heading :global(h3) {
		font-family: var(--sg-font-display);
		font-weight: var(--sg-weight-semibold);
		line-height: var(--sg-leading-tight);
		color: var(--sg-text);
		margin: 0;
	}

	.sg-section-heading__title--h1 {
		font-size: var(--sg-text-screen);
	}
	.sg-section-heading__title--h2 {
		font-size: var(--sg-text-section);
	}
	.sg-section-heading__title--h3 {
		font-size: var(--sg-text-row);
	}
</style>
