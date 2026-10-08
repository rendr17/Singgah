<script lang="ts">
	import type { PageProps } from './$types';
	import { resolve } from '$app/paths';
	import BackNav from '$lib/components/app-shell/BackNav.svelte';
	import ModeIcon from '$lib/components/map/ModeIcon.svelte';
	import { SectionHeading, StateBlock } from '@singgah/ui';
	import { badgeTextColor, MODE_ICON, MODE_LABELS } from '$lib/route-groups';

	let { data }: PageProps = $props();
	const route = $derived(data.route);
	const lineColor = $derived(route?.color ? '#' + route.color : 'var(--sg-line-default)');
	const icon = $derived(route ? MODE_ICON[route.mode] : undefined);
	// Provider order can fold multiple topology segments (TRUNK/BRANCH/…)
	// into one list — a loop corridor like TJ:7F then repeats stops. Group
	// consecutive runs by segmentKind so each piece reads as its own strip.
	const stopGroups = $derived.by(() => {
		const out: { kind: string; stops: NonNullable<typeof route>['stops'] }[] = [];
		for (const s of route?.stops ?? []) {
			const kind = s.segmentKind ?? '';
			const last = out[out.length - 1];
			if (last && last.kind === kind) last.stops.push(s);
			else out.push({ kind, stops: [s] });
		}
		return out;
	});
</script>

<svelte:head
	><title>{route ? `${route.shortName || route.longName} · Singgah` : 'Rute · Singgah'}</title
	></svelte:head
>

{#if route === null}
	<StateBlock kind="error">{data.error}</StateBlock>
	<BackNav href="/routes" label="Semua rute" />
{:else}
	<BackNav href="/routes" label="Semua rute" />

	<h1 class="sg-page-title head">
		{#if route.shortName}
			<span
				class="badge"
				style:background-color={route.color ? '#' + route.color : undefined}
				style:color={route.color ? badgeTextColor(route.color) : undefined}>{route.shortName}</span
			>
		{:else if route.color}
			<span class="sg-dot" style:background-color={'#' + route.color}></span>
		{/if}
		{route.longName || route.shortName}
	</h1>
	<p class="sg-meta meta-line">
		{#if icon}<ModeIcon mode={icon} size={14} />{/if}
		{MODE_LABELS[route.mode]}{#if route.agencyName}
			· {route.agencyName}{/if}
		· {route.stops.length} perhentian
	</p>

	<section>
		<SectionHeading>Stasiun yang dilayani</SectionHeading>
		{#if route.stops.length === 0}
			<StateBlock kind="empty">Belum ada stasiun tercatat untuk rute ini.</StateBlock>
		{:else}
			{#each stopGroups as group (group.stops[0]?.seq)}
				{#if stopGroups.length > 1 && group.kind}
					<p class="seg-label">{group.kind}</p>
				{/if}
				<ol class="stops" style:--line={lineColor}>
					{#each group.stops as stop (stop.seq)}
						<li>
							<a href={resolve('/stations/[id]', { id: stop.id })}>
								<span class="rail" aria-hidden="true"></span>
								{#if stop.stationNumber}<span class="num muted">{stop.stationNumber}</span>{/if}
								<span class="stop-name">{stop.name}</span>
								{#if stop.code}<span class="muted">· {stop.code}</span>{/if}
							</a>
						</li>
					{/each}
				</ol>
			{/each}
		{/if}
	</section>

	<p class="sg-meta source">
		Sumber: {route.source.provider}
		{#if route.source.fetchedAt}
			· diambil {new Date(route.source.fetchedAt).toLocaleString('id-ID')}
		{/if}
	</p>
{/if}

<style>
	.head {
		display: flex;
		align-items: center;
		gap: var(--sg-space-3);
	}

	/* Corridor lozenge — same badge the /routes index uses, scaled to title size. */
	.head .badge {
		flex: none;
		min-width: 3rem;
		padding: var(--sg-space-1) var(--sg-space-3);
		border-radius: var(--sg-radius-button);
		background-color: var(--sg-line-default);
		color: var(--sg-brand-contrast);
		font-size: var(--sg-text-body);
		font-weight: var(--sg-weight-bold);
		font-variant-numeric: tabular-nums;
		text-align: center;
	}

	.meta-line {
		display: flex;
		align-items: center;
		gap: var(--sg-space-2);
	}

	/* Corridor strip: the served stops read as the line's own diagram — rail
	   in the corridor color, a dot per stop, filled termini. */
	.stops {
		list-style: none;
		margin: 0;
		padding: 0;
		max-width: 40rem;
	}

	.stops > li {
		display: flex;
	}

	.stops a {
		display: flex;
		flex: 1;
		align-items: center;
		gap: var(--sg-space-3);
		min-height: var(--sg-target-min);
		padding-inline: var(--sg-space-2);
		border-radius: var(--sg-radius-button);
		color: var(--sg-text);
		text-decoration: none;
	}

	.stops a:hover {
		background-color: var(--sg-surface-muted);
	}

	.rail {
		position: relative;
		align-self: stretch;
		flex: none;
		width: 1.25rem;
	}

	.rail::before {
		content: '';
		position: absolute;
		inset-block: 0;
		inset-inline-start: 50%;
		width: 2px;
		translate: -50% 0;
		background-color: var(--line);
	}

	.stops > li:first-child .rail::before {
		inset-block-start: 50%;
	}

	.stops > li:last-child .rail::before {
		inset-block-end: 50%;
	}

	.rail::after {
		content: '';
		position: absolute;
		inset-block-start: 50%;
		inset-inline-start: 50%;
		translate: -50% -50%;
		box-sizing: border-box;
		width: 0.625rem;
		height: 0.625rem;
		border: 2px solid var(--line);
		border-radius: 50%;
		background-color: var(--sg-canvas);
	}

	.stops > li:first-child .rail::after,
	.stops > li:last-child .rail::after {
		width: 0.75rem;
		height: 0.75rem;
		background-color: var(--line);
	}

	.stops > li:first-child .stop-name,
	.stops > li:last-child .stop-name {
		font-weight: var(--sg-weight-medium);
	}

	/* Segment runs get their own label — provider vocabulary kept verbatim
	   (TRUNK/BRANCH/…), not translated. */
	.seg-label {
		margin: var(--sg-space-4) 0 var(--sg-space-2);
		color: var(--sg-text-muted);
		font-size: var(--sg-text-meta);
		font-weight: var(--sg-weight-medium);
		letter-spacing: 0.04em;
	}

	.seg-label:first-child {
		margin-block-start: 0;
	}

	.muted {
		color: var(--sg-text-muted);
		font-size: var(--sg-text-secondary);
	}
	.num {
		display: inline-block;
		min-width: 2.25rem;
		font-variant-numeric: tabular-nums;
	}
</style>
