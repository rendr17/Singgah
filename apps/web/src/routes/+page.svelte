<script lang="ts">
	import { onMount } from 'svelte';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import { Surface } from '@singgah/ui';
	const appPanel = (panel: 'plan' | 'journey' | 'explore' | 'passport') =>
		resolve(`/app?panel=${panel}` as `/app?${string}`);

	const explorationThemes = [
		{
			name: 'Makan & ngopi',
			category: 'ISTIRAHAT',
			description: 'Cari tempat singgah setelah turun dari transit.',
			art: 'coffee'
		},
		{
			name: 'Taman & ruang kota',
			category: 'JALAN SANTAI',
			description: 'Temukan alasan untuk berhenti sejenak dan menikmati kota.',
			art: 'park'
		},
		{
			name: 'Seni & budaya',
			category: 'JELAJAH',
			description: 'Lihat koleksi tempat yang terhubung dengan perjalananmu.',
			art: 'museum'
		}
	];

	let mobileMenuOpen = $state(false);
	let headerSolid = $state(false);
	let heroWorld: HTMLDivElement;
	let heroVideo: HTMLVideoElement;

	onMount(() => {
		let scrollFrame = 0;
		let loopTimer: number | undefined;
		let loopFading = false;
		const syncHeader = () => {
			if (scrollFrame) return;
			scrollFrame = requestAnimationFrame(() => {
				const next = window.scrollY > 48;
				if (next !== headerSolid) headerSolid = next;
				scrollFrame = 0;
			});
		};
		const motionPreference = window.matchMedia('(prefers-reduced-motion: reduce)');
		const syncMotion = () => {
			if (motionPreference.matches) heroVideo.pause();
			else heroVideo.play().catch(() => {});
		};
		const markVideoReady = () => {
			heroWorld.style.setProperty('--hero-video-opacity', '1');
		};
		const softenVideoLoop = () => {
			if (
				loopFading ||
				motionPreference.matches ||
				(!heroVideo.ended &&
					(!Number.isFinite(heroVideo.duration) ||
						heroVideo.currentTime < heroVideo.duration - 0.36))
			)
				return;

			loopFading = true;
			heroWorld.style.setProperty('--hero-video-opacity', '0');
			loopTimer = window.setTimeout(() => {
				if (motionPreference.matches) {
					heroWorld.style.setProperty('--hero-video-opacity', '1');
					loopFading = false;
					return;
				}
				heroVideo.currentTime = 0;
				void heroVideo.play().catch(() => {});
				window.requestAnimationFrame(() => {
					heroWorld.style.setProperty('--hero-video-opacity', '1');
					loopFading = false;
				});
			}, 360);
		};
		syncHeader();
		syncMotion();
		heroVideo.addEventListener('canplay', markVideoReady);
		heroVideo.addEventListener('loadeddata', markVideoReady);
		heroVideo.addEventListener('timeupdate', softenVideoLoop);
		heroVideo.addEventListener('ended', softenVideoLoop);
		if (heroVideo.readyState >= HTMLMediaElement.HAVE_FUTURE_DATA) markVideoReady();
		window.addEventListener('scroll', syncHeader, { passive: true });
		motionPreference.addEventListener('change', syncMotion);
		return () => {
			if (scrollFrame) cancelAnimationFrame(scrollFrame);
			if (loopTimer) window.clearTimeout(loopTimer);
			heroVideo.removeEventListener('canplay', markVideoReady);
			heroVideo.removeEventListener('loadeddata', markVideoReady);
			heroVideo.removeEventListener('timeupdate', softenVideoLoop);
			heroVideo.removeEventListener('ended', softenVideoLoop);
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
			{ threshold: 0.12 }
		);
		observer.observe(node);
		return { destroy: () => observer.disconnect() };
	}
	const featureCards = [
		{
			title: 'Rute anti-nyasar',
			description: 'Rute multimoda yang realistis, dengan waktu jalan dan transit yang jelas.',
			link: appPanel('plan'),
			linkLabel: 'Cari rute'
		},
		{
			title: 'Jadwal yang jelas sumbernya',
			description: 'Lihat jadwal keberangkatan dan ketahui kapan datanya diperbarui.',
			link: appPanel('journey'),
			linkLabel: 'Lihat perjalanan'
		},
		{
			title: 'City Explorer',
			description: 'Temukan tempat dekat stasiun: makan, ngopi, taman, museum, sampai hidden gem.',
			link: appPanel('explore'),
			linkLabel: 'Jelajah kota'
		},
		{
			title: 'Transit Passport',
			description: 'Tandai stasiun yang kamu datangi, ikuti rute, lalu simpan ceritanya.',
			link: appPanel('passport'),
			linkLabel: 'Buka Passport'
		}
	];
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

<div class="landing">
	<header class="site-header" class:site-header--solid={headerSolid}>
		<div class="site-header__inner">
			<a class="brand" href="#top" aria-label="Singgah, kembali ke beranda">
				<span>Singgah</span>
			</a>

			<nav class="desktop-nav" aria-label="Navigasi halaman">
				<a href="#top">Beranda</a>
				<a href="#features">Fitur</a>
				<a href="#city-explorer">Jelajah Kota</a>
				<a href="#passport">Transit Passport</a>
				<a href="#about">Tentang</a>
			</nav>

			<a class="header-cta" href={resolve('/app')}
				>Buka Aplikasi <span aria-hidden="true">↗</span></a
			>

			<details class="mobile-menu" bind:open={mobileMenuOpen}>
				<summary aria-label="Buka menu navigasi">
					<span>Menu</span>
					<svg viewBox="0 0 24 24" aria-hidden="true">
						<path d="M4 7h16M4 12h16M4 17h16" />
					</svg>
				</summary>
				<nav aria-label="Navigasi halaman">
					<a href="#top" onclick={() => (mobileMenuOpen = false)}>Beranda</a>
					<a href="#features" onclick={() => (mobileMenuOpen = false)}>Fitur</a>
					<a href="#city-explorer" onclick={() => (mobileMenuOpen = false)}>Jelajah Kota</a>
					<a href="#passport" onclick={() => (mobileMenuOpen = false)}>Transit Passport</a>
					<a href="#about" onclick={() => (mobileMenuOpen = false)}>Tentang</a>
					<a class="mobile-menu__cta" href={resolve('/app')}>Buka Aplikasi</a>
				</nav>
			</details>
		</div>
	</header>

	<section class="hero" id="top" aria-labelledby="hero-title">
		<div class="hero__world" bind:this={heroWorld} aria-hidden="true">
			<img
				src="/illustrations/singgah-pixel-world-loop-poster.webp"
				alt=""
				width="1280"
				height="720"
				fetchpriority="high"
				decoding="async"
			/>
			<video
				bind:this={heroVideo}
				autoplay
				muted
				playsinline
				preload="metadata"
				poster="/illustrations/singgah-pixel-world-loop-poster.webp"
				tabindex="-1"
			>
				<source src="/illustrations/singgah-pixel-world-loop.webm" type="video/webm" />
				<source src="/illustrations/singgah-pixel-world-loop.mp4" type="video/mp4" />
			</video>
		</div>
		<div class="hero__content container">
			<div class="hero__copy">
				<p class="eyebrow"><span class="eyebrow__dot"></span> JELAJAH TRANSIT · JAKARTA</p>
				<h1 id="hero-title">
					<span>Pergi boleh spontan.</span>
					<span>Rute jangan.</span>
				</h1>
				<p class="hero__description">
					Rencanakan perjalanan, temukan tempat dekat transit, dan nikmati kotanya dengan lebih
					tenang.
				</p>
				<div class="hero__actions">
					<a class="action action--primary" href={appPanel('plan')}>
						Cari Jalan <span aria-hidden="true">↗</span>
					</a>
					<a class="action action--secondary" href="#features"
						>Jelajahi Singgah <span aria-hidden="true">↓</span></a
					>
				</div>
			</div>
			<div class="hero__chips" aria-hidden="true">
				<div class="hero-chip hero-chip--one">
					<span>01</span><strong>Pilih tujuan</strong><small>Mulai dari mana saja</small>
				</div>
				<div class="hero-chip hero-chip--two">
					<span>02</span><strong>Temukan rute</strong><small>Transit + jalan kaki</small>
				</div>
				<div class="hero-chip hero-chip--three">
					<span>03</span><strong>Singgah sebentar</strong><small>Kota punya banyak cerita</small>
				</div>
			</div>
			<a class="hero__scroll" href="#features"
				>Gulir untuk melihat <span aria-hidden="true">↓</span></a
			>
		</div>
	</section>

	<section
		class="section section--features"
		id="features"
		aria-labelledby="features-title"
		use:reveal
	>
		<div class="container">
			<div class="section-heading section-heading--center">
				<div>
					<p class="eyebrow">BERGERAK · SINGGAH · JELAJAH</p>
					<h2 id="features-title">Kota terasa lebih dekat<br />saat jalannya jelas.</h2>
				</div>
				<p>
					Satu tempat untuk merencanakan perjalanan, mengikuti informasi transit, dan menemukan
					alasan untuk singgah.
				</p>
			</div>

			<div class="product-window" aria-label="Pintu masuk ke perencana perjalanan Singgah">
				<div class="product-window__bar">
					<span class="product-window__brand">Singgah</span><span>Mulai perjalanan</span><span
						aria-hidden="true">↗</span
					>
				</div>
				<div class="product-window__body">
					<div class="product-window__panel">
						<p class="product-window__label">RENCANAKAN PERJALANAN</p>
						<h3>Ke mana hari ini?</h3>
						<a class="product-window__field" href={appPanel('plan')}>
							<span aria-hidden="true">○</span> Titik berangkat
						</a>
						<a class="product-window__field" href={appPanel('plan')}>
							<span aria-hidden="true">◇</span> Tujuan perjalanan
						</a>
						<a class="product-window__button" href={appPanel('plan')}
							>Temukan rute <span aria-hidden="true">↗</span></a
						>
						<p>Bandingkan pilihan transit dan langkah kaki dalam satu tampilan.</p>
					</div>
					<div class="product-window__scene" aria-hidden="true">
						<div class="product-window__route"></div>
						<span class="product-window__stop product-window__stop--one"></span>
						<span class="product-window__stop product-window__stop--two"></span>
						<span class="product-window__stop product-window__stop--three"></span>
						<div class="product-window__note">
							<strong>Jelajahi dari transit</strong><span>Makan · taman · budaya</span>
						</div>
					</div>
				</div>
			</div>

			<div class="feature-list">
				{#each featureCards as feature, index (feature.title)}
					<a href={feature.link} class="feature-row">
						<span class="feature-row__number">0{index + 1}</span>
						<span class="feature-row__copy"
							><strong>{feature.title}</strong><span>{feature.description}</span></span
						>
						<span class="feature-row__link"
							>{feature.linkLabel} <span aria-hidden="true">↗</span></span
						>
					</a>
				{/each}
			</div>
		</div>
	</section>

	<section class="section how-section" aria-labelledby="how-title" use:reveal>
		<div class="container">
			<div class="section-heading section-heading--center">
				<div>
					<p class="eyebrow">DARI BERANGKAT SAMPAI SINGGAH</p>
					<h2 id="how-title">Cara Kerja</h2>
				</div>
				<p>Cuma 4 langkah, jalan-jalan jadi lebih terarah.</p>
			</div>
			<ol class="steps">
				<li>
					<span class="step-number">01</span>
					<h3>Cari tujuan</h3>
					<p>Masukkan lokasi yang ingin kamu tuju.</p>
				</li>
				<li>
					<span class="step-number">02</span>
					<h3>Pilih rute</h3>
					<p>Bandingkan waktu, jalan kaki, dan jumlah transit.</p>
				</li>
				<li>
					<span class="step-number">03</span>
					<h3>Ikuti perjalanan</h3>
					<p>Lihat informasi yang kamu butuhkan selama di jalan.</p>
				</li>
				<li>
					<span class="step-number">04</span>
					<h3>Sampai &amp; jelajah</h3>
					<p>Turun, cari tempat menarik di sekitar transit.</p>
				</li>
			</ol>
		</div>
	</section>

	<section
		class="section explorer-section"
		id="city-explorer"
		aria-labelledby="explorer-title"
		use:reveal
	>
		<div class="container">
			<div class="section-heading">
				<div>
					<p class="eyebrow">BOLEH SINGGAH SEBENTAR</p>
					<h2 id="explorer-title">Jelajah Kota dari Transit</h2>
				</div>
				<p>Tempat seru di sekitar stasiun, biar perjalananmu nggak cuma soal sampai.</p>
			</div>

			<div class="place-grid">
				{#each explorationThemes as place (place.name)}
					<a class="place-card" href={appPanel('explore')}>
						<div
							class="place-art"
							class:place-art--coffee={place.art === 'coffee'}
							class:place-art--park={place.art === 'park'}
							class:place-art--museum={place.art === 'museum'}
							aria-hidden="true"
						>
							<div class="place-art__block place-art__block--one"></div>
							<div class="place-art__block place-art__block--two"></div>
							<div class="place-art__sun"></div>
							<svg viewBox="0 0 64 64" fill="none">
								{#if place.art === 'coffee'}
									<path
										d="M16 23h27v20a9 9 0 0 1-9 9h-9a9 9 0 0 1-9-9V23Zm27 5h5a6 6 0 0 1 0 12h-5M22 16c-2-3 2-4 0-7m10 7c-2-3 2-4 0-7m10 7c-2-3 2-4 0-7"
									/>
								{:else if place.art === 'park'}
									<path d="M32 8 15 31h10L13 45h16v9h6v-9h16L39 31h10L32 8Z" />
								{:else}
									<path
										d="M10 52h44M15 52V23h34v29M10 23 32 11l22 12M22 31h5v7h-5zm15 0h5v7h-5zM29 52V40h7v12"
									/>
								{/if}
							</svg>
							<span class="place-art__pin"><span></span></span>
						</div>
						<div class="place-card__body">
							<p class="place-category">{place.category}</p>
							<h3>{place.name}</h3>
							<p class="place-description">{place.description}</p>
							<span class="place-card__action"
								>Jelajahi koleksi <span aria-hidden="true">↗</span></span
							>
						</div>
					</a>
				{/each}
			</div>

			<a class="text-link explorer-link" href={appPanel('explore')}>
				Lihat City Explorer <span aria-hidden="true">↗</span>
			</a>
		</div>
	</section>

	<section
		class="section passport-section"
		id="passport"
		aria-labelledby="passport-title"
		use:reveal
	>
		<div class="container">
			<div class="section-heading">
				<div>
					<p class="eyebrow">CERITA YANG IKUT PULANG</p>
					<h2 id="passport-title">Transit Passport &amp; Journal</h2>
				</div>
				<p>Setiap perjalanan punya cerita. Kumpulkan, jelajahi, dan simpan semuanya.</p>
			</div>

			<div class="passport-grid">
				<Surface class="passport-preview">
					<div class="preview-caption">
						<span>TRANSIT PASSPORT</span><span>KENANGAN PERJALANAN</span>
					</div>
					<div class="passport-preview__head">
						<div>
							<p class="eyebrow">TRANSIT PASSPORT</p>
							<h3>Stasiun yang pernah kamu singgahi.</h3>
							<p>Tandai kunjungan dan simpan catatanmu.</p>
						</div>
						<img
							src="/stickers/sticker-checkpoint-masuk.webp"
							alt=""
							width="180"
							height="180"
							loading="lazy"
						/>
					</div>
					<div class="passport-progress">
						<div><strong>Catat dengan caramu</strong><span>di Paspor</span></div>
						<div class="passport-stamps" aria-label="Langkah di Paspor">
							<span>Pilih stasiun</span><span>Tandai kunjungan</span><span>Tulis cerita</span>
						</div>
					</div>
				</Surface>

				<Surface class="journal-preview">
					<div class="journal-preview__top">
						<span class="journal-mark" aria-hidden="true"></span>
						<span>JURNAL PRIBADI</span>
					</div>
					<p class="eyebrow">SETELAH PERJALANAN</p>
					<h3>Yang ingin kamu ingat.</h3>
					<p class="journal-quote">Cerita kecil dari setiap stasiun bisa kamu simpan di sini.</p>
					<div class="journal-preview__footer">
						<span>Catatan pribadi</span>
						<a href={appPanel('passport')}>Lihat Passport <span aria-hidden="true">↗</span></a>
					</div>
				</Surface>
			</div>
		</div>
	</section>

	<section class="section trail-section" aria-labelledby="trail-title" use:reveal>
		<div class="trail-layout container">
			<div class="trail-copy">
				<p class="eyebrow">DUA CARA MELIHAT KOTA</p>
				<h2 id="trail-title">Pilih peta yang cocok dengan langkahmu.</h2>
				<p>
					Lihat posisi stasiun di peta geografis, lalu pindah ke peta integrasi untuk memahami
					hubungan antarjalur.
				</p>
				<a class="text-link" href={resolve('/app')}>
					Buka peta <span aria-hidden="true">↗</span>
				</a>
			</div>
			<div class="trail-map">
				<div class="trail-map__heading">
					<div>
						<span class="trail-map__kicker">PETA SINGGAH</span>
						<h3>Dari lokasi ke koneksi.</h3>
					</div>
					<img
						src="/stickers/sticker-side-quest.webp"
						alt=""
						width="150"
						height="150"
						loading="lazy"
					/>
				</div>
				<ol class="trail-stops">
					<li>
						<span class="trail-stop__node"></span><strong>Peta geografis</strong><small
							>Letak di kota</small
						>
					</li>
					<li>
						<span class="trail-stop__node"></span><strong>Peta integrasi</strong><small
							>Hubungan jalur</small
						>
					</li>
					<li>
						<span class="trail-stop__node"></span><strong>Detail stasiun</strong><small
							>Jadwal &amp; sekitar</small
						>
					</li>
				</ol>
			</div>
		</div>
	</section>

	<section class="section final-section" aria-labelledby="final-title" use:reveal>
		<div class="final-cta">
			<div class="final-cta__copy">
				<p class="eyebrow">SATU KOTA. BANYAK CERITA.</p>
				<h2 id="final-title">Untuk yang suka spontan,<br />tapi tetap pengen sampai.</h2>
				<p>Jelajahi Jakarta dengan perjalanan yang lebih masuk akal.</p>
				<div class="hero__actions">
					<a class="action action--light" href={appPanel('plan')}
						>Cari Jalan <span aria-hidden="true">↗</span></a
					>
					<a class="action action--outline-light" href={resolve('/app')}>Buka Aplikasi</a>
				</div>
			</div>
			<div class="final-cta__art" aria-hidden="true">
				<img
					src="/illustrations/singgah-pixel-street-v2.webp"
					alt=""
					width="1672"
					height="941"
					loading="lazy"
				/>
				<div class="final-cta__app">
					<strong>Ke mana hari ini?</strong><span>Mulai perjalananmu dari sini.</span><span
						class="final-cta__app-line">○ &nbsp; Titik berangkat</span
					><span class="final-cta__app-line">◇ &nbsp; Tujuan perjalanan</span><span
						class="final-cta__app-line">Temukan rute &nbsp; ↗</span
					>
				</div>
			</div>
		</div>
	</section>

	<footer class="site-footer" id="about">
		<div class="footer-grid container">
			<div class="footer-brand">
				<a class="brand" href="#top">
					<img src="/brand/app-icon-blue.svg" alt="" width="36" height="36" loading="lazy" />
					<span>Singgah</span>
				</a>
				<p>Jelajah transit Jakarta</p>
				<p class="footer-note">Kota lebih dekat dengan cerita baik.</p>
			</div>
			<div>
				<h2>Produk</h2>
				<a href="#features">Fitur</a>
				<a href="#passport">Transit Passport</a>
				<a href="#city-explorer">Jelajah Kota</a>
			</div>
			<div>
				<h2>Proyek</h2>
				<a href="#about">Tentang</a>
				<a href="https://github.com/rendr17/Singgah" target="_blank" rel="noreferrer">GitHub</a>
				<a href="https://github.com/rendr17/Singgah" target="_blank" rel="noreferrer">Open Source</a
				>
			</div>
			<div>
				<h2>Sumber</h2>
				<a href={resolve('/providers')}>Penyedia data &amp; atribusi</a>
			</div>
		</div>
		<div class="footer-bottom container">
			<span>Singgah · Jakarta</span><a href="#top">Kembali ke atas ↑</a>
		</div>
	</footer>
</div>

<style>
	:global(html) {
		scroll-padding-top: 5rem;
	}

	.landing {
		--landing-heading: var(--sg-font-display);
		--landing-label: var(--sg-font-label);
		color: var(--sg-text);
		background: var(--sg-canvas);
		font-family: var(--sg-font-body);
		overflow: clip;
	}

	.container {
		width: min(calc(100% - 2.5rem), 82rem);
		margin-inline: auto;
	}

	h1,
	h2,
	h3 {
		font-family: var(--landing-heading);
		letter-spacing: -0.035em;
	}

	.site-header {
		position: fixed;
		top: 0;
		left: 0;
		right: 0;
		z-index: 20;
		color: white;
		background: transparent;
		border-bottom: 1px solid transparent;
		transition:
			background-color 280ms ease,
			color 280ms ease,
			border-color 280ms ease;
	}

	.site-header--solid {
		color: var(--sg-text);
		background: var(--sg-canvas);
		border-bottom-color: var(--sg-border);
	}

	.site-header__inner {
		width: min(calc(100% - 2.5rem), 82rem);
		min-height: 5.65rem;
		margin-inline: auto;
		display: grid;
		grid-template-columns: auto 1fr auto;
		align-items: center;
		gap: clamp(1rem, 3vw, 3rem);
	}

	.brand {
		display: inline-flex;
		align-items: center;
		gap: 0.55rem;
		color: inherit;
		font-family: var(--landing-heading);
		font-size: 1.25rem;
		font-weight: 600;
		text-decoration: none;
	}

	.brand img {
		width: 2.25rem;
		height: 2.25rem;
		border-radius: 0.55rem;
	}

	.desktop-nav {
		display: flex;
		align-items: center;
		justify-content: center;
		gap: clamp(0.75rem, 2vw, 1.8rem);
	}

	.desktop-nav a,
	.mobile-menu nav a {
		min-height: 2.75rem;
		display: inline-flex;
		align-items: center;
		color: inherit;
		font-size: 0.925rem;
		font-weight: 500;
		text-decoration: none;
		transition: color var(--sg-motion-fast) var(--sg-ease-standard);
	}

	.desktop-nav a:hover,
	.mobile-menu nav a:hover {
		opacity: 0.68;
	}

	.header-cta,
	.mobile-menu__cta {
		min-height: 2.75rem;
		padding: 0 1rem;
		display: inline-flex;
		align-items: center;
		justify-content: center;
		gap: 0.6rem;
		border: 1px solid currentColor;
		border-radius: var(--sg-radius-button);
		color: inherit;
		font-weight: 500;
		text-decoration: none;
		transition:
			background-color var(--sg-motion-fast) var(--sg-ease-standard),
			color var(--sg-motion-fast) var(--sg-ease-standard);
	}

	.header-cta:hover,
	.mobile-menu__cta:hover {
		background: white;
		color: var(--sg-text) !important;
	}

	.site-header--solid .header-cta {
		border-color: var(--sg-text);
		color: var(--sg-text);
	}

	.site-header--solid .header-cta:hover {
		background: var(--sg-text);
		color: white !important;
	}

	.mobile-menu {
		display: none;
		position: relative;
	}

	.mobile-menu summary {
		min-height: 2.75rem;
		padding: 0 0.75rem;
		list-style: none;
		display: inline-flex;
		align-items: center;
		gap: 0.55rem;
		border: 1px solid currentColor;
		border-radius: var(--sg-radius-button);
		font-weight: 500;
		cursor: pointer;
	}

	.mobile-menu summary::-webkit-details-marker {
		display: none;
	}

	.mobile-menu summary svg {
		width: 1.2rem;
		height: 1.2rem;
		fill: none;
		stroke: currentColor;
		stroke-width: 1.8;
		stroke-linecap: round;
	}

	.mobile-menu nav {
		position: absolute;
		top: calc(100% + 0.5rem);
		right: 0;
		width: min(18rem, calc(100vw - 2rem));
		padding: 0.75rem;
		display: grid;
		gap: 0.15rem;
		border: 1px solid var(--sg-border);
		border-radius: var(--sg-radius-card);
		background: var(--sg-surface);
		box-shadow: var(--sg-shadow-overlay);
	}

	.mobile-menu nav a {
		color: var(--sg-text);
		padding-inline: 0.75rem;
		border-radius: var(--sg-radius-button);
	}

	.mobile-menu__cta {
		margin-top: 0.35rem;
		color: var(--sg-text) !important;
	}

	.hero {
		min-height: max(100svh, 43rem);
		position: relative;
		isolation: isolate;
		overflow: hidden;
		background: var(--sg-hero);
		color: white;
	}

	.hero__world,
	.hero__world::after {
		position: absolute;
		inset: 0;
	}

	.hero__world {
		z-index: -1;
		contain: paint;
		transform: translateZ(0);
	}

	.hero__world img,
	.hero__world video {
		width: 100%;
		height: 100%;
		display: block;
		object-fit: cover;
		object-position: center 53%;
		backface-visibility: hidden;
	}

	.hero__world video {
		position: absolute;
		inset: 0;
		opacity: var(--hero-video-opacity, 0);
		transition: opacity 360ms var(--sg-ease-standard);
		will-change: opacity;
	}

	.hero__world::after {
		background:
			linear-gradient(
				90deg,
				color-mix(in srgb, var(--sg-hero-deep) 52%, transparent) 0%,
				color-mix(in srgb, var(--sg-hero-deep) 31%, transparent) 37%,
				transparent 70%
			),
			linear-gradient(
				0deg,
				color-mix(in srgb, var(--sg-hero-deep) 32%, transparent),
				transparent 28%
			);
		content: '';
	}

	.hero__content {
		min-height: inherit;
		position: relative;
		padding-block: clamp(8.25rem, 17vh, 11rem) 4rem;
	}

	.hero__copy {
		max-width: 36rem;
	}

	.eyebrow {
		margin: 0 0 0.7rem;
		color: var(--sg-brand);
		font-family: var(--landing-label);
		font-size: 0.68rem;
		font-weight: 700;
		letter-spacing: 0.1em;
		line-height: 1.4;
		text-transform: uppercase;
	}

	/* Cofounder-inspired chapter labels are compact and monospaced; the
	   headline and body remain human-readable sans-serif. */
	.product-window__label,
	.place-category,
	.preview-caption,
	.journal-preview__top,
	.trail-map__kicker {
		font-family: var(--landing-label);
	}

	.eyebrow__dot {
		width: 0.5rem;
		height: 0.5rem;
		margin-right: 0.4rem;
		display: inline-block;
		border-radius: 50%;
		background: var(--sg-warm);
	}

	.hero h1 {
		max-width: 13ch;
		margin: 0;
		font-size: clamp(3rem, 4vw, 3.65rem);
		font-weight: 400;
		line-height: 1.08;
		letter-spacing: -0.045em;
	}

	.hero h1 span {
		display: block;
	}

	.hero .eyebrow {
		margin-bottom: 1.15rem;
		color: white;
		font-weight: 500;
	}

	.hero .eyebrow__dot {
		background: var(--sg-warm);
	}

	.hero__description {
		max-width: 30rem;
		margin: 1.2rem 0 0;
		color: color-mix(in srgb, var(--sg-brand-contrast) 94%, transparent);
		font-size: clamp(1rem, 1.3vw, 1.1rem);
		line-height: 1.5;
	}

	.hero__actions {
		margin-top: 1.5rem;
		display: flex;
		align-items: center;
		flex-wrap: wrap;
		gap: 0.75rem;
	}

	.action {
		min-height: 2.9rem;
		padding: 0 1.25rem;
		display: inline-flex;
		align-items: center;
		justify-content: center;
		gap: 0.75rem;
		border: 1px solid transparent;
		border-radius: var(--sg-radius-button);
		font-weight: 500;
		text-decoration: none;
		transition:
			background-color var(--sg-motion-fast) var(--sg-ease-standard),
			color var(--sg-motion-fast) var(--sg-ease-standard),
			transform var(--sg-motion-fast) var(--sg-ease-standard);
	}

	.action:hover {
		transform: translateY(-1px);
	}

	.action--primary {
		background: var(--sg-warm);
		color: var(--sg-text);
	}

	.action--primary:hover {
		background: white;
	}

	.action--secondary {
		border-color: color-mix(in srgb, var(--sg-brand-contrast) 75%, transparent);
		background: color-mix(in srgb, var(--sg-brand-contrast) 12%, transparent);
		color: white;
	}

	.action--secondary:hover {
		background: color-mix(in srgb, var(--sg-brand-contrast) 22%, transparent);
	}

	.hero__chips {
		position: absolute;
		inset: 0;
		pointer-events: none;
	}

	.hero-chip {
		width: 14rem;
		padding: 0.9rem 1rem;
		position: absolute;
		display: grid;
		grid-template-columns: auto 1fr;
		column-gap: 0.7rem;
		align-items: center;
		border: 1px solid color-mix(in srgb, var(--sg-brand-contrast) 55%, transparent);
		border-radius: 0.7rem;
		background: color-mix(in srgb, var(--sg-brand-contrast) 70%, transparent);
		box-shadow: 0 0.7rem 2.5rem color-mix(in srgb, var(--sg-hero-deep) 12%, transparent);
		backdrop-filter: blur(12px);
		color: var(--sg-text);
	}

	.hero-chip > span {
		width: 1.9rem;
		height: 1.9rem;
		grid-row: span 2;
		display: grid;
		place-items: center;
		border-radius: 0.4rem;
		background: var(--sg-warm);
		font-size: 0.7rem;
	}

	.hero-chip strong {
		font-size: 0.84rem;
		font-weight: 600;
	}

	.hero-chip small {
		color: var(--sg-text-muted);
		font-size: 0.68rem;
	}

	.hero-chip--one {
		top: 32%;
		left: 58%;
	}
	.hero-chip--two {
		top: 52%;
		left: 69%;
	}
	.hero-chip--three {
		top: 69%;
		left: 50%;
	}

	.hero__scroll {
		position: absolute;
		bottom: 2rem;
		left: 0;
		display: inline-flex;
		gap: 0.75rem;
		color: white;
		font-size: 0.78rem;
		text-decoration: none;
	}

	.hero__scroll span {
		font-size: 1rem;
	}

	.hero::after {
		position: absolute;
		bottom: -1px;
		left: 0;
		right: 0;
		height: 4rem;
		background: linear-gradient(transparent, var(--sg-canvas));
		pointer-events: none;
		content: '';
	}

	.section {
		padding-block: clamp(5rem, 8vw, 7.5rem);
	}

	.section--features {
		background: var(--sg-canvas);
	}

	.section-heading {
		margin-bottom: 3.5rem;
		display: flex;
		align-items: end;
		justify-content: space-between;
		gap: 2rem;
	}

	.section-heading h2,
	.trail-copy h2 {
		max-width: 18ch;
		margin: 0;
		font-size: clamp(2rem, 3.4vw, 3.25rem);
		font-weight: 400;
		line-height: 1.13;
	}

	.section-heading > p,
	.trail-copy > p:not(.eyebrow) {
		max-width: 35rem;
		margin: 0;
		color: var(--sg-text-muted);
		font-size: 1.05rem;
		line-height: 1.6;
	}

	.product-window {
		margin: 0 auto clamp(5rem, 8vw, 8rem);
		overflow: hidden;
		border: 1px solid var(--sg-border);
		border-radius: 0.55rem;
		background: white;
		box-shadow: 0 2rem 5rem color-mix(in srgb, var(--sg-text) 8%, transparent);
	}

	.product-window__bar {
		min-height: 3.5rem;
		padding-inline: 1.5rem;
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 1rem;
		border-bottom: 1px solid var(--sg-border);
		color: var(--sg-text-muted);
		font-size: 0.75rem;
	}

	.product-window__brand {
		color: var(--sg-text);
		font-size: 1rem;
		font-weight: 600;
	}

	.product-window__body {
		min-height: 34rem;
		display: grid;
		grid-template-columns: minmax(18rem, 0.39fr) minmax(0, 0.61fr);
	}

	.product-window__panel {
		padding: clamp(1.5rem, 4vw, 3.5rem);
		border-right: 1px solid var(--sg-border);
	}

	.product-window__label {
		margin: 0;
		color: var(--sg-brand);
		font-size: 0.68rem;
		font-weight: 600;
		letter-spacing: 0.1em;
	}

	.product-window__panel h3 {
		margin: 1.2rem 0 2.5rem;
		font-size: clamp(2rem, 3vw, 2.8rem);
		font-weight: 400;
	}

	.product-window__field {
		min-height: 3.5rem;
		margin-bottom: 0.65rem;
		padding: 0 1rem;
		display: flex;
		align-items: center;
		gap: 0.75rem;
		border: 1px solid var(--sg-border);
		border-radius: 0.4rem;
		color: var(--sg-text-muted);
		font-size: 0.92rem;
		text-decoration: none;
	}

	.product-window__field span {
		color: var(--sg-brand);
		font-size: 1.3rem;
	}

	.product-window__field:hover {
		border-color: var(--sg-brand);
		color: var(--sg-text);
	}

	.product-window__button {
		min-height: 3.3rem;
		margin-top: 1.1rem;
		padding-inline: 1rem;
		display: flex;
		align-items: center;
		justify-content: space-between;
		border-radius: 0.4rem;
		background: var(--sg-text);
		color: white;
		font-size: 0.9rem;
		text-decoration: none;
	}

	.product-window__button:hover {
		background: var(--sg-brand);
	}

	.product-window__panel > p:last-child {
		max-width: 19rem;
		margin: 1.5rem 0 0;
		color: var(--sg-text-muted);
		font-size: 0.88rem;
		line-height: 1.55;
	}

	.product-window__scene {
		position: relative;
		overflow: hidden;
		background-color: var(--sg-scene-sky);
		background-image:
			linear-gradient(
				90deg,
				transparent 48%,
				color-mix(in srgb, var(--sg-brand-contrast) 85%, transparent) 48%,
				color-mix(in srgb, var(--sg-brand-contrast) 85%, transparent) 51%,
				transparent 51%
			),
			linear-gradient(
				transparent 48%,
				color-mix(in srgb, var(--sg-brand-contrast) 85%, transparent) 48%,
				color-mix(in srgb, var(--sg-brand-contrast) 85%, transparent) 51%,
				transparent 51%
			);
		background-size: 5.5rem 5.5rem;
	}

	.product-window__scene::before {
		width: 32rem;
		height: 23rem;
		position: absolute;
		top: -8rem;
		right: -10rem;
		border-radius: 43%;
		background: var(--sg-scene-leaf);
		transform: rotate(-20deg);
		content: '';
	}

	.product-window__route {
		width: 55%;
		height: 54%;
		position: absolute;
		top: 24%;
		left: 18%;
		border: 0.65rem solid var(--sg-scene-route);
		border-left: 0;
		border-radius: 0 6rem 6rem 0;
		transform: rotate(-13deg);
	}

	.product-window__stop {
		width: 1.5rem;
		height: 1.5rem;
		position: absolute;
		z-index: 1;
		border: 0.4rem solid var(--sg-scene-route);
		border-radius: 50%;
		background: white;
		box-shadow: 0 0 0 0.4rem color-mix(in srgb, var(--sg-brand-contrast) 65%, transparent);
	}

	.product-window__stop--one {
		top: 60%;
		left: 17%;
	}
	.product-window__stop--two {
		top: 24%;
		left: 54%;
	}
	.product-window__stop--three {
		top: 70%;
		left: 68%;
	}

	.product-window__note {
		min-width: 12rem;
		padding: 0.9rem 1rem;
		position: absolute;
		right: 8%;
		bottom: 11%;
		display: grid;
		gap: 0.2rem;
		border: 1px solid var(--sg-border);
		border-radius: 0.4rem;
		background: white;
		box-shadow: 0 0.8rem 2rem color-mix(in srgb, var(--sg-text) 10%, transparent);
		font-size: 0.78rem;
	}

	.product-window__note span {
		color: var(--sg-text-muted);
	}

	.feature-list {
		border-bottom: 1px solid var(--sg-border);
	}

	.feature-row {
		min-height: 8rem;
		padding: 1.6rem 0.5rem;
		display: grid;
		grid-template-columns: 3rem minmax(0, 1fr) auto;
		align-items: center;
		gap: 1.5rem;
		border-top: 1px solid var(--sg-border);
		color: var(--sg-text);
		text-decoration: none;
		transition:
			background-color 240ms ease,
			box-shadow 240ms ease;
	}

	.feature-row:hover {
		background: var(--sg-surface);
		box-shadow: inset 3px 0 var(--sg-brand);
	}
	.feature-row__number {
		align-self: start;
		color: var(--sg-text-muted);
		font-size: 0.85rem;
	}
	.feature-row__copy {
		display: grid;
		gap: 0.45rem;
	}
	.feature-row__copy strong {
		font-size: clamp(1.45rem, 2vw, 2rem);
		font-weight: 400;
		letter-spacing: -0.03em;
	}
	.feature-row__copy > span {
		max-width: 43rem;
		color: var(--sg-text-muted);
		line-height: 1.5;
	}
	.feature-row__link {
		color: var(--sg-brand);
		white-space: nowrap;
		font-size: 0.85rem;
	}

	.text-link {
		min-height: 2.75rem;
		margin-top: auto;
		display: inline-flex;
		align-items: center;
		gap: 0.55rem;
		color: var(--sg-brand);
		font-weight: 700;
		text-decoration: none;
	}

	.text-link span {
		color: var(--sg-warm);
		transition: transform var(--sg-motion-fast) var(--sg-ease-standard);
	}

	.text-link:hover span {
		transform: translate(2px, -2px);
	}

	.how-section {
		background: var(--sg-canvas);
	}

	.section-heading--center {
		align-items: center;
		flex-direction: column;
		gap: 1rem;
		text-align: center;
	}

	.steps {
		position: relative;
		margin: 0;
		padding: 0;
		display: grid;
		grid-template-columns: repeat(4, minmax(0, 1fr));
		list-style: none;
	}

	.steps::before {
		position: absolute;
		top: 1.3rem;
		left: 11%;
		right: 11%;
		border-top: 1px dashed var(--sg-border);
		content: '';
	}

	.steps li {
		position: relative;
		padding: 0 1.5rem 0 0;
	}

	.step-number {
		width: 2.65rem;
		height: 2.65rem;
		margin-bottom: 1rem;
		position: relative;
		z-index: 1;
		display: grid;
		place-items: center;
		border: 1px solid var(--sg-brand);
		border-radius: 50%;
		background: var(--sg-canvas);
		color: var(--sg-brand);
		font-size: 0.82rem;
		font-weight: 700;
	}

	.steps li h3 {
		margin: 0;
		font-size: 1.2rem;
	}

	.steps li p {
		max-width: 16rem;
		margin: 0.5rem 0 0;
		color: var(--sg-text-muted);
		line-height: 1.5;
	}

	.explorer-section {
		background: var(--sg-canvas);
	}

	.place-grid {
		display: grid;
		grid-template-columns: repeat(3, minmax(0, 1fr));
		gap: 1rem;
	}

	.place-card {
		display: block;
		padding: 0;
		overflow: hidden;
		border: 1px solid var(--sg-border);
		border-radius: var(--sg-radius-card);
		color: var(--sg-text);
		text-decoration: none;
		transition: border-color var(--sg-motion-fast) var(--sg-ease-standard);
	}

	.place-card:hover {
		border-color: var(--sg-brand);
	}

	.place-art {
		min-height: 12rem;
		aspect-ratio: 1.7;
		position: relative;
		display: grid;
		place-items: center;
		overflow: hidden;
		background: var(--sg-scene-sand);
	}

	.place-art--park {
		background: var(--sg-scene-park);
	}

	.place-art--museum {
		background: var(--sg-scene-museum);
	}

	.place-art__block {
		position: absolute;
		bottom: 0;
		width: 17%;
		background: color-mix(in srgb, var(--sg-brand) 14%, transparent);
	}

	.place-art__block--one {
		left: 10%;
		height: 33%;
	}

	.place-art__block--two {
		right: 12%;
		height: 48%;
		background: color-mix(in srgb, var(--sg-accent) 16%, transparent);
	}

	.place-art__sun {
		width: 3.5rem;
		height: 3.5rem;
		position: absolute;
		top: 18%;
		right: 18%;
		border-radius: 50%;
		background: var(--sg-scene-sun);
	}

	.place-art svg {
		width: 4rem;
		height: 4rem;
		position: relative;
		z-index: 1;
		stroke: var(--sg-brand);
		stroke-width: 2.5;
		stroke-linecap: round;
		stroke-linejoin: round;
	}

	.place-art__pin {
		width: 1.25rem;
		height: 1.55rem;
		position: absolute;
		left: 24%;
		top: 24%;
		border-radius: 60% 60% 60% 0;
		background: var(--sg-warm);
		transform: rotate(-45deg);
	}

	.place-art__pin span {
		width: 0.4rem;
		height: 0.4rem;
		position: absolute;
		top: 0.35rem;
		left: 0.4rem;
		border-radius: 50%;
		background: var(--sg-surface);
	}

	.place-card__body {
		padding: 1.1rem 1.2rem 1.25rem;
	}

	.place-category {
		margin: 0 0 0.45rem;
		color: var(--sg-brand);
		font-size: 0.78rem;
		font-weight: 700;
	}

	.place-card h3 {
		margin: 0;
		font-size: 1.25rem;
	}

	.place-description {
		margin: 0.75rem 0 1.25rem;
		color: var(--sg-text-muted);
		font-size: 0.95rem;
		line-height: 1.5;
	}

	.place-card__action {
		color: var(--sg-brand);
		font-size: 0.9rem;
		font-weight: 600;
	}

	.explorer-link {
		margin-top: 1rem;
	}

	.passport-section {
		background: var(--sg-canvas);
	}

	.passport-grid {
		display: grid;
		grid-template-columns: 1.1fr 0.9fr;
		align-items: stretch;
		gap: 1rem;
	}

	:global(.passport-preview),
	:global(.journal-preview) {
		min-width: 0;
		padding: clamp(1.25rem, 3vw, 2rem);
		border-radius: var(--sg-radius-card);
	}

	.preview-caption,
	.journal-preview__top {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 1rem;
		color: var(--sg-text-muted);
		font-size: 0.7rem;
		font-weight: 700;
		letter-spacing: 0.09em;
	}

	.passport-preview__head {
		min-height: 9.5rem;
		margin-top: 1.25rem;
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 1rem;
	}

	.passport-preview__head h3,
	:global(.journal-preview h3) {
		margin: 0;
		font-size: clamp(1.5rem, 2.7vw, 2.25rem);
		line-height: 1.15;
	}

	.passport-preview__head > div > p:last-child {
		margin: 0.6rem 0 0;
		color: var(--sg-text-muted);
	}

	.passport-preview__head img {
		width: clamp(6rem, 12vw, 9rem);
		height: clamp(6rem, 12vw, 9rem);
		flex: none;
		object-fit: contain;
	}

	.passport-progress {
		padding-top: 1rem;
		border-top: 1px solid var(--sg-border);
	}

	.passport-progress > div:first-child {
		display: flex;
		align-items: baseline;
		gap: 0.45rem;
	}

	.passport-progress > div:first-child strong {
		color: var(--sg-brand);
		font-family: var(--landing-heading);
		font-size: 1.35rem;
	}

	.passport-progress > div:first-child span {
		color: var(--sg-text-muted);
		font-size: 0.85rem;
	}

	.passport-stamps {
		margin-top: 0.75rem;
		display: flex;
		flex-wrap: wrap;
		gap: 0.45rem;
	}

	.passport-stamps span {
		padding: 0.3rem 0.55rem;
		border: 1px solid var(--sg-border);
		border-radius: 0.45rem;
		color: var(--sg-text-muted);
		font-size: 0.75rem;
	}

	:global(.journal-preview) {
		display: flex;
		flex-direction: column;
		background: var(--sg-nyaman);
	}

	.journal-preview__top {
		margin-bottom: auto;
	}

	.journal-mark {
		width: 0.9rem;
		height: 0.9rem;
		border-radius: 0 50% 50% 50%;
		background: var(--sg-warm);
		transform: rotate(45deg);
	}

	:global(.journal-preview > .eyebrow) {
		margin-top: 1.8rem;
		font-size: 0.7rem;
		letter-spacing: 0.08em;
	}

	:global(.journal-preview h3) {
		max-width: 14ch;
	}

	.journal-quote {
		max-width: 25rem;
		margin: 1.1rem 0 2rem;
		color: var(--sg-text-muted);
		font-size: 1.05rem;
		font-style: italic;
		line-height: 1.6;
	}

	.journal-preview__footer {
		margin-top: auto;
		padding-top: 0.9rem;
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 1rem;
		border-top: 1px solid var(--sg-border);
		font-size: 0.8rem;
	}

	.journal-preview__footer > span {
		color: var(--sg-text-muted);
	}

	.journal-preview__footer a {
		color: var(--sg-brand);
		font-weight: 700;
		text-decoration: none;
	}

	.trail-section {
		background: var(--sg-canvas);
	}

	.trail-layout {
		display: grid;
		grid-template-columns: minmax(0, 0.85fr) minmax(0, 1.15fr);
		align-items: center;
		gap: clamp(2rem, 6vw, 6rem);
	}

	.trail-copy h2 {
		max-width: 13ch;
	}

	.trail-copy > p:not(.eyebrow) {
		margin-top: 1rem;
	}

	.trail-copy > .text-link {
		margin-top: 1rem;
	}

	.trail-map {
		padding: clamp(1rem, 2.5vw, 1.75rem);
		border: 1px solid var(--sg-border);
		border-radius: var(--sg-radius-card);
		background: var(--sg-surface);
	}

	.trail-map__heading {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 1rem;
	}

	.trail-map__kicker {
		color: var(--sg-brand);
		font-size: 0.7rem;
		font-weight: 700;
		letter-spacing: 0.1em;
	}

	.trail-map__heading h3 {
		margin: 0.3rem 0 0;
		font-size: 1.45rem;
	}

	.trail-map__heading img {
		width: 5.5rem;
		height: 5.5rem;
		flex: none;
		object-fit: contain;
	}

	.trail-stops {
		margin: 0.5rem 0 0;
		padding: 0;
		list-style: none;
	}

	.trail-stops li {
		min-height: 3.4rem;
		position: relative;
		padding-left: 1.9rem;
		display: flex;
		align-items: center;
		gap: 0.75rem;
	}

	.trail-stops li:not(:last-child)::before {
		position: absolute;
		top: 1.75rem;
		bottom: -0.05rem;
		left: 0.55rem;
		border-left: 2px solid var(--sg-brand);
		content: '';
	}

	.trail-stop__node {
		width: 0.75rem;
		height: 0.75rem;
		position: absolute;
		left: 0.2rem;
		border: 2px solid var(--sg-brand);
		border-radius: 50%;
		background: var(--sg-surface);
	}

	.trail-stops li:first-child .trail-stop__node {
		background: var(--sg-brand);
	}

	.trail-stops li strong {
		font-size: 0.95rem;
	}

	.trail-stops li small {
		margin-left: auto;
		color: var(--sg-text-muted);
		font-size: 0.78rem;
	}

	.final-section {
		padding-block: 0;
		background: var(--sg-hero);
	}

	.final-cta {
		min-height: 42rem;
		position: relative;
		overflow: hidden;
		color: white;
	}

	.final-cta__copy {
		max-width: 40rem;
		margin-left: max(1.25rem, calc((100vw - 82rem) / 2));
		padding-block: clamp(7rem, 13vw, 10rem);
		position: relative;
		z-index: 2;
	}

	.final-cta .eyebrow {
		color: white;
	}

	.final-cta h2 {
		margin: 0;
		font-size: clamp(2.1rem, 4.5vw, 4rem);
		font-weight: 400;
		line-height: 1.05;
		letter-spacing: -0.055em;
	}

	.final-cta__copy > p:not(.eyebrow) {
		margin: 1rem 0 0;
		color: color-mix(in srgb, var(--sg-brand-contrast) 92%, transparent);
		font-size: 1.05rem;
	}

	.action--light {
		background: var(--sg-warm);
		color: var(--sg-text);
	}

	.action--light:hover {
		background: white;
	}

	.action--outline-light {
		border-color: color-mix(in srgb, var(--sg-brand-contrast) 55%, transparent);
		color: white;
	}

	.action--outline-light:hover {
		background: color-mix(in srgb, var(--sg-brand-contrast) 10%, transparent);
	}

	.final-cta__art {
		position: absolute;
		inset: 0;
		z-index: 0;
	}

	.final-cta__art img {
		width: 100%;
		height: 100%;
		object-fit: cover;
		object-position: center 28%;
		filter: saturate(0.95);
	}

	.final-cta__art::after {
		position: absolute;
		inset: 0;
		background: linear-gradient(
			90deg,
			color-mix(in srgb, var(--sg-hero-deep) 70%, transparent),
			color-mix(in srgb, var(--sg-hero-deep) 32%, transparent) 55%,
			transparent
		);
		content: '';
	}

	.final-cta__app {
		width: clamp(16rem, 27vw, 26rem);
		min-height: 19rem;
		padding: 1.5rem;
		position: absolute;
		z-index: 1;
		right: 7%;
		bottom: 12%;
		display: grid;
		align-content: start;
		gap: 1rem;
		border: 1px solid color-mix(in srgb, var(--sg-brand-contrast) 55%, transparent);
		border-radius: 0.55rem;
		background: color-mix(in srgb, var(--sg-brand-contrast) 91%, transparent);
		box-shadow: 0 1.5rem 4rem color-mix(in srgb, var(--sg-hero-deep) 16%, transparent);
		color: var(--sg-text);
	}

	.final-cta__app strong {
		font-size: 1.35rem;
		font-weight: 600;
	}
	.final-cta__app > span:not(.final-cta__app-line) {
		margin-bottom: 0.8rem;
		color: var(--sg-text-muted);
		font-size: 0.85rem;
	}
	.final-cta__app-line {
		min-height: 2.4rem;
		padding-inline: 0.7rem;
		display: flex;
		align-items: center;
		border: 1px solid var(--sg-border);
		border-radius: 0.4rem;
		background: var(--sg-scene-soft);
		color: var(--sg-text-muted);
		font-size: 0.75rem;
	}
	.final-cta__app-line:last-child {
		justify-content: center;
		background: var(--sg-warm);
		border-color: var(--sg-warm);
		color: var(--sg-text);
		font-weight: 600;
	}

	.site-footer {
		padding-block: 3rem 1rem;
		background: var(--sg-surface);
		border-top: 1px solid var(--sg-border);
	}

	.footer-grid {
		padding-bottom: 2.5rem;
		display: grid;
		grid-template-columns: 1.4fr repeat(3, 1fr);
		gap: clamp(1.5rem, 4vw, 4rem);
	}

	.footer-brand .brand {
		margin-bottom: 0.6rem;
	}

	.footer-brand p {
		margin: 0.3rem 0;
		color: var(--sg-text-muted);
	}

	.footer-brand .footer-note {
		max-width: 15rem;
		margin-top: 1rem;
		color: var(--sg-brand);
		font-family: var(--sg-font-display);
		font-size: 0.9rem;
		font-style: italic;
	}

	.footer-grid h2 {
		margin: 0 0 0.65rem;
		font-family: var(--sg-font-body);
		font-size: 0.85rem;
		font-weight: 700;
		letter-spacing: 0;
	}

	.footer-grid > div:not(:first-child) > a {
		min-height: 2.25rem;
		display: flex;
		align-items: center;
		color: var(--sg-text-muted);
		font-size: 0.9rem;
		text-decoration: none;
	}

	.footer-grid > div:not(:first-child) > a:hover,
	.footer-bottom a:hover {
		color: var(--sg-brand);
	}

	.footer-grid > div:last-child p {
		max-width: 14rem;
		margin: 0;
		color: var(--sg-text-muted);
		font-size: 0.9rem;
		line-height: 1.5;
	}

	.footer-bottom {
		min-height: 3.5rem;
		padding-top: 0.5rem;
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 1rem;
		border-top: 1px solid var(--sg-border);
		color: var(--sg-text-muted);
		font-size: 0.8rem;
	}

	.footer-bottom a {
		min-height: 2.5rem;
		display: inline-flex;
		align-items: center;
		color: var(--sg-text-muted);
		text-decoration: none;
	}

	@media (max-width: 900px) {
		.site-header__inner {
			grid-template-columns: auto 1fr auto;
		}

		.desktop-nav {
			display: none;
		}

		.mobile-menu {
			display: block;
		}

		.hero__copy {
			max-width: 32rem;
		}

		.hero-chip--one {
			left: auto;
			right: 4%;
		}

		.hero-chip--two {
			left: auto;
			right: 10%;
		}

		.hero-chip--three {
			display: none;
		}

		.section-heading {
			align-items: flex-start;
			flex-direction: column;
			gap: 0.75rem;
		}

		.steps li {
			padding-right: 0.9rem;
		}
	}

	@media (max-width: 700px) {
		.container,
		.site-header__inner {
			width: min(calc(100% - 2rem), 82rem);
		}

		.site-header__inner {
			min-height: 4.4rem;
			grid-template-columns: 1fr auto auto;
			gap: 0.5rem;
		}

		.desktop-nav {
			display: none;
		}
		.header-cta {
			min-height: 2.65rem;
			padding-inline: 0.6rem;
			display: inline-flex;
			font-size: 0.75rem;
		}

		.mobile-menu {
			display: block;
		}
		.mobile-menu summary {
			padding-inline: 0.65rem;
		}
		.mobile-menu summary span {
			display: none;
		}

		.hero {
			min-height: max(100svh, 45rem);
		}
		.hero__world img,
		.hero__world video {
			object-position: 37% center;
		}
		.hero__world::after {
			background: linear-gradient(
				180deg,
				color-mix(in srgb, var(--sg-hero-deep) 34%, transparent),
				color-mix(in srgb, var(--sg-hero-deep) 24%, transparent) 50%,
				color-mix(in srgb, var(--sg-hero-deep) 18%, transparent)
			);
		}
		.hero__content {
			padding-block: clamp(10rem, 20vh, 12rem) 3rem;
		}
		.hero__copy {
			max-width: 25rem;
		}

		.hero h1 {
			max-width: 13ch;
			font-size: clamp(2.15rem, 8.8vw, 2.8rem);
		}

		.hero__description {
			max-width: 19rem;
			margin-top: 1.1rem;
			font-size: 1rem;
		}

		.hero__actions {
			gap: 0.45rem;
		}

		.hero__actions .action {
			min-height: 2.8rem;
			padding-inline: 0.85rem;
			font-size: 0.8rem;
		}

		.hero-chip--one,
		.hero-chip--three {
			display: none;
		}
		.hero-chip--two {
			top: auto;
			bottom: 18%;
			left: auto;
			right: 0;
		}
		.hero__scroll {
			bottom: 1.5rem;
		}

		.section {
			padding-block: 3.5rem;
		}

		.section-heading {
			margin-bottom: 1.5rem;
		}

		.section-heading h2,
		.trail-copy h2 {
			font-size: clamp(1.9rem, 8vw, 2.65rem);
		}

		.product-window {
			margin-bottom: 3rem;
		}
		.product-window__bar {
			padding-inline: 1rem;
		}
		.product-window__body {
			grid-template-columns: 1fr;
		}
		.product-window__panel {
			padding: 1.5rem;
			border-right: 0;
			border-bottom: 1px solid var(--sg-border);
		}
		.product-window__panel h3 {
			margin-block: 0.8rem 1.5rem;
		}
		.product-window__scene {
			min-height: 17rem;
		}
		.feature-row {
			grid-template-columns: 2rem minmax(0, 1fr);
			align-items: start;
			gap: 0.7rem;
		}
		.feature-row__link {
			grid-column: 2;
		}

		.section-heading--center {
			align-items: flex-start;
		}

		.steps {
			grid-template-columns: 1fr;
			gap: 1.4rem;
		}

		.steps::before {
			top: 1.4rem;
			bottom: 1.4rem;
			left: 1.25rem;
			right: auto;
			border-top: 0;
			border-left: 1px dashed var(--sg-border);
		}

		.steps li {
			min-height: 4.5rem;
			padding: 0 0 0 3.75rem;
		}

		.step-number {
			position: absolute;
			left: 0;
			top: 0;
			margin: 0;
		}

		.steps li p {
			max-width: 100%;
		}

		.place-grid {
			grid-auto-columns: minmax(16rem, 82vw);
			grid-auto-flow: column;
			grid-template-columns: none;
			overflow-x: auto;
			padding-bottom: 0.5rem;
			scroll-snap-type: x mandatory;
			overscroll-behavior-inline: contain;
		}

		.place-grid > * {
			scroll-snap-align: start;
		}

		.passport-grid,
		.trail-layout {
			grid-template-columns: 1fr;
		}

		.passport-grid {
			gap: 0.75rem;
		}

		.trail-layout {
			gap: 1.75rem;
		}

		.final-cta {
			min-height: 44rem;
		}
		.final-cta__copy {
			max-width: 28rem;
			margin-inline: 1rem;
			padding-block: 5rem;
		}
		.final-cta__art img {
			object-position: 37% center;
		}
		.final-cta__art::after {
			background: linear-gradient(
				180deg,
				color-mix(in srgb, var(--sg-hero-deep) 72%, transparent),
				color-mix(in srgb, var(--sg-hero-deep) 22%, transparent)
			);
		}

		.final-cta h2 {
			font-size: clamp(2rem, 9vw, 3rem);
		}

		.final-cta__app {
			width: min(17rem, calc(100% - 2rem));
			min-height: 11rem;
			right: 1rem;
			bottom: 1rem;
		}

		.footer-grid {
			grid-template-columns: repeat(2, minmax(0, 1fr));
			gap: 1.5rem;
		}

		.footer-brand {
			grid-column: 1 / -1;
		}
	}

	@keyframes hero-enter {
		from {
			opacity: 0;
			transform: translateY(1rem);
		}
		to {
			opacity: 1;
			transform: translateY(0);
		}
	}

	@keyframes route-reveal {
		from {
			clip-path: inset(0 100% 0 0);
		}
		to {
			clip-path: inset(0);
		}
	}

	@media (prefers-reduced-motion: no-preference) {
		.hero__copy > * {
			animation: hero-enter 600ms cubic-bezier(0.23, 1, 0.32, 1) both;
		}
		.hero__copy > .eyebrow {
			animation-delay: 100ms;
		}
		.hero__copy > h1 {
			animation-delay: 300ms;
		}
		.hero__copy > .hero__description {
			animation-delay: 540ms;
		}
		.hero__copy > .hero__actions {
			animation-delay: 780ms;
		}
		.hero-chip {
			animation: hero-enter 650ms var(--sg-ease-enter) both;
		}
		.hero-chip--one {
			animation-delay: 950ms;
		}
		.hero-chip--two {
			animation-delay: 1200ms;
		}
		.hero-chip--three {
			animation-delay: 1450ms;
		}
		.hero__scroll {
			animation: hero-enter 700ms ease 1.5s both;
		}
		.landing :global(.section[data-in-view='true'] .product-window__route) {
			animation: route-reveal var(--sg-motion-route) var(--sg-ease-enter) 300ms both;
		}
		.landing :global(.section[data-in-view='true'] .section-heading),
		.landing :global(.section[data-in-view='true'] .trail-copy),
		.landing :global(.section[data-in-view='true'] .final-cta__copy) {
			animation: hero-enter 700ms cubic-bezier(0.23, 1, 0.32, 1) both;
		}
		.landing :global(.section[data-in-view='true'] .product-window),
		.landing :global(.section[data-in-view='true'] .feature-list),
		.landing :global(.section[data-in-view='true'] .steps),
		.landing :global(.section[data-in-view='true'] .place-grid),
		.landing :global(.section[data-in-view='true'] .passport-grid),
		.landing :global(.section[data-in-view='true'] .trail-map) {
			animation: hero-enter 850ms cubic-bezier(0.23, 1, 0.32, 1) 120ms both;
		}
	}

	@media (prefers-reduced-motion: reduce) {
		.hero__world video {
			display: none;
		}
		.hero *,
		.section * {
			animation: none !important;
			transition-duration: 0.01ms !important;
		}
	}

	@media (max-width: 390px) {
		.hero h1 {
			font-size: clamp(2.45rem, 10.5vw, 2.9rem);
		}

		.footer-grid {
			gap: 1.25rem 0.75rem;
		}
	}
</style>
