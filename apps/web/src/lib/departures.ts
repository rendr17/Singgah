// Departure board helpers. Timetable times are Asia/Jakarta wall clock, so
// countdowns must compare against WIB — not the device's own timezone.

const wib = new Intl.DateTimeFormat('en-GB', {
	timeZone: 'Asia/Jakarta',
	hour: '2-digit',
	minute: '2-digit',
	hourCycle: 'h23'
});

/** Minutes since midnight in Asia/Jakarta for `now`. */
export function jakartaMinutes(now: Date = new Date()): number {
	const [h, m] = wib.format(now).split(':').map(Number);
	return h * 60 + m;
}

/**
 * Whole minutes until an "HH:MM" departure, wrapped at midnight — 00:26 is
 * "soon" when viewed late at night.
 */
export function minutesUntil(hhmm: string, now: Date = new Date()): number {
	const [h, m] = hhmm.split(':').map(Number);
	return (h * 60 + m - jakartaMinutes(now) + 1440) % 1440;
}

/**
 * Short countdown for the next departure. Past the hour mark the plain clock
 * time says more than "137 mnt".
 */
export function countdownLabel(mins: number): string {
	if (mins <= 0) return 'Sekarang';
	if (mins < 60) return `${mins} mnt`;
	return '';
}

/** "07:04" -> "07.04" — id-ID clock convention. */
export function fmtTime(hhmm: string): string {
	return hhmm.replace(':', '.');
}
