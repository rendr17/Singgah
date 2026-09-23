// The data-truth states are semantic, not decorative — never relabel
// scheduled data as live (docs/08_UX_PRINCIPLES.md, docs/AGENTS.md §9).
export type TransitStatus = 'live' | 'estimated' | 'scheduled' | 'stale' | 'disruption';

export const STATUS_LABELS: Record<TransitStatus, string> = {
	live: 'Live',
	estimated: 'Perkiraan',
	scheduled: 'Jadwal',
	stale: 'Data lama',
	disruption: 'Gangguan'
};
