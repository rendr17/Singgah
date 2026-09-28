<script lang="ts">
	import type { PageProps } from './$types';
	import { SectionHeading, StateBlock } from '@singgah/ui';

	let { data }: PageProps = $props();
</script>

<svelte:head><title>Jelajah · Singgah</title></svelte:head>

<h1 class="sg-page-title">Jelajah</h1>
<p class="sg-meta">Tempat dan jalur jalan kaki di sekitar transit.</p>

<section>
	<SectionHeading>Koleksi stasiun</SectionHeading>
	{#if data.error}
		<StateBlock kind="error">{data.error}</StateBlock>
	{:else if data.collections.length === 0}
		<StateBlock kind="empty">Belum ada koleksi.</StateBlock>
	{:else}
		<ul class="sg-list">
			{#each data.collections as c (c.id)}
				<li>
					<div class="coll-row">
						<span class="coll-title">{c.title}</span>
						<span class="muted">{c.itemCount} stasiun</span>
					</div>
					{#if c.description}<p class="muted desc">{c.description}</p>{/if}
				</li>
			{/each}
		</ul>
		<p class="sg-meta">
			Set kurasi editorial — progresmu per koleksi terlihat di Paspor.
		</p>
	{/if}
</section>

<style>
	.coll-row {
		display: flex;
		justify-content: space-between;
		align-items: baseline;
		gap: var(--sg-space-2);
	}
	.coll-title {
		font-weight: var(--sg-weight-semibold);
	}
	.muted {
		color: var(--sg-text-muted);
		font-size: var(--sg-text-secondary);
	}
	.desc {
		margin-block: 0.125rem 0;
	}
</style>
