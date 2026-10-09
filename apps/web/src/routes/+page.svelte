<script lang="ts">
	import { onMount } from 'svelte';
	import { asset, resolve } from '$app/paths';
	import { page } from '$app/state';
	import LandingPreview from '$lib/components/landing/LandingPreview.svelte';
	import HeroScene from '$lib/components/landing/HeroScene.svelte';
	import '@fontsource/poppins/latin-400.css';
	import '@fontsource/poppins/latin-500.css';
	import '@fontsource/poppins/latin-600.css';

	const appPanel = (panel: 'plan' | 'explore' | 'passport') =>
		resolve(`/app?panel=${panel}` as `/app?${string}`);
	const features = [
		{
			id: 'plan',
			label: '1.0 — BERANGKAT',
			title: 'Singgah bantu cari jalan yang masuk akal.',
			description:
				'Mulai dari stasiun awal dan tujuanmu. Bandingkan pilihan transit, pahami perpindahannya, lalu berangkat dengan lebih tenang.',
			topics: [
				{
					title: 'Rute multimoda',
					text: 'Lihat pilihan perjalanan dengan jaringan transit yang tersedia, tanpa harus membuka peta satu per satu.'
				},
				{
					title: 'Perpindahan yang jelas',
					text: 'Kenali stasiun dan urutan perpindahan sebelum mulai perjalanan.'
				},
				{
					title: 'Konteks jalan kaki',
					text: 'Perjalanan tidak berhenti di peron. Lihat bagian berjalan kaki dalam rencana perjalananmu.'
				},
				{
					title: 'Jadwal & sumber data',
					text: 'Jadwal bukan data live. Sumber dan keterbatasan data tetap ditampilkan apa adanya.'
				}
			]
		},
		{
			id: 'explore',
			label: '2.0 — SINGGAH',
			title: 'Turun bentar. Siapa tahu nemu tempat favorit baru.',
			description:
				'Jangan cuma lewat. Temukan tempat dekat stasiun, lihat konteks jalan kakinya, dan pilih alasan untuk menikmati kota.',
			topics: [
				{
					title: 'Tempat dekat transit',
					text: 'Pilih stasiun untuk mencari tempat dengan akses transit yang relevan, bukan sekadar dekat di peta.'
				},
				{
					title: 'Makan, taman & budaya',
					text: 'Jelajahi kategori makanan, kafe, hiburan, ruang hijau, budaya, dan belanja.'
				},
				{
					title: 'Walking trails',
					text: 'Ikuti jalur jelajah kurasi dengan urutan tempat dan konteks transit yang jelas.'
				},
				{
					title: 'Simpan untuk nanti',
					text: 'Simpan tempat yang menarik dan tandai kunjungan di sesi pribadimu.'
				}
			]
		},
		{
			id: 'passport',
			label: '3.0 — CERITA',
			title: 'Perjalanan selesai. Ceritanya tetap ikut pulang.',
			description:
				'Tandai stasiun yang pernah kamu singgahi dan simpan catatan kecil dari perjalanan. Paspor milikmu, cerita juga milikmu.',
			topics: [
				{
					title: 'Transit Passport',
					text: 'Kumpulkan kunjungan stasiun dengan check-in yang kamu pilih sendiri.'
				},
				{
					title: 'Catatan perjalanan',
					text: 'Simpan cerita, catatan exit, atau hal kecil yang ingin kamu ingat dari sebuah stasiun.'
				},
				{
					title: 'Progres jelajah',
					text: 'Lihat stasiun dan koleksi yang telah kamu kunjungi tanpa mengganggu kebutuhan perjalanan.'
				},
				{
					title: 'Tetap pribadi',
					text: 'Paspor tidak membutuhkan pelacakan lokasi sepanjang hari. Kunjungan dimulai dari tindakanmu.'
				}
			]
		}
	] as const;
	const chapters = [
		{
			title: 'Mulai dari tujuan',
			label: 'PERJALANAN',
			icon: 'transfer',
			href: appPanel('plan'),
			roman: 'I'
		},
		{
			title: 'Kenali transitnya',
			label: 'PETA & STASIUN',
			icon: 'station',
			href: resolve('/app'),
			roman: 'II'
		},
		{
			title: 'Singgah sebentar',
			label: 'CITY EXPLORER',
			icon: 'explore',
			href: appPanel('explore'),
			roman: 'III'
		},
		{
			title: 'Bawa pulang cerita',
			label: 'TRANSIT PASSPORT',
			icon: 'passport',
			href: appPanel('passport'),
			roman: 'IV'
		}
	];
	const demoTabs = [
		{
			id: 'network',
			name: 'Peta & perjalanan',
			description: 'Pahami hubungan transit sebelum berangkat.'
		},
		{
			id: 'explore',
			name: 'Jelajah kota',
			description: 'Cari alasan untuk singgah dekat stasiun.'
		},
		{
			id: 'passport',
			name: 'Paspor & jurnal',
			description: 'Simpan kunjungan dan cerita pribadimu.'
		}
	] as const;
	const wordRows = [
		'K O T A P E R J A L A N A N',
		'M A K A N S I N G G A H K R L',
		'J A L A N K A K I T A M A N',
		'T R A N S I T M U S E U M M R T',
		'N G O P I B U D A Y A J U R N A L',
		'S T A S I U N J E L A J A H',
		'P A S P O R C E R I T A K O T A'
	];
	let activeDemo = $state(0);
	let mobileMenuOpen = $state(false);
	let mobileMenuToggle: HTMLElement | undefined;
	let headerSolid = $state(false);
	let motionPaused = $state(false);
	let prefersReducedMotion = $state(false);
	let heroMotionReady = $state(false);

	function toggleHeroMotion() {
		motionPaused = !motionPaused;
	}

	function demoKeydown(event: KeyboardEvent, index: number) {
		let next: number;
		if (event.key === 'ArrowRight') next = (index + 1) % demoTabs.length;
		else if (event.key === 'ArrowLeft') next = (index + demoTabs.length - 1) % demoTabs.length;
		else if (event.key === 'Home') next = 0;
		else if (event.key === 'End') next = demoTabs.length - 1;
		else return;
		event.preventDefault();
		activeDemo = next;
		(event.currentTarget as HTMLElement).parentElement
			?.querySelectorAll<HTMLButtonElement>('[role="tab"]')
			[next]?.focus();
	}

	onMount(() => {
		let scrollFrame = 0;
		const syncHeader = () => {
			if (scrollFrame) return;
			scrollFrame = requestAnimationFrame(() => {
				headerSolid = window.scrollY > 48;
				scrollFrame = 0;
			});
		};
		const motionPreference = window.matchMedia('(prefers-reduced-motion: reduce)');
		const syncMotion = () => {
			prefersReducedMotion = motionPreference.matches;
		};
		syncHeader();
		syncMotion();
		heroMotionReady = true;
		window.addEventListener('scroll', syncHeader, { passive: true });
		motionPreference.addEventListener('change', syncMotion);
		return () => {
			if (scrollFrame) cancelAnimationFrame(scrollFrame);
			window.removeEventListener('scroll', syncHeader);
			motionPreference.removeEventListener('change', syncMotion);
		};
	});

	function reveal(node: HTMLElement) {
		if (!('IntersectionObserver' in window)) return;
		const observer = new IntersectionObserver(
			(entries) => {
				if (entries[0]?.isIntersecting) {
					node.dataset.inView = 'true';
					observer.disconnect();
				}
			},
			{ threshold: 0.1 }
		);
		observer.observe(node);
		return { destroy: () => observer.disconnect() };
	}
</script>

<svelte:head>
	<title>Singgah — Pergi boleh spontan. Rute jangan.</title>
	<meta
		name="description"
		content="Jelajahi Jakarta dengan rute transit yang masuk akal, tempat menarik dekat stasiun, dan cerita yang tersimpan."
	/>
	<meta name="theme-color" content="#1a6fd1" />
	<meta property="og:type" content="website" />
	<meta property="og:locale" content="id_ID" />
	<meta property="og:title" content="Singgah — Pergi boleh spontan. Rute jangan." />
	<meta
		property="og:description"
		content="Rencanakan perjalanan transit, jelajahi tempat dekat stasiun, dan simpan cerita perjalananmu di Jakarta."
	/>
	<meta property="og:url" content={page.url.origin + page.url.pathname} />
	<meta property="og:image" content={`${page.url.origin}/brand/singgah-social-preview.png`} />
	<meta property="og:image:width" content="1200" />
	<meta property="og:image:height" content="630" />
	<meta
		property="og:image:alt"
		content="Singgah dan ilustrasi pixel art perjalanan transit di Jakarta"
	/>
	<meta name="twitter:card" content="summary_large_image" />
</svelte:head>

<svelte:window
	onkeydown={(event) => {
		if (event.key === 'Escape' && mobileMenuOpen) {
			event.preventDefault();
			mobileMenuOpen = false;
			mobileMenuToggle?.focus();
		}
	}}
/>

<div class="landing">
	<header class="site-header" class:site-header--solid={headerSolid}>
		<div class="site-header__inner">
			<a class="wordmark" href="#top" aria-label="Singgah, kembali ke beranda">Singgah</a>
			<nav class="desktop-nav" aria-label="Navigasi halaman">
				<div class="nav-group">
					<span>Mulai dari</span><a href="#plan">Perjalanan</a><a href="#explore">Jelajah</a><a
						href="#passport">Paspor</a
					>
				</div>
				<a class="nav-surface" href="#guide">Panduan</a><a
					class="nav-surface"
					href={resolve('/providers')}>Sumber data</a
				>
			</nav>
			<a class="button button--light header-cta" href={resolve('/app')}>Buka aplikasi</a>
			<details class="mobile-menu" bind:open={mobileMenuOpen}>
				<summary bind:this={mobileMenuToggle} aria-label="Buka menu navigasi"
					><svg
						viewBox="0 0 24 24"
						fill="none"
						stroke="currentColor"
						stroke-width="1.5"
						aria-hidden="true"><path d="M4 7h16M4 12h16M4 17h16" /></svg
					></summary
				>
				<nav aria-label="Navigasi halaman mobile">
					{#each features as feature (feature.id)}<a
							href="#{feature.id}"
							onclick={() => (mobileMenuOpen = false)}
							>{feature.id === 'plan'
								? 'Perjalanan'
								: feature.id === 'explore'
									? 'Jelajah kota'
									: 'Transit Passport'}</a
						>{/each}
					<a href="#guide" onclick={() => (mobileMenuOpen = false)}>Panduan</a><a
						href={resolve('/providers')}>Sumber data</a
					>
				</nav>
			</details>
		</div>
	</header>

	<section class="hero" id="top" aria-labelledby="hero-title">
		<div class="hero__world" aria-hidden="true">
			<HeroScene paused={!heroMotionReady || motionPaused || prefersReducedMotion} />
		</div>
		<div class="hero__content">
			<h1 id="hero-title">Pergi boleh spontan.<br />Rute jangan.</h1>
			<p>
				Rencanakan perjalanan, temukan tempat dekat transit,<br class="desktop-break" /> dan nikmati Jakarta
				dengan lebih tenang.
			</p>
			<div class="hero__actions">
				<a class="button button--light" href={appPanel('plan')}>Cari Jalan</a><a
					class="button button--glass"
					href="#features">Jelajahi Singgah</a
				>
			</div>
		</div>
		<div class="hero__notes" aria-hidden="true">
			<div><i></i><span>01 · Mulai perjalanan</span><strong>Pilih tujuan</strong></div>
			<div><i></i><span>02 · Pahami transitnya</span><strong>Temukan rute</strong></div>
			<div><i></i><span>03 · Nikmati kotanya</span><strong>Singgah sebentar</strong></div>
		</div>
		<p class="hero__caption">Ilustrasi kota · bukan pelacakan live</p>
		{#if heroMotionReady && !prefersReducedMotion}
			<button class="button button--glass hero__motion" type="button" onclick={toggleHeroMotion}>
				{motionPaused ? 'Putar animasi' : 'Jeda animasi'}
			</button>
		{/if}
	</section>

	<div class="transit-strip" aria-label="Jaringan transit Jakarta">
		<div class="transit-strip__marks">
			{#each ['KRL Commuterline', 'MRT Jakarta', 'TransJakarta', 'LRT Jakarta & Jabodebek'] as mode (mode)}<a
					href={resolve('/providers')}
					><svg
						viewBox="0 0 64 64"
						fill="none"
						stroke="currentColor"
						stroke-width="3"
						aria-hidden="true"><use href="/icons/transit.svg#station" /></svg
					><span>{mode}</span></a
				>{/each}
		</div>
		<p>
			Ketersediaan data mengikuti masing-masing sumber. <a href={resolve('/providers')}
				>Lihat sumber data.</a
			>
		</p>
	</div>

	<section class="overview section" id="features" aria-labelledby="overview-title" use:reveal>
		<div class="container">
			<h2 id="overview-title">
				Singgah mendekatkan kamu dengan kota.<br /><span
					>Dari perjalanan transit sampai cerita di setiap singgahan.</span
				>
			</h2>
			<div class="overview__preview" use:reveal><LandingPreview /></div>
			<div class="overview__principles" use:reveal>
				<p>
					<strong>Perjalanan yang jelas</strong> — Rute, perpindahan, dan jalan kaki dalam satu konteks
					transit.
				</p>
				<p>
					<strong>Data apa adanya</strong> — Jadwal, estimasi, dan sumber data tidak disamarkan sebagai
					live.
				</p>
				<p>
					<strong>Jelajah dengan caramu</strong> — Pilih tempat untuk singgah dan simpan cerita pribadimu.
				</p>
			</div>
			<div class="overview__cta" use:reveal>
				<p>Mulai dari tujuanmu. Sisanya, cari jalan yang paling masuk akal.</p>
				<a class="button button--dark" href={appPanel('plan')}>Mulai di Singgah</a>
			</div>
		</div>
	</section>

	<section class="features section" aria-labelledby="features-title">
		<div class="container">
			<div class="features__intro" use:reveal>
				<p class="eyebrow">YANG BISA KAMU LAKUKAN</p>
				<h2 id="features-title">
					Kamu nikmati kotanya.<br /><span>Singgah bantu cari jalannya.</span>
				</h2>
				<p>
					Berangkat dengan rencana, singgah dengan alasan, pulang dengan cerita. Tidak perlu rumit.
				</p>
			</div>
			{#each features as feature, index (feature.id)}
				<div class="feature" class:feature--reverse={index === 1} id={feature.id} use:reveal>
					<div class="feature__copy">
						<p class="eyebrow">{feature.label}</p>
						<h3>{feature.title}</h3>
						<p>{feature.description}</p>
						<div class="feature__details">
							{#each feature.topics as topic, topicIndex (topic.title)}<details>
									<summary
										><span class="topic-number">{index + 1}.{topicIndex + 1}</span
										>{topic.title}<span class="topic-plus" aria-hidden="true"></span></summary
									>
									<p>{topic.text}</p>
								</details>{/each}
						</div>
					</div>
					<div class="feature__preview"><LandingPreview variant={feature.id} /></div>
				</div>
			{/each}
		</div>
	</section>

	<section class="guide section" id="guide" aria-labelledby="guide-title" use:reveal>
		<div class="container">
			<div class="center-heading">
				<h2 id="guide-title">Kenali kota. Satu langkah dulu.</h2>
				<p>
					Empat pintu masuk untuk perjalanan yang lebih tenang.<br />Pilih yang kamu butuhkan hari
					ini.
				</p>
				<a class="button button--dark" href={resolve('/app')}>Mulai jelajah</a>
			</div>
			<div class="books">
				{#each chapters as chapter, index (chapter.title)}<a
						class="book-link"
						href={chapter.href}
						use:reveal
						><article class="book">
							<h3>Bab {index + 1}<br />{chapter.title}</h3>
							<hr />
							<p class="book__chapter">Panduan {chapter.roman} · {chapter.label}</p>
							<svg
								viewBox="0 0 64 64"
								fill="none"
								stroke="currentColor"
								stroke-width="1.5"
								aria-hidden="true"><use href="/icons/transit.svg#{chapter.icon}" /></svg
							>
							<div class="book__footer">
								<span>Singgah · Jakarta</span><span>0{index + 1}</span>
							</div>
						</article>
						<span class="book-link__caption"
							>Buka bab ({chapter.roman}) <span aria-hidden="true">↗</span></span
						></a
					>{/each}
			</div>
		</div>
	</section>

	<section class="product-demo" aria-labelledby="demo-title" use:reveal>
		<div class="container">
			<div class="center-heading">
				<h2 id="demo-title">
					Satu tempat untuk bergerak.<br /><span>Banyak alasan untuk singgah.</span>
				</h2>
				<p>
					Dari rencana perjalanan sampai catatan pribadi.<br />Kamu yang pilih langkah berikutnya.
				</p>
			</div>
			<div class="demo-tabs" role="tablist" aria-label="Pratinjau aplikasi Singgah">
				{#each demoTabs as tab, index (tab.id)}<button
						type="button"
						role="tab"
						id="demo-tab-{tab.id}"
						aria-controls="demo-panel-{tab.id}"
						aria-selected={activeDemo === index}
						tabindex={activeDemo === index ? 0 : -1}
						onclick={() => (activeDemo = index)}
						onkeydown={(event) => demoKeydown(event, index)}
						><strong>{tab.name}</strong><span>{tab.description}</span></button
					>{/each}
			</div>
			{#each demoTabs as tab, index (tab.id)}<div
					class="demo-stage"
					id="demo-panel-{tab.id}"
					role="tabpanel"
					aria-labelledby="demo-tab-{tab.id}"
					tabindex="0"
					hidden={activeDemo !== index}
				>
					{#if activeDemo === index}<LandingPreview variant={tab.id} />{/if}
				</div>{/each}
		</div>
	</section>

	<section class="city-section section" aria-labelledby="city-title" use:reveal>
		<div class="container">
			<div class="center-heading">
				<h2 id="city-title">Satu kota. Banyak cerita.</h2>
				<p>
					Makan, ngopi, ruang hijau, budaya, hiburan, atau belanja.<br />Selalu ada alasan untuk
					singgah sebentar.
				</p>
			</div>
			<div class="city-word-grid" aria-hidden="true">
				{#each wordRows as row (row)}<div>{row}</div>{/each}
			</div>
			<a class="city-section__link" href={appPanel('explore')}
				>Temukan tempat dekat transit <span aria-hidden="true">↗</span></a
			>
		</div>
	</section>

	<footer class="site-footer" id="about">
		<div class="footer-layout container">
			<div class="footer-copy">
				<h2>Pergi boleh spontan.<br /><span>Rute jangan.</span></h2>
				<nav class="footer-chapters" aria-label="Bagian Singgah">
					<a href="#plan">Perjalanan</a><a href="#explore">Jelajah</a><a href="#passport">Paspor</a
					><a href="#guide">Panduan</a>
				</nav>
				<div class="footer-links">
					<a href="#top">Beranda</a><a href={resolve('/providers')}>Sumber &amp; atribusi</a><a
						href={resolve('/app')}>Buka aplikasi</a
					><a href={resolve('/explore/trails')}>Walking trails</a><a
						href="https://github.com/rendr17/Singgah"
						target="_blank"
						rel="noopener noreferrer">GitHub ↗</a
					><a href={appPanel('passport')}>Jurnal pribadi</a>
					<a href={asset('/illustrations/landing/ATTRIBUTION.md')}>Kredit ilustrasi</a>
				</div>
				<p class="footer-note">
					Jelajah transit Jakarta.<br />Data punya sumber. Cerita tetap punya kamu.
				</p>
				<a class="wordmark footer-wordmark" href="#top">Singgah</a>
			</div>
			<div class="footer-window">
				<img
					src="/illustrations/singgah-pixel-street-v2.webp"
					alt="Ilustrasi asli Singgah: jalan dan transit di Jakarta"
					width="1672"
					height="941"
					loading="lazy"
				/>
				<div>
					<p>Kota terasa lebih dekat<br /><span>saat jalannya jelas.</span></p>
					<a class="button button--light" href={appPanel('plan')}>Cari Jalan</a>
				</div>
			</div>
		</div>
		<p class="footer-bottom">
			Dibuat untuk perjalanan dan cerita di Jakarta. <a href="#top">Kembali ke atas ↑</a>
		</p>
	</footer>
</div>

<style>
	:global(html) {
		scroll-padding-top: 110px;
	}
	.landing {
		--landing-enter: 600ms;
		--landing-paper: var(--sg-surface);
		--landing-blue: var(--sg-story-blue);
		--landing-blue-soft: var(--sg-story-blue-soft);
		color: var(--sg-text);
		background: var(--sg-canvas);
		overflow: clip;
	}
	.container {
		width: min(calc(100% - 40px), 1080px);
		margin-inline: auto;
	}
	h1,
	h2,
	h3 {
		font-weight: 400;
		letter-spacing: -0.035em;
	}
	h2 {
		margin: 0;
		font-size: 40px;
		line-height: 1.15;
	}
	h2 > span {
		color: var(--sg-text-muted);
	}
	a {
		text-decoration: none;
	}
	.button {
		display: inline-flex;
		justify-content: center;
		align-items: center;
		min-height: var(--sg-target-min);
		padding: 0 14px;
		border: 1px solid var(--sg-border);
		border-radius: 8px;
		font-size: 14px;
		font-weight: 500;
		line-height: 1.4;
		text-decoration: none;
		white-space: nowrap;
		transition:
			background var(--sg-motion-fast),
			box-shadow var(--sg-motion-fast);
	}
	.button--light {
		background: var(--sg-canvas);
		color: var(--sg-text);
		box-shadow:
			inset 0 1px var(--sg-surface),
			0 1px 3px color-mix(in srgb, var(--sg-text) 8%, transparent);
	}
	.button--light:hover {
		background: var(--sg-surface);
	}
	.button--dark {
		border-color: var(--sg-text-muted);
		background: color-mix(in srgb, var(--sg-text) 85%, var(--sg-surface));
		color: var(--sg-surface);
		box-shadow:
			inset 0 1px 1px color-mix(in srgb, var(--sg-surface) 30%, transparent),
			0 2px 4px color-mix(in srgb, var(--sg-text) 18%, transparent);
	}
	.button--dark:hover {
		background: var(--sg-text);
	}
	.button--glass {
		border-color: color-mix(in srgb, var(--sg-surface) 25%, transparent);
		background: color-mix(in srgb, var(--sg-hero-deep) 35%, transparent);
		color: var(--sg-surface);
		box-shadow: inset 0 1px color-mix(in srgb, var(--sg-surface) 18%, transparent);
	}
	.button--glass:hover {
		background: color-mix(in srgb, var(--sg-hero-deep) 55%, transparent);
	}
	.site-header {
		position: fixed;
		inset: 0 0 auto;
		z-index: 20;
		color: var(--sg-surface);
		border-bottom: 1px solid transparent;
		transition:
			background var(--sg-motion-base),
			color var(--sg-motion-base);
	}
	.site-header__inner {
		max-width: 1440px;
		margin-inline: auto;
		padding: 24px 20px 22px;
		display: flex;
		align-items: center;
		gap: 12px;
	}
	.wordmark {
		font:
			30px/1 Georgia,
			'Times New Roman',
			serif;
		color: inherit;
		letter-spacing: -0.04em;
		display: inline-flex;
		align-items: center;
		min-height: var(--sg-target-min);
	}
	.desktop-nav {
		margin-left: auto;
		display: flex;
		align-items: center;
		gap: 12px;
	}
	.nav-group,
	.nav-surface {
		display: flex;
		align-items: center;
		min-height: var(--sg-target-min);
		border: 1px solid color-mix(in srgb, var(--sg-surface) 16%, transparent);
		border-radius: 8px;
		background: color-mix(in srgb, var(--sg-hero-deep) 76%, transparent);
		box-shadow:
			inset 0 1px color-mix(in srgb, var(--sg-surface) 15%, transparent),
			0 1px 4px color-mix(in srgb, var(--sg-text) 10%, transparent);
	}
	.nav-group {
		padding-inline: 8px;
	}
	.nav-group > span {
		padding-inline: 12px;
		font-size: 14px;
		color: color-mix(in srgb, var(--sg-surface) 75%, transparent);
	}
	.nav-group a,
	.nav-surface {
		padding-inline: 16px;
		font-size: 14px;
		color: inherit;
		white-space: nowrap;
	}
	.nav-group a {
		min-height: var(--sg-target-min);
		display: inline-flex;
		align-items: center;
		position: relative;
	}
	.nav-group a + a::before {
		content: '';
		position: absolute;
		left: 0;
		height: 12px;
		border-left: 1px solid color-mix(in srgb, var(--sg-surface) 20%, transparent);
	}
	.nav-group a:hover,
	.nav-surface:hover {
		background: color-mix(in srgb, var(--sg-surface) 12%, transparent);
		border-radius: 5px;
	}
	.site-header--solid {
		background: var(--sg-canvas);
		color: var(--sg-text);
		border-color: var(--sg-border);
	}
	.site-header--solid .nav-group,
	.site-header--solid .nav-surface {
		background: var(--sg-surface-muted);
		border-color: var(--sg-border);
		box-shadow: inset 0 1px var(--sg-surface);
	}
	.site-header--solid .nav-group > span {
		color: var(--sg-text-muted);
	}
	.site-header--solid .nav-group a + a::before {
		border-color: var(--sg-border);
	}
	.site-header--solid .nav-group a:hover,
	.site-header--solid .nav-surface:hover {
		background: var(--sg-surface);
	}
	.header-cta {
		min-width: 130px;
	}
	.mobile-menu {
		display: none;
		position: relative;
	}
	.mobile-menu summary {
		display: grid;
		place-items: center;
		width: var(--sg-target-min);
		height: var(--sg-target-min);
		list-style: none;
		cursor: pointer;
		border-radius: 8px;
		border: 1px solid color-mix(in srgb, var(--sg-surface) 30%, transparent);
		background: color-mix(in srgb, var(--sg-hero-deep) 25%, transparent);
	}
	.mobile-menu summary::-webkit-details-marker {
		display: none;
	}
	.mobile-menu summary svg {
		width: 24px;
		height: 24px;
	}
	.mobile-menu nav {
		position: absolute;
		right: 0;
		top: calc(100% + 12px);
		width: min(280px, calc(100vw - 40px));
		padding: 8px;
		display: grid;
		border: 1px solid var(--sg-border);
		border-radius: 8px;
		background: var(--sg-canvas);
		box-shadow: var(--sg-shadow-overlay);
		color: var(--sg-text);
	}
	.mobile-menu nav a {
		display: flex;
		align-items: center;
		min-height: var(--sg-target-min);
		padding-inline: 12px;
		font-size: 14px;
		color: inherit;
		border-radius: 4px;
	}
	.mobile-menu nav a:hover {
		background: var(--sg-surface-muted);
	}
	.site-header--solid .mobile-menu summary {
		border-color: var(--sg-border);
		background: var(--sg-surface-muted);
	}
	.hero {
		--hero-font: 'Poppins', var(--sg-font-body);
		position: relative;
		isolation: isolate;
		min-height: max(100svh, 620px);
		margin-bottom: 69px;
		color: var(--sg-surface);
		background: var(--sg-hero);
		font-family: var(--hero-font);
	}
	.hero__world {
		position: absolute;
		inset: 0;
		overflow: hidden;
		z-index: -1;
		background: var(--sg-hero);
	}
	.hero__world::after {
		content: '';
		position: absolute;
		inset: 0;
		background: linear-gradient(
			90deg,
			color-mix(in srgb, var(--sg-hero-deep) 34%, transparent) 0%,
			color-mix(in srgb, var(--sg-hero-deep) 34%, transparent) 38%,
			transparent 67%
		);
	}
	.hero::after {
		content: '';
		position: absolute;
		inset-inline: 0;
		bottom: -40px;
		height: 72px;
		background: var(--sg-canvas);
		mask: url('/illustrations/pixel-fringe.svg') repeat-x left bottom / 512px 72px;
		pointer-events: none;
	}
	.hero__content {
		max-width: 1440px;
		padding: 150px 20px 50px;
		margin-inline: auto;
	}
	.hero h1 {
		max-width: 640px;
		font-family: var(--hero-font);
		font-size: 46px;
		font-weight: 500;
		line-height: 1.14;
		margin: 0;
		letter-spacing: -0.025em;
		text-shadow: 0 1px 3px color-mix(in srgb, var(--sg-hero-deep) 15%, transparent);
	}
	.hero__content > p {
		max-width: 520px;
		margin: 20px 0 0;
		font-size: 16px;
		line-height: 1.4;
		font-weight: 400;
		text-shadow: 0 1px 3px var(--sg-hero-deep);
	}
	.hero__actions {
		display: flex;
		flex-wrap: wrap;
		gap: 12px;
		margin-top: 24px;
	}
	.hero__motion {
		position: absolute;
		right: 20px;
		bottom: 48px;
		font-size: 12px;
		background: color-mix(in srgb, var(--sg-hero-deep) 85%, transparent);
	}
	.hero__caption {
		position: absolute;
		left: 20px;
		bottom: 48px;
		margin: 0;
		padding: 6px 9px;
		border-radius: 4px;
		background: color-mix(in srgb, var(--sg-hero-deep) 85%, transparent);
		font: 10px/1.5 var(--sg-font-label);
	}
	.hero :focus-visible,
	.site-header:not(.site-header--solid) :focus-visible {
		outline-color: var(--sg-surface);
	}
	.hero__notes {
		position: absolute;
		top: 33%;
		left: 57%;
		display: grid;
		gap: 8px;
		transform: perspective(900px) rotateY(-10deg) rotateZ(-2deg);
		pointer-events: none;
	}
	.hero__notes > div {
		position: relative;
		overflow: hidden;
		display: flex;
		align-items: center;
		gap: 8px;
		width: clamp(290px, 24vw, 360px);
		min-height: 38px;
		padding: 8px 12px;
		border: 1px solid color-mix(in srgb, var(--sg-surface) 15%, transparent);
		border-radius: 6px;
		background: color-mix(in srgb, var(--sg-hero-deep) 80%, transparent);
		box-shadow: inset 0 1px color-mix(in srgb, var(--sg-surface) 12%, transparent);
		font-size: 12px;
		font-weight: 400;
		animation: hero-step-loop 12s ease-in-out infinite;
	}
	.hero__notes > div:nth-child(2) {
		animation-delay: -4s;
	}
	.hero__notes > div:nth-child(2)::after {
		animation-delay: -4s;
	}
	.hero__notes > div:nth-child(2)::before {
		animation-delay: -4s;
	}
	.hero__notes > div:nth-child(3) {
		animation-delay: -8s;
	}
	.hero__notes > div:nth-child(3)::after {
		animation-delay: -8s;
	}
	.hero__notes > div:nth-child(3)::before {
		animation-delay: -8s;
	}
	.hero:has(:global(.hero-scene.is-paused)) .hero__notes > div {
		animation-play-state: paused;
	}
	.hero:has(:global(.hero-scene.is-paused)) .hero__notes > div::before {
		animation-play-state: paused;
	}
	.hero:has(:global(.hero-scene.is-paused)) .hero__notes > div::after {
		animation-play-state: paused;
	}
	@keyframes hero-step-loop {
		0%,
		3%,
		20%,
		100% {
			background: color-mix(in srgb, var(--sg-hero-deep) 94%, transparent);
			border-color: color-mix(in srgb, var(--sg-surface) 18%, transparent);
			transform: none;
			box-shadow: inset 0 1px color-mix(in srgb, var(--sg-surface) 12%, transparent);
		}
		5%,
		18% {
			background: color-mix(in srgb, var(--sg-hero-deep) 70%, transparent);
			border-color: color-mix(in srgb, var(--sg-surface) 68%, transparent);
			transform: translateX(7px) scale(1.01);
			box-shadow:
				inset 0 1px color-mix(in srgb, var(--sg-surface) 30%, transparent),
				0 4px 14px color-mix(in srgb, var(--sg-hero-deep) 18%, transparent);
		}
	}
	.hero__notes > div::before {
		position: absolute;
		z-index: 0;
		inset-block: -40%;
		left: 0;
		width: 38%;
		background: linear-gradient(
			110deg,
			transparent 12%,
			color-mix(in srgb, var(--sg-surface) 28%, transparent) 50%,
			transparent 88%
		);
		content: '';
		opacity: 0;
		transform: translateX(-145%) skewX(-16deg);
		animation: hero-step-sheen 12s ease-in-out infinite;
		pointer-events: none;
	}
	.hero__notes > div::after {
		position: absolute;
		z-index: 2;
		inset-inline: 0;
		bottom: 0;
		height: 2px;
		background: var(--sg-scene-leaf);
		content: '';
		transform: scaleX(0);
		transform-origin: left center;
		animation: hero-step-progress 12s linear infinite;
		pointer-events: none;
	}
	.hero__notes i,
	.hero__notes span,
	.hero__notes strong {
		position: relative;
		z-index: 1;
	}
	@keyframes hero-step-sheen {
		0%,
		4%,
		100% {
			opacity: 0;
			transform: translateX(-145%) skewX(-16deg);
		}
		5% {
			opacity: 0.7;
		}
		17% {
			opacity: 0.45;
			transform: translateX(280%) skewX(-16deg);
		}
		20% {
			opacity: 0;
			transform: translateX(330%) skewX(-16deg);
		}
	}
	@keyframes hero-step-progress {
		0%,
		5%,
		100% {
			transform: scaleX(0);
		}
		18% {
			transform: scaleX(1);
		}
		20% {
			transform: scaleX(0);
		}
	}
	.hero__notes i {
		width: 5px;
		height: 5px;
		background: var(--sg-scene-leaf);
		flex: none;
	}
	.hero__notes span {
		color: color-mix(in srgb, var(--sg-surface) 88%, transparent);
	}
	.hero__notes strong {
		font-size: 12px;
		font-weight: 600;
	}
	.transit-strip {
		padding: 38px 20px 28px;
		text-align: center;
	}
	.transit-strip__marks {
		display: grid;
		grid-template-columns: repeat(4, minmax(0, 1fr));
		max-width: 730px;
		margin-inline: auto;
		gap: 16px;
	}
	.transit-strip__marks a {
		min-height: 84px;
		padding: 16px;
		display: flex;
		align-items: center;
		justify-content: center;
		gap: 10px;
		border: 1px solid var(--sg-border);
		border-radius: 10px;
		background: color-mix(in srgb, var(--sg-surface) 40%, var(--sg-canvas));
		box-shadow:
			inset 0 1px var(--sg-surface),
			0 0 0 3px color-mix(in srgb, var(--sg-surface) 55%, transparent);
		color: var(--sg-text-muted);
		font-size: 13px;
	}
	.transit-strip__marks a:hover {
		background: var(--sg-surface);
	}
	.transit-strip__marks svg {
		width: 24px;
		height: 24px;
		flex: none;
	}
	.transit-strip > p {
		font-size: 12px;
		color: var(--sg-text-muted);
		margin: 32px 0 0;
	}
	.transit-strip > p a {
		color: inherit;
		text-decoration: underline;
		text-underline-offset: 3px;
	}
	.section {
		padding-block: 88px;
	}
	.overview h2 {
		text-align: center;
	}
	.overview__preview {
		margin-top: clamp(56px, 7vw, 88px);
		perspective: 1400px;
	}
	.overview__principles {
		display: grid;
		grid-template-columns: repeat(3, minmax(0, 1fr));
		gap: 32px;
		margin-top: 40px;
	}
	.overview__principles p {
		margin: 0;
		font-size: 14px;
		line-height: 1.5;
		color: var(--sg-text-muted);
	}
	.overview__principles strong {
		color: var(--sg-text);
		font-weight: 500;
	}
	.overview__cta {
		display: flex;
		justify-content: center;
		align-items: center;
		gap: 48px;
		margin-top: 40px;
	}
	.overview__cta p {
		max-width: 440px;
		color: var(--sg-text-muted);
		font-size: 14px;
		margin: 0;
	}
	.features {
		padding-top: 112px;
	}
	.features__intro {
		max-width: 480px;
	}
	.eyebrow {
		font: 10px/1.5 var(--sg-font-label);
		letter-spacing: 0.12em;
		color: var(--sg-text-muted);
		margin: 0 0 16px;
	}
	.features__intro > p:last-child,
	.feature__copy > p:not(.eyebrow) {
		font-size: 14px;
		line-height: 1.6;
		color: var(--sg-text-muted);
		margin: 24px 0 0;
	}
	.feature {
		display: grid;
		grid-template-columns: minmax(280px, 340px) minmax(0, 1fr);
		align-items: center;
		gap: clamp(40px, 5vw, 72px);
		margin-top: clamp(112px, 12vw, 168px);
		min-height: 510px;
		scroll-margin-top: 120px;
	}
	.feature--reverse {
		grid-template-columns: minmax(0, 1fr) 340px;
	}
	.feature--reverse .feature__copy {
		order: 2;
	}
	.feature__copy h3 {
		margin: 0;
		font-size: 36px;
		line-height: 1.12;
	}
	.feature__details {
		display: grid;
		gap: 8px;
		margin-top: 32px;
	}
	.feature__details details {
		border: 1px solid var(--sg-border);
		border-radius: 6px;
		background: color-mix(in srgb, var(--sg-surface) 70%, var(--sg-canvas));
	}
	.feature__details summary {
		display: flex;
		align-items: center;
		gap: 10px;
		min-height: var(--sg-target-min);
		padding: 8px 12px;
		list-style: none;
		cursor: pointer;
		font-size: 12px;
	}
	.feature__details summary::-webkit-details-marker {
		display: none;
	}
	.topic-number {
		font: 10px var(--sg-font-label);
		color: var(--sg-text-muted);
	}
	.topic-plus {
		margin-left: auto;
		position: relative;
		width: 10px;
		height: 10px;
		color: var(--sg-text-muted);
	}
	.topic-plus::before,
	.topic-plus::after {
		content: '';
		position: absolute;
		top: 4px;
		width: 10px;
		border-top: 1px solid currentColor;
	}
	.topic-plus::after {
		transform: rotate(90deg);
	}
	.feature__details details[open] .topic-plus::after {
		display: none;
	}
	.feature__details summary:hover {
		background: var(--sg-surface-muted);
		border-radius: 6px;
	}
	.feature__details details > p {
		margin: 0;
		padding: 0 12px 16px;
		font-size: 12px;
		line-height: 1.6;
		color: var(--sg-text-muted);
	}
	.feature__preview {
		min-width: 0;
		perspective: 1200px;
	}
	.feature--reverse .feature__preview {
		max-width: 460px;
	}
	.feature__preview :global(.preview:not(.preview--network)) {
		background: transparent;
		box-shadow: none;
		border-color: transparent;
	}
	.feature__preview :global(.preview__bar) {
		display: none;
	}
	.center-heading {
		text-align: center;
	}
	.center-heading p {
		font-size: 14px;
		color: var(--sg-text-muted);
		margin: 24px 0;
		line-height: 1.6;
	}
	.guide {
		padding-top: 112px;
		padding-bottom: 160px;
		background-image:
			linear-gradient(color-mix(in srgb, var(--sg-border) 20%, transparent) 1px, transparent 1px),
			linear-gradient(
				90deg,
				color-mix(in srgb, var(--sg-border) 20%, transparent) 1px,
				transparent 1px
			);
		background-size: 96px 96px;
	}
	.books {
		display: grid;
		grid-template-columns: repeat(2, 290px);
		justify-content: center;
		gap: 64px 80px;
		margin-top: 128px;
	}
	.book-link {
		color: var(--sg-text);
		min-width: 0;
	}
	.book {
		position: relative;
		min-height: 350px;
		padding: 36px 24px 20px 36px;
		display: flex;
		flex-direction: column;
		border: 1px solid var(--sg-border);
		border-radius: 4px 12px 12px 4px;
		background: var(--landing-paper);
		box-shadow:
			inset 15px 0 12px -15px var(--sg-border),
			0 6px 0 -1px var(--sg-canvas),
			0 7px 0 -1px var(--sg-border),
			0 12px 0 -2px var(--sg-canvas),
			0 13px 0 -2px var(--sg-border),
			0 20px 24px -12px color-mix(in srgb, var(--sg-text) 10%, transparent);
		transition:
			transform var(--landing-enter) var(--sg-ease-enter),
			box-shadow var(--landing-enter) var(--sg-ease-enter);
		transform-origin: 20% 70%;
	}
	.book h3 {
		margin: 0;
		font-size: 20px;
		line-height: 1.1;
	}
	.book hr {
		margin-block: 28px 16px;
		border-color: var(--sg-border);
	}
	.book__chapter {
		font: 8px/1.7 var(--sg-font-label);
		color: var(--sg-text-muted);
	}
	.book > svg {
		align-self: center;
		width: 100px;
		height: 100px;
		margin: 25px 0;
		color: var(--sg-border-strong);
	}
	.book__footer {
		display: flex;
		justify-content: space-between;
		margin-top: auto;
		font-size: 9px;
		color: var(--sg-text-muted);
	}
	.book-link__caption {
		display: block;
		text-align: center;
		font: 12px var(--sg-font-label);
		color: var(--sg-text-muted);
		margin-top: 40px;
	}
	.book-link:hover .book {
		transform: perspective(900px) rotateY(-7deg) rotateZ(-2deg) translateY(-7px);
	}
	.product-demo {
		padding-block: 190px 210px;
		background: linear-gradient(180deg, var(--landing-blue), var(--landing-blue-soft));
		color: var(--sg-surface);
	}
	.product-demo .center-heading h2 > span {
		color: var(--sg-surface);
	}
	.product-demo .center-heading p {
		color: var(--sg-text);
	}
	.demo-tabs {
		display: grid;
		grid-template-columns: repeat(3, minmax(0, 1fr));
		width: min(750px, 100%);
		margin: 40px auto 48px;
	}
	.demo-tabs button {
		text-align: left;
		padding: 4px 16px;
		min-height: var(--sg-target-min);
		border-left: 1px solid color-mix(in srgb, var(--sg-surface) 35%, transparent);
		color: var(--sg-text);
		cursor: pointer;
		font-size: 12px;
		line-height: 1.5;
	}
	.demo-tabs strong {
		display: block;
		font-weight: 500;
		margin-bottom: 3px;
	}
	.demo-tabs button > span {
		display: block;
	}
	.demo-tabs button[aria-selected='true'],
	.demo-tabs button:hover {
		border-left-width: 3px;
		background: color-mix(in srgb, var(--sg-surface) 20%, transparent);
	}
	.demo-tabs button:focus-visible {
		outline-color: var(--sg-hero-deep);
	}
	.demo-stage {
		max-width: 800px;
		margin-inline: auto;
		min-height: 620px;
		padding: 7px;
		border: 1px solid color-mix(in srgb, var(--sg-surface) 55%, transparent);
		border-radius: 14px;
		background: color-mix(in srgb, var(--sg-surface) 20%, transparent);
		box-shadow: 0 0 0 4px color-mix(in srgb, var(--sg-surface) 15%, transparent);
	}
	.demo-stage[hidden] {
		display: none;
	}
	.demo-stage :global(.preview) {
		min-height: 604px;
		background: var(--sg-canvas);
		display: flex;
		flex-direction: column;
	}
	.demo-stage :global(.workspace) {
		flex: 1;
		grid-template-columns: minmax(0, 1fr) 37%;
	}
	.demo-stage :global(.editorial-sheet),
	.demo-stage :global(.passport-sheet) {
		margin-block: auto;
		width: min(100%, 510px);
	}
	.demo-stage :global(.preview__disclaimer) {
		margin-top: auto;
		padding-top: 12px;
	}
	.city-section {
		padding-block: 112px;
		text-align: center;
	}
	.city-section h2 {
		font-size: 30px;
	}
	.city-word-grid {
		margin: 64px auto 32px;
		font: 14px/2.4 var(--sg-font-label);
		letter-spacing: 0.55em;
		color: var(--sg-text-muted);
	}
	.city-section__link {
		min-height: var(--sg-target-min);
		display: inline-flex;
		align-items: center;
		gap: 8px;
		font-size: 13px;
		color: var(--sg-text);
	}
	.city-section__link:hover {
		text-decoration: underline;
		text-underline-offset: 4px;
	}
	.site-footer {
		padding-top: 120px;
	}
	.footer-layout {
		display: grid;
		grid-template-columns: 1fr 280px;
		align-items: start;
		gap: 80px;
	}
	.footer-copy h2 {
		font-size: 32px;
	}
	.footer-chapters {
		display: flex;
		flex-wrap: wrap;
		gap: 8px 16px;
		margin-top: 40px;
	}
	.footer-chapters a,
	.footer-links a {
		font-size: 12px;
		color: var(--sg-text-muted);
		min-height: var(--sg-target-min);
		display: inline-flex;
		align-items: center;
	}
	.footer-links {
		display: grid;
		grid-template-columns: repeat(2, minmax(0, 170px));
		margin-top: 12px;
	}
	.footer-chapters a:hover,
	.footer-links a:hover {
		color: var(--sg-text);
		text-decoration: underline;
		text-underline-offset: 4px;
	}
	.footer-note {
		margin-top: 32px;
		font-size: 10px;
		line-height: 1.8;
		color: var(--sg-text-muted);
	}
	.footer-wordmark {
		margin-top: 24px;
		font-size: 24px;
	}
	.footer-window {
		min-height: 380px;
		border: 1px solid var(--sg-border);
		border-radius: 12px;
		padding: 12px;
		background: var(--sg-surface);
		box-shadow:
			inset 0 0 0 3px var(--sg-canvas),
			0 2px 8px color-mix(in srgb, var(--sg-text) 4%, transparent);
		display: flex;
		flex-direction: column;
	}
	.footer-window > img {
		width: 100%;
		height: 210px;
		object-fit: cover;
		border-radius: 6px;
	}
	.footer-window > div {
		padding: 16px 4px 4px;
	}
	.footer-window p {
		font-size: 18px;
		line-height: 1.2;
		margin-bottom: 16px;
	}
	.footer-window p > span {
		color: var(--sg-text-muted);
	}
	.footer-window .button {
		min-height: var(--sg-target-min);
		font-size: 12px;
	}
	.footer-bottom {
		text-align: center;
		padding: 112px 20px 28px;
		font-size: 10px;
		color: var(--sg-text-muted);
	}
	.footer-bottom a {
		color: inherit;
		min-height: var(--sg-target-min);
		display: inline-flex;
		align-items: center;
		margin-left: 16px;
	}
	@media (max-width: 999px) {
		.desktop-nav {
			display: none;
		}
		.header-cta {
			margin-left: auto;
		}
		.mobile-menu {
			display: block;
		}
		.site-header__inner {
			padding-block: 18px;
		}
		.hero h1 {
			font-size: 38px;
		}
		.hero__notes {
			display: none;
		}
		h2 {
			font-size: 32px;
		}
		.feature,
		.feature--reverse {
			grid-template-columns: minmax(0, 0.9fr) minmax(0, 1.1fr);
			gap: 32px;
		}
		.feature--reverse {
			grid-template-columns: minmax(0, 1.1fr) minmax(0, 0.9fr);
		}
		.feature__copy h3 {
			font-size: 30px;
		}
		.city-word-grid {
			font-size: 12px;
			letter-spacing: 0.4em;
		}
	}
	@media (max-width: 767px) {
		.wordmark {
			font-size: 24px;
		}
		.header-cta {
			min-width: 0;
			font-size: 13px;
			padding-inline: 12px;
		}
		.hero {
			min-height: max(100svh, 720px);
		}
		.hero__content {
			padding-top: 166px;
		}
		.hero__world::after {
			background: linear-gradient(
				180deg,
				color-mix(in srgb, var(--sg-hero-deep) 40%, transparent) 0%,
				color-mix(in srgb, var(--sg-hero-deep) 40%, transparent) 46%,
				transparent 66%
			);
		}
		.hero h1 {
			font-size: clamp(28px, 8.2vw, 34px);
			line-height: 1.18;
		}
		.hero__content > p {
			max-width: 340px;
			margin-top: 24px;
		}
		.desktop-break {
			display: none;
		}
		.hero__actions {
			margin-top: 24px;
			gap: 12px;
		}
		.hero__caption {
			bottom: 34px;
			font-size: 8px;
		}
		.hero__motion {
			bottom: 66px;
			font-size: 10px;
		}
		.transit-strip {
			padding-top: 32px;
		}
		.transit-strip__marks {
			grid-template-columns: repeat(2, minmax(0, 1fr));
			gap: 16px;
		}
		.transit-strip__marks a {
			font-size: 11px;
			padding: 12px;
		}
		.transit-strip > p {
			line-height: 1.6;
		}
		.section {
			padding-block: 72px;
		}
		h2 {
			font-size: 28px;
		}
		.overview__preview {
			margin-top: 56px;
		}
		.overview__principles {
			grid-template-columns: 1fr;
			gap: 20px;
			margin-top: 32px;
		}
		.overview__cta {
			flex-direction: column;
			align-items: flex-start;
			gap: 20px;
			margin-top: 32px;
		}
		.feature,
		.feature--reverse {
			grid-template-columns: 1fr;
			gap: 48px;
			margin-top: 112px;
			min-height: 0;
		}
		.feature--reverse .feature__copy {
			order: 0;
		}
		.feature--reverse .feature__preview {
			max-width: none;
		}
		.feature__copy h3 {
			max-width: 19ch;
			font-size: 30px;
		}
		.guide {
			padding-top: 88px;
			padding-bottom: 112px;
		}
		.books {
			grid-template-columns: minmax(0, 290px);
			gap: 64px;
			margin-top: 80px;
		}
		.product-demo {
			padding-block: 112px;
		}
		.demo-tabs {
			gap: 4px;
			margin-block: 40px;
		}
		.demo-tabs button {
			padding: 8px;
			font-size: 11px;
		}
		.demo-tabs button > span {
			display: none;
		}
		.demo-stage {
			min-height: 760px;
			padding: 5px;
		}
		.demo-stage :global(.preview) {
			min-height: 748px;
		}
		.demo-stage :global(.workspace) {
			grid-template-columns: 1fr;
		}
		.demo-stage :global(.inspector__body) {
			min-height: 300px;
		}
		.city-section {
			padding-block: 88px;
		}
		.city-section h2 {
			font-size: 28px;
		}
		.city-word-grid {
			font-size: 9px;
			letter-spacing: 0.16em;
			line-height: 2.8;
			margin-top: 48px;
			white-space: nowrap;
		}
		.site-footer {
			padding-top: 56px;
		}
		.footer-layout {
			grid-template-columns: 1fr;
			gap: 64px;
		}
		.footer-copy h2 {
			font-size: 30px;
		}
		.footer-window {
			width: min(280px, 100%);
		}
		.footer-bottom {
			padding-top: 80px;
			line-height: 1.7;
		}
	}
	@media (prefers-reduced-motion: no-preference) {
		.hero__content > * {
			animation: landing-enter var(--landing-enter) var(--sg-ease-enter) both;
		}
		.hero__content > h1 {
			animation-delay: 100ms;
		}
		.hero__content > p {
			animation-delay: 500ms;
		}
		.hero__actions {
			animation-delay: 900ms;
		}

		.feature:global([data-in-view='true']) > .feature__copy,
		.features__intro:global([data-in-view='true']),
		.overview:global([data-in-view='true']) h2,
		.overview__principles:global([data-in-view='true']),
		.overview__cta:global([data-in-view='true']),
		.book-link:global([data-in-view='true']),
		.guide:global([data-in-view='true']) .center-heading,
		.product-demo:global([data-in-view='true']) .center-heading,
		.city-section:global([data-in-view='true']) .center-heading {
			animation: landing-enter var(--landing-enter) var(--sg-ease-enter) both;
		}
		.feature:global([data-in-view='true']) > .feature__preview,
		.overview__preview:global([data-in-view='true']) {
			animation: preview-enter 800ms var(--sg-ease-enter) both;
			animation-delay: 150ms;
		}
		.feature:global([data-in-view='true']) .feature__details > details {
			animation: landing-enter var(--landing-enter) var(--sg-ease-enter) both;
			animation-delay: 200ms;
		}
		.feature:global([data-in-view='true']) .feature__details > details:nth-child(2) {
			animation-delay: 280ms;
		}
		.feature:global([data-in-view='true']) .feature__details > details:nth-child(3) {
			animation-delay: 360ms;
		}
		.feature:global([data-in-view='true']) .feature__details > details:nth-child(4) {
			animation-delay: 440ms;
		}
		.demo-stage:not([hidden]) {
			animation: landing-enter 360ms var(--sg-ease-enter) both;
		}
	}
	@keyframes landing-enter {
		from {
			opacity: 0;
			transform: translateY(16px);
		}
		to {
			opacity: 1;
			transform: none;
		}
	}
	@keyframes preview-enter {
		from {
			opacity: 0;
			transform: translateY(28px) rotateX(4deg) scale(0.98);
		}
		to {
			opacity: 1;
			transform: none;
		}
	}
	@media (prefers-reduced-motion: reduce) {
		.hero__motion {
			display: none;
		}
		.book-link:hover .book {
			transform: none;
		}
	}
</style>
