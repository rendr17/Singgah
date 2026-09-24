<script lang="ts">
	import { resolve } from '$app/paths';
	import type { Pathname } from '$app/types';

	interface Props {
		href: Pathname;
		label: string;
	}

	let { href, label }: Props = $props();
</script>

<nav class="back-nav" aria-label="Navigasi balik">
	<a href={resolve(href)}>← {label}</a>
</nav>

<style>
	.back-nav {
		margin-bottom: var(--sg-space-4);
		font-size: var(--sg-font-sm);
	}
</style>
