package gtfsrt

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/MobilityData/gtfs-realtime-bindings/golang/gtfs"
)

func translated(lang, text string) *gtfs.TranslatedString {
	return &gtfs.TranslatedString{
		Translation: []*gtfs.TranslatedString_Translation{
			{Language: str(lang), Text: str(text)},
		},
	}
}

func TestFetchAlertsNormalizes(t *testing.T) {
	start := uint64(1760000000)
	end := uint64(1760007200)
	feed := &gtfs.FeedMessage{
		Header: &gtfs.FeedHeader{GtfsRealtimeVersion: str("2.0")},
		Entity: []*gtfs.FeedEntity{
			{
				Id: str("a1"),
				Alert: &gtfs.Alert{
					Cause:  gtfs.Alert_CONSTRUCTION.Enum(),
					Effect: gtfs.Alert_DETOUR.Enum(),
					ActivePeriod: []*gtfs.TimeRange{
						{Start: u32(start), End: u32(end)},
					},
					InformedEntity: []*gtfs.EntitySelector{
						{RouteId: str("1")}, {RouteId: str("1")}, // dupes collapse
						{RouteId: str("9")}, // unresolvable → dropped, stays scoped
					},
					HeaderText:      translated("id", "Rekayasa lalin Sudirman"),
					DescriptionText: translated("en", "Detour in effect"),
					Url:             translated("id", "https://example.test/a1"),
				},
			},
			{
				Id: str("a2"),
				Alert: &gtfs.Alert{
					Cause:           gtfs.Alert_MAINTENANCE.Enum(),
					Effect:          gtfs.Alert_MODIFIED_SERVICE.Enum(),
					InformedEntity:  nil, // network-wide
					HeaderText:      translated("en", "Maintenance window"),
					DescriptionText: translated("id", "Perawatan sistem"),
				},
			},
			{Id: str("v1"), Vehicle: &gtfs.VehiclePosition{Vehicle: &gtfs.VehicleDescriptor{Id: str("bus")}}},
		},
	}
	srv := serveFeed(t, feed)

	resolve := func(_ context.Context, rid string) (string, bool) {
		if rid == "1" {
			return "canon-route-1", true
		}
		return "", false
	}
	as, err := NewClient(srv.URL, "tj-rt", resolve).FetchAlerts(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(as) != 2 {
		t.Fatalf("want 2 alerts, got %+v", as)
	}
	a := as[0]
	if a.ID != "a1" || a.Source != "tj-rt" || a.Cause != "construction" || a.Effect != "detour" {
		t.Fatalf("identity: %+v", a)
	}
	if a.Header != "Rekayasa lalin Sudirman" || a.Description != "Detour in effect" || a.URL == "" {
		t.Fatalf("translation pick: %+v", a)
	}
	if !a.Scoped || len(a.RouteIDs) != 1 || a.RouteIDs[0] != "canon-route-1" {
		t.Fatalf("route resolution: %+v", a)
	}
	if len(a.Periods) != 1 || a.Periods[0].Start == nil || !a.ActiveAt(time.Unix(int64(start)+60, 0)) {
		t.Fatalf("period: %+v", a.Periods)
	}
	if a.ActiveAt(time.Unix(int64(end)+60, 0)) || a.ActiveAt(time.Unix(int64(start)-60, 0)) {
		t.Fatal("outside the active window the alert must be inactive")
	}
	if as[1].Scoped {
		t.Fatal("alert without informed entities is network-wide, not scoped")
	}
}

func TestFetchAlertsHTTPErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()
	c := NewClient(srv.URL, "test", nil)
	if _, err := c.FetchAlerts(context.Background()); err == nil {
		t.Fatal("non-200 must fail")
	}
}
