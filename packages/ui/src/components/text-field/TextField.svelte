<script lang="ts">
	import type { HTMLInputAttributes } from 'svelte/elements';

	interface Props extends HTMLInputAttributes {
		label: string;
		hint?: string;
		error?: string;
	}

	let { label, hint, error, id, class: className, value = $bindable(), ...rest }: Props = $props();

	const uid = $props.id();
	const fieldId = $derived(id ?? `sg-field-${uid}`);
	const hintId = $derived(`${fieldId}-hint`);
	const errorId = $derived(`${fieldId}-error`);
	const describedBy = $derived(error ? errorId : hint ? hintId : undefined);
</script>

<div class={['sg-field', className]}>
	<label class="sg-field__label" for={fieldId}>{label}</label>
	<input
		id={fieldId}
		class="sg-field__input"
		aria-invalid={error ? true : undefined}
		aria-describedby={describedBy}
		bind:value
		{...rest}
	/>
	{#if error}
		<p class="sg-field__error" id={errorId}>{error}</p>
	{:else if hint}
		<p class="sg-field__hint" id={hintId}>{hint}</p>
	{/if}
</div>

<style>
	.sg-field {
		display: grid;
		gap: var(--sg-space-1);
	}

	.sg-field__label {
		font-size: var(--sg-text-secondary);
		font-weight: var(--sg-weight-semibold);
		color: var(--sg-text);
	}

	.sg-field__input {
		min-height: var(--sg-target-min);
		padding: 0 var(--sg-space-3);
		border: 1px solid var(--sg-border);
		border-radius: var(--sg-radius-input);
		background-color: var(--sg-surface);
		color: var(--sg-text);
		font: inherit;
		transition:
			border-color var(--sg-motion-fast) var(--sg-ease-standard),
			box-shadow var(--sg-motion-fast) var(--sg-ease-standard);
	}

	.sg-field__input:focus-visible {
		border-color: var(--sg-brand);
		box-shadow: 0 0 0 3px var(--sg-brand-subtle);
	}

	.sg-field__input::placeholder {
		color: var(--sg-text-muted);
	}

	.sg-field__input[aria-invalid='true'] {
		border-color: var(--sg-danger);
	}

	.sg-field__hint,
	.sg-field__error {
		margin: 0;
		font-size: var(--sg-text-meta);
	}

	.sg-field__hint {
		color: var(--sg-text-muted);
	}

	.sg-field__error {
		color: var(--sg-danger);
	}
</style>
