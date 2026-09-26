import { describe, expect, it } from 'vitest';
import { MapStore } from './map.svelte';

describe('MapStore', () => {
	it('defaults to geographic mode with nothing selected', () => {
		const s = new MapStore();
		expect(s.mode).toBe('geographic');
		expect(s.fromStationId).toBeUndefined();
		expect(s.toStationId).toBeUndefined();
		expect(s.selectedStationId).toBeUndefined();
		expect(s.selectedLineId).toBeUndefined();
		expect(s.selectedJourneyId).toBeUndefined();
	});

	it('switches mode without clearing route or selection state', () => {
		const s = new MapStore();
		s.setFrom('a');
		s.setTo('b');
		s.selectStation('a');
		s.selectLine('l1');
		s.selectJourney('j1');

		s.setMode('integration');
		expect(s.mode).toBe('integration');
		expect(s.fromStationId).toBe('a');
		expect(s.toStationId).toBe('b');
		expect(s.selectedStationId).toBe('a');
		expect(s.selectedLineId).toBe('l1');
		expect(s.selectedJourneyId).toBe('j1');

		s.setMode('geographic');
		expect(s.mode).toBe('geographic');
		expect(s.selectedStationId).toBe('a');
	});

	it('clearRoute clears origin/destination/journey but keeps mode and station', () => {
		const s = new MapStore();
		s.setMode('integration');
		s.setFrom('a');
		s.setTo('b');
		s.selectStation('a');
		s.selectJourney('j1');

		s.clearRoute();
		expect(s.fromStationId).toBeUndefined();
		expect(s.toStationId).toBeUndefined();
		expect(s.selectedJourneyId).toBeUndefined();
		expect(s.mode).toBe('integration');
		expect(s.selectedStationId).toBe('a');
	});

	it('keeps geographic and integration cameras independent', () => {
		const s = new MapStore();
		s.setGeographicCamera({ latitude: -6.2, longitude: 106.8, zoom: 12 });
		s.setIntegrationCamera({ tx: 100, ty: 50, scale: 2 });

		expect(s.geographicCamera).toEqual({ latitude: -6.2, longitude: 106.8, zoom: 12 });
		expect(s.integrationCamera).toEqual({ tx: 100, ty: 50, scale: 2 });
	});
});
