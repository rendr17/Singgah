<script lang="ts">
	import { dev } from '$app/environment';
	import UiPreview from '$lib/dev/UiPreview.svelte';
</script>

<h1>Singgah</h1>
<p>Pergi boleh spontan. Rute jangan.</p>

{#if dev}
	<UiPreview />
{/if}
