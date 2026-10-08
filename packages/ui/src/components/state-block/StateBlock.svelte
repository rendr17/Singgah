<script lang="ts">
	import type { Snippet } from 'svelte';

	// One state block for the four honest states (docs/08 truthful data):
	// kind decides the ARIA role so call sites cannot mislabel an error as a
	// passive note or a loading shimmer as an alert.
	interface Props {
		kind?: 'loading' | 'empty' | 'error' | 'offline';
		/** Brand illustration (static asset path) — decorative, the text
		   still carries the message. Use sparingly (docs/09). */
		illustration?: string;
		children: Snippet;
		class?: string;
	}

	let { kind = 'empty', illustration, children, class: className }: Props = $props();

	const role = $derived(
		kind === 'error' ? 'alert' : kind === 'loading' || kind === 'offline' ? 'status' : undefined
	);
</script>

<div class={['sg-state', `sg-state--${kind}`, className]}>
	{#if illustration}
		<img class="sg-state__art" src={illustration} alt="" loading="lazy" />
	{/if}
	<p class="sg-state__text" {role}>
		{@render children()}
	</p>
</div>

<style>
	.sg-state {
		margin: var(--sg-space-3) 0;
		font-size: var(--sg-text-secondary);
		color: var(--sg-text-muted);
		max-width: 22rem;
		animation: sg-content-enter var(--sg-motion-base) var(--sg-ease-enter) both;
	}

	.sg-state__art {
		display: block;
		width: 100%;
		max-width: 16rem;
		height: auto;
		margin-block-end: var(--sg-space-3);
		border-radius: var(--sg-radius-card);
	}

	.sg-state__text {
		margin: 0;
	}

	.sg-state--error {
		color: var(--sg-danger);
	}

	.sg-state--offline {
		color: var(--sg-warning);
	}
</style>
