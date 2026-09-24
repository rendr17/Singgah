<script lang="ts">
	import {
		Button,
		IconButton,
		SearchField,
		StatusBadge,
		Surface,
		SectionHeading,
		StateBlock,
		TextField
	} from '@singgah/ui';

	let query = $state('');
</script>

<div class="preview">
	<SectionHeading level={2}>Design foundations preview</SectionHeading>

	<section>
		<h3>Buttons</h3>
		<div class="row">
			<Button>Cari Jalan</Button>
			<Button variant="secondary">Rencanakan Perjalanan</Button>
			<Button variant="ghost">Lihat di Peta</Button>
			<IconButton label="Bagikan">
				<svg
					viewBox="0 0 24 24"
					fill="none"
					stroke="currentColor"
					stroke-width="2"
					stroke-linecap="round"
					stroke-linejoin="round"
					width="20"
					height="20"
					aria-hidden="true"
				>
					<path d="M4 12v8a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2v-8" />
					<path d="m16 6-4-4-4 4" />
					<path d="M12 2v13" />
				</svg>
			</IconButton>
		</div>
	</section>

	<section>
		<h3>Fields</h3>
		<SearchField
			label="Cari stasiun atau halte"
			placeholder="Mau ke mana hari ini?"
			bind:value={query}
		/>
		<TextField label="Nama" placeholder="Nama kamu" hint="Dipakai untuk passport." />
		<TextField label="Contoh error" error="Wajib diisi." value="" />
	</section>

	<section>
		<h3>Status</h3>
		<div class="row">
			<StatusBadge status="live" />
			<StatusBadge status="estimated" />
			<StatusBadge status="scheduled" />
			<StatusBadge status="stale" />
			<StatusBadge status="disruption" />
		</div>
	</section>

	<section>
		<h3>Surface</h3>
		<Surface>
			<SectionHeading level={3}>Stasiun Cikini</SectionHeading>
			<p class="meta">450 m dari kamu · KRL</p>
			<p class="meta sg-tabular">3 mnt ke Manggarai · 8 mnt ke Jakarta Kota</p>
		</Surface>
	</section>

	<section>
		<h3>States</h3>
		<StateBlock kind="loading">Mencari rute…</StateBlock>
		<StateBlock kind="empty">Tidak ada stasiun yang cocok.</StateBlock>
		<StateBlock kind="error">Data stasiun gagal dimuat.</StateBlock>
		<StateBlock kind="offline">Tidak ada koneksi — data mungkin tersimpan lama.</StateBlock>
	</section>
</div>

<style>
	.preview {
		display: grid;
		gap: var(--sg-space-6);
		max-inline-size: 40rem;
		padding-block-start: var(--sg-space-6);
	}

	.preview section {
		display: grid;
		gap: var(--sg-space-3);
	}

	.preview h3 {
		font-size: var(--sg-text-secondary);
		font-weight: var(--sg-weight-bold);
		color: var(--sg-text-muted);
		text-transform: uppercase;
		letter-spacing: 0.04em;
		margin: 0;
	}

	.row {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: var(--sg-space-3);
	}

	.meta {
		margin: var(--sg-space-1) 0 0;
		font-size: var(--sg-text-secondary);
		color: var(--sg-text-muted);
	}
</style>
