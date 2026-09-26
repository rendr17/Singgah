import { describe, expect, it } from 'vitest';

import { countdownLabel, fmtTime, jakartaMinutes, minutesUntil } from './departures';

// 2026-09-24 03:00 UTC = 10:00 WIB.
const tenAMWIB = new Date('2026-09-24T03:00:00Z');
// 2026-09-24 16:30 UTC = 23:30 WIB.
const lateNightWIB = new Date('2026-09-24T16:30:00Z');

describe('jakartaMinutes', () => {
	it('reads the clock in WIB regardless of device zone', () => {
		expect(jakartaMinutes(tenAMWIB)).toBe(10 * 60);
		expect(jakartaMinutes(lateNightWIB)).toBe(23 * 60 + 30);
	});
});

describe('minutesUntil', () => {
	it('counts forward from now', () => {
		expect(minutesUntil('10:05', tenAMWIB)).toBe(5);
		expect(minutesUntil('10:00', tenAMWIB)).toBe(0);
	});
	it('wraps past midnight', () => {
		expect(minutesUntil('00:26', lateNightWIB)).toBe(56);
		expect(minutesUntil('23:57', lateNightWIB)).toBe(27);
	});
	it('yesterday-side times wrap to next service', () => {
		expect(minutesUntil('09:55', tenAMWIB)).toBe(1435);
	});
});

describe('countdownLabel', () => {
	it('labels now and near departures', () => {
		expect(countdownLabel(0)).toBe('Sekarang');
		expect(countdownLabel(5)).toBe('5 mnt');
	});
	it('stays quiet beyond the hour', () => {
		expect(countdownLabel(60)).toBe('');
	});
});

describe('fmtTime', () => {
	it('renders id-ID dot-separated clock', () => {
		expect(fmtTime('07:04')).toBe('07.04');
	});
});
