package ingest

import (
	"archive/zip"
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	generated "singgah/services/api/db/generated"
	"singgah/services/api/internal/provider/gtfs"
	"singgah/services/api/internal/provider/osm"
)

// ShapesReport summarizes one GTFS shape ingest — matched routes plus every
// feed route that has no canonical counterpart (new/renamed upstream lines).
type ShapesReport struct {
	Parsed    int
	Matched   int
	Unmatched []string // feed route_short_name with no canonical route
	Parse     gtfs.Stats
	FetchedAt time.Time
}

// Shapes imports path geometry from a GTFS feed into route_shapes. Shapes are
// matched onto canonical routes via routePrefix+route_short_name against
// provider_entity_id (e.g. "TJ:" + "6A") — GTFS route ids live in their own
// namespace and never become canonical ids.
func Shapes(ctx context.Context, pool *pgxpool.Pool, zr *zip.Reader,
	provider generated.UpsertProviderParams, matchProviderCode, routePrefix string) (ShapesReport, error) {
	fetchedAt := pgtype.Timestamptz{Time: time.Now(), Valid: true}
	report := ShapesReport{FetchedAt: fetchedAt.Time}

	shapes, stats, err := gtfs.ParseFeed(zr)
	if err != nil {
		return report, fmt.Errorf("ingest shapes: parse feed: %w", err)
	}
	report.Parsed = stats.RouteShapes
	report.Parse = stats

	q0 := generated.New(pool)
	if _, err := q0.UpsertProvider(ctx, provider); err != nil {
		return report, fmt.Errorf("ingest shapes: provider registration: %w", err)
	}
	if err := q0.TouchProviderLastAttempt(ctx, provider.Code); err != nil {
		return report, fmt.Errorf("ingest shapes: last_attempt_at: %w", err)
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return report, fmt.Errorf("ingest shapes: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := q0.WithTx(tx)

	for _, s := range shapes {
		routeID, err := q.GetRouteByProviderEntityID(ctx, generated.GetRouteByProviderEntityIDParams{
			Code:             matchProviderCode,
			ProviderEntityID: routePrefix + s.RouteShortName,
		})
		if err != nil {
			report.Unmatched = append(report.Unmatched, s.RouteShortName)
			continue
		}
		if _, err := q.UpsertRouteShape(ctx, generated.UpsertRouteShapeParams{
			RouteID: routeID,
			DirectionID: pgtype.Int2{
				Int16: int16(s.DirectionID),
				Valid: s.DirectionID >= 0,
			},
			StGeomfromtext: s.WKT(),
			Source:         provider.Code,
			SourceShapeID:  s.ShapeID,
			FetchedAt:      fetchedAt,
		}); err != nil {
			return report, fmt.Errorf("ingest shapes: shape %s: %w", s.ShapeID, err)
		}
		report.Matched++
	}

	if err := q.TouchProviderLastSuccess(ctx, provider.Code); err != nil {
		return report, fmt.Errorf("ingest shapes: last_success_at: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return report, fmt.Errorf("ingest shapes: commit: %w", err)
	}
	return report, nil
}

// OSMReport summarizes one OpenStreetMap rail-shape ingest.
type OSMReport struct {
	Relations  int
	Components int // stitched polylines written to route_shapes
	Matched    int
	Unmatched  []string // entity ids with no canonical route
	FetchedAt  time.Time
}

// OSMShapes imports rail path geometry from OSM route relations. Geometries
// arrive pre-fetched (Overpass or the main API — the caller picks), stitched
// into components, and stored per component — a relation whose ways don't
// connect keeps its gaps as separate rows rather than fabricating a bridge.
func OSMShapes(ctx context.Context, pool *pgxpool.Pool, geoms []osm.RelationGeom,
	mapping map[string][]int64, matchProviderCode string) (OSMReport, error) {
	fetchedAt := pgtype.Timestamptz{Time: time.Now(), Valid: true}
	report := OSMReport{FetchedAt: fetchedAt.Time}

	entityIDs := make([]string, 0, len(mapping))
	for entityID := range mapping {
		entityIDs = append(entityIDs, entityID)
	}
	sort.Strings(entityIDs)

	byID := make(map[int64]osm.RelationGeom, len(geoms))
	for _, g := range geoms {
		byID[g.ID] = g
	}

	q0 := generated.New(pool)
	if _, err := q0.UpsertProvider(ctx, osm.ProviderRegistration); err != nil {
		return report, fmt.Errorf("ingest osm: provider registration: %w", err)
	}
	if err := q0.TouchProviderLastAttempt(ctx, osm.ProviderRegistration.Code); err != nil {
		return report, fmt.Errorf("ingest osm: last_attempt_at: %w", err)
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return report, fmt.Errorf("ingest osm: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := q0.WithTx(tx)

	for _, entityID := range entityIDs {
		routeID, err := q.GetRouteByProviderEntityID(ctx, generated.GetRouteByProviderEntityIDParams{
			Code:             matchProviderCode,
			ProviderEntityID: entityID,
		})
		if err != nil {
			report.Unmatched = append(report.Unmatched, entityID)
			continue
		}
		for _, relID := range mapping[entityID] {
			g, ok := byID[relID]
			if !ok {
				report.Unmatched = append(report.Unmatched, fmt.Sprintf("%s:rel%d missing upstream", entityID, relID))
				continue
			}
			report.Relations++
			for i, comp := range g.Components {
				if _, err := q.UpsertRouteShape(ctx, generated.UpsertRouteShapeParams{
					RouteID:        routeID,
					StGeomfromtext: osm.WKT(comp),
					Source:         osm.ProviderRegistration.Code,
					SourceShapeID:  fmt.Sprintf("rel%d:%d", relID, i),
					FetchedAt:      fetchedAt,
				}); err != nil {
					return report, fmt.Errorf("ingest osm: rel %d comp %d: %w", relID, i, err)
				}
				report.Components++
			}
		}
		report.Matched++
	}

	if err := q.TouchProviderLastSuccess(ctx, osm.ProviderRegistration.Code); err != nil {
		return report, fmt.Errorf("ingest osm: last_success_at: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return report, fmt.Errorf("ingest osm: commit: %w", err)
	}
	return report, nil
}
