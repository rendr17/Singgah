<script lang="ts">
	import type { components } from '@singgah/api-client';
	import { countdownLabel, fmtTime, minutesUntil } from '$lib/departures';

	type Line = components['schemas']['DepartureLine'];

	interface Props {
		lines: Line[];
		/** Sheet context shows only the next departure per direction; the full
		    board (station page) lists every remaining time in the window. */
		compact?: boolean;
		/** Countdown anchor — defaults to when the board was rendered. */
		now?: Date;
	}

	let { lines, compact = false, now = new Date() }: Props = $props();
</script>

{#each lines as line (line.lineCode)}
	<section class="line">
		<h3 class="line-name">
			{#if line.route?.color}
				<span class="sg-dot" style:background-color={'#' + line.route.color}></span>
			{/if}
			{line.route ? line.route.longName || line.route.shortName : `Lin ${line.lineCode}`}
		</h3>
		<ul class="dirs">
			{#each line.directions as dir (dir.boundFor)}
				<li class="dir">
					<span class="dir-name">arah {dir.boundFor}</span>
					<span class="dir-next">
						{#if dir.departures.length > 0}
							{@const dep = dir.departures[0]}
							<strong class="sg-tabular">{fmtTime(dep.time)}</strong>
							{@const label = countdownLabel(minutesUntil(dep.time, now))}
							{#if label}<span class="countdown">{label}</span>{/if}
						{:else}
							<span class="sg-meta">—</span>
						{/if}
						{#if dir.previousDeparture}
							<span class="prev sg-meta">lalu {fmtTime(dir.previousDeparture.time)}</span>
						{/if}
					</span>
					{#if !compact && dir.departures.length > 1}
						<span class="later sg-meta sg-tabular">
							{dir.departures
								.slice(1)
								.map((d) => fmtTime(d.time))
								.join(' · ')}
						</span>
					{/if}
				</li>
			{/each}
		</ul>
	</section>
{/each}

<style>
	.line + .line {
		margin-top: var(--sg-space-4);
	}
	.line-name {
		display: flex;
		align-items: center;
		gap: var(--sg-space-2);
		margin: 0 0 var(--sg-space-1);
		font-size: var(--sg-text-body);
	}
	.dirs {
		list-style: none;
		margin: 0;
		padding: 0;
	}
	.dir {
		display: grid;
		grid-template-columns: 1fr auto;
		align-items: baseline;
		gap: 0 var(--sg-space-3);
		padding-block: var(--sg-space-2);
		border-top: 1px solid var(--sg-border);
	}
	.dir-name {
		font-size: var(--sg-text-secondary);
	}
	.dir-next {
		display: flex;
		align-items: baseline;
		gap: var(--sg-space-2);
	}
	.dir-next strong {
		font-size: var(--sg-text-section);
	}
	.countdown {
		color: var(--sg-brand);
		font-size: var(--sg-text-secondary);
		font-weight: var(--sg-weight-semibold);
	}
	.later {
		grid-column: 1 / -1;
		line-height: var(--sg-leading-normal);
	}
</style>
