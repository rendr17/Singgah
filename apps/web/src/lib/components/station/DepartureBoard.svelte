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

	// The feed reports seconds; riders read minutes. Zero is a real report —
	// "tepat waktu" is feed truth, not schedule assumption.
	function delayLabel(sec: number): string {
		const m = Math.round(sec / 60);
		if (m > 0) return `+${m} mnt`;
		if (m < 0) return `−${-m} mnt`;
		return 'tepat waktu';
	}

	function depText(d: {
		time: string;
		estimated?: boolean;
		delaySec?: number;
		canceled?: boolean;
	}): string {
		let s = fmtTime(d.time) + (d.estimated ? '≈' : '');
		if (d.canceled) return s + ' batal';
		if (d.delaySec != null && d.delaySec !== 0) s += ` ${delayLabel(d.delaySec)}`;
		return s;
	}
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
							<strong class="sg-tabular" class:is-canceled={dep.canceled}
								>{fmtTime(dep.time)}{#if dep.estimated}<span
										class="est"
										title="Estimasi — layanan headway, bukan jadwal pasti">≈</span
									>{/if}</strong
							>
							{#if dep.canceled}
								<span class="dep-flag dep-flag--cancel">dibatalkan</span>
							{:else}
								{@const label = countdownLabel(minutesUntil(dep.time, now))}
								{#if label}<span class="countdown">{label}</span>{/if}
								{#if dep.delaySec != null}
									<span
										class="dep-flag"
										class:dep-flag--delay={dep.delaySec > 0}
										class:dep-flag--ontime={dep.delaySec === 0}>{delayLabel(dep.delaySec)}</span
									>
								{/if}
							{/if}
						{:else}
							<span class="sg-meta">—</span>
						{/if}
						{#if dir.previousDeparture}
							<span class="prev sg-meta">lalu {depText(dir.previousDeparture)}</span>
						{/if}
					</span>
					{#if !compact && dir.departures.length > 1}
						<span class="later sg-meta sg-tabular">
							{dir.departures.slice(1).map(depText).join(' · ')}
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
	/* Trip-updates annotations: text + color, never color alone (docs/09). */
	.dep-flag {
		font-size: var(--sg-text-secondary);
		font-weight: var(--sg-weight-semibold);
	}
	.dep-flag--delay {
		color: var(--sg-warning);
	}
	.dep-flag--ontime {
		color: var(--sg-brand);
	}
	.dep-flag--cancel {
		color: var(--sg-danger);
	}
	.is-canceled {
		text-decoration: line-through;
		opacity: 0.55;
	}
	.later {
		grid-column: 1 / -1;
		line-height: var(--sg-leading-normal);
	}
</style>
