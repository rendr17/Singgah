import { env } from '$env/dynamic/public';

// Basemap is Protomaps' hosted tile API — the neutral 'light' flavor keeps
// operator/route overlays dominant (71_ICONS_MAP_STYLE.md). Source and
// licensing are registered in 35_DATA_SOURCES.md.
const PROTOMAPS_STYLE = 'https://api.protomaps.com/styles/v5/light/en.json';

// PUBLIC_MAP_STYLE_URL remains a full override for self-hosted/custom styles.
// Without PUBLIC_PROTOMAPS_API_KEY the tiles simply fail to load — the map's
// existing error state surfaces that honestly.
export function basemapStyleUrl(): string {
	const override = env.PUBLIC_MAP_STYLE_URL;
	if (override) return override;
	const key = env.PUBLIC_PROTOMAPS_API_KEY ?? '';
	if (!key) console.warn('PUBLIC_PROTOMAPS_API_KEY is not set — basemap tiles will fail to load.');
	return `${PROTOMAPS_STYLE}?key=${key}`;
}
