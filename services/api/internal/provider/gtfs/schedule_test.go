package gtfs

import (
	"testing"
)

const schedCalendarCSV = `service_id,monday,tuesday,wednesday,thursday,friday,saturday,sunday,start_date,end_date
SH,1,1,1,1,1,1,1,20040115,20271231
HK,1,1,1,1,1,0,0,20040115,20271231
`

const schedRoutesCSV = `route_id,route_short_name,route_type
R1,6A,3
`

const schedTripsCSV = `trip_id,route_id,service_id,trip_headsign,direction_id
t1,R1,SH,Blok M,0
t2,R1,HK,Kota,
`

const schedStopsCSV = `stop_id,stop_name,stop_lat,stop_lon,location_type,parent_station
H00001P,Halte A,-6.200,106.820,1,
H00002P,Halte B,-6.210,106.830,1,
G00001,Halte A Anak,-6.2001,106.8201,0,H00001P
GBAD,,0,0,0,
`

const schedStopTimesCSV = `trip_id,stop_sequence,stop_id,arrival_time,departure_time
t1,1,H00001P,25:10:00,25:10:00
t1,0,G00001,25:00:00,25:00:00
t2,0,H00002P,08:00:00,08:00:10
tGONE,0,H00001P,09:00:00,09:00:00
`

const schedFreqCSV = `trip_id,start_time,end_time,headway_secs,exact_times
t1,05:00:00,22:00:00,600,0
tGONE,05:00:00,06:00:00,300,1
`

func TestParseScheduleBuildsCanonicalSet(t *testing.T) {
	zr := feedZip(t, map[string]string{
		"calendar.txt":    schedCalendarCSV,
		"routes.txt":      schedRoutesCSV,
		"trips.txt":       schedTripsCSV,
		"stops.txt":       schedStopsCSV,
		"stop_times.txt":  schedStopTimesCSV,
		"frequencies.txt": schedFreqCSV,
	})
	feed, stats, err := ParseSchedule(zr)
	if err != nil {
		t.Fatal(err)
	}
	if stats.Services != 2 || stats.Trips != 2 || stats.Frequencies != 1 {
		t.Fatalf("stats: %+v", stats)
	}
	if stats.OrphanStopTimes != 1 || stats.OrphanFreqs != 1 {
		t.Fatalf("orphans: %+v", stats)
	}
	// One malformed stop row (lat/lon 0,0 is in-range — GBAD parses; the
	// coordinate check only rejects out-of-range). Just count valid rows.
	if stats.Stops != 4 {
		t.Fatalf("Stops = %d, want 4", stats.Stops)
	}

	var sh *ScheduleService
	for i := range feed.Services {
		if feed.Services[i].ID == "SH" {
			sh = &feed.Services[i]
		}
	}
	if sh == nil || sh.DayMask != 127 {
		t.Fatalf("SH day_mask = %+v, want 127", sh)
	}
	var hk *ScheduleService
	for i := range feed.Services {
		if feed.Services[i].ID == "HK" {
			hk = &feed.Services[i]
		}
	}
	if hk == nil || hk.DayMask != 31 {
		t.Fatalf("HK day_mask = %+v, want 31 (Mon-Fri)", hk)
	}

	sts := feed.StopTimes["t1"]
	if len(sts) != 2 {
		t.Fatalf("t1 stop_times = %d, want 2", len(sts))
	}
	// Ordered by stop_sequence, not file order; 25:00 → 90000 s proves the
	// >24 h after-midnight convention survives.
	if sts[0].Seq != 0 || sts[0].DepartureSeconds != 90000 {
		t.Fatalf("t1[0] = %+v", sts[0])
	}
	if sts[1].Seq != 1 || sts[1].StopID != "H00001P" {
		t.Fatalf("t1[1] = %+v", sts[1])
	}

	t2 := feed.Trips[1]
	if t2.RouteShortName != "6A" || t2.ServiceID != "HK" || t2.DirectionID != -1 {
		t.Fatalf("t2 = %+v, want 6A/HK/no-direction", t2)
	}

	f := feed.Frequencies[0]
	if f.HeadwaySeconds != 600 || f.ExactTimes {
		t.Fatalf("frequency = %+v", f)
	}

	if feed.Stops["G00001"].ParentStation != "H00001P" ||
		feed.Stops["H00002P"].LocationType != 1 {
		t.Fatalf("stops map: %+v", feed.Stops)
	}
}

func TestParseScheduleMissingFrequenciesTolerated(t *testing.T) {
	zr := feedZip(t, map[string]string{
		"calendar.txt":   schedCalendarCSV,
		"routes.txt":     schedRoutesCSV,
		"trips.txt":      schedTripsCSV,
		"stops.txt":      schedStopsCSV,
		"stop_times.txt": schedStopTimesCSV,
	})
	feed, _, err := ParseSchedule(zr)
	if err != nil {
		t.Fatal(err)
	}
	if len(feed.Frequencies) != 0 {
		t.Fatalf("frequencies = %d, want 0 (file absent)", len(feed.Frequencies))
	}
}

func TestParseScheduleMissingRequiredFile(t *testing.T) {
	zr := feedZip(t, map[string]string{"routes.txt": schedRoutesCSV})
	if _, _, err := ParseSchedule(zr); err == nil {
		t.Fatal("missing calendar.txt must error")
	}
}

func TestParseGtfsTime(t *testing.T) {
	cases := map[string]int{
		"00:00:00": 0,
		"05:09:20": 18560,
		"25:10:00": 90600,
	}
	for in, want := range cases {
		got, err := parseGtfsTime(in)
		if err != nil || got != want {
			t.Fatalf("parseGtfsTime(%q) = %d, %v — want %d", in, got, err, want)
		}
	}
	for _, bad := range []string{"5:9", "aa:00:00", "12:60:00", "12:00:60", "-1:00:00"} {
		if _, err := parseGtfsTime(bad); err == nil {
			t.Fatalf("parseGtfsTime(%q) must fail", bad)
		}
	}
}
