import { describe, expect, it } from 'vitest';
import { STATUS_LABELS, type TransitStatus } from './status';

const ALL_STATUSES: TransitStatus[] = ['live', 'estimated', 'scheduled', 'stale', 'disruption'];

describe('STATUS_LABELS', () => {
	it('has a label for every transit truth state', () => {
		for (const status of ALL_STATUSES) {
			expect(STATUS_LABELS[status], `missing label for ${status}`).toBeTruthy();
		}
	});
});
