<script lang="ts">
	import { resolve } from '$app/paths';

	let { variant = 'network' }: { variant?: 'network' | 'plan' | 'explore' | 'passport' } = $props();
	let selectedPanel = $state<'plan' | 'explore' | 'passport'>('plan');
	const panelHref = (panel: 'plan' | 'explore' | 'passport') =>
		resolve(`/app?panel=${panel}` as `/app?${string}`);
	const nodes = [
		{ name: 'Perjalanan', panel: 'plan', x: 45, y: 13 },
		{ name: 'Stasiun', panel: '', x: 71, y: 24 },
		{ name: 'Jelajah', panel: 'explore', x: 84, y: 54 },
		{ name: 'Paspor', panel: 'passport', x: 66, y: 76 },
		{ name: 'Jurnal', panel: 'passport', x: 45, y: 88 },
		{ name: 'Tempat', panel: 'explore', x: 27, y: 77 },
		{ name: 'Jadwal', panel: '', x: 15, y: 52 },
		{ name: 'Rute', panel: 'plan', x: 24, y: 24 }
	] as const;
	const panels = [
		{ id: 'plan', name: 'Perjalanan' },
		{ id: 'explore', name: 'Jelajah' },
		{ id: 'passport', name: 'Paspor' }
	] as const;
	const stages = [
		{ title: 'Berangkat', items: ['Pilih stasiun awal', 'Atur waktu perjalanan'], icon: 'station' },
		{ title: 'Perjalanan', items: ['Bandingkan rute', 'Lihat perpindahan'], icon: 'transfer' },
		{ title: 'Sampai', items: ['Kenali stasiun tujuan', 'Jelajahi sekitarnya'], icon: 'explore' }
	];
</script>

<div class="preview" class:preview--network={variant === 'network'}>
	<div class="preview__bar">
		<span class="preview__identity"
			><img src="/brand/app-icon-blue.svg" alt="" width="18" height="18" /> Singgah
			<span aria-hidden="true">⌄</span></span
		>
		<span class="preview__caption">PRATINJAU FITUR</span>
	</div>
	{#if variant === 'network'}
		<div class="workspace">
			<div class="network">
				<svg
					class="network__lines"
					viewBox="0 0 100 100"
					preserveAspectRatio="none"
					aria-hidden="true"
				>
					{#each nodes as node (node.name)}<path
							d={`M 48 49 Q ${node.x} 49 ${node.x} ${node.y}`}
						/>{/each}
				</svg>
				{#each nodes as node (node.name)}
					<a
						class="network__node"
						style:left="{node.x}%"
						style:top="{node.y}%"
						href={node.panel ? panelHref(node.panel) : resolve('/app')}>{node.name}</a
					>
				{/each}
				<a class="network__center" href={resolve('/app')}
					><img
						src="/stickers/sticker-side-quest.webp"
						alt=""
						width="64"
						height="64"
						loading="lazy"
					/><span>Singgah</span></a
				>
				<p class="network__note">Satu tempat untuk perjalanan dan cerita kota.</p>
			</div>
			<div class="inspector">
				<div class="inspector__tabs" aria-label="Pilih pratinjau fitur">
					{#each panels as panel (panel.id)}
						<button
							type="button"
							aria-pressed={selectedPanel === panel.id}
							onclick={() => (selectedPanel = panel.id)}>{panel.name}</button
						>
					{/each}
				</div>
				{#key selectedPanel}<div class="inspector__body" aria-live="polite">
						{#if selectedPanel === 'plan'}
							<span class="small-label">RENCANAKAN PERJALANAN</span>
							<h3>Ke mana hari ini?</h3>
							<p>Mulai dari stasiun, lalu pilih jalan yang paling masuk akal.</p>
							<a class="field" href={panelHref('plan')}
								><span aria-hidden="true">○</span> Pilih stasiun awal</a
							>
							<a class="field" href={panelHref('plan')}
								><span aria-hidden="true">◇</span> Pilih stasiun tujuan</a
							>
						{:else if selectedPanel === 'explore'}
							<span class="small-label">CITY EXPLORER</span>
							<h3>Turun. Singgah sebentar.</h3>
							<p>Pilih stasiun untuk menemukan tempat dengan konteks jalan kaki dan transit.</p>
							<div class="category-list">
								<span>Makan &amp; ngopi</span><span>Taman &amp; ruang kota</span><span
									>Seni &amp; budaya</span
								>
							</div>
						{:else}
							<span class="small-label">TRANSIT PASSPORT</span>
							<h3>Cerita yang ikut pulang.</h3>
							<p>Tandai stasiun yang kamu kunjungi dan simpan catatan pribadi.</p>
							<div class="category-list">
								<span>Pilih stasiun</span><span>Tandai kunjungan</span><span>Tulis cerita</span>
							</div>
						{/if}
						<a class="preview__action" href={panelHref(selectedPanel)}
							>Buka {panels.find((panel) => panel.id === selectedPanel)?.name}
							<span aria-hidden="true">↑</span></a
						>
					</div>{/key}
			</div>
		</div>
	{:else if variant === 'plan'}
		<div class="planning-board">
			{#each stages as stage (stage.title)}
				<div class="planning-stage">
					<p>{stage.title}<span aria-hidden="true">···</span></p>
					{#each stage.items as item (item)}
						<a class="planning-ticket" href={panelHref('plan')}
							><svg
								viewBox="0 0 64 64"
								fill="none"
								stroke="currentColor"
								stroke-width="3"
								aria-hidden="true"><use href="/icons/transit.svg#{stage.icon}" /></svg
							><span>{item}<small>Langkah perjalanan</small></span><span aria-hidden="true">↗</span
							></a
						>
					{/each}
				</div>
			{/each}
		</div>
	{:else if variant === 'explore'}
		<div class="editorial-sheet">
			<div class="sheet-heading">
				<span class="small-label">CITY EXPLORER</span><svg
					viewBox="0 0 64 64"
					fill="none"
					stroke="currentColor"
					stroke-width="3"
					aria-hidden="true"><use href="/icons/transit.svg#explore" /></svg
				>
			</div>
			<h3>Jalan dikit.<br />Cerita nambah.</h3>
			<div class="sheet-row"><span>Mulai dari</span><strong>Stasiun pilihanmu</strong></div>
			<div class="sheet-row"><span>Temukan</span><strong>Tempat dekat transit</strong></div>
			<div class="sheet-row">
				<span>Perhatikan</span><strong>Jarak &amp; waktu berjalan</strong>
			</div>
			<p>
				Makan, ngopi, taman, atau museum. Pilih alasan untuk singgah, bukan sekadar titik di peta.
			</p>
			<a class="preview__action" href={panelHref('explore')}
				>Pilih stasiun <span aria-hidden="true">↗</span></a
			>
		</div>
	{:else}
		<div class="passport-sheet">
			<div class="passport-sheet__head">
				<div>
					<span class="small-label">PASPOR / JURNAL PRIBADI</span>
					<h3>Yang ingin kamu ingat.</h3>
				</div>
				<img
					src="/stickers/sticker-checkpoint-masuk.webp"
					alt=""
					width="90"
					height="90"
					loading="lazy"
				/>
			</div>
			<ol class="passport-steps">
				<li><span>01</span>Pilih stasiun</li>
				<li><span>02</span>Tandai kunjungan</li>
				<li><span>03</span>Tulis cerita</li>
			</ol>
			<div class="journal-lines">
				<p>Catatan kecil dari perjalananmu.</p>
				<span></span><span></span>
			</div>
			<a class="preview__action" href={panelHref('passport')}
				>Buka Paspor <span aria-hidden="true">↗</span></a
			>
		</div>
	{/if}
	<p class="preview__disclaimer">Ilustrasi antarmuka · bukan peta, hasil rute, atau data akun.</p>
</div>

<style>
	.preview {
		color: var(--sg-text);
		background: var(--sg-canvas);
		border: 1px solid var(--sg-border);
		border-radius: 12px;
		padding: 8px;
		box-shadow:
			0 0 0 3px color-mix(in srgb, var(--sg-surface) 60%, transparent),
			0 2px 7px color-mix(in srgb, var(--sg-text) 7%, transparent);
		font-size: 13px;
		overflow: hidden;
		min-width: 0;
	}
	.preview__bar {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 12px;
		height: 34px;
		padding: 0 6px 8px;
		color: var(--sg-text-muted);
	}
	.preview__identity {
		display: flex;
		align-items: center;
		gap: 8px;
		border: 1px solid var(--sg-border);
		border-radius: 4px;
		padding: 4px 8px;
		font-size: 11px;
	}
	.preview__identity > span {
		margin-left: 16px;
	}
	.preview__caption,
	.small-label,
	.preview__disclaimer,
	.network__note {
		font: 9px/1.5 var(--sg-font-label);
		letter-spacing: 0.04em;
		color: var(--sg-text-muted);
	}
	.preview__disclaimer {
		margin: 9px 4px 0;
	}
	.workspace {
		display: grid;
		grid-template-columns: minmax(0, 1fr) 31%;
		min-height: 550px;
	}
	.network {
		position: relative;
		margin: 20px 15px 32px;
		min-width: 0;
		background-image: radial-gradient(var(--sg-border) 0.6px, transparent 0.6px);
		background-size: 14px 14px;
	}
	.network__lines {
		position: absolute;
		inset: 0;
		width: 100%;
		height: 100%;
		fill: none;
		stroke: var(--sg-border);
		stroke-width: 0.15;
	}
	.network__lines path {
		stroke-dasharray: 0.6 0.65;
	}
	.network__node,
	.network__center > span {
		min-height: var(--sg-target-min);
		display: inline-flex;
		align-items: center;
		justify-content: center;
		border: 1px solid var(--sg-border);
		border-radius: 8px;
		background: var(--sg-surface);
		padding: 8px 12px;
		text-decoration: none;
		color: var(--sg-text-muted);
		box-shadow: 0 3px 10px color-mix(in srgb, var(--sg-text) 4%, transparent);
		font-size: 11px;
	}
	.network__node {
		position: absolute;
		transform: translate(-50%, -50%);
		transition:
			translate var(--sg-motion-panel) var(--sg-ease-enter),
			box-shadow var(--sg-motion-panel),
			border-color var(--sg-motion-panel);
	}
	.network__node:hover {
		border-color: var(--sg-text-muted);
		color: var(--sg-text);
		translate: 0 -3px;
		box-shadow: 0 6px 12px color-mix(in srgb, var(--sg-text) 8%, transparent);
	}
	.network__center {
		position: absolute;
		top: 50%;
		left: 50%;
		transform: translate(-50%, -50%);
		color: var(--sg-text);
		text-decoration: none;
	}
	.network__center img {
		position: absolute;
		bottom: 100%;
		left: 50%;
		transform: translateX(-50%);
		object-fit: contain;
	}
	.network__center > span {
		padding-inline: 20px;
		font-family: Georgia, serif;
		font-size: 16px;
	}
	.network__note {
		position: absolute;
		bottom: -30px;
		left: 0;
		margin: 0;
	}
	.inspector {
		display: flex;
		flex-direction: column;
		background: var(--sg-surface);
		border: 1px solid var(--sg-border);
		border-radius: 8px;
		padding: 8px;
		box-shadow: 0 2px 9px color-mix(in srgb, var(--sg-text) 8%, transparent);
	}
	.inspector__tabs {
		display: flex;
		gap: 2px;
		padding-bottom: 8px;
	}
	.inspector__tabs button {
		flex: 1;
		padding: 6px;
		min-height: var(--sg-target-min);
		border: 1px solid transparent;
		border-radius: 4px;
		color: var(--sg-text-muted);
		font-size: 10px;
		cursor: pointer;
	}
	.inspector__tabs button[aria-pressed='true'] {
		background: var(--sg-surface-muted);
		border-color: var(--sg-border);
		color: var(--sg-text);
	}
	.inspector__tabs button:hover {
		background: var(--sg-surface-muted);
	}
	.inspector__body {
		flex: 1;
		display: flex;
		flex-direction: column;
		padding: 24px 12px 12px;
		border: 1px solid var(--sg-border);
		border-radius: 8px;
	}
	.inspector__body h3 {
		margin: 12px 0;
		font-size: 22px;
		letter-spacing: -0.035em;
	}
	.inspector__body p {
		color: var(--sg-text-muted);
		font-size: 12px;
		line-height: 1.6;
		margin: 0 0 24px;
	}
	.field {
		display: flex;
		align-items: center;
		gap: 10px;
		min-height: var(--sg-target-min);
		padding: 8px;
		margin-bottom: 8px;
		border: 1px solid var(--sg-border);
		border-radius: 5px;
		color: var(--sg-text-muted);
		text-decoration: none;
		font-size: 11px;
	}
	.field:hover {
		border-color: var(--sg-text-muted);
	}
	.preview__action {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 12px;
		min-height: var(--sg-target-min);
		margin-top: auto;
		padding: 8px 12px;
		background: var(--sg-surface);
		border: 1px solid var(--sg-border);
		border-radius: 6px;
		text-decoration: none;
		color: var(--sg-text);
		font-size: 12px;
		box-shadow: 0 1px 3px color-mix(in srgb, var(--sg-text) 5%, transparent);
	}
	.preview__action:hover {
		background: var(--sg-surface-muted);
	}
	.category-list {
		display: grid;
		gap: 10px;
		font-size: 12px;
		color: var(--sg-text-muted);
	}
	.planning-board {
		display: grid;
		grid-template-columns: repeat(3, minmax(0, 1fr));
		min-height: 360px;
		border: 1px solid var(--sg-border);
		border-radius: 8px;
		background-image: radial-gradient(var(--sg-border) 0.6px, transparent 0.6px);
		background-size: 12px 12px;
	}
	.planning-stage {
		padding: 8px;
		border-right: 1px solid var(--sg-border);
	}
	.planning-stage:last-child {
		border: 0;
	}
	.planning-stage > p {
		display: flex;
		justify-content: space-between;
		color: var(--sg-text-muted);
		margin: 0 0 70px;
		font: 10px var(--sg-font-label);
	}
	.planning-stage:nth-child(2) > p {
		margin-bottom: 100px;
	}
	.planning-stage:nth-child(3) > p {
		margin-bottom: 45px;
	}
	.planning-ticket {
		display: flex;
		align-items: center;
		gap: 6px;
		min-height: var(--sg-target-min);
		padding: 8px;
		margin-bottom: 24px;
		border: 1px solid var(--sg-border);
		border-radius: 5px;
		background: var(--sg-surface);
		color: var(--sg-text);
		text-decoration: none;
		box-shadow: 0 2px 5px color-mix(in srgb, var(--sg-text) 4%, transparent);
		font-size: 10px;
		transition: translate var(--sg-motion-panel) var(--sg-ease-enter);
	}
	.planning-ticket:hover {
		translate: 0 -3px;
	}
	.planning-ticket svg {
		width: 20px;
		height: 20px;
		flex: none;
		color: var(--sg-text-muted);
	}
	.planning-ticket small {
		display: block;
		margin-top: 4px;
		color: var(--sg-text-muted);
		font-size: 8px;
	}
	.planning-ticket > span:last-child {
		margin-left: auto;
	}
	.editorial-sheet,
	.passport-sheet {
		max-width: 440px;
		margin: 32px auto;
		padding: 24px;
		border: 1px solid var(--sg-border);
		border-radius: 8px;
		background: var(--sg-surface);
		box-shadow: 0 5px 15px color-mix(in srgb, var(--sg-text) 4%, transparent);
	}
	.sheet-heading {
		display: flex;
		align-items: center;
		justify-content: space-between;
	}
	.sheet-heading svg {
		width: 20px;
		height: 20px;
		color: var(--sg-text-muted);
	}
	.editorial-sheet h3,
	.passport-sheet h3 {
		font-size: 30px;
		line-height: 1.15;
		letter-spacing: -0.035em;
		margin: 24px 0;
	}
	.sheet-row {
		display: grid;
		grid-template-columns: 90px 1fr;
		gap: 12px;
		border-top: 1px solid var(--sg-border);
		padding-block: 12px;
		font-size: 11px;
	}
	.sheet-row > span {
		color: var(--sg-text-muted);
	}
	.sheet-row strong {
		font-weight: 500;
	}
	.editorial-sheet > p {
		color: var(--sg-text-muted);
		font-size: 12px;
		line-height: 1.65;
		margin: 24px 0;
	}
	.passport-sheet {
		max-width: 550px;
	}
	.passport-sheet__head {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 12px;
	}
	.passport-sheet__head h3 {
		font-size: 24px;
		margin-top: 12px;
	}
	.passport-sheet__head img {
		width: 72px;
		height: 72px;
		object-fit: contain;
	}
	.passport-steps {
		display: grid;
		grid-template-columns: repeat(3, minmax(0, 1fr));
		list-style: none;
		padding: 16px 0;
		margin: 0;
		border-block: 1px solid var(--sg-border);
		gap: 12px;
		font-size: 11px;
	}
	.passport-steps span {
		display: block;
		color: var(--sg-text-muted);
		font: 20px/1.5 var(--sg-font-label);
	}
	.journal-lines {
		margin: 24px 0;
	}
	.journal-lines p {
		font-size: 12px;
		color: var(--sg-text-muted);
	}
	.journal-lines span {
		display: block;
		height: 26px;
		border-bottom: 1px solid var(--sg-border);
	}
	@media (prefers-reduced-motion: no-preference) {
		.inspector__body {
			animation: panel-enter 240ms var(--sg-ease-enter) both;
		}
	}
	@keyframes panel-enter {
		from {
			opacity: 0;
			transform: translateY(8px);
		}
		to {
			opacity: 1;
			transform: none;
		}
	}
	@media (prefers-reduced-motion: reduce) {
		.network__node:hover,
		.planning-ticket:hover {
			translate: none;
		}
	}
	@media (max-width: 767px) {
		.workspace {
			grid-template-columns: 1fr;
			min-height: 0;
		}
		.network {
			height: 280px;
			margin: 0 10px 32px;
		}
		.network__node {
			padding-inline: 6px;
			font-size: 9px;
		}
		.network__center img {
			width: 42px;
			height: 42px;
		}
		.network__note {
			font-size: 8px;
		}
		.inspector__body {
			min-height: 280px;
			padding-top: 16px;
		}
		.inspector__body > .preview__action {
			margin-top: 24px;
		}
		.planning-board {
			min-height: 320px;
		}
		.planning-stage {
			padding: 5px;
		}
		.planning-ticket {
			padding: 6px;
			font-size: 9px;
			flex-wrap: wrap;
		}
		.planning-ticket svg {
			width: 16px;
			height: 16px;
		}
		.planning-ticket > span:last-child {
			display: none;
		}
		.editorial-sheet,
		.passport-sheet {
			margin: 16px 0;
			padding: 16px;
		}
		.preview__caption {
			font-size: 8px;
		}
		.passport-sheet__head img {
			width: 54px;
			height: 54px;
		}
	}
</style>
