package commute

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

// Contract fixtures: real provider payload shapes live in testdata/ so
// upstream schema drift fails here before it reaches the network or the DB.

func loadFixture[T any](t *testing.T, name string) T {
	t.Helper()
	raw, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	var env envelope[T]
	if err := json.Unmarshal(raw, &env); err != nil {
		t.Fatalf("fixture %s no longer matches the envelope shape: %v", name, err)
	}
	if env.Status != 200 {
		t.Fatalf("fixture %s status=%d", name, env.Status)
	}
	return env.Data
}

var fetchedAt = pgtype.Timestamptz{Time: time.Unix(1_800_000_000, 0), Valid: true}

func TestNormalizeMode(t *testing.T) {
	cases := map[string]string{
		"RAIL": "rail", "SUBWAY": "subway", "TRAM": "tram", "BUS": "bus",
		"FERRY": "ferry", "MONORAIL": "other", "": "other", "gondola": "other",
	}
	for in, want := range cases {
		if got := NormalizeMode(in); got != want {
			t.Errorf("NormalizeMode(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestNormalizeStationFixture(t *testing.T) {
	stations := loadFixture[[]Station](t, "stations.json")
	if len(stations) == 0 {
		t.Fatal("empty fixture")
	}

	params, rej := NormalizeStation(stations[0], fetchedAt)
	if rej != nil {
		t.Fatalf("valid station rejected: %v", rej.Reason)
	}
	if params.ProviderEntityID != "KCI-SUD" || params.Kind != "station" {
		t.Errorf("identity: %+v", params)
	}
	if params.Wgs84Point != 106.8237 || params.Wgs84Point_2 != -6.2024 {
		t.Errorf("coords swapped: lon=%v lat=%v", params.Wgs84Point, params.Wgs84Point_2)
	}
	var meta map[string]any
	if err := json.Unmarshal(params.Metadata, &meta); err != nil {
		t.Fatalf("metadata not json: %v", err)
	}
	if meta["official_name"] != "SUDIRMAN" || meta["region_code"] != "CGK" {
		t.Errorf("metadata missing aliases: %v", meta)
	}
}

func TestNormalizeStationRejectsMissingCoords(t *testing.T) {
	st := Station{ID: "KCI-XX", Name: "Unsurveyed", Operator: "KCI"}
	if _, rej := NormalizeStation(st, fetchedAt); rej == nil {
		t.Error("expected rejection for null coordinates")
	}

	lat, lon := -91.0, 106.8
	st.Latitude, st.Longitude = &lat, &lon
	if _, rej := NormalizeStation(st, fetchedAt); rej == nil {
		t.Error("expected rejection for out-of-range latitude")
	}
}

// Identity fields are mandatory upstream: a station without id or name can
// neither be deduplicated nor displayed — reject, never synthesize.
func TestNormalizeStationRejectsMissingIdentity(t *testing.T) {
	lat, lon := -6.2, 106.8
	for _, st := range []Station{
		{ID: "", Name: "No ID", Latitude: &lat, Longitude: &lon},
		{ID: "KCI-X", Name: "", Latitude: &lat, Longitude: &lon},
	} {
		if _, rej := NormalizeStation(st, fetchedAt); rej == nil {
			t.Errorf("expected rejection for %+v", st)
		}
	}
}

func TestNormalizeOperatorAndLine(t *testing.T) {
	ops := loadFixture[[]Operator](t, "operators.json")
	kci := ops[0]

	params := NormalizeOperator(kci, fetchedAt)
	if params.ProviderEntityID != "KCI" || params.Timezone != "Asia/Jakarta" {
		t.Errorf("agency params: %+v", params)
	}

	var agencyID pgtype.UUID
	line := NormalizeLine(kci, kci.Lines[0], agencyID, fetchedAt)
	if line.ProviderEntityID != "KCI:C" || line.Mode != "rail" || line.Color.String != "0083D0" {
		t.Errorf("route params: %+v", line)
	}

	// Line with empty mode falls back to the operator's mode.
	line2 := NormalizeLine(kci, kci.Lines[1], agencyID, fetchedAt)
	if line2.Mode != "rail" {
		t.Errorf("operator-mode fallback: got %q", line2.Mode)
	}
}

func TestNormalizeTransfer(t *testing.T) {
	transfers := loadFixture[[]Transfer](t, "transfers-KCI-SUD.json")

	var mrtID, lrtID pgtype.UUID
	mrtID.Valid, lrtID.Valid = true, true
	stopIDs := map[string]pgtype.UUID{"MRTJ-DKA": mrtID, "LRTJBDB-DKA": lrtID}

	params, rej := NormalizeTransfer(transfers[0], stopIDs)
	if rej != nil {
		t.Fatalf("INTERNAL transfer rejected: %v", rej.Reason)
	}
	if params.ToStopID != mrtID || params.WalkDistanceM.Int32 != 90 {
		t.Errorf("transfer params: %+v", params)
	}
	var acc map[string]string
	_ = json.Unmarshal(params.Accessibility, &acc)
	if acc["notes"] == "" {
		t.Error("walking notes not preserved")
	}

	// EXTERNAL has no resolvable station id — must skip, not invent one.
	if _, rej := NormalizeTransfer(transfers[2], stopIDs); rej == nil {
		t.Error("expected EXTERNAL transfer to be skipped with a rejection")
	}

	// INTERNAL target absent from the catalog — loud rejection, not FK crash.
	orphan := transfers[0]
	var raw map[string]any
	_ = json.Unmarshal(orphan.ToStation, &raw)
	raw["id"] = "KCI-GONE"
	orphan.ToStation, _ = json.Marshal(raw)
	if _, rej := NormalizeTransfer(orphan, stopIDs); rej == nil {
		t.Error("expected rejection for unknown transfer target")
	}
}
