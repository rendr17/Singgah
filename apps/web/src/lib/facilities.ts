// Human labels for the provider-declared amenity types (contract: lowercase
// snake). Unknown future types fall back to the raw type string.
export const FACILITY_LABELS: Record<string, string> = {
	toilet: 'Toilet',
	toilet_accessible: 'Toilet aksesibel',
	praying_room: 'Musala',
	nursing_room: 'Ruang menyusui',
	elevator_paid: 'Lift (area berbayar)',
	elevator_unpaid: 'Lift (area gratis)',
	escalator_paid: 'Eskalator (area berbayar)',
	escalator_unpaid: 'Eskalator (area gratis)',
	parking: 'Parkir',
	bike_parking: 'Parkir sepeda',
	lockers: 'Loker',
	charging_station: 'Stasiun pengisian daya'
};

export function facilityLabel(type: string): string {
	return FACILITY_LABELS[type] ?? type;
}
