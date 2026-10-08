<script lang="ts">
	import type { PageProps } from './$types';
	import { resolve } from '$app/paths';
	import BackNav from '$lib/components/app-shell/BackNav.svelte';
	import ModeIcon from '$lib/components/map/ModeIcon.svelte';
	import { SearchField, SectionHeading, StateBlock } from '@singgah/ui';
	import {
		badgeTextColor,
		groupRoutes,
		matchesQuery,
		MODE_ICON,
		MODE_LABELS
	} from '$lib/route-groups';

	let { data }: PageProps = $props();

	let query = $state('');
	/** agencyCode of the active chip; '' = semua. */
	let agency = $state('');

	const groups = $derived(groupRoutes(data.routes));
	const visible = $derived(
		groups
			.filter((g) => !agency || g.key === agency)
			.map((g) => ({ ...g, routes: g.routes.filter((r) => matchesQuery(r, query)) }))
			.filter((g) => g.routes.length > 0)
	);
</script>

<svelte:head><title>Rute · Singgah</title></svelte:head>

<BackNav href="/" label="Beranda" />
<h1 class="sg-page-title">Rute</h1>
{#if !data.error && data.routes.length > 0}
	<p class="sg-meta">{data.routes.length} rute · {groups.length} operator</p>
{/if}

{#if data.error}
	<StateBlock kind="error">{data.error}</StateBlock>
{:else if data.routes.length === 0}
	<StateBlock kind="empty">Belum ada rute yang tercatat.</StateBlock>
{:else}
	<div class="controls">
		<SearchField label="Cari rute" placeholder="Nomor atau nama rute" bind:value={query} />
		<div class="chips" role="group" aria-label="Saring per operator">
			<button type="button" class="chip" aria-pressed={agency === ''} onclick={() => (agency = '')}>
				Semua
			</button>
			{#each groups as group (group.key)}
				<button
					type="button"
					class="chip"
					aria-pressed={agency === group.key}
					onclick={() => (agency = group.key)}
				>
					{group.name}
				</button>
			{/each}
		</div>
	</div>

	{#if visible.length === 0}
		<StateBlock kind="empty">Tidak ada rute yang cocok.</StateBlock>
	{:else}
		{#each visible as group (group.key)}
			{@const icon = MODE_ICON[group.mode]}
			<section aria-label={group.name}>
				<SectionHeading>
					<span class="group-title">
						{#if icon}
							<ModeIcon mode={icon} size={18} />
						{/if}
						{group.name}
					</span>
				</SectionHeading>
				<p class="sg-meta group-meta">
					{MODE_LABELS[group.mode]} · {group.routes.length} rute
				</p>
				<ul class="sg-list">
					{#each group.routes as route (route.id)}
						<li>
							<a href={resolve('/routes/[id]', { id: route.id })}>
								{#if route.shortName}
									<span
										class="badge"
										style:background-color={route.color ? '#' + route.color : undefined}
										style:color={route.color ? badgeTextColor(route.color) : undefined}
									>
										{route.shortName}
									</span>
								{:else if route.color}
									<span class="sg-dot" style:background-color={'#' + route.color}></span>
								{/if}
								<span class="name">{route.longName || route.shortName}</span>
							</a>
						</li>
					{/each}
				</ul>
			</section>
		{/each}
	{/if}
{/if}

<style>
	.controls {
		display: grid;
		gap: var(--sg-space-3);
		max-width: 40rem;
		margin-block-start: var(--sg-space-3);
	}

	.chips {
		display: flex;
		flex-wrap: wrap;
		gap: var(--sg-space-2);
	}

	.chip {
		min-height: var(--sg-target-min);
		padding: 0 var(--sg-space-4);
		border: 1px solid var(--sg-border);
		border-radius: var(--sg-radius-pill);
		background-color: var(--sg-surface);
		color: var(--sg-text);
		font: inherit;
		font-size: var(--sg-text-secondary);
		cursor: pointer;
	}

	.chip:hover {
		border-color: var(--sg-brand);
	}

	.chip[aria-pressed='true'] {
		border-color: var(--sg-brand);
		background-color: var(--sg-brand);
		color: var(--sg-brand-contrast);
		font-weight: var(--sg-weight-semibold);
	}

	.group-title {
		display: inline-flex;
		align-items: center;
		gap: var(--sg-space-2);
	}

	.group-meta {
		margin-block: var(--sg-space-1) var(--sg-space-2);
	}

	/* Line-code lozenge — the badge a rider looks for on signage, using the
	   operator's published color. */
	.badge {
		flex: none;
		min-width: 2.5rem;
		padding: var(--sg-space-1) var(--sg-space-2);
		border-radius: var(--sg-radius-button);
		background-color: var(--sg-line-default);
		color: var(--sg-brand-contrast);
		font-size: var(--sg-text-secondary);
		font-weight: var(--sg-weight-bold);
		font-variant-numeric: tabular-nums;
		text-align: center;
	}

	.name {
		font-weight: var(--sg-weight-medium);
	}
</style>
