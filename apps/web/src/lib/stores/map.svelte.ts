// Shared state for the dual map — one selection/journey surfaced through the
// geographic and integration views (docs/INTEGRATION_MAP_BLUEPRINT.md §8).
// Switching map mode must never clear route or selection state.

export type MapMode = 'geographic' | 'integration';

export interface GeographicCamera {
	latitude: number;
	longitude: number;
	zoom: number;
	bearing?: number;
	pitch?: number;
}

export interface IntegrationCamera {
	// Screen-space transform: screen = world * scale + t (blueprint §17/§21).
	tx: number;
	ty: number;
	/** scale <= 0 means "not yet fitted" — the renderer fits on first mount. */
	scale: number;
}

export class MapStore {
	mode = $state<MapMode>('geographic');

	fromStationId = $state<string | undefined>();
	toStationId = $state<string | undefined>();

	selectedStationId = $state<string | undefined>();
	selectedLineId = $state<string | undefined>();
	selectedJourneyId = $state<string | undefined>();

	// Jakarta center — matches the TransitMap default so the store and the
	// first geographic render agree before any camera event is wired up.
	geographicCamera = $state<GeographicCamera>({
		latitude: -6.1945,
		longitude: 106.8229,
		zoom: 10.5
	});
	integrationCamera = $state<IntegrationCamera>({ tx: 0, ty: 0, scale: 0 });

	setMode(mode: MapMode) {
		this.mode = mode;
	}
	setFrom(stationId?: string) {
		this.fromStationId = stationId;
	}
	setTo(stationId?: string) {
		this.toStationId = stationId;
	}
	selectStation(stationId?: string) {
		this.selectedStationId = stationId;
	}
	selectLine(lineId?: string) {
		this.selectedLineId = lineId;
	}
	selectJourney(journeyId?: string) {
		this.selectedJourneyId = journeyId;
	}
	setGeographicCamera(camera: GeographicCamera) {
		this.geographicCamera = camera;
	}
	setIntegrationCamera(camera: IntegrationCamera) {
		this.integrationCamera = camera;
	}

	clearRoute() {
		this.fromStationId = undefined;
		this.toStationId = undefined;
		this.selectedJourneyId = undefined;
	}
}

export const mapStore = new MapStore();
