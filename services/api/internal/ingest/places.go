package ingest

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	generated "singgah/services/api/db/generated"
	"singgah/services/api/internal/provider/osm"
)

// Transit-access linkage parameters (ADR-011): a place is reachable when a
// live stop sits within accessRadiusM straight-line; walk_seconds is the
// distance × detourFactor at walkSpeedMPerMin — an ESTIMATE, geometry stays
// NULL until real foot routing exists.
const (
	accessRadiusM    = 1500.0
	detourFactor     = 1.3
	walkSpeedMPerMin = 80.0
)

type PlacesReport struct {
	Fetched    int
	Upserted   int
	AccessRows int
}

// placeBatchSize bounds transaction size — a full Jabodetabek import is tens
// of thousands of rows; one tx per batch keeps lock time and rollback cost
// bounded, and a crashed run resumes cleanly because upserts are idempotent.
const placeBatchSize = 1000

// Places upserts OSM POIs and recomputes their transit access against live
// stops in batches of placeBatchSize, one transaction each. Re-runs are
// safe: the place upsert is keyed on (provider, entity id) and never touches
// editorial_status, while access rows are replaced wholesale per place.
func Places(ctx context.Context, pool *pgxpool.Pool, pois []osm.POI) (PlacesReport, error) {
	report := PlacesReport{Fetched: len(pois)}

	q0 := generated.New(pool)
	provider, err := q0.UpsertProvider(ctx, osm.ProviderRegistration)
	if err != nil {
		return report, fmt.Errorf("ingest places: provider registration: %w", err)
	}

	for start := 0; start < len(pois); start += placeBatchSize {
		end := start + placeBatchSize
		if end > len(pois) {
			end = len(pois)
		}
		if err := ingestPlaceBatch(ctx, pool, q0, provider.ID, pois[start:end], &report); err != nil {
			return report, err
		}
	}
	return report, nil
}

func ingestPlaceBatch(ctx context.Context, pool *pgxpool.Pool, q0 *generated.Queries, providerID pgtype.UUID, pois []osm.POI, report *PlacesReport) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("ingest places: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := q0.WithTx(tx)

	now := pgtype.Timestamptz{Time: time.Now(), Valid: true}
	for _, p := range pois {
		access := []byte("{}")
		if w := p.Tags["wheelchair"]; w == "yes" || w == "limited" {
			access = []byte(`{"wheelchair":"` + w + `"}`)
		}
		payload, err := json.Marshal(map[string]any{"tags": p.Tags})
		if err != nil {
			return fmt.Errorf("ingest places: marshal tags %q: %w", p.EntityID, err)
		}
		placeID, err := q.UpsertPlace(ctx, generated.UpsertPlaceParams{
			ProviderID:       providerID,
			ProviderEntityID: p.EntityID,
			Name:             p.Name,
			PrimaryCategory:  p.Category,
			Wgs84Point:       p.Lon,
			Wgs84Point_2:     p.Lat,
			Accessibility:    access,
			SourcePayload:    payload,
			SourceUpdatedAt:  pgtype.Timestamptz{Time: p.UpdatedAt, Valid: !p.UpdatedAt.IsZero()},
		})
		if err != nil {
			return fmt.Errorf("ingest places: upsert %q: %w", p.EntityID, err)
		}
		report.Upserted++

		stops, err := q.ListStopsWithinRadiusOfPoint(ctx, generated.ListStopsWithinRadiusOfPointParams{
			Wgs84Point: p.Lon, Wgs84Point_2: p.Lat, StDwithin: accessRadiusM,
		})
		if err != nil {
			return fmt.Errorf("ingest places: stops near %q: %w", p.EntityID, err)
		}
		if err := q.ReplacePlaceTransitAccess(ctx, placeID); err != nil {
			return fmt.Errorf("ingest places: replace access %q: %w", p.EntityID, err)
		}
		for _, s := range stops {
			walkSec := int32(math.Round(float64(s.DistanceM) * detourFactor / walkSpeedMPerMin * 60))
			if err := q.InsertPlaceTransitAccess(ctx, generated.InsertPlaceTransitAccessParams{
				PlaceID:       placeID,
				StopID:        s.ID,
				WalkDistanceM: s.DistanceM,
				WalkSeconds:   pgtype.Int4{Int32: walkSec, Valid: true},
				ComputedAt:    now,
			}); err != nil {
				return fmt.Errorf("ingest places: access %q→%s: %w", p.EntityID, s.Name, err)
			}
			report.AccessRows++
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("ingest places: commit: %w", err)
	}
	return nil
}
