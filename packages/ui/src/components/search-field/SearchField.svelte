<script lang="ts">
	import type { HTMLInputAttributes } from 'svelte/elements';

	interface Props extends Omit<HTMLInputAttributes, 'type'> {
		/** Used as the input's aria-label — keep it a real Indonesian phrase. */
		label?: string;
	}

	let { label = 'Pencarian', value = $bindable(''), class: className, ...rest }: Props = $props();
</script>

<div class={['sg-search', className]} role="search">
	<svg
		class="sg-search__icon"
		viewBox="0 0 24 24"
		fill="none"
		stroke="currentColor"
		stroke-width="2"
		stroke-linecap="round"
		aria-hidden="true"
	>
		<circle cx="11" cy="11" r="7" />
		<path d="m21 21-4.35-4.35" />
	</svg>
	<input type="search" class="sg-search__input" aria-label={label} bind:value {...rest} />
</div>

<style>
	.sg-search {
		display: flex;
		align-items: center;
		gap: var(--sg-space-2);
		min-height: var(--sg-target-min);
		padding-inline: var(--sg-space-3);
		border: 1px solid var(--sg-border);
		border-radius: var(--sg-radius-input);
		background-color: var(--sg-surface);
		color: var(--sg-text-muted);
		transition:
			border-color var(--sg-motion-fast) var(--sg-ease-standard),
			box-shadow var(--sg-motion-fast) var(--sg-ease-standard);
	}

	.sg-search:focus-within {
		border-color: var(--sg-brand);
		box-shadow: 0 0 0 3px var(--sg-brand-subtle);
	}

	.sg-search__icon {
		inline-size: 1.25rem;
		block-size: 1.25rem;
		flex: none;
	}

	.sg-search__input {
		flex: 1;
		min-inline-size: 0;
		border: 0;
		background: none;
		color: var(--sg-text);
		font: inherit;
		outline: none;
	}

	.sg-search__input::placeholder {
		color: var(--sg-text-muted);
	}
</style>
