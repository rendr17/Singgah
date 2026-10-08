<script lang="ts">
	import { STATUS_LABELS, type TransitStatus } from './status';

	interface Props {
		status: TransitStatus;
		/** Optional override; default labels come from STATUS_LABELS. */
		label?: string;
		class?: string;
	}

	let { status, label, class: className }: Props = $props();
</script>

<span class={['sg-badge', `sg-badge--${status}`, className]}>
	<span class="sg-badge__dot" aria-hidden="true"></span>
	{label ?? STATUS_LABELS[status]}
</span>

<style>
	.sg-badge {
		display: inline-flex;
		align-items: center;
		gap: var(--sg-space-1);
		padding: var(--sg-space-1) var(--sg-space-2);
		border: 1px solid currentColor;
		border-radius: var(--sg-radius-pill);
		font-size: var(--sg-text-meta);
		font-weight: var(--sg-weight-semibold);
		line-height: 1.4;
	}

	.sg-badge__dot {
		inline-size: 0.5rem;
		block-size: 0.5rem;
		border-radius: 50%;
		background-color: currentColor;
	}

	.sg-badge--live {
		color: var(--sg-status-live);
		background-color: var(--sg-status-live-bg);
	}
	.sg-badge--estimated {
		color: var(--sg-status-estimated);
		background-color: var(--sg-status-estimated-bg);
	}
	.sg-badge--scheduled {
		color: var(--sg-status-scheduled);
		background-color: var(--sg-status-scheduled-bg);
	}
	.sg-badge--stale {
		color: var(--sg-status-stale);
		background-color: var(--sg-status-stale-bg);
	}
	.sg-badge--disruption {
		color: var(--sg-status-disruption);
		background-color: var(--sg-status-disruption-bg);
	}
</style>
