<script lang="ts">
	import { onMount } from 'svelte';

	let { paused = false }: { paused?: boolean } = $props();
	let stage: HTMLDivElement;
	let inView = $state(true);
	let documentHidden = $state(false);
	const skyline = [
		{ x: 12, y: 287, w: 22, h: 47 },
		{ x: 42, y: 265, w: 26, h: 70 },
		{ x: 76, y: 281, w: 20, h: 52 },
		{ x: 104, y: 254, w: 32, h: 80 },
		{ x: 145, y: 274, w: 18, h: 60 },
		{ x: 170, y: 268, w: 26, h: 67 },
		{ x: 238, y: 282, w: 31, h: 51 },
		{ x: 281, y: 260, w: 19, h: 72 },
		{ x: 315, y: 275, w: 27, h: 58 }
	];
	const shrubs = [
		{ x: -22, y: 310, w: 94, h: 57 },
		{ x: 40, y: 322, w: 69, h: 42 },
		{ x: 92, y: 305, w: 83, h: 51 },
		{ x: 158, y: 321, w: 67, h: 41 },
		{ x: 236, y: 307, w: 86, h: 52 },
		{ x: 304, y: 314, w: 74, h: 45 },
		{ x: 365, y: 301, w: 92, h: 56 },
		{ x: 420, y: 302, w: 105, h: 64 }
	];
	const grasses = [
		[78, 471],
		[156, 493],
		[279, 461],
		[348, 517],
		[394, 489],
		[681, 444],
		[748, 413],
		[802, 451],
		[890, 504],
		[926, 421]
	];

	onMount(() => {
		// Only decorative objects move; the camera and layout never animate.
		const observer = new IntersectionObserver(([entry]) => {
			inView = entry.isIntersecting;
		});
		observer.observe(stage);
		const syncVisibility = () => {
			documentHidden = document.hidden;
		};
		syncVisibility();
		document.addEventListener('visibilitychange', syncVisibility);
		return () => {
			observer.disconnect();
			document.removeEventListener('visibilitychange', syncVisibility);
		};
	});
</script>

<div
	class="hero-scene"
	class:is-paused={paused || !inView || documentHidden}
	bind:this={stage}
	aria-hidden="true"
>
	<svg viewBox="0 0 960 560" preserveAspectRatio="xMidYMax slice" shape-rendering="crispEdges">
		<defs>
			<pattern id="hero-grass" width="48" height="30" patternUnits="userSpaceOnUse">
				<rect width="48" height="30" fill="#79a955" />
				<path d="M0 4h12v4H0zM25 18h18v4H25z" fill="#8bbb59" />
				<path d="M8 24h8v3H8zM36 3h7v3h-7z" fill="#66964e" />
				<path d="M22 6h3v3h-3zM3 16h4v2H3z" fill="#bed875" />
			</pattern>
			<pattern id="hero-shadow" width="36" height="26" patternUnits="userSpaceOnUse">
				<rect width="36" height="26" fill="#315f4d" />
				<path d="M0 3h15v5H0zM19 16h17v4H19z" fill="#3e7655" />
				<path d="M5 20h8v4H5zM23 4h7v3h-7z" fill="#558d59" />
			</pattern>
			<clipPath id="hero-track"><rect x="0" y="324" width="540" height="45" /></clipPath>
			<clipPath id="hero-screen"><rect x="501" y="298" width="163" height="106" /></clipPath>
		</defs>
		<rect class="scene-sky" width="960" height="560" />
		<g class="scene-clouds scene-motion">
			<image href="/illustrations/landing/clouds.webp" x="348" y="20" width="760" height="224" />
			<image
				href="/illustrations/landing/clouds.webp"
				x="-170"
				y="224"
				width="500"
				height="147"
				opacity="0.6"
			/>
		</g>
		<g class="scene-skyline" fill="#a8d5e6">
			{#each skyline as building (building.x)}
				<rect x={building.x} y={building.y} width={building.w} height={building.h} />
				<rect
					x={building.x + 5}
					y={building.y + 6}
					width={building.w - 9}
					height="3"
					fill="#d4eaf0"
				/>
				{#each [0, 1, 2, 3, 4] as row (row)}
					<rect
						x={building.x + 5}
						y={building.y + 15 + row * 9}
						width="4"
						height="4"
						fill="#79b4cc"
					/>
					<rect
						x={building.x + building.w - 9}
						y={building.y + 15 + row * 9}
						width="4"
						height="4"
						fill="#79b4cc"
					/>
				{/each}
			{/each}
			<path d="M211 239h7v84h-7zM204 322h21v8h-21z" fill="#e5e8cd" />
			<path d="M210 232h9v8h-9zM212 225h5v8h-5z" fill="#ecd88c" />
		</g>
		<path d="M0 330h230v-7h90v-10h127v-8h144v-22h153v277H0z" fill="#94bd68" />
		{#each shrubs as shrub, index (shrub.x)}
			<image
				class="scene-bush scene-motion"
				style={`animation-delay: ${-(index % 4) * 0.9}s`}
				href="/illustrations/landing/bush.webp"
				x={shrub.x}
				y={shrub.y}
				width={shrub.w}
				height={shrub.h}
			/>
		{/each}
		<g class="scene-rail">
			<path d="M0 365h561v8H0zM0 360h553v3H0z" fill="#607b72" />
			<path d="M0 354h553v3H0z" fill="#d2dcc3" />
			{#each [25, 95, 165, 235, 305, 375, 445, 515] as x (x)}
				<rect {x} y="370" width="8" height="48" fill="#839881" />
			{/each}
		</g>
		<g clip-path="url(#hero-track)">
			<g class="scene-train scene-motion">
				{#each [0, 1, 2] as carriage (carriage)}
					<g transform={`translate(${138 + carriage * 53} 332)`}>
						<rect width="50" height="23" rx="3" fill="#edf2de" />
						<rect y="17" width="50" height="4" fill="#277ca4" />
						<rect x="5" y="5" width="9" height="7" fill="#376b7d" />
						<rect x="20" y="5" width="9" height="7" fill="#376b7d" />
						<rect x="35" y="5" width="9" height="7" fill="#376b7d" />
						<rect x="9" y="23" width="6" height="3" fill="#375358" />
						<rect x="37" y="23" width="6" height="3" fill="#375358" />
					</g>
				{/each}
			</g>
		</g>
		<path
			d="M0 399l77-5 22 8 136-14 50 7 96-21 60-7 97-27 90-8 50-26 84-13 40-33h158v300H0z"
			fill="url(#hero-grass)"
		/>
		<path
			d="M0 436l100-14 109 4 87-20 114-8 107-32 39-4-93 51-113 23-87 11-89 27L0 500z"
			fill="#b6ba82"
		/>
		<path
			d="M0 447l94-15 105 5 90-20 110-8 100-31-79 41-115 22-83 18-102 26L0 500z"
			fill="#c4c796"
		/>
		<image
			class="scene-tree scene-motion"
			href="/illustrations/landing/tree.webp"
			x="676"
			y="-54"
			width="508"
			height="474"
		/>
		<image
			class="scene-tree scene-tree--near scene-motion"
			href="/illustrations/landing/tree.webp"
			x="537"
			y="179"
			width="241"
			height="225"
		/>
		<path
			d="M0 500l85-29 96-9 101 14 62-7 91-24 126 4 84-28 90-9 92 10 47-25 86 28v135H0z"
			fill="url(#hero-shadow)"
		/>
		<path
			d="M329 486l88-13 125 15 146-4 104 11-69 36-159 10-140-20z"
			fill="#275247"
			opacity="0.7"
		/>
		{#each grasses as grass, index (grass[0])}
			<g transform={`translate(${grass[0]} ${grass[1]})`}>
				<g class="scene-grass scene-motion" style={`animation-delay: ${-(index % 5) * 0.8}s`}>
					<path d="M-2 0v-24h3v24zM-7 0v-16h3v16zM4 0v-19h3v19z" fill="#91b45e" />
					<path d="M-2-22h3v7h-3zM4-18h3v5H4z" fill="#b1cc73" />
				</g>
			</g>
		{/each}
		<g class="scene-laptop" transform="translate(52 28) scale(0.87)">
			<path d="M489 286h187v131H489z" fill="#253d43" />
			<path d="M495 292h175v118H495z" fill="#577475" />
			<path d="M501 298h163v106H501z" fill="#d9e8df" />
			<rect x="504" y="301" width="157" height="12" fill="#bdd6ca" />
			<rect x="511" y="305" width="31" height="3" fill="#4a8179" />
			<path
				d="M519 338h36l12 21h29l21-24h29M540 328v66M622 317v76"
				stroke="#9cc7b7"
				stroke-width="3"
				fill="none"
			/>
			<g clip-path="url(#hero-screen)">
				<path d="M522 384l28-38h22l15 17 40-26" stroke="#92bbb7" stroke-width="4" fill="none" />
				<path
					class="scene-screen-route scene-motion"
					d="M522 384l28-38h22l15 17 40-26"
					pathLength="1"
					stroke="#3386a8"
					stroke-width="4"
					fill="none"
				/>
				<rect
					class="scene-screen-cursor scene-motion"
					x="-3"
					y="-3"
					width="6"
					height="6"
					fill="#f5efbb"
					stroke="#3386a8"
					stroke-width="2"
				/>
			</g>
			{#each [[522, 384], [550, 346], [587, 363], [627, 337]] as stop (stop[0])}
				<rect
					x={stop[0] - 3}
					y={stop[1] - 3}
					width="6"
					height="6"
					fill="#f5efbb"
					stroke="#3386a8"
					stroke-width="2"
				/>
			{/each}
			<path d="M489 417h187l-63 76H404z" fill="#c5c6b0" />
			<path d="M497 424h163l-35 40H440z" fill="#35474a" />
			{#each [0, 1, 2, 3] as row (row)}
				<path d={`M ${500 - row * 12} ${431 + row * 8} h 141`} stroke="#73847e" stroke-width="2" />
			{/each}
			<path d="M472 470h87l-10 11h-93z" fill="#9daea3" />
			<path d="M404 493h209v6H404z" fill="#87988b" />
		</g>
		<g class="scene-journal" transform="translate(10 38) scale(0.88)">
			<path d="M662 473l103-21 78 41-116 23-75-34z" fill="#394e43" />
			<path d="M660 466l105-21 78 42-116 24-75-34z" fill="#a59a68" />
			<path d="M664 461l104-21 73 40-114 25-70-35z" fill="#d9c592" />
			<path d="M671 462l95-17 65 34-105 23z" fill="#d1b979" />
			<path d="M710 459l51 28 7-2-51-29z" fill="#456477" />
			<path d="M680 462l40 23" stroke="#a08f61" stroke-width="2" />
			<rect x="760" y="476" width="13" height="6" fill="#ede0b2" />
		</g>
		<image
			class="scene-bush scene-motion"
			href="/illustrations/landing/bush.webp"
			x="-46"
			y="467"
			width="252"
			height="153"
		/>
		<image
			class="scene-bush scene-motion"
			style="animation-delay: -2s"
			href="/illustrations/landing/bush.webp"
			x="59"
			y="514"
			width="162"
			height="99"
		/>
		<image
			class="scene-bush scene-motion"
			style="animation-delay: -4s"
			href="/illustrations/landing/bush.webp"
			x="876"
			y="457"
			width="175"
			height="107"
		/>
		<g fill="#c4d883">
			<path d="M236 496h3v14h-3zM230 508h15v3h-15zM864 483h3v16h-3zM857 495h18v3h-18z" />
		</g>
		<g fill="#e8d995"
			><rect x="231" y="488" width="12" height="10" /><rect
				x="859"
				y="476"
				width="12"
				height="9"
			/></g
		>
	</svg>
</div>

<style>
	.hero-scene {
		position: absolute;
		inset: 0;
		overflow: hidden;
	}
	svg {
		display: block;
		width: 100%;
		height: 100%;
		image-rendering: pixelated;
	}
	.scene-sky {
		fill: var(--sg-story-blue);
	}
	.scene-clouds {
		animation: clouds-drift 24s ease-in-out infinite alternate;
	}
	.scene-tree,
	.scene-bush,
	.scene-grass {
		transform-box: fill-box;
		transform-origin: 50% 100%;
	}
	.scene-tree {
		animation: tree-sway 9s ease-in-out -2s infinite alternate;
	}
	.scene-tree--near {
		animation-duration: 7s;
		animation-delay: -4s;
	}
	.scene-bush {
		animation: bush-sway 6s ease-in-out infinite alternate;
	}
	.scene-grass {
		animation: grass-sway 4s ease-in-out infinite alternate;
	}
	.scene-screen-route {
		stroke-dasharray: 1;
		animation: screen-route 10s ease-in-out -2s infinite;
	}
	.scene-screen-cursor {
		offset-path: path('M522 384l28-38h22l15 17 40-26');
		offset-rotate: 0deg;
		animation: screen-cursor 10s ease-in-out -2s infinite;
	}
	.scene-train {
		animation: train-pass 38s linear -12s infinite;
	}
	.is-paused .scene-motion {
		animation-play-state: paused;
	}
	@keyframes tree-sway {
		from {
			transform: rotate(-0.45deg);
		}
		to {
			transform: rotate(0.45deg);
		}
	}
	@keyframes bush-sway {
		from {
			transform: skewX(-0.8deg);
		}
		to {
			transform: skewX(0.8deg);
		}
	}
	@keyframes grass-sway {
		from {
			transform: skewX(-7deg);
		}
		to {
			transform: skewX(7deg);
		}
	}
	@keyframes screen-route {
		0%,
		100% {
			stroke-dashoffset: 1;
		}
		42%,
		78% {
			stroke-dashoffset: 0;
		}
	}
	@keyframes screen-cursor {
		0%,
		10% {
			offset-distance: 0%;
			opacity: 0;
		}
		15% {
			opacity: 1;
		}
		65%,
		80% {
			offset-distance: 100%;
			opacity: 1;
		}
		90% {
			offset-distance: 100%;
			opacity: 0;
		}
		100% {
			offset-distance: 0%;
			opacity: 0;
		}
	}
	@keyframes clouds-drift {
		from {
			transform: translate(-12px, 0);
		}
		to {
			transform: translate(12px, -3px);
		}
	}
	@keyframes train-pass {
		from {
			transform: translateX(-370px);
		}
		to {
			transform: translateX(850px);
		}
	}
	@media (prefers-reduced-motion: reduce) {
		.scene-motion {
			animation-play-state: paused;
		}
	}
	@media (max-width: 767px) {
		/* Copy shares the sky on narrow crops; keep its contrast stable as clouds move. */
		.scene-clouds {
			opacity: 0.2;
		}
	}
</style>
