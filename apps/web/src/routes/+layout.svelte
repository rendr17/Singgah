<script lang="ts">
	import './layout.css';
	import favicon from '$lib/assets/favicon.svg';

	let { children } = $props();
</script>

<svelte:head><link rel="icon" href={favicon} /></svelte:head>

<a class="skip-link" href="#content">Langsung ke konten</a>
<main id="content">
	{@render children()}
</main>

<style>
	main {
		margin-inline: auto;
		max-width: 72rem;
		padding: var(--sg-space-4) var(--sg-space-3);
	}

	.skip-link {
		position: absolute;
		left: var(--sg-space-3);
		top: -100%;
		z-index: 10;
		padding: var(--sg-space-2) var(--sg-space-3);
		border-radius: var(--sg-radius-sm);
		background: var(--sg-accent);
		color: #fff;
	}

	.skip-link:focus {
		top: var(--sg-space-2);
	}
</style>
