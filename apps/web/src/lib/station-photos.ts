/**
 * Curated real photos of well-known stasiun/halte, self-hosted under
 * /static/stations. Sources are Wikimedia Commons files with free licenses —
 * per-file author, license, and source URL are listed in
 * static/stations/ATTRIBUTION.md (shown in-app via the `credit` field).
 *
 * Keyed by the station's display name (lowercase). Only names whose stop is
 * actually the depicted facility are mapped — unmapped stops fall back to the
 * decorative illustration.
 */
export interface StationPhoto {
	src: string;
	alt: string;
	credit: string;
}

const PHOTOS: Record<string, StationPhoto> = {
	'dukuh atas': {
		src: '/stations/dukuh-atas.webp',
		alt: 'Stasiun MRT Dukuh Atas BNI, Jakarta',
		credit: 'Syaifan Bahtiar Nirwansyah · CC BY-SA 4.0'
	},
	'dukuh atas bni': {
		src: '/stations/dukuh-atas.webp',
		alt: 'Stasiun MRT Dukuh Atas BNI, Jakarta',
		credit: 'Syaifan Bahtiar Nirwansyah · CC BY-SA 4.0'
	},
	sudirman: {
		src: '/stations/sudirman.webp',
		alt: 'Stasiun Sudirman, Jakarta',
		credit: 'Medelam · CC BY-SA 4.0'
	},
	'bundaran hi': {
		src: '/stations/bundaran-hi.webp',
		alt: 'Peron Stasiun MRT Bundaran HI, Jakarta',
		credit: 'VulcanSphere · CC BY 4.0'
	},
	'bundaran hi astra': {
		src: '/stations/bundaran-hi.webp',
		alt: 'Peron Stasiun MRT Bundaran HI, Jakarta',
		credit: 'VulcanSphere · CC BY 4.0'
	},
	'blok m': {
		src: '/stations/blok-m.webp',
		alt: 'Peron Stasiun MRT Blok M, Jakarta',
		credit: 'VulcanSphere · CC BY 4.0'
	},
	'blok m bca': {
		src: '/stations/blok-m.webp',
		alt: 'Peron Stasiun MRT Blok M, Jakarta',
		credit: 'VulcanSphere · CC BY 4.0'
	},
	'lebak bulus': {
		src: '/stations/lebak-bulus.webp',
		alt: 'Stasiun MRT Lebak Bulus, Jakarta',
		credit: 'VulcanSphere · CC BY 4.0'
	},
	'lebak bulus bank syariah indonesia': {
		src: '/stations/lebak-bulus.webp',
		alt: 'Stasiun MRT Lebak Bulus, Jakarta',
		credit: 'VulcanSphere · CC BY 4.0'
	},
	'underpass lebak bulus': {
		src: '/stations/underpass-lebak-bulus.webp',
		alt: 'Halte Transjakarta Underpass Lebak Bulus, Jakarta',
		credit: 'Enperfectify World · CC BY 4.0'
	},
	'tanah abang': {
		src: '/stations/tanah-abang.webp',
		alt: 'Pintu masuk Stasiun Tanah Abang, Jakarta',
		credit: 'Irvan Cahyo N · CC BY-SA 3.0'
	},
	'pasar senen': {
		src: '/stations/pasar-senen.webp',
		alt: 'Stasiun Pasar Senen, Jakarta',
		credit: 'Muhamad Fahrizal Leo Pratama · CC BY-SA 4.0'
	},
	'jakarta kota': {
		src: '/stations/jakarta-kota.webp',
		alt: 'Stasiun Jakarta Kota, Jakarta',
		credit: 'Jedidiahmarada · CC BY-SA 4.0'
	},
	kota: {
		src: '/stations/jakarta-kota.webp',
		alt: 'Stasiun Jakarta Kota, Jakarta',
		credit: 'Jedidiahmarada · CC BY-SA 4.0'
	},
	harmoni: {
		src: '/stations/harmoni.webp',
		alt: 'Halte Transjakarta Harmoni, Jakarta',
		credit: 'Enperfectify World · CC BY-SA 4.0'
	},
	tosari: {
		src: '/stations/tosari.webp',
		alt: 'Halte Transjakarta Tosari, Jakarta',
		credit: 'RasyaAbhirama13 · CC BY-SA 3.0'
	},
	manggarai: {
		src: '/stations/manggarai.webp',
		alt: 'Stasiun Manggarai, Jakarta',
		credit: 'Syaifan Bahtiar Nirwansyah · CC BY-SA 4.0'
	}
};

export function stationPhoto(name: string): StationPhoto | undefined {
	return PHOTOS[name.trim().toLowerCase()];
}
