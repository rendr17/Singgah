<script lang="ts">
	import type { Snippet } from 'svelte';

	// One state block for the four honest states (docs/08 truthful data):
	// kind decides the ARIA role so call sites cannot mislabel an error as a
	// passive note or a loading shimmer as an alert.
	interface Props {
		kind?: 'loading' | 'empty' | 'error' | 'offline';
		children: Snippet;
		class?: string;
	}

	let { kind = 'empty', children, class: className }: Props = $props();

	const role = $derived(
		kind === 'error' ? 'alert' : kind === 'loading' || kind === 'offline' ? 'status' : undefined
	);
</script>

<p class={['sg-state', `sg-state--${kind}`, className]} {role}>
	{@render children()}
</p>

<style>
	.sg-state {
		margin: var(--sg-space-3) 0;
		font-size: var(--sg-text-secondary);
		color: var(--sg-text-muted);
	}

	.sg-state--error {
		color: var(--sg-danger);
	}

	.sg-state--offline {
		color: var(--sg-warning);
	}
</style>
