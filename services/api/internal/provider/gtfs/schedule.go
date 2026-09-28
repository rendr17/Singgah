package gtfs

import (
	"archive/zip"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"

	generated "singgah/services/api/db/generated"
)

// TJProviderRegistration is the providers-row payload for the TransJakarta
// GTFS feed (docs/35_DATA_SOURCES.md) — official open data published by PT
// Transportasi Jakarta via PPID, served from gtfs.transjakarta.co.id. The
// license field stays honest: the feed ships no feed_info.txt and the PPID
// page states no formal license.
var TJProviderRegistration = generated.UpsertProviderParams{
	Code:            "tj-gtfs",
	Name:            "TransJakarta GTFS",
	SourceUrl:       pgtype.Text{String: "https://gtfs.transjakarta.co.id/files/file_gtfs.zip", Valid: true},
	TermsUrl:        pgtype.Text{String: "https://ppid.transjakarta.co.id/pusat-data/data-terbuka/transjakarta-gtfs-feed", Valid: true},
	LicenseName:     pgtype.Text{String: "official open data — no formal license stated", Valid: true},
	AttributionText: pgtype.Text{String: "Data rute dan jadwal oleh PT Transportasi Jakarta (GTFS feed)", Valid: true},
	AllowedUse:      pgtype.Text{String: "route path geometry for map display; scheduled services for journey planning", Valid: true},
	RefreshCadence:  pgtype.Text{String: "monthly", Valid: true},
	Owner:           pgtype.Text{String: "PT Transportasi Jakarta", Valid: true},
	KnownLimitations: pgtype.Text{
		String: "no fares here — Commute remains the fare source; trips are headway-based (frequencies exact_times=0), so TJ times are estimates not per-departure schedules; ~6 corridor codes in the catalog have no GTFS counterpart",
		Valid:  true,
	},
}

// ScheduleService is one calendar.txt row — which days the trips on it run.
// DayMask is bit 0 = Monday .. bit 6 = Sunday.
type ScheduleService struct {
	ID        string
	DayMask   int
	StartDate string // YYYYMMDD, feed-local
	EndDate   string
}

// ScheduleTrip is one trips.txt row resolved onto its route's short_name —
// the caller matches that name against canonical provider_entity_id.
type ScheduleTrip struct {
	ID             string
	RouteShortName string
	ServiceID      string
	Headsign       string
	DirectionID    int // -1 when the feed omits direction_id
}

// ScheduleStopTime is one stop_times.txt row; times are seconds since
// service-day midnight and may exceed 86400 (after-midnight service date).
type ScheduleStopTime struct {
	TripID           string
	StopID           string
	Seq              int
	ArrivalSeconds   int
	DepartureSeconds int
}

// ScheduleFrequency is one frequencies.txt row. ExactTimes false means the
// trip's stop_times are a headway template, not concrete departures.
type ScheduleFrequency struct {
	TripID         string
	StartSeconds   int
	EndSeconds     int
	HeadwaySeconds int
	ExactTimes     bool
}

// FeedStop is one stops.txt row — kept because trip stop_times reference
// child boarding points whose parent_station chains up to the catalog's
// station-level ids, or whose coordinates are the only link at all.
type FeedStop struct {
	ID            string
	ParentStation string
	Lat           float64
	Lon           float64
	LocationType  int // 0 = boarding point, 1 = station
}

// Schedule is the parsed timetable set. StopTimes groups rows per trip in
// stop_sequence order.
type Schedule struct {
	Services    []ScheduleService
	Trips       []ScheduleTrip
	StopTimes   map[string][]ScheduleStopTime
	Frequencies []ScheduleFrequency
	Stops       map[string]FeedStop
}

// ScheduleStats reports parse quality — malformed rows are counted loudly
// instead of failing the feed or silently dropping trips.
type ScheduleStats struct {
	Services        int
	Trips           int
	StopTimes       int
	Frequencies     int
	Stops           int
	SkippedRows     int
	OrphanStopTimes int // stop_times referencing no trip in trips.txt
	OrphanFreqs     int // frequencies referencing no trip in trips.txt
}

// calendar column order maps bit 0..6 to monday..sunday.
var calendarDays = []string{"monday", "tuesday", "wednesday", "thursday", "friday", "saturday", "sunday"}

// ParseSchedule reads calendar.txt, trips.txt, stop_times.txt and the
// optional frequencies.txt into a Schedule. Rows referencing trips the feed
// does not declare are counted as orphans and skipped — fabricating a trip
// for them would invent schedule data.
func ParseSchedule(zr *zip.Reader) (*Schedule, ScheduleStats, error) {
	var stats ScheduleStats

	services := []ScheduleService{}
	if err := eachCSV(zr, "calendar.txt", func(row map[string]string) error {
		mask := 0
		for i, col := range calendarDays {
			if row[col] == "1" {
				mask |= 1 << i
			}
		}
		if row["service_id"] == "" || len(row["start_date"]) != 8 || len(row["end_date"]) != 8 {
			stats.SkippedRows++
			return nil
		}
		services = append(services, ScheduleService{
			ID:        row["service_id"],
			DayMask:   mask,
			StartDate: row["start_date"],
			EndDate:   row["end_date"],
		})
		return nil
	}); err != nil {
		return nil, stats, err
	}
	stats.Services = len(services)

	routeNames, err := csvMap(zr, "routes.txt", "route_id", "route_short_name")
	if err != nil {
		return nil, stats, err
	}

	tripIDs := map[string]struct{}{}
	trips := []ScheduleTrip{}
	if err := eachCSV(zr, "trips.txt", func(row map[string]string) error {
		id := row["trip_id"]
		if id == "" || row["service_id"] == "" {
			stats.SkippedRows++
			return nil
		}
		dir := -1
		if v := row["direction_id"]; v != "" {
			d, err := strconv.Atoi(v)
			if err != nil {
				stats.SkippedRows++
				return nil
			}
			dir = d
		}
		tripIDs[id] = struct{}{}
		trips = append(trips, ScheduleTrip{
			ID:             id,
			RouteShortName: routeNames[row["route_id"]],
			ServiceID:      row["service_id"],
			Headsign:       row["trip_headsign"],
			DirectionID:    dir,
		})
		return nil
	}); err != nil {
		return nil, stats, err
	}
	stats.Trips = len(trips)

	stopTimes := map[string][]ScheduleStopTime{}
	if err := eachCSV(zr, "stop_times.txt", func(row map[string]string) error {
		tripID := row["trip_id"]
		arr, err1 := parseGtfsTime(row["arrival_time"])
		dep, err2 := parseGtfsTime(row["departure_time"])
		seq, err3 := strconv.Atoi(row["stop_sequence"])
		if tripID == "" || row["stop_id"] == "" || err1 != nil || err2 != nil || err3 != nil {
			stats.SkippedRows++
			return nil
		}
		if _, ok := tripIDs[tripID]; !ok {
			stats.OrphanStopTimes++
			return nil
		}
		stopTimes[tripID] = append(stopTimes[tripID], ScheduleStopTime{
			TripID:           tripID,
			StopID:           row["stop_id"],
			Seq:              seq,
			ArrivalSeconds:   arr,
			DepartureSeconds: dep,
		})
		return nil
	}); err != nil {
		return nil, stats, err
	}
	for _, sts := range stopTimes {
		sort.Slice(sts, func(i, j int) bool { return sts[i].Seq < sts[j].Seq })
		stats.StopTimes += len(sts)
	}

	frequencies := []ScheduleFrequency{}
	err = eachCSV(zr, "frequencies.txt", func(row map[string]string) error {
		tripID := row["trip_id"]
		start, err1 := parseGtfsTime(row["start_time"])
		end, err2 := parseGtfsTime(row["end_time"])
		headway, err3 := strconv.Atoi(row["headway_secs"])
		if tripID == "" || err1 != nil || err2 != nil || err3 != nil || headway <= 0 {
			stats.SkippedRows++
			return nil
		}
		if _, ok := tripIDs[tripID]; !ok {
			stats.OrphanFreqs++
			return nil
		}
		frequencies = append(frequencies, ScheduleFrequency{
			TripID:         tripID,
			StartSeconds:   start,
			EndSeconds:     end,
			HeadwaySeconds: headway,
			ExactTimes:     row["exact_times"] == "1",
		})
		return nil
	})
	if err != nil {
		// frequencies.txt is optional in GTFS — a missing file means every
		// trip is concrete, not a parse failure.
		if !strings.Contains(err.Error(), "missing from feed") {
			return nil, stats, err
		}
		err = nil
	}
	stats.Frequencies = len(frequencies)

	stops := map[string]FeedStop{}
	if err := eachCSV(zr, "stops.txt", func(row map[string]string) error {
		lat, err1 := strconv.ParseFloat(row["stop_lat"], 64)
		lon, err2 := strconv.ParseFloat(row["stop_lon"], 64)
		lt, err3 := strconv.Atoi(row["location_type"])
		if row["stop_id"] == "" || err1 != nil || err2 != nil ||
			lat < -90 || lat > 90 || lon < -180 || lon > 180 {
			stats.SkippedRows++
			return nil
		}
		if err3 != nil {
			lt = 0 // GTFS default when the column is empty
		}
		stops[row["stop_id"]] = FeedStop{
			ID:            row["stop_id"],
			ParentStation: row["parent_station"],
			Lat:           lat,
			Lon:           lon,
			LocationType:  lt,
		}
		return nil
	}); err != nil {
		return nil, stats, err
	}
	stats.Stops = len(stops)

	return &Schedule{
		Services:    services,
		Trips:       trips,
		StopTimes:   stopTimes,
		Frequencies: frequencies,
		Stops:       stops,
	}, stats, nil
}

// parseGtfsTime reads "H+:MM:SS" — GTFS times may exceed 24:00 for
// after-midnight service on the same service date.
func parseGtfsTime(s string) (int, error) {
	parts := strings.Split(s, ":")
	if len(parts) != 3 {
		return 0, fmt.Errorf("bad gtfs time %q", s)
	}
	h, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, err
	}
	m, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, err
	}
	sec, err := strconv.Atoi(parts[2])
	if err != nil {
		return 0, err
	}
	if m > 59 || sec > 59 || h < 0 || m < 0 || sec < 0 {
		return 0, fmt.Errorf("bad gtfs time %q", s)
	}
	return h*3600 + m*60 + sec, nil
}
